package search

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"

	"codeberg.org/n0ne/parsec/internal/config"
	"codeberg.org/n0ne/parsec/internal/mdb"
	"codeberg.org/n0ne/parsec/internal/mdb/tmdb"
	"codeberg.org/n0ne/parsec/internal/mdb/tvdb"
	"codeberg.org/n0ne/parsec/internal/metadata"
	"codeberg.org/n0ne/parsec/internal/metadata/filename"
	"codeberg.org/n0ne/parsec/internal/ui"
)

func InteractiveSearch(meta *metadata.Metadata, unattended bool) (*mdb.SearchResult, error) {
	if meta.Title == "" && meta.ImdbID == "" && meta.TmdbID == 0 && meta.TvdbID == 0 {
		return nil, fmt.Errorf("title is required (either from filename or --title flag) OR an ID (--imdb, --tmdb, --tvdb)")
	}

	mediaType := "movie"
	if meta.IsTV {
		mediaType = "tv"
	}

	if meta.ImdbID != "" || meta.TmdbID > 0 || meta.TvdbID > 0 {
		ui.Println(ui.Info.Render(fmt.Sprintf("Searching by ID: IMDB:%s TMDB:%d TVDB:%d [%s]...", meta.ImdbID, meta.TmdbID, meta.TvdbID, mediaType)))
		result, err := SearchByID(meta.ImdbID, meta.TmdbID, meta.TvdbID, meta.IsTV)
		if err != nil {
			return nil, err
		}
		if result == nil {
			return nil, fmt.Errorf("no results found")
		}
		return result, nil
	}

	// Replace dots with spaces for the search query
	searchQuery := filename.DeobfuscateTitle(meta.Title)
	ui.Println(ui.Info.Render(fmt.Sprintf("Searching for %s (%d) [%s]...", searchQuery, meta.Year, mediaType)))
	results, err := FuzzySearch(searchQuery, meta.Year, meta.IsTV)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no results found")
	}

	if len(results) == 1 || unattended {
		return &results[0], nil
	}

	ui.Println("\n" + ui.Header.Render("AMBIGUOUS CORRELATIONS DETECTED:"))
	headers := []string{"#", "Title", "Year", "Match", "Language"}
	var rows [][]string
	for i, r := range results {
		rows = append(rows, []string{
			strconv.Itoa(i),
			r.Title,
			strconv.Itoa(r.Year),
			fmt.Sprintf("%.0f%%", r.Similarity*100),
			r.OriginalLanguage,
		})
	}
	ui.Println(ui.TrackTable(headers, rows))

	fmt.Print(ui.Info.Render("\nSelect a result [default 0]: "))
	var input string
	fmt.Scanln(&input)
	if input == "" {
		return &results[0], nil
	}
	choice, err := strconv.Atoi(input)
	if err != nil || choice < 0 || choice >= len(results) {
		return nil, fmt.Errorf("invalid selection")
	}
	return &results[choice], nil
}

func SearchMovie(query string, year int) ([]mdb.SearchResult, error) {
	return search("movie", query, year)
}

func SearchTV(query string, year int) ([]mdb.SearchResult, error) {
	return search("tv", query, year)
}

func search(mediaType, query string, year int) ([]mdb.SearchResult, error) {
	var resultsTMDB []mdb.SearchResult
	var resultsTVDB []mdb.SearchResult
	var errTMDB, errTVDB error
	var wg sync.WaitGroup

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

	if errTMDB != nil {
		return nil, fmt.Errorf("TMDB search failed: %v", errTMDB)
	}
	if errTVDB != nil {
		return nil, fmt.Errorf("TVDB search failed: %v", errTVDB)
	}

	return MergeResults(resultsTMDB, resultsTVDB), nil
}

func MergeResults(resultsTMDB, resultsTVDB []mdb.SearchResult) []mdb.SearchResult {
	merged := make([]mdb.SearchResult, 0, len(resultsTMDB)+len(resultsTVDB))
	tvdbMap := make(map[int]mdb.SearchResult)
	tmdbMap := make(map[int]mdb.SearchResult)
	imdbMap := make(map[string]mdb.SearchResult)

	for _, r := range resultsTVDB {
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

	matchedTVDB := make(map[int]bool)

	for _, r := range resultsTMDB {
		var matched bool
		var tvdbRes mdb.SearchResult

		// Match by TVDB ID
		if r.TvdbID != 0 {
			if res, ok := tvdbMap[r.TvdbID]; ok {
				tvdbRes = res
				matched = true
			}
		}

		// Match by TMDB ID (if not already matched by TVDB ID)
		if !matched && r.TmdbID != 0 {
			if res, ok := tmdbMap[r.TmdbID]; ok {
				tvdbRes = res
				matched = true
			}
		}

		// Match by IMDB ID (if not already matched)
		if !matched && r.ImdbID != "" {
			if res, ok := imdbMap[r.ImdbID]; ok {
				tvdbRes = res
				matched = true
			}
		}

		if matched {
			// Merge TVDB data into TMDB result
			if r.TvdbID == 0 {
				r.TvdbID = tvdbRes.TvdbID
			}
			if r.TvdbType == "" {
				r.TvdbType = tvdbRes.TvdbType
			}
			if r.TvdbSlug == "" {
				r.TvdbSlug = tvdbRes.TvdbSlug
			}
			if r.ImdbID == "" {
				r.ImdbID = tvdbRes.ImdbID
			}
			if tvdbRes.OriginalLanguage != "" {
				r.OriginalLanguage = tvdbRes.OriginalLanguage
			}
			if r.Overview == "" {
				r.Overview = tvdbRes.Overview
			}
			if r.Year == 0 {
				r.Year = tvdbRes.Year
			}

			// Merge Titles
			if tvdbRes.Title != "" && tvdbRes.Title != r.Title {
				r.AltTitle = addUniqueAltTitle(r.AltTitle, tvdbRes.Title, r.Title, r.OriginalTitle)
			}
			for _, alt := range tvdbRes.AltTitle {
				r.AltTitle = addUniqueAltTitle(r.AltTitle, alt, r.Title, r.OriginalTitle)
			}

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

func addUniqueAltTitle(titles []string, newTitle string, existingTitles ...string) []string {
	if newTitle == "" {
		return titles
	}
	for _, et := range existingTitles {
		if newTitle == et {
			return titles
		}
	}
	for _, t := range titles {
		if t == newTitle {
			return titles
		}
	}
	return append(titles, newTitle)
}

// FuzzySearch combines search and filtering/sorting to find the best match
func FuzzySearch(query string, year int, isTV bool) ([]mdb.SearchResult, error) {
	var results []mdb.SearchResult
	var err error

	if isTV {
		results, err = SearchTV(query, year)
	} else {
		results, err = SearchMovie(query, year)
	}

	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, nil
	}

	queryLower := strings.ToLower(query)

	for i := range results {
		titleSim := mdb.CalculateSimilarity(queryLower, strings.ToLower(results[i].Title))
		origSim := mdb.CalculateSimilarity(queryLower, strings.ToLower(results[i].OriginalTitle))
		results[i].Similarity = math.Max(titleSim, origSim)

		// Bonus for year match
		if year > 0 && results[i].Year == year {
			results[i].Similarity += 0.05 // Slight boost for exact year match
		}
	}

	// Sort results by similarity, then popularity
	sort.Slice(results, func(i, j int) bool {
		if math.Abs(results[i].Similarity-results[j].Similarity) > 0.001 {
			return results[i].Similarity > results[j].Similarity
		}
		return results[i].Popularity > results[j].Popularity
	})

	// Return top 5 results
	if len(results) > 5 {
		results = results[:5]
	}

	return results, nil
}

func SearchByID(imdbID string, tmdbID int, tvdbID int, isTV bool) (*mdb.SearchResult, error) {
	var result *mdb.SearchResult
	var err error

	mediaType := "movie"
	if isTV {
		mediaType = "tv"
	}

	if imdbID != "" {
		result, err = tmdb.GetByImdbID(imdbID, isTV)
		if err != nil {
			return nil, err
		}
	} else if tmdbID > 0 {
		result, err = tmdb.GetByID(tmdbID, mediaType)
		if err != nil {
			return nil, err
		}
	} else if tvdbID > 0 {
		result, err = tvdb.GetByID(tvdbID, mediaType)
		if err != nil {
			return nil, err
		}
	}

	if result == nil {
		return nil, nil
	}

	// If we have a TMDB result but it's missing TVDB info, try to fetch it if we have a TVDB ID
	if result.TvdbID > 0 && (result.TvdbSlug == "" || len(result.AltTitle) == 0) {
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

	// Vice versa, if we have a TVDB result but it's missing TMDB info
	if result.TmdbID > 0 && (result.TmdbType == "" || len(result.AltTitle) == 0) {
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

	return result, nil
}

func FindEpisode(result mdb.SearchResult, season, episode int) mdb.EpisodeResult {
	preferred := config.GetPreferredLanguage()
	langs := []string{preferred, result.OriginalLanguage, "en"}

	// Remove duplicates and empty strings
	uniqueLangs := []string{}
	seen := make(map[string]bool)
	for _, l := range langs {
		if l != "" && !seen[l] {
			uniqueLangs = append(uniqueLangs, l)
			seen[l] = true
		}
	}

	if result.TvdbID > 0 {
		for _, lang := range uniqueLangs {
			data, err := tvdb.GetEpisodeMetadata(result.TvdbID, season, episode, lang)
			if err == nil && data.Name != "" {
				return data
			}
		}
	}

	// Fallback to TMDB if TVDB id is missing or fails
	if result.TmdbID > 0 {
		for _, lang := range uniqueLangs {
			data, err := tmdb.GetEpisodeMetadata(result.TmdbID, season, episode, lang)
			if err == nil && data.Name != "" {
				return data
			}
		}
	}
	return mdb.EpisodeResult{}
}

func IdentifyEpisode(result mdb.SearchResult, meta *metadata.Metadata) mdb.EpisodeResult {
	if result.TvdbID == 0 {
		return mdb.EpisodeResult{}
	}

	return tvdb.IdentifyEpisode(result.TvdbID, meta.EpisodeTitle, meta.Date, result.OriginalLanguage)
}
