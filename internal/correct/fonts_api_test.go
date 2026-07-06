package correct

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

// roundTripFunc implements http.RoundTripper
type roundTripFunc func(req *http.Request) *http.Response

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func newMockClient(fn roundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

//nolint:paralleltest // mutates global state
func TestResolveGoogleFontsGitHub(t *testing.T) {
	config.NoCache = true
	defer func() { config.NoCache = false }()

	tempCache := t.TempDir()
	_ = os.Setenv("XDG_CACHE_HOME", tempCache)

	defer func() { _ = os.Unsetenv("XDG_CACHE_HOME") }()

	// Override checkFontFileMatches to always match the requested font
	checkFontFileMatches = func(_, fontName string) ([]string, bool) {
		return []string{fontName}, true
	}
	defer func() { checkFontFileMatches = fontFileMatches }()

	checkGetFontNames = func(_ []byte) ([]string, error) {
		return []string{"Open Sans"}, nil
	}
	defer func() { checkGetFontNames = matroska.GetFontNames }()

	client := newMockClient(func(req *http.Request) *http.Response {
		// Log requests for debugging
		t.Logf("GitHub API mock got request: %s %s", req.Method, req.URL.String())

		if strings.Contains(req.URL.Path, "ofl/opensans") {
			contents := []githubContent{
				{Name: "OpenSans-Regular.ttf", Type: "file", DownloadURL: "https://raw.githubusercontent.com/mock/opensans.ttf"},
			}
			b, _ := json.Marshal(contents)

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(b)),
			}
		}

		if req.URL.String() == "https://raw.githubusercontent.com/mock/opensans.ttf" {
			// Provide dummy TTF data
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("dummy ttf data"))),
			}
		}

		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(bytes.NewReader([]byte("not found"))),
		}
	})

	resolver := defaultFontResolver{client: client}
	resolved, ok := resolver.resolveGoogleFontsGitHub("Open Sans")

	if !ok {
		t.Fatalf("expected Open Sans to be resolved")
	}

	if resolved.Source != fontSourceGoogleGitHub {
		t.Errorf("expected source google-fonts-github, got %v", resolved.Source)
	}

	if filepath.Ext(resolved.Path) != ".ttf" {
		t.Errorf("expected .ttf extension, got %v", resolved.Path)
	}
}

//nolint:funlen,paralleltest // mutates global state
func TestResolveGoogleFontsAPI(t *testing.T) {
	config.NoCache = true
	defer func() { config.NoCache = false }()

	tempCache := t.TempDir()
	_ = os.Setenv("XDG_CACHE_HOME", tempCache)

	defer func() { _ = os.Unsetenv("XDG_CACHE_HOME") }()

	checkFontFileMatches = func(_, fontName string) ([]string, bool) {
		return []string{fontName}, true
	}
	defer func() { checkFontFileMatches = fontFileMatches }()

	checkGetFontNames = func(_ []byte) ([]string, error) {
		return []string{"Roboto"}, nil
	}
	defer func() { checkGetFontNames = matroska.GetFontNames }()

	client := newMockClient(func(req *http.Request) *http.Response {
		t.Logf("Google API mock got request: %s %s", req.Method, req.URL.String())

		if req.URL.Host == "www.googleapis.com" {
			resp := googleFontsAPIResponse{
				Items: []googleFontFamily{
					{
						Family: "Roboto",
						Files: map[string]string{
							"regular": "http://fonts.gstatic.com/mock/roboto.ttf",
						},
					},
				},
			}
			b, _ := json.Marshal(resp)

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(b)),
			}
		}

		if req.URL.Host == "fonts.gstatic.com" {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("dummy ttf data"))),
			}
		}

		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(bytes.NewReader([]byte("not found"))),
		}
	})

	resolver := defaultFontResolver{client: client}
	resolved, ok := resolver.resolveGoogleFontsAPI("Roboto", "dummy-key")

	if !ok {
		t.Fatalf("expected Roboto to be resolved")
	}

	if resolved.Source != fontSourceGoogleAPI {
		t.Errorf("expected source google-fonts-api, got %v", resolved.Source)
	}
}
