package search

import (
	"math"
	"sort"
	"strings"

	"codeberg.org/n0ne/parsec/internal/mdb"
	"codeberg.org/n0ne/parsec/internal/mdb/tmdb"
)

func SearchMovie(query string, year int) ([]mdb.SearchResult, error) {
	return search("movie", query, year)
}

func SearchTV(query string, year int) ([]mdb.SearchResult, error) {
	return search("tv", query, year)
}

func search(mediaType, query string, year int) ([]mdb.SearchResult, error) {
	results, err := tmdb.Search(mediaType, query, year)
	if err != nil {
		return nil, err
	}

	return results, nil
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
	data, err := tmdb.GetEpisodeMetadata(result.TmdbID, season, episode)
	if err != nil {
		return mdb.EpisodeResult{}
	}
	return data
}
