package search

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"codeberg.org/n0ne/parsec/internal/mdb"
	"codeberg.org/n0ne/parsec/internal/mdb/tmdb"
	"codeberg.org/n0ne/parsec/internal/mdb/tvdb"
)

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
			if r.Overview == "" {
				r.Overview = tvdbRes.Overview
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

// FuzzySearch combines search and filtering/sorting to find the best match
func FuzzySearch(query string, year int, isTV bool) (*mdb.SearchResult, error) {
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

	// Sort results by popularity descending as a baseline
	sort.Slice(results, func(i, j int) bool {
		return results[i].Popularity > results[j].Popularity
	})

	// If year is provided, prioritize exact year matches
	if year > 0 {
		var yearMatches []mdb.SearchResult
		for _, r := range results {
			if r.Year == year {
				yearMatches = append(yearMatches, r)
			}
		}
		if len(yearMatches) > 0 {
			results = yearMatches
		}
	}

	// Simple fuzzy title matching: prefer result with title closest in length or matching exactly (ignoring case)
	queryLower := strings.ToLower(query)
	bestMatchIndex := 0
	minDiff := math.MaxInt32

	for i, r := range results {
		titleLower := strings.ToLower(r.Title)
		origLower := strings.ToLower(r.OriginalTitle)

		if titleLower == queryLower || origLower == queryLower {
			return &results[i], nil
		}

		// Calculate a simple difference in length as a proxy for "fuzziness"
		diff := int(math.Abs(float64(len(titleLower) - len(queryLower))))
		if diff < minDiff {
			minDiff = diff
			bestMatchIndex = i
		}
	}

	return &results[bestMatchIndex], nil
}

func FindEpisode(result mdb.SearchResult, season, episode int) mdb.EpisodeResult {
	if result.TvdbID > 0 {
		data, err := tvdb.GetEpisodeMetadata(result.TvdbID, season, episode)
		if err == nil {
			return data
		}
	}

	// Fallback to TMDB if TVDB id is missing or fails
	if result.TmdbID > 0 {
		data, err := tmdb.GetEpisodeMetadata(result.TmdbID, season, episode)
		if err == nil {
			return data
		}
	}
	return mdb.EpisodeResult{}
}
