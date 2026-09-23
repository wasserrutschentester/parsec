package imdb

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"codeberg.org/upPollo/parsec/internal/cache"
	"codeberg.org/upPollo/parsec/internal/mdb"
	"codeberg.org/upPollo/parsec/internal/metadata"
)

func setupTest(t *testing.T) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "imdb_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cache.SetDir(tempDir)
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })
}

func newMockServer(t *testing.T, resp any) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))

	t.Cleanup(srv.Close)

	return srv
}

func setBaseURL(t *testing.T, url string) {
	t.Helper()

	old := BaseURL
	BaseURL = url

	t.Cleanup(func() { BaseURL = old })
}

func TestNormalizeLanguage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id   string
		text string
		want string
	}{
		{"en", "English", "en"},
		{"ko", "Korean", "ko"},
		{"ja", "Japanese", "ja"},
		{"fr", "French", "fr"},
		{"de", "German", "de"},
		{"es", "Spanish", "es"},
		{"zxx", "No Dialogue", "zxx"},
		{"", "Korean", "ko"},
		{"", "English", "en"},
		{"", "", ""},
	}

	for _, tt := range tests {
		got := NormalizeLanguage(tt.id, tt.text)
		if got != tt.want {
			t.Errorf("NormalizeLanguage(%q, %q) = %q, want %q", tt.id, tt.text, got, tt.want)
		}
	}
}

func TestFormatIMDbID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"tt111161", "tt0111161"},
		{"111161", "tt0111161"},
		{"tt0111161", "tt0111161"},
		{"tt12345678", "tt12345678"},
		{"", ""},
	}

	for _, tt := range tests {
		got := FormatIMDbID(tt.input)
		if got != tt.want {
			t.Errorf("FormatIMDbID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func parasiteResponse() any {
	return map[string]any{
		"data": map[string]any{
			"title": map[string]any{
				"id":                "tt6751668",
				"titleText":         map[string]any{"text": "Parasite"},
				"originalTitleText": map[string]any{"text": "Gisaengchung"},
				"releaseYear":       map[string]any{"year": 2019, "endYear": nil},
				"titleType":         map[string]any{"id": "movie", "isSeries": false},
				"runtime":           map[string]any{"seconds": 7920},
				"plot":              map[string]any{"plotText": map[string]any{"plainText": "Greed and class discrimination threaten the newly formed symbiotic relationship between the wealthy Park family and the destitute Kim clan."}},
				"titleGenres": map[string]any{
					"genres": []any{
						map[string]any{"genre": map[string]any{"text": "Drama"}},
						map[string]any{"genre": map[string]any{"text": "Thriller"}},
					},
				},
				"countriesOfOrigin": map[string]any{
					"countries": []any{map[string]any{"id": "KR", "text": "South Korea"}},
				},
				"spokenLanguages": map[string]any{
					"spokenLanguages": []any{
						map[string]any{"id": "ko", "text": "Korean"},
						map[string]any{"id": "en", "text": "English"},
					},
				},
				"akas": map[string]any{
					"edges": []any{
						map[string]any{"node": map[string]any{"text": "Parasite"}},
						map[string]any{"node": map[string]any{"text": "Parasite: Black & White Edition"}},
					},
				},
			},
		},
	}
}

//nolint:paralleltest // mutates package-level BaseURL; cannot run in parallel
func TestGetByIDAndTitleDetails(t *testing.T) {
	setupTest(t)

	srv := newMockServer(t, parasiteResponse())
	setBaseURL(t, srv.URL)

	res, err := GetByID(t.Context(), "tt6751668")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	checkParasiteSearchResult(t, res)

	details, err := GetTitleDetails(t.Context(), "tt6751668")
	if err != nil {
		t.Fatalf("GetTitleDetails failed: %v", err)
	}

	if details.Title != "Parasite" {
		t.Errorf("details.Title = %q, want 'Parasite'", details.Title)
	}

	if details.Type != mdb.TitleTypeMovie {
		t.Errorf("details.Type = %q, want %q", details.Type, mdb.TitleTypeMovie)
	}

	if len(details.AltTitles) != 1 || details.AltTitles[0] != "Parasite: Black & White Edition" {
		t.Errorf("AltTitles = %v, want ['Parasite: Black & White Edition']", details.AltTitles)
	}
}

func checkParasiteBasicFields(t *testing.T, res *mdb.SearchResult) {
	t.Helper()

	if res.Title != "Parasite" {
		t.Errorf("Title = %q, want 'Parasite'", res.Title)
	}

	if res.TitleType != mdb.TitleTypeMovie {
		t.Errorf("TitleType = %q, want %q", res.TitleType, mdb.TitleTypeMovie)
	}

	if res.OriginalTitle != "Gisaengchung" {
		t.Errorf("OriginalTitle = %q, want 'Gisaengchung'", res.OriginalTitle)
	}

	if res.OriginalLanguage != "ko" {
		t.Errorf("OriginalLanguage = %q, want 'ko'", res.OriginalLanguage)
	}

	if res.Year != 2019 {
		t.Errorf("Year = %d, want 2019", res.Year)
	}

	if res.Runtime != 132 {
		t.Errorf("Runtime = %d, want 132", res.Runtime)
	}
}

func checkParasiteMetaFields(t *testing.T, res *mdb.SearchResult) {
	t.Helper()

	if res.Status != "Released" {
		t.Errorf("Status = %q, want 'Released'", res.Status)
	}

	if len(res.Countries) != 1 || res.Countries[0] != "KR" {
		t.Errorf("Countries = %v, want ['KR']", res.Countries)
	}

	if len(res.Genres) != 2 || res.Genres[0] != "Drama" {
		t.Errorf("Genres = %v, want ['Drama', 'Thriller']", res.Genres)
	}
}

func checkParasiteSearchResult(t *testing.T, res *mdb.SearchResult) {
	t.Helper()
	checkParasiteBasicFields(t, res)
	checkParasiteMetaFields(t, res)
}

func breakingBadResponse() any {
	return map[string]any{
		"data": map[string]any{
			"title": map[string]any{
				"id":          "tt0903747",
				"titleText":   map[string]any{"text": "Breaking Bad"},
				"releaseYear": map[string]any{"year": 2008, "endYear": 2013},
				"titleType":   map[string]any{"id": "tvSeries", "isSeries": true},
				"episodes": map[string]any{
					"episodes": map[string]any{
						"total": 2,
						"edges": []any{
							map[string]any{"node": map[string]any{
								"id": "tt0959621", "titleText": map[string]any{"text": "Pilot"},
								"releaseDate": map[string]any{"year": 2008, "month": 1, "day": 20},
								"runtime":     map[string]any{"seconds": 3480},
								"plot":        map[string]any{"plotText": map[string]any{"plainText": "Pilot episode."}},
								"series": map[string]any{"displayableEpisodeNumber": map[string]any{
									"displayableSeason": map[string]any{"season": "1"},
									"episodeNumber":     map[string]any{"text": "1"},
								}},
							}},
							map[string]any{"node": map[string]any{
								"id": "tt1054724", "titleText": map[string]any{"text": "Cat's in the Bag..."},
								"releaseDate": map[string]any{"year": 2008, "month": 1, "day": 27},
								"runtime":     map[string]any{"seconds": 2880},
								"plot":        map[string]any{"plotText": map[string]any{"plainText": "Second episode."}},
								"series": map[string]any{"displayableEpisodeNumber": map[string]any{
									"displayableSeason": map[string]any{"season": "1"},
									"episodeNumber":     map[string]any{"text": "2"},
								}},
							}},
						},
					},
				},
			},
		},
	}
}

//nolint:paralleltest // mutates package-level BaseURL; cannot run in parallel
func TestGetSeasonEpisodes(t *testing.T) {
	setupTest(t)

	srv := newMockServer(t, breakingBadResponse())
	setBaseURL(t, srv.URL)

	eps, err := GetSeasonEpisodes(t.Context(), "tt0903747", 1)
	if err != nil {
		t.Fatalf("GetSeasonEpisodes failed: %v", err)
	}

	if len(eps) != 2 {
		t.Fatalf("len(eps) = %d, want 2", len(eps))
	}

	if eps[0].TotalEpisodes != 2 {
		t.Errorf("TotalEpisodes = %d, want 2", eps[0].TotalEpisodes)
	}
}

//nolint:paralleltest // mutates package-level BaseURL; cannot run in parallel
func TestIdentifyEpisode(t *testing.T) {
	setupTest(t)

	srv := newMockServer(t, breakingBadResponse())
	setBaseURL(t, srv.URL)

	searchRes := mdb.SearchResult{ImdbID: "tt0903747", IsTV: true}

	identified, err := IdentifyEpisode(t.Context(), searchRes, &metadata.Metadata{Season: 1, Episodes: []int{2}}, false)
	if err != nil {
		t.Fatalf("IdentifyEpisode by S01E02 failed: %v", err)
	}

	if identified.Name != "Cat's in the Bag..." {
		t.Errorf("identified.Name = %q, want Cat's in the Bag...", identified.Name)
	}

	identifiedByDate, err := IdentifyEpisode(t.Context(), searchRes, &metadata.Metadata{Date: "2008-01-20"}, false)
	if err != nil {
		t.Fatalf("IdentifyEpisode by Date failed: %v", err)
	}

	if identifiedByDate.Name != "Pilot" {
		t.Errorf("identifiedByDate.Name = %q, want Pilot", identifiedByDate.Name)
	}
}

func shawshankSearchResponse() any {
	return map[string]any{
		"data": map[string]any{
			"advancedTitleSearch": map[string]any{
				"total": 1,
				"edges": []any{
					map[string]any{
						"node": map[string]any{
							"title": map[string]any{
								"id":                "tt0111161",
								"titleText":         map[string]any{"text": "The Shawshank Redemption"},
								"originalTitleText": map[string]any{"text": "The Shawshank Redemption"},
								"titleType":         map[string]any{"id": "movie", "isSeries": false},
								"releaseYear":       map[string]any{"year": 1994},
								"plot":              map[string]any{"plotText": map[string]any{"plainText": "Two imprisoned men bond over a number of years."}},
							},
						},
					},
				},
			},
		},
	}
}

//nolint:paralleltest // mutates package-level BaseURL; cannot run in parallel
func TestSearch(t *testing.T) {
	setupTest(t)

	srv := newMockServer(t, shawshankSearchResponse())
	setBaseURL(t, srv.URL)

	results, err := Search(t.Context(), "Shawshank", 1994, false)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}

	if results[0].Title != "The Shawshank Redemption" {
		t.Errorf("results[0].Title = %q, want 'The Shawshank Redemption'", results[0].Title)
	}

	if results[0].ImdbID != "tt0111161" {
		t.Errorf("results[0].ImdbID = %q, want 'tt0111161'", results[0].ImdbID)
	}
}

//nolint:paralleltest // mutates package-level BaseURL; cannot run in parallel
func TestRetryAfter429(t *testing.T) {
	setupTest(t)

	var callCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := callCount.Add(1)
		if count == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(shawshankSearchResponse())
	}))
	defer server.Close()

	setBaseURL(t, server.URL)

	start := time.Now()

	results, err := Search(t.Context(), "Shawshank", 1994, false)
	if err != nil {
		t.Fatalf("Expected retry to succeed, got: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if callCount.Load() != 2 {
		t.Errorf("Expected 2 calls (1 failure + 1 retry), got %d", callCount.Load())
	}

	if time.Since(start) < 900*time.Millisecond {
		t.Errorf("Expected delay of at least ~1s for Retry-After, took %v", time.Since(start))
	}
}

//nolint:funlen // mock response data
func mixedSearchResponse() any {
	return map[string]any{
		"data": map[string]any{
			"advancedTitleSearch": map[string]any{
				"total": 6,
				"edges": []any{
					map[string]any{
						"node": map[string]any{
							"title": map[string]any{
								"id":                "tt1000001",
								"titleText":         map[string]any{"text": "Sample Feature Movie"},
								"originalTitleText": map[string]any{"text": "Sample Feature Movie"},
								"titleType":         map[string]any{"id": "movie", "isSeries": false},
								"releaseYear":       map[string]any{"year": 2021},
								"plot":              map[string]any{"plotText": map[string]any{"plainText": "A movie."}},
							},
						},
					},
					map[string]any{
						"node": map[string]any{
							"title": map[string]any{
								"id":                "tt1000002",
								"titleText":         map[string]any{"text": "Sample TV Series"},
								"originalTitleText": map[string]any{"text": "Sample TV Series"},
								"titleType":         map[string]any{"id": "tvSeries", "isSeries": true},
								"releaseYear":       map[string]any{"year": 2021},
								"plot":              map[string]any{"plotText": map[string]any{"plainText": "A TV series."}},
							},
						},
					},
					map[string]any{
						"node": map[string]any{
							"title": map[string]any{
								"id":                "tt1000003",
								"titleText":         map[string]any{"text": "Sample Music Video"},
								"originalTitleText": map[string]any{"text": "Sample Music Video"},
								"titleType":         map[string]any{"id": "musicVideo", "isSeries": false},
								"releaseYear":       map[string]any{"year": 2021},
								"plot":              map[string]any{"plotText": map[string]any{"plainText": "Music video clip."}},
							},
						},
					},
					map[string]any{
						"node": map[string]any{
							"title": map[string]any{
								"id":                "tt1000004",
								"titleText":         map[string]any{"text": "Sample Podcast Show"},
								"originalTitleText": map[string]any{"text": "Sample Podcast Show"},
								"titleType":         map[string]any{"id": "podcastSeries", "isSeries": true},
								"releaseYear":       map[string]any{"year": 2021},
								"plot":              map[string]any{"plotText": map[string]any{"plainText": "A podcast show."}},
							},
						},
					},
					map[string]any{
						"node": map[string]any{
							"title": map[string]any{
								"id":                "tt1000005",
								"titleText":         map[string]any{"text": "Sample Podcast Episode"},
								"originalTitleText": map[string]any{"text": "Sample Podcast Episode"},
								"titleType":         map[string]any{"id": "podcastEpisode", "isSeries": false},
								"releaseYear":       map[string]any{"year": 2021},
								"plot":              map[string]any{"plotText": map[string]any{"plainText": "A podcast episode."}},
							},
						},
					},
					map[string]any{
						"node": map[string]any{
							"title": map[string]any{
								"id":                "tt1000006",
								"titleText":         map[string]any{"text": "Sample Video Game"},
								"originalTitleText": map[string]any{"text": "Sample Video Game"},
								"titleType":         map[string]any{"id": "videoGame", "isSeries": false},
								"releaseYear":       map[string]any{"year": 2021},
								"plot":              map[string]any{"plotText": map[string]any{"plainText": "A video game."}},
							},
						},
					},
				},
			},
		},
	}
}

//nolint:paralleltest // mutates package-level BaseURL; cannot run in parallel
func TestSearchExcludesMusicVideosAndPodcasts(t *testing.T) {
	setupTest(t)

	srv := newMockServer(t, mixedSearchResponse())
	setBaseURL(t, srv.URL)

	movieResults, err := Search(t.Context(), "Sample", 2021, false)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(movieResults) != 1 {
		t.Fatalf("len(movieResults) = %d, want 1", len(movieResults))
	}

	if movieResults[0].Title != "Sample Feature Movie" || movieResults[0].TitleType != mdb.TitleTypeMovie {
		t.Errorf("got %+v, want Sample Feature Movie with TitleTypeMovie", movieResults[0])
	}

	tvResults, err := Search(t.Context(), "Sample", 2021, true)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(tvResults) != 1 {
		t.Fatalf("len(tvResults) = %d, want 1", len(tvResults))
	}

	if tvResults[0].Title != "Sample TV Series" || tvResults[0].TitleType != mdb.TitleTypeTVSeries {
		t.Errorf("got %+v, want Sample TV Series with TitleTypeTVSeries", tvResults[0])
	}
}
