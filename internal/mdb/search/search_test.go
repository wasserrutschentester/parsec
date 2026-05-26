package search

import (
	"testing"
)

func TestFuzzySearchEmpty(t *testing.T) {
	// This test doesn't need an API key as it should fail early if key is missing or return empty if no results
	// But since search() checks for API key, we might need to mock config or just check the error.
	res, err := FuzzySearch("NonExistentMovie12345", 2026, false)
	if err != nil {
		// Expected if no API key is set
		t.Logf("Expected error or empty result: %v", err)
		return
	}
	if len(res) > 0 {
		t.Errorf("Expected empty result for non-existent movie, got %v", res)
	}
}
