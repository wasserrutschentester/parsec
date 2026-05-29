package tvdb

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/n0ne/parsec/internal/config"
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

func TestAcceptLanguageHeader(t *testing.T) {
	config.InitDefaults()
	viper.Set("api_keys.tvdb", "dummy_key")
	viper.Set("preferred_language", "de")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			w.Write([]byte(`{"status": "success", "data": {"token": "dummy_token"}}`))
			return
		}

		lang := r.Header.Get("Accept-Language")
		if lang != "deu" {
			t.Errorf("Expected Accept-Language 'deu', got %q", lang)
		}

		w.Write([]byte(`{"status": "success", "data": {}}`))
	}))
	defer server.Close()

	originalBaseURL := BaseURL
	BaseURL = server.URL
	defer func() { BaseURL = originalBaseURL }()

	var dummy interface{}
	get("dummy", &dummy)
}

func TestTranslatedFields(t *testing.T) {
	m := tvdbMedia{
		Name:               "Original",
		NameTranslated:     "Translated",
		Overview:           "Original Overview",
		OverviewTranslated: []string{"Translated Overview"},
	}
	res := m.toSearchResult()
	if res.Title != "Translated" {
		t.Errorf("Expected title 'Translated', got %q", res.Title)
	}
	if res.Overview != "Translated Overview" {
		t.Errorf("Expected overview 'Translated Overview', got %q", res.Overview)
	}
}

func TestGetByIDWithTranslation(t *testing.T) {
	config.InitDefaults()
	viper.Set("api_keys.tvdb", "dummy_key")
	viper.Set("preferred_language", "de")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			w.Write([]byte(`{"status": "success", "data": {"token": "dummy_token"}}`))
		} else if r.URL.Path == "/series/123" {
			w.Write([]byte(`{"status": "success", "data": {"id": 123, "name": "Danish Title", "overview": "Danish Overview", "type": "series"}}`))
		} else if r.URL.Path == "/series/123/translations/deu" {
			w.Write([]byte(`{"status": "success", "data": {"name": "German Title", "overview": "German Overview"}}`))
		} else if r.URL.Path == "/series/123/extended" {
			w.Write([]byte(`{"status": "success", "data": {"originalLanguage": "dan"}}`))
		} else {
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
	if result.Overview != "German Overview" {
		t.Errorf("Expected overview 'German Overview', got %q", result.Overview)
	}
}

func TestGetEpisodeMetadataWithTranslation(t *testing.T) {
	config.InitDefaults()
	viper.Set("api_keys.tvdb", "dummy_key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			w.Write([]byte(`{"status": "success", "data": {"token": "dummy_token"}}`))
		} else if r.URL.Path == "/series/123/episodes/default/de" {
			w.Write([]byte(`{"status": "success", "data": {"episodes": [{"id": 456, "number": 1, "seasonNumber": 1, "name": "Danish Ep", "overview": "Danish Overview"}]}}`))
		} else if r.URL.Path == "/episodes/456/translations/deu" {
			w.Write([]byte(`{"status": "success", "data": {"name": "German Ep", "overview": "German Overview"}}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	originalBaseURL := BaseURL
	BaseURL = server.URL
	defer func() { BaseURL = originalBaseURL }()

	result, err := GetEpisodeMetadata(123, 1, 1, "de")
	if err != nil {
		t.Fatalf("GetEpisodeMetadata failed: %v", err)
	}

	if result.Name != "German Ep" {
		t.Errorf("Expected name 'German Ep', got %q", result.Name)
	}
	if result.Overview != "German Overview" {
		t.Errorf("Expected overview 'German Overview', got %q", result.Overview)
	}
}
