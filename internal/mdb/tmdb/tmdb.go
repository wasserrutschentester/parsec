// Package tmdb provides a client for the TheMovieDB (TMDB) API.
package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/text/language"

	"codeberg.org/upPollo/parsec/internal/cache"
	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/mdb"
)

var (
	// BaseURL is the TMDB API base URL.
	BaseURL = "https://api.themoviedb.org/3"
	// HTTPClient is the HTTP client used for TMDB requests.
	HTTPClient = http.DefaultClient
)

type tmdbMedia struct {
	ID               int     `json:"id"`
	Title            string  `json:"title"`          // For movies
	Name             string  `json:"name"`           // For TV shows
	OriginalTitle    string  `json:"original_title"` // For movies
	OriginalName     string  `json:"original_name"`  // For TV shows
	OriginalLanguage string  `json:"original_language"`
	ReleaseDate      string  `json:"release_date"`   // For movies
	FirstAirDate     string  `json:"first_air_date"` // For TV shows
	Popularity       float64 `json:"popularity"`
	Overview         string  `json:"overview"`
}

func (m *tmdbMedia) toSearchResult(mediaType string) mdb.SearchResult {
	title := m.Title
	originalTitle := m.OriginalTitle
	date := m.ReleaseDate

	if mediaType == "tv" {
		title = m.Name
		originalTitle = m.OriginalName
		date = m.FirstAirDate
	}

	resYear := 0
	if len(date) >= 4 {
		resYear, _ = strconv.Atoi(date[:4])
	}

	origLang := m.OriginalLanguage
	if origLang == "xx" {
		origLang = "zxx"
	}

	return mdb.SearchResult{
		TmdbID:           m.ID,
		TmdbType:         mediaType,
		Title:            title,
		OriginalTitle:    originalTitle,
		OriginalLanguage: origLang,
		Year:             resYear,
		IsTV:             mediaType == "tv",
		Popularity:       m.Popularity,
		Overview:         m.Overview,
	}
}

type tmdbSearchResponse struct {
	Results []tmdbMedia `json:"results"`
}

type tmdbExternalIDsResponse struct {
	Tvdb int    `json:"tvdb_id"`
	Imdb string `json:"imdb_id"`
}

type tmdbEpisodeResponse struct {
	Name          string `json:"name"`
	AirDate       string `json:"air_date"`
	SeasonNumber  int    `json:"season_number"`
	EpisodeNumber int    `json:"episode_number"`
	Overview      string `json:"overview"`
}

func getFromCache(key string, target any) (bool, error) {
	cached, err := cache.Get(key)
	if err != nil {
		return false, nil
	}

	if err := json.Unmarshal(cached, target); err != nil {
		return true, fmt.Errorf("failed to unmarshal cached TMDB response: %w", err)
	}

	return true, nil
}

var (
	errNotConfigured = errors.New("TMDB API key not configured")
	errTMDBStatus    = errors.New("TMDB API returned status")
)

func get(endpoint string, query url.Values, target any) error {
	apiKey := config.GetTmdbAPIKey()
	if apiKey == "" {
		return errNotConfigured
	}

	if query == nil {
		query = url.Values{}
	}

	if query.Get("language") == "" {
		query.Set("language", config.GetPreferredLanguage())
	}

	cacheKey := fmt.Sprintf("tmdb:%s?%s", endpoint, query.Encode())
	if ok, err := getFromCache(cacheKey, target); ok {
		return err
	}

	query.Set("api_key", apiKey)
	u := fmt.Sprintf("%s/%s?%s", BaseURL, endpoint, query.Encode())

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("failed to create TMDB request: %w", err)
	}

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("TMDB request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w %d", errTMDBStatus, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read TMDB response body: %w", err)
	}

	_ = cache.Set(cacheKey, body)

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("failed to unmarshal TMDB response: %w", err)
	}

	return nil
}

// Search searches for media on TMDB by query and optionally by year.
func Search(mediaType, query string, year int) ([]mdb.SearchResult, error) {
	params := url.Values{}
	params.Add("query", query)

	if year > 0 {
		if mediaType == "movie" {
			params.Add("primary_release_year", strconv.Itoa(year))
		} else {
			params.Add("first_air_date_year", strconv.Itoa(year))
		}
	}

	var data tmdbSearchResponse
	if err := get("search/"+mediaType, params, &data); err != nil {
		return nil, err
	}

	results := make([]mdb.SearchResult, 0, len(data.Results))
	for _, r := range data.Results {
		result := r.toSearchResult(mediaType)
		applyExternalIDs(&result, mediaType)
		applyAltTitles(&result, mediaType)
		results = append(results, result)
	}

	return results, nil
}

func applyExternalIDs(result *mdb.SearchResult, mediaType string) {
	externalIDs, err := getExternalIDs(result.TmdbID, mediaType)
	if err != nil {
		return
	}

	if externalIDs.Imdb != "" {
		result.ImdbID = externalIDs.Imdb
	}

	if externalIDs.Tvdb == 0 {
		return
	}

	result.TvdbID = externalIDs.Tvdb

	result.TvdbType = "movies"
	if result.IsTV {
		result.TvdbType = "series"
	}
}

func applyAltTitles(result *mdb.SearchResult, mediaType string) {
	altTitles, err := getAlternativeTitles(result.TmdbID, mediaType, result.OriginalLanguage)
	if err == nil {
		result.AltTitle = altTitles
	}
}

// GetByID retrieves a single media item from TMDB by its ID.
func GetByID(tmdbID int, mediaType string) (*mdb.SearchResult, error) {
	var r tmdbMedia
	if err := get(fmt.Sprintf("%s/%d", mediaType, tmdbID), nil, &r); err != nil {
		return nil, err
	}

	result := r.toSearchResult(mediaType)
	applyExternalIDs(&result, mediaType)
	applyAltTitles(&result, mediaType)

	return &result, nil
}

// GetByImdbID retrieves a media item from TMDB using its IMDB ID.
func GetByImdbID(imdbID string, isTV bool) (*mdb.SearchResult, error) {
	params := url.Values{}
	params.Set("external_source", "imdb_id")

	var data struct {
		MovieResults []tmdbMedia `json:"movie_results"`
		TVResults    []tmdbMedia `json:"tv_results"`
	}

	if err := get("find/"+imdbID, params, &data); err != nil {
		return nil, err
	}

	if isTV {
		if len(data.TVResults) > 0 {
			return finalizeImdbResult(data.TVResults[0], "tv", imdbID), nil
		}
	} else {
		if len(data.MovieResults) > 0 {
			return finalizeImdbResult(data.MovieResults[0], "movie", imdbID), nil
		}
	}

	// No strict match found.
	return nil, mdb.ErrNotFound
}

func finalizeImdbResult(m tmdbMedia, mediaType, imdbID string) *mdb.SearchResult {
	result := m.toSearchResult(mediaType)
	result.ImdbID = imdbID
	applyExternalIDs(&result, mediaType)
	applyAltTitles(&result, mediaType)

	if result.IsTV {
		result.TvdbType = "series"
	} else {
		result.TvdbType = "movies"
	}

	return &result
}

func getExternalIDs(tmdbID int, mediaType string) (tmdbExternalIDsResponse, error) {
	var data tmdbExternalIDsResponse
	if err := get(fmt.Sprintf("%s/%d/external_ids", mediaType, tmdbID), nil, &data); err != nil {
		return tmdbExternalIDsResponse{}, err
	}

	return data, nil
}

func getAlternativeTitles(tmdbID int, mediaType, originalLanguage string) ([]string, error) {
	var data struct {
		Titles []struct {
			Title string `json:"title"`
			ISO   string `json:"iso_3166_1"`
		} `json:"titles"` // Movies
		Results []struct {
			Title string `json:"title"`
			ISO   string `json:"iso_3166_1"`
		} `json:"results"` // TV
	}

	if err := get(fmt.Sprintf("%s/%d/alternative_titles", mediaType, tmdbID), nil, &data); err != nil {
		return nil, err
	}

	prefTag := language.Make(config.GetPreferredLanguage())
	origTag := language.Make(originalLanguage)

	// Determine country code for original language
	origCountry, _ := origTag.Region()
	origCountryStr := origCountry.String()

	titles := []string{}
	process := func(title, iso string) {
		iso = strings.ToUpper(iso)
		isPreferred := iso == strings.ToUpper(prefTag.String())
		isEnglish := iso == "US" || iso == "GB" || iso == "CA" || iso == "AU"
		isOriginal := iso == origCountryStr

		if isPreferred || isEnglish || isOriginal {
			titles = append(titles, title)
		}
	}

	for _, t := range data.Titles {
		process(t.Title, t.ISO)
	}

	for _, t := range data.Results {
		process(t.Title, t.ISO)
	}

	return titles, nil
}

// GetEpisodeMetadata retrieves detailed metadata for a specific TV episode from TMDB.
func GetEpisodeMetadata(seriesID, season, episode int, lang string) (mdb.EpisodeResult, error) {
	var data tmdbEpisodeResponse

	params := url.Values{}
	if lang != "" {
		params.Set("language", lang)
	}

	endpoint := fmt.Sprintf("tv/%d/season/%d/episode/%d", seriesID, season, episode)
	if err := get(endpoint, params, &data); err != nil {
		return mdb.EpisodeResult{}, err
	}

	return mdb.EpisodeResult{
		Name:     data.Name,
		Airdate:  data.AirDate,
		Overview: data.Overview,
		Season:   season,
		Episode:  episode,
	}, nil
}
