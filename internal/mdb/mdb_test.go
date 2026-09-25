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

func TestShouldDisplaySpokenLanguages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		spoken   []string
		origLang string
		want     bool
	}{
		{"monolingual matching", []string{"en"}, "en", false},
		{"monolingual differing silent", []string{"zxx"}, "en", true},
		{"monolingual differing language", []string{"fr"}, "en", true},
		{"two languages with zxx", []string{"de", "zxx"}, "de", true},
		{"two languages with zxx first", []string{"zxx", "en"}, "en", true},
		{"two languages neither matching orig", []string{"fr", "de"}, "en", true},
		{"two regular languages with orig", []string{"en", "es"}, "en", true},
		{"more than two languages", []string{"en", "de", "fr"}, "en", true},
		{"more than two languages 4 langs", []string{"en", "de", "fr", "it"}, "en", true},
		{"empty spoken", []string{}, "en", false},
		{"pure silent same orig", []string{"zxx"}, "zxx", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := shouldDisplaySpokenLanguages(tt.spoken, tt.origLang)
			if got != tt.want {
				t.Errorf("shouldDisplaySpokenLanguages(%v, %q) = %v, want %v", tt.spoken, tt.origLang, got, tt.want)
			}
		})
	}
}

func TestGetResultBodySpokenLanguages(t *testing.T) {
	t.Parallel()

	// Inglourious Basterds: > 2 languages
	body1 := getResultBody(SearchResult{
		Title:            "Inglourious Basterds",
		OriginalLanguage: "en",
		SpokenLanguages:  []string{"en", "de", "fr", "it"},
	})
	if !strings.Contains(body1, "Spoken Langs") || !strings.Contains(body1, "en, de, fr, it") {
		t.Errorf("expected Spoken Langs for multilingual movie, got %q", body1)
	}

	// Metropolis: silent with de + zxx
	body2 := getResultBody(SearchResult{
		Title:            "Metropolis",
		OriginalLanguage: "de",
		SpokenLanguages:  []string{"de", "zxx"},
	})
	if !strings.Contains(body2, "Spoken Langs") || !strings.Contains(body2, "de, zxx (No Dialogue)") {
		t.Errorf("expected Spoken Langs for silent film, got %q", body2)
	}

	// Standard English movie: 1 language -> no Spoken Langs
	body3 := getResultBody(SearchResult{
		Title:            "Finding Nemo",
		OriginalLanguage: "en",
		SpokenLanguages:  []string{"en"},
	})
	if strings.Contains(body3, "Spoken Langs") {
		t.Errorf("did not expect Spoken Langs for monolingual movie, got %q", body3)
	}
}
