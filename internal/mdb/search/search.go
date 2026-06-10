// Package search implements interactive and automatic search logic for media databases.
package search

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/mdb"
	"codeberg.org/upPollo/parsec/internal/mdb/tmdb"
	"codeberg.org/upPollo/parsec/internal/mdb/tvdb"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/filename"
	"codeberg.org/upPollo/parsec/internal/ui"
)

// InteractiveSearch performs a search by ID or title, prompting the user if multiple matches are found.
func InteractiveSearch(meta *metadata.Metadata, unattended bool) (*mdb.SearchResult, error) {
	if meta.Title == "" && meta.ImdbID == "" && meta.TmdbID == 0 && meta.TvdbID == 0 {
		return nil, fmt.Errorf("title is required (either from filename or --title flag) OR an ID (--imdb, --tmdb, --tvdb)")
	}

	if meta.ImdbID != "" || meta.TmdbID > 0 || meta.TvdbID > 0 {
		return interactiveSearchByID(meta)
	}

	return interactiveSearchByTitle(meta, unattended)
}

func interactiveSearchByID(meta *metadata.Metadata) (*mdb.SearchResult, error) {
	mediaType := "movie"
	if meta.IsTV {
		mediaType = "tv"
	}

	ui.Println(ui.Info.Render(fmt.Sprintf("Searching by ID: IMDB:%s TMDB:%d TVDB:%d [%s]...", meta.ImdbID, meta.TmdbID, meta.TvdbID, mediaType)))

	result, err := searchByID(meta.ImdbID, meta.TmdbID, meta.TvdbID, meta.IsTV)
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, fmt.Errorf("no results found")
	}

	ui.PrintDebug(fmt.Sprintf("InteractiveSearch ID result: %+v", result))

	return result, nil
}

func interactiveSearchByTitle(meta *metadata.Metadata, unattended bool) (*mdb.SearchResult, error) {
	mediaType := "movie"
	if meta.IsTV {
		mediaType = "tv"
	}

	// Replace dots with spaces for the search query
	searchQuery := filename.DeobfuscateTitle(meta.Title)
	ui.Println(ui.Info.Render(fmt.Sprintf("Searching for %s (%d) [%s]...", searchQuery, meta.Year, mediaType)))

	results, err := fuzzySearch(searchQuery, meta.Year, meta.IsTV)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no results found")
	}

	if len(results) == 1 || unattended {
		ui.PrintDebug(fmt.Sprintf("InteractiveSearch auto-selected: %+v", results[0]))
		return &results[0], nil
	}

	return promptForResultSelection(results)
}

func promptForResultSelection(results []mdb.SearchResult) (*mdb.SearchResult, error) {
	ui.Println("\n" + ui.Header.Render("AMBIGUOUS CORRELATIONS DETECTED:"))

	headers := []string{"#", "Title", "Year", "Match", "Language"}

	rows := make([][]string, 0, len(results))
	for i, r := range results {
		rows = append(rows, []string{
			strconv.Itoa(i),
			r.Title,
			strconv.Itoa(r.Year),
			fmt.Sprintf("%.0f%%", r.Similarity*100),
			mdb.FormatLanguage(r.OriginalLanguage),
		})
	}

	ui.Println(ui.TrackTable(headers, rows))

	fmt.Print(ui.Info.Render("\nSelect a result [default 0]: "))

	var input string

	_, _ = fmt.Scanln(&input)
	if input == "" {
		return &results[0], nil
	}

	choice, err := strconv.Atoi(input)
	if err != nil || choice < 0 || choice >= len(results) {
		return nil, fmt.Errorf("invalid selection")
	}

	result := &results[choice]
	ui.PrintDebug(fmt.Sprintf("InteractiveSearch selected: %+v", result))

	return result, nil
}

func searchMovie(query string, year int) ([]mdb.SearchResult, error) {
	return search("movie", query, year)
}

func searchTV(query string, year int) ([]mdb.SearchResult, error) {
	return search("tv", query, year)
}

func search(mediaType, query string, year int) ([]mdb.SearchResult, error) {
	ui.PrintDebug(fmt.Sprintf("Starting parallel MDB search: type=%s, query=%s, year=%d", mediaType, query, year))

	var (
		resultsTMDB      []mdb.SearchResult
		resultsTVDB      []mdb.SearchResult
		errTMDB, errTVDB error
		wg               sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()

		resultsTMDB, errTMDB = tmdb.Search(mediaType, query, year)
	}()
	go func() {
		defer wg.Done()

		resultsTVDB, errTVDB = tvdb.Search(mediaType, query, year)
	}()

	wg.Wait()

	ui.PrintDebug(fmt.Sprintf("MDB search results: TMDB=%d, TVDB=%d", len(resultsTMDB), len(resultsTVDB)))

	if errTMDB != nil {
		return nil, fmt.Errorf("TMDB search failed: %w", errTMDB)
	}

	if errTVDB != nil {
		return nil, fmt.Errorf("TVDB search failed: %w", errTVDB)
	}

	return mergeResults(resultsTMDB, resultsTVDB), nil
}

func mergeResults(resultsTMDB, resultsTVDB []mdb.SearchResult) []mdb.SearchResult {
	merged := make([]mdb.SearchResult, 0, len(resultsTMDB)+len(resultsTVDB))
	tvdbMap, tmdbMap, imdbMap := buildResultMaps(resultsTVDB)

	matchedTVDB := make(map[int]bool)

	for _, r := range resultsTMDB {
		if tvdbRes, ok := findMatchingResult(r, tvdbMap, tmdbMap, imdbMap); ok {
			mergeMatchedResult(&r, &tvdbRes)
			matchedTVDB[tvdbRes.TvdbID] = true
		}

		merged = append(merged, r)
	}

	// Add TVDB results that weren't matched with TMDB
	for _, r := range resultsTVDB {
		if !matchedTVDB[r.TvdbID] {
			merged = append(merged, r)
		}
	}

	return merged
}

func buildResultMaps(results []mdb.SearchResult) (tvdbMap, tmdbMap map[int]mdb.SearchResult, imdbMap map[string]mdb.SearchResult) {
	tvdbMap = make(map[int]mdb.SearchResult)
	tmdbMap = make(map[int]mdb.SearchResult)
	imdbMap = make(map[string]mdb.SearchResult)

	for _, r := range results {
		if r.TvdbID != 0 {
			tvdbMap[r.TvdbID] = r
		}

		if r.TmdbID != 0 {
			tmdbMap[r.TmdbID] = r
		}

		if r.ImdbID != "" {
			imdbMap[r.ImdbID] = r
		}
	}

	return
}

func findMatchingResult(r mdb.SearchResult, tvdbMap, tmdbMap map[int]mdb.SearchResult, imdbMap map[string]mdb.SearchResult) (mdb.SearchResult, bool) {
	if r.TvdbID != 0 {
		if res, ok := tvdbMap[r.TvdbID]; ok {
			return res, true
		}
	}

	if r.TmdbID != 0 {
		if res, ok := tmdbMap[r.TmdbID]; ok {
			return res, true
		}
	}

	if r.ImdbID != "" {
		if res, ok := imdbMap[r.ImdbID]; ok {
			return res, true
		}
	}

	return mdb.SearchResult{}, false
}

// nolint:cyclop
func mergeMatchedResult(res, tvdbRes *mdb.SearchResult) {
	// Merge TVDB data into TMDB result
	if res.TvdbID == 0 {
		res.TvdbID = tvdbRes.TvdbID
	}

	if res.TvdbType == "" {
		res.TvdbType = tvdbRes.TvdbType
	}

	if res.TvdbSlug == "" {
		res.TvdbSlug = tvdbRes.TvdbSlug
	}

	if res.ImdbID == "" {
		res.ImdbID = tvdbRes.ImdbID
	}

	if tvdbRes.OriginalLanguage != "" {
		res.OriginalLanguage = tvdbRes.OriginalLanguage
	}

	if res.Overview == "" {
		res.Overview = tvdbRes.Overview
	}

	if res.Year == 0 {
		res.Year = tvdbRes.Year
	}

	// Merge Titles
	if tvdbRes.Title != "" && tvdbRes.Title != res.Title {
		res.AltTitle = addUniqueAltTitle(res.AltTitle, tvdbRes.Title, res.Title, res.OriginalTitle)
	}

	for _, alt := range tvdbRes.AltTitle {
		res.AltTitle = addUniqueAltTitle(res.AltTitle, alt, res.Title, res.OriginalTitle)
	}
}

func addUniqueAltTitle(titles []string, newTitle string, existingTitles ...string) []string {
	if newTitle == "" {
		return titles
	}

	for _, et := range existingTitles {
		if strings.EqualFold(newTitle, et) {
			return titles
		}
	}

	for _, t := range titles {
		if strings.EqualFold(t, newTitle) {
			return titles
		}
	}

	return append(titles, newTitle)
}

func queryWithRetry(query string, year int, isTV bool) ([]mdb.SearchResult, error) {
	var (
		results []mdb.SearchResult
		err     error
	)

	if isTV {
		results, err = searchTV(query, year)
	} else {
		results, err = searchMovie(query, year)
	}

	if err != nil {
		return nil, err
	}

	// retry without year
	if len(results) == 0 && year > 0 {
		results, err = queryWithRetry(query, 0, isTV)
	}

	return results, err
}

// fuzzySearch combines search and filtering/sorting to find the best match
func fuzzySearch(query string, year int, isTV bool) ([]mdb.SearchResult, error) {
	var (
		results []mdb.SearchResult
		err     error
	)

	results, err = queryWithRetry(query, year, isTV)
	if err != nil {
		return nil, err
	}

	results = sortBySimilarity(results, query, year)

	if len(results) > 0 {
		results = filterResults(results)
	}

	// Return top 5 results
	if len(results) > 5 {
		results = results[:5]
	}

	return results, nil
}

func sortBySimilarity(results []mdb.SearchResult, query string, year int) []mdb.SearchResult {
	queryLower := strings.ToLower(query)

	for i := range results {
		titleSim := mdb.CalculateSimilarity(queryLower, strings.ToLower(results[i].Title))
		origSim := mdb.CalculateSimilarity(queryLower, strings.ToLower(results[i].OriginalTitle))
		results[i].Similarity = math.Max(titleSim, origSim)

		titleSim = calculateAltSimilarity(&results[i], queryLower, titleSim)

		// Bonus for year match
		if year > 0 && results[i].Year == year {
			results[i].Similarity += 0.05 // Slight boost for exact year match
		}

		ui.PrintDebug(fmt.Sprintf("Result: %s (%d) - Similarity: %.2f (Title: %.2f, Orig: %.2f)",
			results[i].Title, results[i].Year, results[i].Similarity, titleSim, origSim))
	}

	// Sort results by similarity, then popularity
	sort.Slice(results, func(i, j int) bool {
		if math.Abs(results[i].Similarity-results[j].Similarity) > 0.001 {
			return results[i].Similarity > results[j].Similarity
		}

		return results[i].Popularity > results[j].Popularity
	})

	return results
}

func calculateAltSimilarity(result *mdb.SearchResult, queryLower string, titleSim float64) float64 {
	for _, alt := range result.AltTitle {
		altSim := mdb.CalculateSimilarity(queryLower, strings.ToLower(alt))
		if altSim > result.Similarity {
			result.Similarity = altSim
			// If an alt title is a better match, we might want to swap it with the primary title
			// especially if it's a 100% match (e.g. translated title matched exactly)
			if altSim > titleSim {
				result.AltTitle = addUniqueAltTitle(result.AltTitle, result.Title)
				result.Title = alt
				titleSim = altSim
			}
		}
	}

	return titleSim
}

// Filter results that are more than 30% worse than the top scoring result
func filterResults(results []mdb.SearchResult) []mdb.SearchResult {
	topScore := results[0].Similarity
	threshold := topScore - 0.3

	filteredResults := make([]mdb.SearchResult, 0, len(results))
	for _, r := range results {
		if r.Similarity >= threshold {
			filteredResults = append(filteredResults, r)
		} else {
			ui.PrintDebug(fmt.Sprintf("Result '%s' (%d) filtered out: Similarity %.2f < dynamic threshold %.2f ",
				r.Title, r.Year, r.Similarity, threshold))
		}
	}

	return filteredResults
}

func searchByID(imdbID string, tmdbID, tvdbID int, isTV bool) (*mdb.SearchResult, error) {
	mediaType := "movie"
	if isTV {
		mediaType = "tv"
	}

	result, err := initialSearchByID(imdbID, tmdbID, tvdbID, isTV, mediaType)
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, fmt.Errorf("no results found")
	}

	// If we have a TMDB result but it's missing TVDB info, try to fetch it if we have a TVDB ID
	if result.TvdbID > 0 && (result.TvdbSlug == "" || len(result.AltTitle) == 0) {
		addMissingTvdbInfo(result, mediaType)
	}

	// Vice versa, if we have a TVDB result but it's missing TMDB info
	if result.TmdbID > 0 && (result.TmdbType == "" || len(result.AltTitle) == 0) {
		addMissingTmdbInfo(result, mediaType)
	}

	return result, nil
}

func initialSearchByID(imdbID string, tmdbID, tvdbID int, isTV bool, mediaType string) (*mdb.SearchResult, error) {
	if imdbID != "" {
		return searchByImdbID(imdbID, isTV, mediaType)
	}

	if tmdbID > 0 {
		return tmdb.GetByID(tmdbID, mediaType)
	}

	if tvdbID > 0 {
		return tvdb.GetByID(tvdbID, mediaType)
	}

	return nil, mdb.ErrNotFound
}

func searchByImdbID(imdbID string, isTV bool, mediaType string) (*mdb.SearchResult, error) {
	result, err := tmdb.GetByImdbID(imdbID, isTV)
	if err != nil {
		return nil, err
	}

	if result != nil {
		return result, nil
	}

	ui.PrintDebug(fmt.Sprintf("IMDB ID %s not found on TMDB as %s, trying TVDB...", imdbID, mediaType))

	return tvdb.GetByRemoteID(imdbID, mediaType)
}

func addMissingTvdbInfo(result *mdb.SearchResult, mediaType string) {
	tvdbResult, err := tvdb.GetByID(result.TvdbID, mediaType)
	if err == nil && tvdbResult != nil {
		if result.TvdbSlug == "" {
			result.TvdbSlug = tvdbResult.TvdbSlug
		}

		if result.TvdbType == "" {
			result.TvdbType = tvdbResult.TvdbType
		}

		if result.OriginalLanguage == "" {
			result.OriginalLanguage = tvdbResult.OriginalLanguage
		}

		if tvdbResult.Title != "" && tvdbResult.Title != result.Title {
			result.AltTitle = addUniqueAltTitle(result.AltTitle, tvdbResult.Title, result.Title, result.OriginalTitle)
		}

		for _, alt := range tvdbResult.AltTitle {
			result.AltTitle = addUniqueAltTitle(result.AltTitle, alt, result.Title, result.OriginalTitle)
		}
	}
}

func addMissingTmdbInfo(result *mdb.SearchResult, mediaType string) {
	tmdbResult, err := tmdb.GetByID(result.TmdbID, mediaType)
	if err == nil && tmdbResult != nil {
		if result.TmdbType == "" {
			result.TmdbType = tmdbResult.TmdbType
		}

		if result.ImdbID == "" {
			result.ImdbID = tmdbResult.ImdbID
		}

		if result.OriginalLanguage == "" {
			result.OriginalLanguage = tmdbResult.OriginalLanguage
		}

		if tmdbResult.Title != "" && tmdbResult.Title != result.Title {
			result.AltTitle = addUniqueAltTitle(result.AltTitle, tmdbResult.Title, result.Title, result.OriginalTitle)
		}

		for _, alt := range tmdbResult.AltTitle {
			result.AltTitle = addUniqueAltTitle(result.AltTitle, alt, result.Title, result.OriginalTitle)
		}
	}
}

// FindEpisode attempts to identify an episode on TVDB or TMDB based on search results and metadata.
func FindEpisode(result mdb.SearchResult, meta *metadata.Metadata, allowSpecials bool) mdb.EpisodeResult {
	if result.TvdbID > 0 {
		ui.PrintDebug(fmt.Sprintf("Searching for episode on TVDB: ID=%d, S%02dE%02d", result.TvdbID, meta.Season, meta.Episode))

		data, err := tvdb.IdentifyEpisode(result, meta, allowSpecials)
		if err == nil && data.Name != "" {
			return data
		}
	}

	// Fallback to TMDB if TVDB id is missing or fails
	preferred := config.GetPreferredLanguage()
	langs := []string{preferred, result.OriginalLanguage, "en"}
	uniqueLangs := metadata.RemoveDuplicates(langs)

	if result.TmdbID > 0 {
		ui.PrintDebug(fmt.Sprintf("Searching for episode on TMDB: ID=%d, S%02dE%02d", result.TmdbID, meta.Season, meta.Episode))

		for _, lang := range uniqueLangs {
			data, err := tmdb.GetEpisodeMetadata(result.TmdbID, meta.Season, meta.Episode, lang)
			if err == nil && data.Name != "" {
				return data
			}
		}
	}

	return mdb.EpisodeResult{}
}
