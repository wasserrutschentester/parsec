package imdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/text/language"

	"codeberg.org/upPollo/parsec/internal/cache"
	"codeberg.org/upPollo/parsec/internal/mdb"
)

const (
	defaultBaseURL         = "https://caching.graphql.imdb.com/"
	imdbOrigin             = "https://www.imdb.com"
	imdbClientName         = "imdb-web-next-localized"
	imdbUserCountry        = "US"
	maxGraphQLResponseSize = 32 << 20
)

var (
	// BaseURL is the IMDb GraphQL gateway endpoint.
	BaseURL = defaultBaseURL
	// HTTPClient is the HTTP client used for IMDb requests.
	HTTPClient = &http.Client{Timeout: 15 * time.Second}

	// ErrNotFound is returned when no matching title or episode is found.
	ErrNotFound = errors.New("imdb: title not found")
	// ErrHTTPStatus is returned when IMDb returns a non-2xx status code.
	ErrHTTPStatus = errors.New("imdb: unexpected http status")
	// ErrGraphQLError is returned when the GraphQL response contains errors.
	ErrGraphQLError = errors.New("imdb: graphql error")
)

func getCachedGraphQL(operationName string, variables map[string]any, target any) (string, bool) {
	var varBytes []byte
	if len(variables) > 0 {
		varBytes, _ = json.Marshal(variables)
	}

	cacheKey := fmt.Sprintf("imdb:%s:%s", operationName, string(varBytes))
	if cachedData, err := cache.Get(cacheKey); err == nil && len(cachedData) > 0 {
		if err := json.Unmarshal(cachedData, target); err == nil {
			return cacheKey, true
		}
	}

	return cacheKey, false
}

func createGraphQLRequest(ctx context.Context, data []byte) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, BaseURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("imdb: create request: %w", err)
	}

	req.Header.Set("Accept", "application/graphql+json, application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", imdbOrigin)
	req.Header.Set("X-Imdb-Client-Name", imdbClientName)
	req.Header.Set("X-Imdb-User-Country", imdbUserCountry)

	return req, nil
}

func readGraphQLResponse(resp *http.Response) ([]byte, error) {
	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxGraphQLResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("imdb: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w %d: %s", ErrHTTPStatus, resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	return respBytes, nil
}

func postGraphQL(ctx context.Context, payload graphQLRequest) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("imdb: marshal request: %w", err)
	}

	for attempt := range 3 {
		req, err := createGraphQLRequest(ctx, data)
		if err != nil {
			return nil, err
		}

		resp, err := HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("imdb: execute request: %w", err)
		}

		if (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusAccepted) && attempt < 2 {
			_ = resp.Body.Close()

			if retryErr := mdb.WaitRetry(ctx, resp); retryErr != nil {
				return nil, retryErr
			}

			continue
		}

		defer func() { _ = resp.Body.Close() }()

		return readGraphQLResponse(resp)
	}

	return nil, fmt.Errorf("%w: max retries reached", ErrHTTPStatus)
}

func executeGraphQL(ctx context.Context, operationName, query string, variables map[string]any, target any) error {
	cacheKey, found := getCachedGraphQL(operationName, variables, target)
	if found {
		return nil
	}

	payload := graphQLRequest{
		OperationName: operationName,
		Variables:     variables,
		Query:         query,
	}

	respBytes, err := postGraphQL(ctx, payload)
	if err != nil {
		return err
	}

	if err := checkGraphQLError(operationName, respBytes); err != nil {
		return err
	}

	if err := json.Unmarshal(respBytes, target); err != nil {
		return fmt.Errorf("imdb: decode response: %w", err)
	}

	_ = cache.Set(cacheKey, respBytes)

	return nil
}

func checkGraphQLError(operationName string, body []byte) error {
	var env graphQLErrorEnvelope
	if err := json.Unmarshal(body, &env); err != nil || len(env.Errors) == 0 {
		return nil
	}

	first := env.Errors[0]

	return fmt.Errorf("%w in %s: %s (code: %s)", ErrGraphQLError, operationName, first.Message, first.Extensions.Code)
}

var englishLanguageNames = map[string]string{
	"english":    "en",
	"korean":     "ko",
	"japanese":   "ja",
	"spanish":    "es",
	"french":     "fr",
	"german":     "de",
	"italian":    "it",
	"chinese":    "zh",
	"mandarin":   "zh",
	"cantonese":  "yue",
	"russian":    "ru",
	"portuguese": "pt",
	"dutch":      "nl",
	"arabic":     "ar",
	"hindi":      "hi",
	"swedish":    "sv",
	"danish":     "da",
	"norwegian":  "no",
	"finnish":    "fi",
	"polish":     "pl",
	"turkish":    "tr",
	"thai":       "th",
	"vietnamese": "vi",
	"hebrew":     "he",
	"greek":      "el",
	"czech":      "cs",
	"hungarian":  "hu",
	"romanian":   "ro",
	"indonesian": "id",
	"ukrainian":  "uk",
}

// NormalizeLanguage converts an IMDb language ID or name to a normalized ISO-639-1 code.
func NormalizeLanguage(id, text string) string {
	id = strings.TrimSpace(strings.ToLower(id))
	if id != "" {
		tag, err := language.Parse(id)
		if err == nil && !tag.IsRoot() {
			base, _ := tag.Base()

			return base.String()
		}

		if len(id) == 2 || len(id) == 3 {
			return id
		}
	}

	text = strings.TrimSpace(strings.ToLower(text))
	if iso, ok := englishLanguageNames[text]; ok {
		return iso
	}

	if text != "" {
		tag, err := language.Parse(text)
		if err == nil && !tag.IsRoot() {
			base, _ := tag.Base()

			return base.String()
		}
	}

	return id
}

// FormatIMDbID canonicalizes an IMDb ID to "tt" followed by at least 7 digits.
func FormatIMDbID(id string) string {
	cleaned := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(id)), "tt")
	if cleaned == "" {
		return ""
	}

	for len(cleaned) < 7 {
		cleaned = "0" + cleaned
	}

	return "tt" + cleaned
}
