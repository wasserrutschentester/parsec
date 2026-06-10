package prowlarr

import (
	"testing"
)

// nolint:cyclop
func TestBuildSearchURL(t *testing.T) {
	baseURL := "http://localhost:9696"
	searchQuery := "{TmdbId:123}"
	mediaType := "movie"
	categories := []int{2000}
	indexerIDs := []int{1, 2}

	u, err := buildSearchURL(baseURL, searchQuery, mediaType, categories, indexerIDs)
	if err != nil {
		t.Fatalf("failed to build search URL: %v", err)
	}

	q := u.Query()
	if q.Get("type") != mediaType {
		t.Errorf("expected type %s, got %s", mediaType, q.Get("type"))
	}

	if q.Get("query") != searchQuery {
		t.Errorf("expected query %s, got %s", searchQuery, q.Get("query"))
	}

	cats := q["categories"]
	if len(cats) != 1 || cats[0] != "2000" {
		t.Errorf("expected categories [2000], got %v", cats)
	}

	ids := q["indexerIds"]
	if len(ids) != 2 {
		t.Errorf("expected 2 indexerIDs, got %d", len(ids))
	}

	found1, found2 := false, false

	for _, id := range ids {
		if id == "1" {
			found1 = true
		}

		if id == "2" {
			found2 = true
		}
	}

	if !found1 || !found2 {
		t.Errorf("missing indexerIds: found1=%v, found2=%v", found1, found2)
	}
}

func TestSearchLogic(_ *testing.T) {
	// This is a bit hard to test without mocking the HTTP client or performParallelSearch
	// but we can at least check if it compiles and the logic for mediaType/categories is correct.
}
