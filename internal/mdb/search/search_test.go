package search

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"codeberg.org/upPollo/parsec/internal/mdb"
	"codeberg.org/upPollo/parsec/internal/mdb/imdb"
)

func TestCalculateSimilarity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		s1   string
		s2   string
		want float64
	}{
		{"abc", "abc", 1.0},
		{"abc", "abd", 2.0 / 3.0},
		{"abc", "", 0.0},
		{"", "abc", 0.0},
		{"kitten", "sitting", 1.0 - (3.0 / 7.0)},
		{"flaw", "lawn", 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.s1+"_"+tt.s2, func(t *testing.T) {
			t.Parallel()

			got := mdb.CalculateSimilarity(tt.s1, tt.s2)
			if (got-tt.want) > 0.001 || (tt.want-got) > 0.001 {
				t.Errorf("CalculateSimilarity(%q, %q) = %v, want %v", tt.s1, tt.s2, got, tt.want)
			}
		})
	}
}

func TestAddUniqueAltTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		titles         []string
		newTitle       string
		existingTitles []string
		want           []string
	}{
		{[]string{"A"}, "B", []string{"C"}, []string{"A", "B"}},
		{[]string{"A"}, "A", []string{"B"}, []string{"A"}},
		{[]string{"A"}, "B", []string{"B"}, []string{"A"}},
		{[]string{"A"}, "", []string{"B"}, []string{"A"}},
		{nil, "A", nil, []string{"A"}},
	}

	for _, tt := range tests {
		t.Run(tt.newTitle, func(t *testing.T) {
			t.Parallel()

			got := addUniqueAltTitle(tt.titles, tt.newTitle, tt.existingTitles...)
			if len(got) != len(tt.want) {
				t.Errorf("addUniqueAltTitle(%v, %q) len = %d, want %d", tt.titles, tt.newTitle, len(got), len(tt.want))

				return
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("addUniqueAltTitle(%v, %q) = %v, want %v", tt.titles, tt.newTitle, got, tt.want)

					break
				}
			}
		})
	}
}

//nolint:funlen,paralleltest // merging results involves many test cases; not easily parallelizable
func TestMergeResults(t *testing.T) {
	tmdbResults := []mdb.SearchResult{
		{
			TmdbID: 1,
			Title:  "Movie 1",
			ImdbID: "tt1",
		},
		{
			TmdbID: 2,
			Title:  "Movie 2",
		},
	}

	tvdbResults := []mdb.SearchResult{
		{
			TvdbID: 101,
			TmdbID: 1,
			Title:  "Movie One",
		},
		{
			TvdbID: 102,
			Title:  "Movie 3",
			ImdbID: "tt3",
		},
	}

	merged := mergeResults(tmdbResults, tvdbResults)

	if len(merged) != 3 {
		t.Errorf("Expected 3 merged results, got %d", len(merged))
	}

	// Check if Movie 1 was merged correctly
	foundMovie1 := false

	for _, r := range merged {
		if r.TmdbID == 1 {
			foundMovie1 = true

			if r.TvdbID != 101 {
				t.Errorf("Movie 1: expected TvdbID 101, got %d", r.TvdbID)
			}

			foundAlt := slices.Contains(r.AltTitle, "Movie One")

			if !foundAlt {
				t.Errorf("Movie 1: expected 'Movie One' in AltTitles")
			}
		}
	}

	if !foundMovie1 {
		t.Errorf("Movie 1 not found in merged results")
	}

	// Check if Movie 3 was added
	foundMovie3 := false

	for _, r := range merged {
		if r.TvdbID == 102 {
			foundMovie3 = true
		}
	}

	if !foundMovie3 {
		t.Errorf("Movie 3 not found in merged results")
	}
}

func TestSearchByImdbIDFallback(t *testing.T) {
	t.Parallel()

	// With no TMDB or TVDB configured, searching by an invalid/unmocked ID returns ErrNotFound
	res, err := searchByImdbID("tt0000000", false, "movie")
	if res != nil {
		t.Errorf("expected nil result, got %+v", res)
	}

	if err == nil {
		t.Errorf("expected error for non-existent ID, got nil")
	}
}

func imdbExclusiveSearchResponse() any {
	return map[string]any{
		"data": map[string]any{
			"advancedTitleSearch": map[string]any{
				"total": 1,
				"edges": []any{
					map[string]any{
						"node": map[string]any{
							"title": map[string]any{
								"id":                "tt9999999",
								"titleText":         map[string]any{"text": "IMDb Exclusive Movie"},
								"originalTitleText": map[string]any{"text": "IMDb Exclusive Movie"},
								"titleType":         map[string]any{"id": "movie", "isSeries": false},
								"releaseYear":       map[string]any{"year": 2024},
								"plot":              map[string]any{"plotText": map[string]any{"plainText": "A movie only found via IMDb fallback."}},
							},
						},
					},
				},
			},
		},
	}
}

func TestExecuteSearchImdbFallback(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(imdbExclusiveSearchResponse())
	}))
	defer server.Close()

	oldBaseURL := imdb.BaseURL
	imdb.BaseURL = server.URL

	defer func() { imdb.BaseURL = oldBaseURL }()

	results, err := executeSearch("movie", "IMDb Exclusive Movie", 2024)
	if err != nil {
		t.Fatalf("executeSearch with IMDb fallback failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].ImdbID != "tt9999999" {
		t.Errorf("expected ImdbID tt9999999, got %s", results[0].ImdbID)
	}

	if results[0].Title != "IMDb Exclusive Movie" {
		t.Errorf("expected Title 'IMDb Exclusive Movie', got %s", results[0].Title)
	}
}
