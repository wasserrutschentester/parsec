package mdb

import (
	"strings"
	"testing"
)

func TestGetResultBodyTitleType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		result      SearchResult
		shouldExist bool
		wantSubstr  string
	}{
		{"standard movie", SearchResult{IsTV: false, TitleType: TitleTypeMovie}, false, ""},
		{"standard TV series", SearchResult{IsTV: true, TitleType: TitleTypeTVSeries}, false, ""},
		{"contradiction: movie classified as TV", SearchResult{IsTV: true, TitleType: TitleTypeMovie}, true, "Movie"},
		{"miniseries", SearchResult{IsTV: true, TitleType: TitleTypeTVMiniSeries}, true, "Mini-Series"},
		{"TV movie", SearchResult{IsTV: false, TitleType: TitleTypeTVMovie}, true, "TV Movie"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := getResultBody(tt.result)

			hasTitleType := strings.Contains(body, "Title Type")
			if hasTitleType != tt.shouldExist {
				t.Fatalf("getResultBody() contains 'Title Type' = %v, want %v; body = %q", hasTitleType, tt.shouldExist, body)
			}

			if tt.shouldExist && !strings.Contains(body, tt.wantSubstr) {
				t.Errorf("getResultBody() does not contain expected substring %q; body = %q", tt.wantSubstr, body)
			}
		})
	}
}
