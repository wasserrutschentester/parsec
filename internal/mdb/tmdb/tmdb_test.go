package tmdb

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"
)

func TestSearch(t *testing.T) {
	// Provide dummy API key
	viper.Set("api_keys.tmdb", "dummy_key")

	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock search response
		if r.URL.Path == "/search/movie" {
			resp := tmdbSearchResponse{
				Results: []tmdbMedia{
					{ID: 1, Title: "Test Movie", ReleaseDate: "2023-01-01"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		} else if r.URL.Path == "/movie/1/external_ids" {
			resp := tmdbExternalIDsResponse{Imdb: "tt123"}
			json.NewEncoder(w).Encode(resp)
		} else if r.URL.Path == "/movie/1/alternative_titles" {
			// Return empty for simplicity
			w.Write([]byte(`{"titles": []}`))
		} else {
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
