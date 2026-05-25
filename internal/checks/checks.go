package checks

import (
	"fmt"
	"regexp"
	"strings"

	"codeberg.org/n0ne/parsec/internal/mdb"
	mdbSearch "codeberg.org/n0ne/parsec/internal/mdb/search"
	"codeberg.org/n0ne/parsec/internal/metadata"
	"codeberg.org/n0ne/parsec/internal/metadata/filename"
)

func RunGenericChecks(meta *metadata.Metadata) {
	if err := CheckYear(meta); err != nil {
		fmt.Println(err)
	}
	if err := CheckStreaming(meta); err != nil {
		fmt.Println(err)
	}
	if err := CheckTvSpecial(meta); err != nil {
		fmt.Println(err)
	}
}

func CheckYear(meta *metadata.Metadata) error {
	if meta.Year == 0 && !meta.IsTV {
		return fmt.Errorf("year is missing for this Movie")
	}

	if meta.Year > 0 && meta.Season > 1900 {
		return fmt.Errorf("The Season already is the year")
	}

	return nil
}

func CheckStreaming(meta *metadata.Metadata) error {
	isWeb := strings.Contains(meta.Source, "WEB")
	if isWeb && meta.Service == "" {
		return fmt.Errorf("Streaming Service Tag is missing for WEB source")
	}

	if !isWeb && meta.Service != "" {
		return fmt.Errorf("Streaming Service Tag is not supported for non-WEB source")
	}

	return nil
}

func CheckTvSpecial(meta *metadata.Metadata) error {
	if meta.IsTV && meta.Season == 0 {
		if meta.Date == "" {
			return fmt.Errorf("Date is missing for TV Special")
		}
		if meta.EpisodeTitle == "" {
			return fmt.Errorf("Episode Title is missing for TV Special")
		}
	}
	return nil
}

func NormalizeForComparison(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, ".", " ")
	s = strings.ReplaceAll(s, "-", " ")
	// remove all non-alphanumeric chars (except spaces)
	re := regexp.MustCompile(`[^a-z0-9 ]`)
	s = re.ReplaceAllString(s, "")
	// collapse multiple spaces
	reSpaces := regexp.MustCompile(`\s+`)
	s = reSpaces.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func RunMdbChecks(meta *metadata.Metadata, imdbID string, tmdbID, tvdbID int) {
	var searchResult *mdb.SearchResult
	var searchErr error

	if imdbID != "" || tmdbID > 0 || tvdbID > 0 {
		searchResult, searchErr = mdbSearch.SearchByID(imdbID, tmdbID, tvdbID, meta.IsTV)
	} else {
		searchQuery := filename.DeobfuscateTitle(meta.Title)
		searchResult, searchErr = mdbSearch.FuzzySearch(searchQuery, meta.Year, meta.IsTV)
	}

	if searchErr != nil {
		fmt.Printf("MDB Error: Could not fetch metadata from TMDB/TVDB: %v\n", searchErr)
		return
	}
	if searchResult == nil {
		fmt.Println("MDB Warning: No matching metadata found on TMDB/TVDB.")
		return
	}

	fmt.Printf("MDB Matched: %s (%d)\n", searchResult.Title, searchResult.Year)

	CheckMovieYear(meta, searchResult)
	if meta.IsTV {
		CheckSeriesYear(meta, searchResult)
		CheckEpisode(meta, searchResult)
	}
}

func CheckMovieYear(meta *metadata.Metadata, result *mdb.SearchResult) {
	if !meta.IsTV && meta.Year > 0 && result.Year > 0 && meta.Year != result.Year {
		fmt.Printf("MDB Warning: Year mismatch. Filename: %d, TMDB: %d\n", meta.Year, result.Year)
	}
}

func CheckSeriesYear(meta *metadata.Metadata, result *mdb.SearchResult) {
	if meta.Year > 0 && result.Year > 0 && meta.Year != result.Year && meta.Season < 1900 {
		fmt.Printf("MDB Warning: Series start year mismatch. Filename: %d, TMDB/TVDB: %d\n", meta.Year, result.Year)
	}
}

func CheckEpisode(meta *metadata.Metadata, result *mdb.SearchResult) {
	if meta.Season > 0 || meta.Episode > 0 {
		epResult := mdbSearch.FindEpisode(*result, meta.Season, meta.Episode)
		if epResult.Name == "" {
			fmt.Printf("MDB Warning: Episode S%02dE%02d not found on TVDB/TMDB.\n", meta.Season, meta.Episode)
		} else {
			fmt.Printf("MDB Episode: %s (S%02dE%02d)\n", epResult.Name, epResult.Season, epResult.Episode)
			CheckEpisodeTitle(meta, epResult)
			CheckSpecialDate(meta, epResult)
		}
	}
}

func CheckEpisodeTitle(meta *metadata.Metadata, epResult mdb.EpisodeResult) {
	if meta.EpisodeTitle != "" {
		normParsed := NormalizeForComparison(filename.DeobfuscateTitle(meta.EpisodeTitle))
		normOfficial := NormalizeForComparison(epResult.Name)
		if normParsed != normOfficial {
			fmt.Printf("MDB Warning: Episode title mismatch.\n  Filename: %s\n  TVDB:     %s\n", meta.EpisodeTitle, epResult.Name)
		}
	}
}

func CheckSpecialDate(meta *metadata.Metadata, epResult mdb.EpisodeResult) {
	if meta.Season == 0 && meta.Date != "" {
		if epResult.Airdate != meta.Date {
			fmt.Printf("MDB Warning: Special episode date mismatch. Filename: %s, TVDB: %s\n", meta.Date, epResult.Airdate)
		}
	}
}
