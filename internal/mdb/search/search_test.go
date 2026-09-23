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

//nolint:paralleltest // mutates package-level imdb.BaseURL; cannot run in parallel
func TestExecuteSearchImdbFallback(t *testing.T) {
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

	if results[0].TitleType != mdb.TitleTypeMovie {
		t.Errorf("expected TitleType %q, got %q", mdb.TitleTypeMovie, results[0].TitleType)
	}
}

func TestMergeIMDbResultsTitleType(t *testing.T) {
	t.Parallel()

	merged := []mdb.SearchResult{
		{
			Title:  "Existing Film",
			ImdbID: "tt0000001",
		},
	}

	imdbResults := []mdb.SearchResult{
		{
			Title:     "Existing Film",
			ImdbID:    "tt0000001",
			TitleType: mdb.TitleTypeMovie,
		},
		{
			Title:     "IMDb Standalone Special",
			ImdbID:    "tt0000004",
			TitleType: mdb.TitleTypeTVSpecial,
		},
	}

	res := mergeIMDbResults(merged, imdbResults)

	if len(res) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res))
	}

	if res[0].ImdbID != "tt0000001" || res[0].TitleType != mdb.TitleTypeMovie {
		t.Errorf("expected Existing Film to have TitleTypeMovie, got %+v", res[0])
	}

	if res[1].ImdbID != "tt0000004" || res[1].TitleType != mdb.TitleTypeTVSpecial {
		t.Errorf("expected IMDb Standalone Special to have TitleTypeTVSpecial, got %+v", res[1])
	}
}

func TestTitleTypePriorityAndRefinement(t *testing.T) {
	t.Parallel()

	t.Run("TVDB refines generic TMDB title type", func(t *testing.T) {
		t.Parallel()

		tmdbRes := mdb.SearchResult{
			TmdbID:    1,
			Title:     "Chernobyl",
			TitleType: mdb.TitleTypeTVSeries,
		}
		tvdbRes := mdb.SearchResult{
			TvdbID:    101,
			Title:     "Chernobyl",
			TitleType: mdb.TitleTypeTVMiniSeries,
		}

		mergeMatchedResult(&tmdbRes, &tvdbRes)

		if tmdbRes.TitleType != mdb.TitleTypeTVMiniSeries {
			t.Errorf("expected TitleTypeTVMiniSeries, got %q", tmdbRes.TitleType)
		}
	})

	t.Run("IMDb overrides contradicting TMDB/TVDB title type", func(t *testing.T) {
		t.Parallel()

		baseRes := mdb.SearchResult{
			TmdbID:    1,
			Title:     "Some Movie",
			TitleType: mdb.TitleTypeMovie,
		}
		imdbRes := mdb.SearchResult{
			ImdbID:    "tt1234567",
			Title:     "Some Movie",
			TitleType: mdb.TitleTypeTVMovie,
		}

		mergeImdbData(&baseRes, &imdbRes)

		if baseRes.TitleType != mdb.TitleTypeTVMovie {
			t.Errorf("expected TitleTypeTVMovie from IMDb, got %q", baseRes.TitleType)
		}

		seriesBase := mdb.SearchResult{
			TmdbID:    2,
			Title:     "Series",
			TitleType: mdb.TitleTypeTVMiniSeries,
		}
		imdbSeries := mdb.SearchResult{
			ImdbID:    "tt7654321",
			Title:     "Series",
			TitleType: mdb.TitleTypeTVSeries,
		}
		mergeImdbData(&seriesBase, &imdbSeries)

		if seriesBase.TitleType != mdb.TitleTypeTVSeries {
			t.Errorf("expected TitleTypeTVSeries from IMDb priority, got %q", seriesBase.TitleType)
		}
	})
}

func TestMergeRatingsAndCertificates(t *testing.T) {
	t.Parallel()

	baseRes := mdb.SearchResult{
		TmdbID:      1,
		Title:       "Movie",
		Certificate: "",
	}

	imdbRes := mdb.SearchResult{
		ImdbID:      "tt9999999",
		Rating:      8.4,
		Votes:       250000,
		Certificate: "PG-13",
	}

	mergeImdbData(&baseRes, &imdbRes)

	if baseRes.Rating != 8.4 || baseRes.Votes != 250000 {
		t.Errorf("expected IMDb rating/votes, got %v (%d votes)", baseRes.Rating, baseRes.Votes)
	}

	if baseRes.Certificate != "PG-13" {
		t.Errorf("expected Certificate 'PG-13', got %q", baseRes.Certificate)
	}
}

func TestMergePrincipalCredits(t *testing.T) {
	t.Parallel()

	baseRes := mdb.SearchResult{
		TmdbID: 1,
		Title:  "Movie",
	}

	imdbRes := mdb.SearchResult{
		ImdbID:    "tt9999999",
		Directors: []string{"Director One"},
		Writers:   []string{"Writer One", "Writer Two"},
		Cast: []mdb.CastMember{
			{Name: "Actor One", Role: "Character One"},
			{Name: "Actor Two", Role: "Character Two"},
		},
	}

	mergeImdbData(&baseRes, &imdbRes)

	if len(baseRes.Directors) != 1 || baseRes.Directors[0] != "Director One" {
		t.Errorf("expected Directors, got %v", baseRes.Directors)
	}

	if len(baseRes.Writers) != 2 || baseRes.Writers[0] != "Writer One" {
		t.Errorf("expected Writers, got %v", baseRes.Writers)
	}

	if len(baseRes.Cast) != 2 || baseRes.Cast[0].Name != "Actor One" || baseRes.Cast[0].Role != "Character One" {
		t.Errorf("expected Cast, got %v", baseRes.Cast)
	}
}
