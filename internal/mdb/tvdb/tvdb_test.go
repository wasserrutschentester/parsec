package tvdb

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"
)

func TestSearch(t *testing.T) {
	// Provide dummy API key
	viper.Set("api_keys.tvdb", "dummy_key")

	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			w.Write([]byte(`{"status": "success", "data": {"token": "dummy_token"}}`))
		} else if r.URL.Path == "/search" {
			resp := tvdbSearchResponse{
				Status: "success",
				Data: []tvdbMedia{
					{TvdbID: "1", Name: "Test Show", Year: "2023", Type: "series"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		} else if r.URL.Path == "/series/1/extended" {
			resp := tvdbExternalIDsResponse{
				Status: "success",
			}
			resp.Data.RemoteIds = []remoteID{{ID: "tt123", SourceName: "IMDB"}}
			json.NewEncoder(w).Encode(resp)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Override BaseURL
	originalBaseURL := BaseURL
	BaseURL = server.URL
	defer func() { BaseURL = originalBaseURL }()

	results, err := Search("tv", "Test", 2023)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if results[0].Title != "Test Show" {
		t.Errorf("Expected title 'Test Show', got %q", results[0].Title)
	}

	if results[0].ImdbID != "tt123" {
		t.Errorf("Expected ImdbID 'tt123', got %q", results[0].ImdbID)
	}
}
