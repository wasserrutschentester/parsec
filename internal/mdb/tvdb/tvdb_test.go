package tvdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/viper"

	"codeberg.org/upPollo/parsec/internal/cache"
	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/mdb"
	"codeberg.org/upPollo/parsec/internal/metadata"
)

func setupTest(t *testing.T) {
	t.Helper()

	config.InitDefaults()

	config.NoCache = true

	cache.SetDir(t.TempDir())
}

//nolint:paralleltest // depends on shared global state (viper, config.NoCache, BaseURL)
func TestIdentifyEpisodeByDate(t *testing.T) {
	setupTest(t)

	viper.Set("api_keys.tvdb", "dummy_key")
	viper.Set("preferred_language", "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			_, _ = w.Write([]byte(`{"status": "success", "data": {"token": "dummy_token"}}`))
		case "/series/999/episodes/default/en":
			_, _ = w.Write([]byte(`{"status": "success", "data": {"episodes": [{"id": 456, "number": 1, "seasonNumber": 1, "aired": "2023-01-01", "name": "", "overview": ""}]}, "links": {"next": ""}}`))
		case "/episodes/456/translations/eng":
			_, _ = w.Write([]byte(`{"status": "success", "data": {"name": "Test Episode", "overview": "Test Overview"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	originalBaseURL := BaseURL

	BaseURL = server.URL
	defer func() { BaseURL = originalBaseURL }()

	result, _ := IdentifyEpisode(t.Context(), mdb.SearchResult{TvdbID: 999, OriginalLanguage: "en"}, &metadata.Metadata{Date: "2023-01-01"}, false)

	if result.TvdbID != 456 {
		t.Errorf("Expected TvdbID 456, got %d", result.TvdbID)
	}

	if result.Name != "Test Episode" {
		t.Errorf("Expected name 'Test Episode', got %q", result.Name)
	}
}

//nolint:paralleltest // depends on shared global state (viper, config.NoCache, BaseURL)
func TestIdentifyEpisodeByTitle(t *testing.T) {
	setupTest(t)

	viper.Set("api_keys.tvdb", "dummy_key")
	viper.Set("preferred_language", "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			_, _ = w.Write([]byte(`{"status": "success", "data": {"token": "dummy_token"}}`))
		case "/series/999/episodes/default/en":
			_, _ = w.Write([]byte(`{"status": "success", "data": {"episodes": [{"id": 789, "number": 2, "seasonNumber": 1, "aired": "2023-01-02", "name": "Test Episode", "overview": ""}]}, "links": {"next": ""}}`))
		case "/episodes/789/translations/eng":
			_, _ = w.Write([]byte(`{"status": "success", "data": {"name": "Test Episode", "overview": "Test Overview"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	originalBaseURL := BaseURL

	BaseURL = server.URL
	defer func() { BaseURL = originalBaseURL }()

	result, _ := IdentifyEpisode(t.Context(), mdb.SearchResult{TvdbID: 999, OriginalLanguage: "en"}, &metadata.Metadata{EpisodeTitles: []string{"Test Episode"}}, false)

	if result.TvdbID != 789 {
		t.Errorf("Expected TvdbID 789, got %d", result.TvdbID)
	}
}

//nolint:paralleltest // depends on shared global state (viper, config.NoCache, BaseURL)
func TestIdentifyEpisodeIgnoreSpecialsByDate(t *testing.T) {
	setupTest(t)

	viper.Set("api_keys.tvdb", "dummy_key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			_, _ = w.Write([]byte(`{"status": "success", "data": {"token": "dummy_token"}}`))
		case "/series/999/episodes/default/en":
			_, _ = w.Write([]byte(`{"status": "success", "data": {"episodes": [
				{"id": 100, "number": 1, "seasonNumber": 0, "aired": "2023-01-01", "name": "Special"},
				{"id": 200, "number": 1, "seasonNumber": 1, "aired": "2023-01-01", "name": "Regular"}
			]}, "links": {"next": ""}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	originalBaseURL := BaseURL

	BaseURL = server.URL
	defer func() { BaseURL = originalBaseURL }()

	// Case 1: Specials NOT allowed, should skip special and find regular
	result1, _ := IdentifyEpisode(t.Context(), mdb.SearchResult{TvdbID: 999, OriginalLanguage: "en"}, &metadata.Metadata{Date: "2023-01-01"}, false)
	if result1.TvdbID != 200 {
		t.Errorf("Expected TvdbID 200 (Regular), got %d", result1.TvdbID)
	}

	// Case 2: Specials allowed
	result2, _ := IdentifyEpisode(t.Context(), mdb.SearchResult{TvdbID: 999, OriginalLanguage: "en"}, &metadata.Metadata{Date: "2023-01-01"}, true)
	if result2.TvdbID != 100 {
		t.Errorf("Expected TvdbID 100 (Special), got %d", result2.TvdbID)
	}
}

//nolint:paralleltest // depends on shared global state (viper, config.NoCache, BaseURL)
func TestIdentifyEpisodeSeason0(t *testing.T) {
	setupTest(t)

	viper.Set("api_keys.tvdb", "dummy_key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			_, _ = w.Write([]byte(`{"status": "success", "data": {"token": "dummy_token"}}`))
		case "/series/999/episodes/default/en":
			_, _ = w.Write([]byte(`{"status": "success", "data": {"episodes": [
				{"id": 100, "number": 1, "seasonNumber": 0, "aired": "2023-01-01", "name": "Special Episode 1"}
			]}, "links": {"next": ""}}`))
		case "/episodes/100/translations/eng":
			_, _ = w.Write([]byte(`{"status": "success", "data": {"name": "Special Episode 1", "overview": "Special Overview"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	originalBaseURL := BaseURL

	BaseURL = server.URL
	defer func() { BaseURL = originalBaseURL }()

	meta := &metadata.Metadata{
		Season:   0,
		Episodes: []int{1},
		IsTV:     true,
	}

	result, err := IdentifyEpisode(t.Context(), mdb.SearchResult{TvdbID: 999, OriginalLanguage: "en"}, meta, false)
	if err != nil {
		t.Fatalf("IdentifyEpisode failed: %v", err)
	}

	if result.TvdbID != 100 {
		t.Errorf("Expected TvdbID 100, got %d", result.TvdbID)
	}
}

//nolint:paralleltest // depends on shared global state (viper, config.NoCache, BaseURL)
func TestIdentifyEpisodeS00E00Placeholder(t *testing.T) {
	setupTest(t)

	viper.Set("api_keys.tvdb", "dummy_key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			_, _ = w.Write([]byte(`{"status": "success", "data": {"token": "dummy_token"}}`))
		case "/series/999/episodes/default/en":
			_, _ = w.Write([]byte(`{"status": "success", "data": {"episodes": [
				{"id": 200, "number": 2, "seasonNumber": 2000, "aired": "2000-01-09", "name": "Stoever - 38 - Blaues Blut"}
			]}, "links": {"next": ""}}`))
		case "/episodes/200/translations/eng":
			_, _ = w.Write([]byte(`{"status": "success", "data": {"name": "Stoever - 38 - Blaues Blut", "overview": "Overview"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	originalBaseURL := BaseURL

	BaseURL = server.URL
	defer func() { BaseURL = originalBaseURL }()

	meta := &metadata.Metadata{
		Season:        0,
		Episodes:      []int{0},
		EpisodeTitles: []string{"Blaues Blut"},
		IsTV:          true,
	}

	result, err := IdentifyEpisode(t.Context(), mdb.SearchResult{TvdbID: 999, OriginalLanguage: "en"}, meta, false)
	if err != nil {
		t.Fatalf("IdentifyEpisode failed: %v", err)
	}

	if result.TvdbID != 200 {
		t.Errorf("Expected TvdbID 200, got %d", result.TvdbID)
	}
}

//nolint:paralleltest // depends on shared global state (viper, config.NoCache, BaseURL)
func TestIdentifyEpisodeSeasonAndTitle(t *testing.T) {
	setupTest(t)

	viper.Set("api_keys.tvdb", "dummy_key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			_, _ = w.Write([]byte(`{"status": "success", "data": {"token": "dummy_token"}}`))
		case "/series/999/episodes/default/en":
			_, _ = w.Write([]byte(`{"status": "success", "data": {"episodes": [
				{"id": 101, "number": 1, "seasonNumber": 1, "aired": "2021-01-01", "name": "Special Episode"},
				{"id": 202, "number": 1, "seasonNumber": 2, "aired": "2022-01-01", "name": "Special Episode"}
			]}, "links": {"next": ""}}`))
		case "/episodes/202/translations/eng":
			_, _ = w.Write([]byte(`{"status": "success", "data": {"name": "Special Episode", "overview": "Season 2 overview"}}`))
		case "/episodes/101/translations/eng":
			_, _ = w.Write([]byte(`{"status": "success", "data": {"name": "Special Episode", "overview": "Season 1 overview"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	originalBaseURL := BaseURL

	BaseURL = server.URL
	defer func() { BaseURL = originalBaseURL }()

	meta := &metadata.Metadata{
		Season:        2,
		EpisodeTitles: []string{"Special Episode"},
		IsTV:          true,
	}

	result, err := IdentifyEpisode(t.Context(), mdb.SearchResult{TvdbID: 999, OriginalLanguage: "en"}, meta, false)
	if err != nil {
		t.Fatalf("IdentifyEpisode failed: %v", err)
	}

	if result.TvdbID != 202 {
		t.Errorf("Expected TvdbID 202 (Season 2), got %d", result.TvdbID)
	}
}

//nolint:paralleltest // depends on shared global state (viper, config.NoCache, BaseURL)
func TestIdentifyEpisodeSeasonAndTitleFallback(t *testing.T) {
	setupTest(t)

	viper.Set("api_keys.tvdb", "dummy_key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			_, _ = w.Write([]byte(`{"status": "success", "data": {"token": "dummy_token"}}`))
		case "/series/999/episodes/default/en":
			_, _ = w.Write([]byte(`{"status": "success", "data": {"episodes": [
				{"id": 101, "number": 1, "seasonNumber": 1, "aired": "2021-01-01", "name": "Only In Season 1"},
				{"id": 202, "number": 1, "seasonNumber": 2, "aired": "2022-01-01", "name": "Season 2 Episode"}
			]}, "links": {"next": ""}}`))
		case "/episodes/101/translations/eng":
			_, _ = w.Write([]byte(`{"status": "success", "data": {"name": "Only In Season 1", "overview": "Overview"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	originalBaseURL := BaseURL

	BaseURL = server.URL
	defer func() { BaseURL = originalBaseURL }()

	meta := &metadata.Metadata{
		Season:        2,
		EpisodeTitles: []string{"Only In Season 1"},
		IsTV:          true,
	}

	result, err := IdentifyEpisode(t.Context(), mdb.SearchResult{TvdbID: 999, OriginalLanguage: "en"}, meta, false)
	if err != nil {
		t.Fatalf("IdentifyEpisode failed: %v", err)
	}

	if result.TvdbID != 101 {
		t.Errorf("Expected TvdbID 101 (Season 1 fallback), got %d", result.TvdbID)
	}
}

//nolint:paralleltest // depends on shared global state (viper, config.NoCache, BaseURL)
func TestSearchRetryAfter429(t *testing.T) {
	setupTest(t)

	viper.Set("api_keys.tvdb", "dummy_key")

	var searchAttempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			_, _ = w.Write([]byte(`{"status": "success", "data": {"token": "dummy_token"}}`))
		case "/search":
			if searchAttempts.Add(1) == 1 {
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)

				return
			}

			_, _ = w.Write([]byte(`{"status": "success", "data": [{"tvdb_id": "123", "name": "Test Series", "type": "series"}]}`))
		case "/series/123/extended":
			_, _ = w.Write([]byte(`{"status": "success", "data": {"remoteIds": []}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	originalBaseURL := BaseURL

	BaseURL = server.URL
	defer func() { BaseURL = originalBaseURL }()

	results, err := Search(t.Context(), "tv", "Test Series", 0)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if searchAttempts.Load() != 2 {
		t.Errorf("expected 2 search attempts, got %d", searchAttempts.Load())
	}

	if len(results) != 1 || results[0].TvdbID != 123 {
		t.Errorf("unexpected results: %+v", results)
	}
}

//nolint:paralleltest // depends on shared global state (viper, config.NoCache, BaseURL)
func TestStructuredError(t *testing.T) {
	setupTest(t)

	viper.Set("api_keys.tvdb", "dummy_key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"status": "failure", "message": "Invalid API key"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	originalBaseURL := BaseURL

	BaseURL = server.URL
	defer func() { BaseURL = originalBaseURL }()

	_, err := Search(t.Context(), "movie", "Any", 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "Invalid API key") {
		t.Errorf("expected error to contain 'Invalid API key', got %q", err.Error())
	}
}

//nolint:paralleltest // depends on shared global state (viper, config.NoCache, BaseURL)
func TestContextCancellation(t *testing.T) {
	setupTest(t)

	viper.Set("api_keys.tvdb", "dummy_key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			_, _ = w.Write([]byte(`{"status": "success", "data": {"token": "dummy_token"}}`))
		case "/search":
			w.Header().Set("Retry-After", "10")
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	originalBaseURL := BaseURL

	BaseURL = server.URL
	defer func() { BaseURL = originalBaseURL }()

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	_, err := Search(ctx, "movie", "Any", 0)
	if err == nil {
		t.Fatal("expected error on cancelled context, got nil")
	}

	if !strings.Contains(err.Error(), "context") {
		t.Errorf("expected error to mention context, got %q", err.Error())
	}
}

func TestDetermineTitleType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mediaType string
		genres    []string
		want      mdb.TitleType
	}{
		{"Series Default", "series", nil, mdb.TitleTypeTVSeries},
		{"Series Miniseries", "series", []string{"Drama", "Mini-Series"}, mdb.TitleTypeTVMiniSeries},
		{"Movie Default", "movie", []string{"Action"}, mdb.TitleTypeMovie},
		{"Movie Made for TV", "movie", []string{"Drama", "Made for TV"}, mdb.TitleTypeTVMovie},
		{"Movie TV Movie", "movie", []string{"TV Movie"}, mdb.TitleTypeTVMovie},
		{"Unknown", "company", nil, mdb.TitleTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := determineTitleType(tt.mediaType, tt.genres)
			if got != tt.want {
				t.Errorf("determineTitleType(%q, %v) = %q, want %q",
					tt.mediaType, tt.genres, got, tt.want)
			}
		})
	}
}
