package tvdb

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/mdb"
	"codeberg.org/upPollo/parsec/internal/metadata"
)

//nolint:paralleltest // depends on shared global state (viper, config.NoCache, BaseURL)
func TestIdentifyEpisodeByDate(t *testing.T) {
	config.InitDefaults()

	config.NoCache = true

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

	result, _ := IdentifyEpisode(mdb.SearchResult{TvdbID: 999, OriginalLanguage: "en"}, &metadata.Metadata{Date: "2023-01-01"}, false)

	if result.TvdbID != 456 {
		t.Errorf("Expected TvdbID 456, got %d", result.TvdbID)
	}

	if result.Name != "Test Episode" {
		t.Errorf("Expected name 'Test Episode', got %q", result.Name)
	}
}

//nolint:paralleltest // depends on shared global state (viper, config.NoCache, BaseURL)
func TestIdentifyEpisodeByTitle(t *testing.T) {
	config.InitDefaults()

	config.NoCache = true

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

	result, _ := IdentifyEpisode(mdb.SearchResult{TvdbID: 999, OriginalLanguage: "en"}, &metadata.Metadata{EpisodeTitle: "Test Episode"}, false)

	if result.TvdbID != 789 {
		t.Errorf("Expected TvdbID 789, got %d", result.TvdbID)
	}
}

//nolint:paralleltest // depends on shared global state (viper, config.NoCache, BaseURL)
func TestIdentifyEpisodeIgnoreSpecialsByDate(t *testing.T) {
	config.InitDefaults()

	config.NoCache = true

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
	result1, _ := IdentifyEpisode(mdb.SearchResult{TvdbID: 999, OriginalLanguage: "en"}, &metadata.Metadata{Date: "2023-01-01"}, false)
	if result1.TvdbID != 200 {
		t.Errorf("Expected TvdbID 200 (Regular), got %d", result1.TvdbID)
	}

	// Case 2: Specials allowed
	result2, _ := IdentifyEpisode(mdb.SearchResult{TvdbID: 999, OriginalLanguage: "en"}, &metadata.Metadata{Date: "2023-01-01"}, true)
	if result2.TvdbID != 100 {
		t.Errorf("Expected TvdbID 100 (Special), got %d", result2.TvdbID)
	}
}
