package tmdb

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/viper"

	"codeberg.org/upPollo/parsec/internal/cache"
	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/mdb"
)

func setupTest(t *testing.T) {
	t.Helper()
	cache.SetDir(t.TempDir())
}

//nolint:paralleltest // depends on shared global state (viper, BaseURL)
func TestSearch(t *testing.T) {
	setupTest(t)
	// Provide dummy API key
	viper.Set("api_keys.tmdb", "dummy_key")

	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock search response
		switch r.URL.Path {
		case "/search/movie":
			resp := tmdbSearchResponse{
				Results: []tmdbMedia{
					{ID: 1, Title: "Test Movie", ReleaseDate: "2023-01-01"},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/movie/1/external_ids":
			resp := tmdbExternalIDsResponse{Imdb: "tt123"}
			_ = json.NewEncoder(w).Encode(resp)
		case "/movie/1/alternative_titles":
			// Return empty for simplicity
			_, _ = w.Write([]byte(`{"titles": []}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Override BaseURL
	originalBaseURL := BaseURL

	BaseURL = server.URL
	defer func() { BaseURL = originalBaseURL }()

	results, err := Search(t.Context(), "movie", "Test", 2023)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if results[0].Title != "Test Movie" {
		t.Errorf("Expected title 'Test Movie', got %q", results[0].Title)
	}

	if results[0].ImdbID != "tt123" {
		t.Errorf("Expected ImdbID 'tt123', got %q", results[0].ImdbID)
	}
}

//nolint:paralleltest // depends on shared global state (viper, BaseURL)
func TestGetByIDLocalization(t *testing.T) {
	setupTest(t)
	config.InitDefaults()
	viper.Set("api_keys.tmdb", "dummy_key")
	viper.Set("preferred_language", "de")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := r.URL.Query().Get("language")
		if lang != "de" {
			t.Errorf("Expected language 'de', got %q", lang)
		}

		switch r.URL.Path {
		case "/tv/123":
			_, _ = w.Write([]byte(`{"id": 123, "name": "German Title", "overview": "German Overview"}`))
		case "/tv/123/external_ids":
			_, _ = w.Write([]byte(`{"tvdb_id": 459258}`))
		case "/tv/123/alternative_titles":
			_, _ = w.Write([]byte(`{"results": []}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	originalBaseURL := BaseURL

	BaseURL = server.URL
	defer func() { BaseURL = originalBaseURL }()

	result, err := GetByID(t.Context(), 123, "tv")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if result.Title != "German Title" {
		t.Errorf("Expected title 'German Title', got %q", result.Title)
	}
}

//nolint:paralleltest,funlen // depends on shared global state and includes mock server setup
func TestGetByIDAppendToResponse(t *testing.T) {
	setupTest(t)
	config.InitDefaults()
	viper.Set("api_keys.tmdb", "dummy_key")

	var requestsCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestsCount.Add(1)

		if r.URL.Path == "/movie/456" {
			appendToResp := r.URL.Query().Get("append_to_response")
			if !strings.Contains(appendToResp, "external_ids") || !strings.Contains(appendToResp, "alternative_titles") {
				t.Errorf("Expected append_to_response containing external_ids,alternative_titles, got %q", appendToResp)
			}

			resp := `{
				"id": 456,
				"title": "Bundled Movie",
				"release_date": "2024-05-01",
				"external_ids": {
					"imdb_id": "tt9999",
					"tvdb_id": 8888
				},
				"alternative_titles": {
					"titles": [
						{"title": "Bundled Alt", "iso_3166_1": "US"}
					]
				}
			}`
			_, _ = w.Write([]byte(resp))

			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	originalBaseURL := BaseURL

	BaseURL = server.URL
	defer func() { BaseURL = originalBaseURL }()

	result, err := GetByID(t.Context(), 456, "movie")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if requestsCount.Load() != 1 {
		t.Errorf("Expected exactly 1 request with append_to_response, got %d", requestsCount.Load())
	}

	if result.ImdbID != "tt9999" {
		t.Errorf("Expected ImdbID 'tt9999', got %q", result.ImdbID)
	}

	if result.TvdbID != 8888 {
		t.Errorf("Expected TvdbID 8888, got %d", result.TvdbID)
	}

	if len(result.AltTitle) != 1 || result.AltTitle[0] != "Bundled Alt" {
		t.Errorf("Expected AltTitle ['Bundled Alt'], got %v", result.AltTitle)
	}
}

//nolint:paralleltest // depends on shared global state (viper, BaseURL)
func TestBearerTokenAuth(t *testing.T) {
	setupTest(t)

	jwtToken := "eyJhbGciOiJIUzI1NiJ9.eyJhdWQiOiIxMjM0NTY3OCJ9.signature_sample_test"
	viper.Set("api_keys.tmdb", jwtToken)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")

		expectedAuth := "Bearer " + jwtToken
		if authHeader != expectedAuth {
			t.Errorf("Expected Authorization header %q, got %q", expectedAuth, authHeader)
		}

		if r.URL.Query().Get("api_key") != "" {
			t.Errorf("Expected no api_key query param for Bearer token, got %q", r.URL.Query().Get("api_key"))
		}

		_, _ = w.Write([]byte(`{"results": []}`))
	}))
	defer server.Close()

	originalBaseURL := BaseURL

	BaseURL = server.URL
	defer func() { BaseURL = originalBaseURL }()

	_, err := Search(t.Context(), "movie", "AuthTest", 2024)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
}

//nolint:paralleltest // depends on shared global state (viper, BaseURL)
func TestRetryAfter429(t *testing.T) {
	setupTest(t)
	viper.Set("api_keys.tmdb", "dummy_key")

	var callCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := callCount.Add(1)
		if count == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)

			return
		}

		_, _ = w.Write([]byte(`{"results": []}`))
	}))
	defer server.Close()

	originalBaseURL := BaseURL

	BaseURL = server.URL
	defer func() { BaseURL = originalBaseURL }()

	start := time.Now()

	_, err := Search(t.Context(), "movie", "RateLimitTest", 2024)
	if err != nil {
		t.Fatalf("Expected retry to succeed, got: %v", err)
	}

	if callCount.Load() != 2 {
		t.Errorf("Expected 2 calls (1 failure + 1 retry), got %d", callCount.Load())
	}

	if time.Since(start) < 900*time.Millisecond {
		t.Errorf("Expected delay of at least ~1s for Retry-After, took %v", time.Since(start))
	}
}

//nolint:paralleltest // depends on shared global state (viper, BaseURL)
func TestStructuredErrorDecoding(t *testing.T) {
	setupTest(t)
	viper.Set("api_keys.tmdb", "dummy_key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{
			"status_code": 7,
			"status_message": "Invalid API key: You must be granted a valid key.",
			"success": false
		}`))
	}))
	defer server.Close()

	originalBaseURL := BaseURL

	BaseURL = server.URL
	defer func() { BaseURL = originalBaseURL }()

	_, err := Search(t.Context(), "movie", "ErrorTest", 2024)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	expectedMsg := "Invalid API key: You must be granted a valid key."
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error message to contain %q, got %q", expectedMsg, err.Error())
	}
}

func TestDetermineTitleType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mediaType string
		tmdbType  string
		genres    []string
		video     bool
		want      mdb.TitleType
	}{
		{"TV Miniseries", "tv", "Miniseries", nil, false, mdb.TitleTypeTVMiniSeries},
		{"TV Video", "tv", "Video", nil, false, mdb.TitleTypeVideo},
		{"TV Scripted", "tv", "Scripted", nil, false, mdb.TitleTypeTVSeries},
		{"TV Default", "tv", "", nil, false, mdb.TitleTypeTVSeries},
		{"Movie Standard", "movie", "", []string{"Action", "Sci-Fi"}, false, mdb.TitleTypeMovie},
		{"Movie TV Movie Genre", "movie", "", []string{"Drama", "TV Movie"}, false, mdb.TitleTypeTVMovie},
		{"Movie Direct to Video", "movie", "", []string{"Action"}, true, mdb.TitleTypeVideo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := determineTitleType(tt.mediaType, tt.tmdbType, tt.genres, tt.video)
			if got != tt.want {
				t.Errorf("determineTitleType(%q, %q, %v, %v) = %q, want %q",
					tt.mediaType, tt.tmdbType, tt.genres, tt.video, got, tt.want)
			}
		})
	}
}

func TestExtractSpokenLanguages(t *testing.T) {
	t.Parallel()

	input := []struct {
		Iso6391 string `json:"iso_639_1"`
	}{
		{Iso6391: "en"},
		{Iso6391: "de"},
		{Iso6391: "xx"},
		{Iso6391: "EN"},
		{Iso6391: ""},
	}

	got := extractSpokenLanguages(input)
	want := []string{"en", "de", "zxx"}

	if !slices.Equal(got, want) {
		t.Errorf("extractSpokenLanguages() = %v, want %v", got, want)
	}
}
