package tmdb

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"

	"codeberg.org/upPollo/parsec/internal/config"
)

func TestSearch(t *testing.T) {
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

	results, err := Search("movie", "Test", 2023)
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

func TestGetByIDLocalization(t *testing.T) {
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

	result, err := GetByID(123, "tv")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if result.Title != "German Title" {
		t.Errorf("Expected title 'German Title', got %q", result.Title)
	}
}
