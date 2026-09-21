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
	"sync"

	"golang.org/x/text/language"

	"codeberg.org/upPollo/parsec/internal/cache"
	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/mdb"
	"codeberg.org/upPollo/parsec/internal/metadata"
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
	Genres           []struct {
		Name string `json:"name"`
	} `json:"genres"`
	Tagline        string `json:"tagline"`
	Status         string `json:"status"`
	Runtime        int    `json:"runtime"`
	EpisodeRunTime []int  `json:"episode_run_time"`

	ProductionCompanies []struct {
		Name string `json:"name"`
	} `json:"production_companies"`

	Networks []struct {
		Name string `json:"name"`
	} `json:"networks"` // Specific to TV shows

	ProductionCountries []struct {
		Iso31661 string `json:"iso_3166_1"`
	} `json:"production_countries"` // Specific to Movies

	OriginCountry     []string                `json:"origin_country"` // Specific to TV shows
	ExternalIDs       tmdbExternalIDsResponse `json:"external_ids"`
	AlternativeTitles struct {
		Titles []struct {
			Title string `json:"title"`
			ISO   string `json:"iso_3166_1"`
		} `json:"titles"` // Movies
		Results []struct {
			Title string `json:"title"`
			ISO   string `json:"iso_3166_1"`
		} `json:"results"` // TV
	} `json:"alternative_titles"`
}

func (m *tmdbMedia) toSearchResult(mediaType string) mdb.SearchResult {
	title := strings.TrimSpace(m.Title)
	originalTitle := strings.TrimSpace(m.OriginalTitle)
	date := m.ReleaseDate

	if mediaType == "tv" {
		title = strings.TrimSpace(m.Name)
		originalTitle = strings.TrimSpace(m.OriginalName)
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

	genres := extractGenres(m.Genres)
	studios := extractNames(m.ProductionCompanies)
	networks := extractNames(m.Networks)
	countries := extractCountries(m.ProductionCountries, m.OriginCountry)

	runtime := m.Runtime
	if mediaType == "tv" && len(m.EpisodeRunTime) > 0 {
		runtime = m.EpisodeRunTime[0]
	}

	return mdb.SearchResult{
		TmdbID:           m.ID,
		TmdbType:         mediaType,
		Title:            title,
		OriginalTitle:    originalTitle,
		OriginalLanguage: origLang,
		Year:             resYear,
		Runtime:          runtime,
		IsTV:             mediaType == "tv",
		Popularity:       m.Popularity,
		Overview:         m.Overview,
		Genres:           genres,
		Studios:          studios,
		Networks:         networks,
		Countries:        countries,
		Tagline:          m.Tagline,
		Status:           mdb.NormalizeStatus(m.Status),
	}
}

// extractGenres extracts genre names.
func extractGenres(input []struct {
	Name string `json:"name"`
},
) []string {
	genres := make([]string, 0, len(input))
	for _, g := range input {
		if g.Name != "" {
			genres = append(genres, g.Name)
		}
	}

	return genres
}

// extractNames extracts names from a list of structs.
func extractNames(input []struct {
	Name string `json:"name"`
},
) []string {
	names := make([]string, 0, len(input))
	for _, item := range input {
		names = append(names, item.Name)
	}

	return names
}

// extractCountries extracts and normalizes country codes.
func extractCountries(prodCountries []struct {
	Iso31661 string `json:"iso_3166_1"`
}, originCountries []string,
) []string {
	countries := make([]string, 0, len(prodCountries)+len(originCountries))
	for _, country := range prodCountries {
		countries = append(countries, strings.ToUpper(country.Iso31661))
	}

	for _, oc := range originCountries {
		countries = append(countries, strings.ToUpper(oc))
	}

	return metadata.RemoveDuplicates(countries)
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
	EpisodeType   string `json:"episode_type"`
	Runtime       int    `json:"runtime"`
	ExternalIDs   struct {
		ImdbID string `json:"imdb_id"`
	} `json:"external_ids"`
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

type tmdbErrorResponse struct {
	StatusCode    int    `json:"status_code"`
	StatusMessage string `json:"status_message"`
	Success       bool   `json:"success"`
}

func parseErrorResponse(body []byte, statusCode int) error {
	var errResp tmdbErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.StatusMessage != "" {
		return fmt.Errorf("%w %d: %s", errTMDBStatus, statusCode, errResp.StatusMessage)
	}

	return fmt.Errorf("%w %d", errTMDBStatus, statusCode)
}

func executeRequest(ctx context.Context, u, apiKey string, isBearer bool) (*http.Response, error) {
	for attempt := range 3 {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create TMDB request: %w", err)
		}

		if isBearer {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}

		resp, err := HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("TMDB request failed: %w", err)
		}

		if (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusAccepted) && attempt < 2 {
			_ = resp.Body.Close()

			if retryErr := mdb.WaitRetry(ctx, resp); retryErr != nil {
				return nil, retryErr
			}

			continue
		}

		return resp, nil
	}

	return nil, errTMDBStatus
}

func prepareQuery(apiKey string, query url.Values) (url.Values, string, bool) {
	if query == nil {
		query = url.Values{}
	}

	if query.Get("language") == "" {
		query.Set("language", config.GetPreferredLanguage())
	}

	cacheQuery := query.Encode()
	isBearer := strings.HasPrefix(apiKey, "ey") || len(apiKey) > 40

	if !isBearer {
		query.Set("api_key", apiKey)
	}

	return query, cacheQuery, isBearer
}

func get(ctx context.Context, endpoint string, query url.Values, target any) error {
	apiKey := config.GetTmdbAPIKey()
	if apiKey == "" {
		return errNotConfigured
	}

	query, cacheQuery, isBearer := prepareQuery(apiKey, query)
	cacheKey := fmt.Sprintf("tmdb:%s?%s", endpoint, cacheQuery)

	if ok, err := getFromCache(cacheKey, target); ok {
		return err
	}

	u := fmt.Sprintf("%s/%s?%s", BaseURL, endpoint, query.Encode())

	resp, err := executeRequest(ctx, u, apiKey, isBearer)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read TMDB response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return parseErrorResponse(body, resp.StatusCode)
	}

	_ = cache.Set(cacheKey, body)

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("failed to unmarshal TMDB response: %w", err)
	}

	return nil
}

// Search searches for media on TMDB by query and optionally by year.
func Search(ctx context.Context, mediaType, query string, year int) ([]mdb.SearchResult, error) {
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
	if err := get(ctx, "search/"+mediaType, params, &data); err != nil {
		return nil, err
	}

	results := make([]mdb.SearchResult, len(data.Results))
	for i, r := range data.Results {
		results[i] = r.toSearchResult(mediaType)
	}

	// Fetch external IDs concurrently for top search results to allow cross-matching with TVDB.
	maxLookups := min(len(results), 8)
	if maxLookups > 0 {
		var wg sync.WaitGroup

		sem := make(chan struct{}, 4)

		for i := range maxLookups {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					return
				}

				applyExternalIDs(ctx, &results[idx], mediaType)
			}(i)
		}

		wg.Wait()
	}

	return results, nil
}

func applyExternalIDsFromResponse(result *mdb.SearchResult, externalIDs tmdbExternalIDsResponse) {
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

func applyExternalIDs(ctx context.Context, result *mdb.SearchResult, mediaType string) {
	externalIDs, err := getExternalIDs(ctx, result.TmdbID, mediaType)
	if err != nil {
		return
	}

	applyExternalIDsFromResponse(result, externalIDs)
}

func parseAlternativeTitles(
	titles []struct {
		Title string `json:"title"`
		ISO   string `json:"iso_3166_1"`
	},
	results []struct {
		Title string `json:"title"`
		ISO   string `json:"iso_3166_1"`
	},
	originalLanguage string,
) []string {
	prefTag := language.Make(config.GetPreferredLanguage())
	origTag := language.Make(originalLanguage)

	origCountry, _ := origTag.Region()
	origCountryStr := origCountry.String()

	altTitles := make([]string, 0, len(titles)+len(results))
	process := func(title, iso string) {
		iso = strings.ToUpper(iso)
		isPreferred := iso == strings.ToUpper(prefTag.String())
		isEnglish := iso == "US" || iso == "GB" || iso == "CA" || iso == "AU"
		isOriginal := iso == origCountryStr

		if isPreferred || isEnglish || isOriginal {
			altTitles = append(altTitles, title)
		}
	}

	for _, t := range titles {
		process(t.Title, t.ISO)
	}

	for _, t := range results {
		process(t.Title, t.ISO)
	}

	return altTitles
}

func applyAltTitles(ctx context.Context, result *mdb.SearchResult, mediaType string) {
	altTitles, err := getAlternativeTitles(ctx, result.TmdbID, mediaType, result.OriginalLanguage)
	if err == nil {
		result.AltTitle = altTitles
	}
}

// GetByID retrieves a single media item from TMDB by its ID.
func GetByID(ctx context.Context, tmdbID int, mediaType string) (*mdb.SearchResult, error) {
	params := url.Values{}
	params.Set("append_to_response", "external_ids,alternative_titles")

	var r tmdbMedia
	if err := get(ctx, fmt.Sprintf("%s/%d", mediaType, tmdbID), params, &r); err != nil {
		return nil, err
	}

	result := r.toSearchResult(mediaType)

	// If external IDs were provided in appended response, use them directly.
	if r.ExternalIDs.Imdb != "" || r.ExternalIDs.Tvdb != 0 {
		applyExternalIDsFromResponse(&result, r.ExternalIDs)
	} else {
		applyExternalIDs(ctx, &result, mediaType)
	}

	// If alternative titles were provided in appended response, parse them directly.
	if len(r.AlternativeTitles.Titles) > 0 || len(r.AlternativeTitles.Results) > 0 {
		result.AltTitle = parseAlternativeTitles(r.AlternativeTitles.Titles, r.AlternativeTitles.Results, result.OriginalLanguage)
	} else {
		applyAltTitles(ctx, &result, mediaType)
	}

	return &result, nil
}

// GetByImdbID retrieves a media item from TMDB using its IMDB ID.
func GetByImdbID(ctx context.Context, imdbID string, isTV bool) (*mdb.SearchResult, error) {
	params := url.Values{}
	params.Set("external_source", "imdb_id")

	var data struct {
		MovieResults []tmdbMedia `json:"movie_results"`
		TVResults    []tmdbMedia `json:"tv_results"`
	}

	if err := get(ctx, "find/"+imdbID, params, &data); err != nil {
		return nil, err
	}

	if isTV {
		if len(data.TVResults) > 0 {
			return finalizeImdbResult(ctx, data.TVResults[0], "tv", imdbID), nil
		}
	} else {
		if len(data.MovieResults) > 0 {
			return finalizeImdbResult(ctx, data.MovieResults[0], "movie", imdbID), nil
		}
	}

	// No strict match found.
	return nil, mdb.ErrNotFound
}

func finalizeImdbResult(ctx context.Context, m tmdbMedia, mediaType, imdbID string) *mdb.SearchResult {
	// /find/ returns a slim search result — fetch full details for overview, runtime, studios, correct original_language, etc.
	if full, err := GetByID(ctx, m.ID, mediaType); err == nil {
		full.ImdbID = imdbID

		return full
	}

	result := m.toSearchResult(mediaType)
	result.ImdbID = imdbID
	applyExternalIDs(ctx, &result, mediaType)
	applyAltTitles(ctx, &result, mediaType)

	if result.IsTV {
		result.TvdbType = "series"
	} else {
		result.TvdbType = "movies"
	}

	return &result
}

func getExternalIDs(ctx context.Context, tmdbID int, mediaType string) (tmdbExternalIDsResponse, error) {
	var data tmdbExternalIDsResponse
	if err := get(ctx, fmt.Sprintf("%s/%d/external_ids", mediaType, tmdbID), nil, &data); err != nil {
		return tmdbExternalIDsResponse{}, err
	}

	return data, nil
}

func getAlternativeTitles(ctx context.Context, tmdbID int, mediaType, originalLanguage string) ([]string, error) {
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

	if err := get(ctx, fmt.Sprintf("%s/%d/alternative_titles", mediaType, tmdbID), nil, &data); err != nil {
		return nil, err
	}

	return parseAlternativeTitles(data.Titles, data.Results, originalLanguage), nil
}

// GetEpisodeMetadata retrieves detailed metadata for a specific TV episode from TMDB.
func GetEpisodeMetadata(ctx context.Context, seriesID, season, episode int, lang string) (mdb.EpisodeResult, error) {
	var data tmdbEpisodeResponse

	params := url.Values{}
	params.Set("append_to_response", "external_ids")

	if lang != "" {
		params.Set("language", lang)
	}

	endpoint := fmt.Sprintf("tv/%d/season/%d/episode/%d", seriesID, season, episode)
	if err := get(ctx, endpoint, params, &data); err != nil {
		return mdb.EpisodeResult{}, err
	}

	return mdb.EpisodeResult{
		Name:     data.Name,
		Airdate:  data.AirDate,
		Overview: data.Overview,
		Season:   season,
		Episode:  episode,
		Runtime:  data.Runtime,
		ImdbID:   data.ExternalIDs.ImdbID,
		IsFinale: data.EpisodeType == "finale" || data.EpisodeType == "series_finale",
	}, nil
}

// GetSeasonMetadata retrieves metadata for all episodes in a specific season from TMDB.
func GetSeasonMetadata(ctx context.Context, seriesID, season int, lang string) ([]mdb.EpisodeResult, error) {
	var data struct {
		Episodes []tmdbEpisodeResponse `json:"episodes"`
	}

	params := url.Values{}
	if lang != "" {
		params.Set("language", lang)
	}

	endpoint := fmt.Sprintf("tv/%d/season/%d", seriesID, season)
	if err := get(ctx, endpoint, params, &data); err != nil {
		return nil, err
	}

	results := make([]mdb.EpisodeResult, 0, len(data.Episodes))
	for _, ep := range data.Episodes {
		results = append(results, mdb.EpisodeResult{
			Name:     ep.Name,
			Airdate:  ep.AirDate,
			Overview: ep.Overview,
			Season:   ep.SeasonNumber,
			Episode:  ep.EpisodeNumber,
			Runtime:  ep.Runtime,
			IsFinale: ep.EpisodeType == "finale" || ep.EpisodeType == "series_finale",
		})
	}

	return results, nil
}
