// Package tvdb provides a client for the TheTVDB (TVDB) API.
package tvdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/text/language"

	"codeberg.org/upPollo/parsec/internal/cache"
	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/mdb"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/filename"
	"codeberg.org/upPollo/parsec/internal/ui"
)

var (
	// BaseURL is the TVDB API base URL.
	BaseURL = "https://api4.thetvdb.com/v4"
	// HTTPClient is the HTTP client used for TVDB requests.
	HTTPClient = http.DefaultClient

	enrichedEpisodesMemoryCache sync.Map
)

type loginResponse struct {
	Status string `json:"status"`
	Data   struct {
		Token string `json:"token"`
	} `json:"data"`
}

type tvdbMedia struct {
	TvdbID             string   `json:"tvdb_id"` // Reliable in Search results (numeric string)
	ID                 any      `json:"id"`      // Integer in GetByID, String in Search (e.g. "series-123")
	Slug               string   `json:"slug"`
	Name               string   `json:"name"`
	NameTranslated     string   `json:"name_translated"`
	Year               string   `json:"year"`
	Type               string   `json:"type"`
	Overview           string   `json:"overview"`
	OverviewTranslated []string `json:"overview_translated"`
	Language           string   `json:"language"`         // language of this record
	PrimaryLanguage    string   `json:"primary_language"` // search results
	OriginalLanguage   string   `json:"originalLanguage"` // direct lookups
	OriginalCountry    string   `json:"originalCountry"`  // direct lookups
	Status             any      `json:"status"`           // search returns string, getByID returns object
	Runtime            int      `json:"runtime"`
	Genres             []string `json:"genres"`
}

func parseTvdbID(m *tvdbMedia) int {
	// 1. Try tvdb_id first (numeric string, but occasionally missing in v4)
	if m.TvdbID != "" {
		if id, err := strconv.Atoi(m.TvdbID); err == nil {
			return id
		}
	}

	// 2. Fallback to id (primary field, but requires parsing if it's a string like "series-123")
	if m.ID != nil {
		switch v := m.ID.(type) {
		case float64:
			return int(v)
		case string:
			if parts := strings.Split(v, "-"); len(parts) > 1 {
				id, _ := strconv.Atoi(parts[len(parts)-1])

				return id
			}

			id, _ := strconv.Atoi(v)

			return id
		}
	}

	return 0
}

func (m *tvdbMedia) toSearchResult() mdb.SearchResult {
	tvdbID := parseTvdbID(m)

	resYear := 0
	if len(m.Year) >= 4 {
		resYear, _ = strconv.Atoi(m.Year[:4])
	}

	origLang := m.OriginalLanguage
	if origLang == "" {
		origLang = m.PrimaryLanguage
	}

	title := m.Name
	if m.NameTranslated != "" {
		title = m.NameTranslated
	}

	overview := m.Overview
	if len(m.OverviewTranslated) > 0 {
		overview = m.OverviewTranslated[0]
	}

	var countries []string
	if m.OriginalCountry != "" {
		countries = append(countries, strings.ToUpper(m.OriginalCountry))
	}

	var statusName string

	switch s := m.Status.(type) {
	case string:
		statusName = s
	case map[string]any:
		if name, ok := s["name"].(string); ok {
			statusName = name
		}
	}

	titleType := determineTitleType(m.Type, m.Genres)

	return mdb.SearchResult{
		TvdbID:           tvdbID,
		TvdbSlug:         m.Slug,
		TvdbType:         m.Type,
		Title:            strings.TrimSpace(title),
		Year:             resYear,
		Runtime:          m.Runtime,
		IsTV:             m.Type == "series",
		TitleType:        titleType,
		Overview:         overview,
		OriginalLanguage: origLang,
		Status:           mdb.NormalizeStatus(statusName),
		Countries:        countries,
	}
}

func determineTitleType(mediaType string, genres []string) mdb.TitleType {
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "series":
		for _, g := range genres {
			if strings.EqualFold(g, "Mini-Series") || strings.EqualFold(g, "Miniseries") {
				return mdb.TitleTypeTVMiniSeries
			}
		}

		return mdb.TitleTypeTVSeries
	case "movie", "movies":
		for _, g := range genres {
			if strings.EqualFold(g, "Made for TV") || strings.EqualFold(g, "TV Movie") {
				return mdb.TitleTypeTVMovie
			}
		}

		return mdb.TitleTypeMovie
	default:
		return mdb.TitleTypeUnknown
	}
}

type tvdbSearchResponse struct {
	Status string      `json:"status"`
	Data   []tvdbMedia `json:"data"`
}

// Episode represents a single episode in the TVDB API.
type Episode struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Aired        string `json:"aired"`
	SeasonNumber int    `json:"seasonNumber"`
	Number       int    `json:"number"`
	Overview     string `json:"overview"`
	FinaleType   string `json:"finaleType"`
	Runtime      int    `json:"runtime"`
}

// ToEpisodeResult converts a TVDB Episode to an mdb.EpisodeResult.
func (e *Episode) ToEpisodeResult() mdb.EpisodeResult {
	return mdb.EpisodeResult{
		Name:     e.Name,
		Airdate:  e.Aired,
		Overview: e.Overview,
		Season:   e.SeasonNumber,
		Episode:  e.Number,
		Runtime:  e.Runtime,
		TvdbID:   e.ID,
		IsFinale: e.FinaleType == "season" || e.FinaleType == "series",
	}
}

type tvdbEpisodeResponse struct {
	Status string `json:"status"`
	Data   struct {
		Episodes []Episode `json:"episodes"`
	} `json:"data"`
	Links struct {
		Prev string `json:"prev"`
		Self string `json:"self"`
		Next string `json:"next"`
	} `json:"links"`
}

type remoteID struct {
	ID         string `json:"id"`
	Type       int    `json:"type"`
	SourceName string `json:"sourceName"`
}

type tvdbExternalIDsResponse struct {
	Status string `json:"status"`
	Data   struct {
		OriginalLanguage string `json:"originalLanguage"`
		Aliases          []struct {
			Name     string `json:"name"`
			Language string `json:"language"`
		} `json:"alias"`
		RemoteIDs []remoteID `json:"remoteIds"`
		Genres    []struct {
			Name string `json:"name"`
		} `json:"genres"`
	} `json:"data"`
}

type tvdbErrorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

var (
	errNotConfigured = errors.New("TVDB API key not configured")
	errLoginStatus   = errors.New("TVDB login failed with status")
	errTVDBStatus    = errors.New("TVDB API returned status")
	errNotFound      = errors.New("no episode found")
)

func parseErrorResponse(body []byte, statusCode int, baseErr error) error {
	var errResp tvdbErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Message != "" {
		return fmt.Errorf("%w %d: %s", baseErr, statusCode, errResp.Message)
	}

	return fmt.Errorf("%w %d", baseErr, statusCode)
}

func login(ctx context.Context) (string, error) {
	tokenKey := "tvdb_token"
	if cached, err := cache.Get(tokenKey); err == nil {
		return string(cached), nil
	}

	apiKey := config.GetTvdbAPIKey()
	if apiKey == "" {
		return "", errNotConfigured
	}

	jsonData, err := json.Marshal(map[string]string{"apikey": apiKey})
	if err != nil {
		return "", fmt.Errorf("failed to marshal login data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, BaseURL+"/login", bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create login request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("login request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read TVDB login response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", parseErrorResponse(body, resp.StatusCode, errLoginStatus)
	}

	var data loginResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("failed to decode login response: %w", err)
	}

	_ = cache.Set(tokenKey, []byte(data.Data.Token))

	return data.Data.Token, nil
}

func getISO3(lang string) string {
	tag := language.Make(lang)
	base, _ := tag.Base()

	return base.ISO3()
}

func get(ctx context.Context, endpoint string, target any) error {
	return getWithRetry(ctx, endpoint, target, true)
}

func getFromCache(key string, target any) (bool, error) {
	cached, err := cache.Get(key)
	if err != nil {
		return false, nil
	}

	if err := json.Unmarshal(cached, target); err != nil {
		return true, fmt.Errorf("failed to unmarshal cached TVDB response: %w", err)
	}

	return true, nil
}

func createTVDBRequest(ctx context.Context, endpoint, token, prefLang string) (*http.Request, error) {
	u := fmt.Sprintf("%s/%s", BaseURL, endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create TVDB request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)

	if prefLang != "" {
		req.Header.Set("Accept-Language", getISO3(prefLang))
	}

	return req, nil
}

func doRequest(ctx context.Context, endpoint, token, prefLang string) (*http.Response, error) {
	for attempt := range 3 {
		req, err := createTVDBRequest(ctx, endpoint, token, prefLang)
		if err != nil {
			return nil, err
		}

		resp, err := HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("TVDB request failed: %w", err)
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

	return nil, errTVDBStatus
}

func handleTVDBResponse(resp *http.Response, cacheKey string, target any) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read TVDB response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return parseErrorResponse(body, resp.StatusCode, errTVDBStatus)
	}

	_ = cache.Set(cacheKey, body)

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("failed to unmarshal TVDB response: %w", err)
	}

	return nil
}

func getWithRetry(ctx context.Context, endpoint string, target any, allowRetry bool) error {
	prefLang := config.GetPreferredLanguage()
	cacheKey := fmt.Sprintf("tvdb:%s:%s", prefLang, endpoint)

	if ok, err := getFromCache(cacheKey, target); ok {
		return err
	}

	token, err := login(ctx)
	if err != nil {
		return err
	}

	resp, err := doRequest(ctx, endpoint, token, prefLang)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized && allowRetry {
		_ = cache.Remove("tvdb_token")

		return getWithRetry(ctx, endpoint, target, false)
	}

	return handleTVDBResponse(resp, cacheKey, target)
}

func toTvdbType(mediaType string) string {
	switch mediaType {
	case "movie":
		return "movies"
	case "tv":
		return "series"
	default:
		return mediaType
	}
}

// Search searches for media on TVDB by query and optionally by year.
func Search(ctx context.Context, mediaType, query string, year int) ([]mdb.SearchResult, error) {
	tvdbType := toTvdbType(mediaType)

	endpoint := fmt.Sprintf("search?query=%s&type=%s", url.QueryEscape(query), tvdbType)
	if year > 0 {
		endpoint = fmt.Sprintf("%s&year=%d", endpoint, year)
	}

	var data tvdbSearchResponse
	if err := get(ctx, endpoint, &data); err != nil {
		return nil, err
	}

	results := make([]mdb.SearchResult, 0, len(data.Data))
	for _, r := range data.Data {
		result := r.toSearchResult()
		applyExternalIDs(ctx, &result, result.TvdbID, r.Type)
		results = append(results, result)
	}

	return results, nil
}

type tvdbRemoteMatch struct {
	Series *tvdbMedia `json:"series"`
	Movie  *tvdbMedia `json:"movie"`
}

type tvdbRemoteIDResponse struct {
	Status string            `json:"status"`
	Data   []tvdbRemoteMatch `json:"data"`
}

// GetByRemoteID retrieves media from TVDB using a remote ID (e.g. IMDB ID).
func GetByRemoteID(ctx context.Context, remoteID, mediaType string) (*mdb.SearchResult, error) {
	endpoint := "search/remoteid/" + url.PathEscape(remoteID)

	var data tvdbRemoteIDResponse
	if err := get(ctx, endpoint, &data); err != nil {
		return nil, err
	}

	if len(data.Data) == 0 {
		return nil, mdb.ErrNotFound
	}

	r, actualType := selectBestRemoteMatch(data.Data, mediaType)
	if r == nil {
		return nil, mdb.ErrNotFound
	}

	// Manually set type for toSearchResult
	r.Type = actualType

	result := r.toSearchResult()

	// TVDB ID is sometimes nested under 'id' in these responses rather than 'tvdb_id'
	tvdbID := parseTvdbID(r)
	applyExternalIDs(ctx, &result, tvdbID, actualType)

	applyTranslation(ctx, &result, tvdbID, actualType)

	return &result, nil
}

func selectBestRemoteMatch(data []tvdbRemoteMatch, mediaType string) (*tvdbMedia, string) {
	// Try requested type first
	if mediaType == "tv" {
		for _, item := range data {
			if item.Series != nil {
				return item.Series, "series"
			}
		}
	} else {
		for _, item := range data {
			if item.Movie != nil {
				return item.Movie, "movies"
			}
		}
	}

	// Fallback to whatever is available
	for _, item := range data {
		if item.Series != nil {
			return item.Series, "series"
		}

		if item.Movie != nil {
			return item.Movie, "movies"
		}
	}

	return nil, ""
}

func applyTranslation(ctx context.Context, result *mdb.SearchResult, tvdbID int, mediaType string) {
	prefLang := config.GetPreferredLanguage()
	if prefLang == "" {
		return
	}

	translation, err := getTranslation(ctx, tvdbID, mediaType, prefLang)
	if err != nil {
		return
	}

	if translation.Data.Name != "" {
		result.Title = translation.Data.Name
	}

	if translation.Data.Overview != "" {
		result.Overview = translation.Data.Overview
	}
}

// GetByID retrieves a single media item from TVDB by its ID.
func GetByID(ctx context.Context, tvdbID int, mediaType string) (*mdb.SearchResult, error) {
	endpoint := toTvdbType(mediaType)

	var data struct {
		Data tvdbMedia `json:"data"`
	}

	if err := get(ctx, fmt.Sprintf("%s/%d", endpoint, tvdbID), &data); err != nil {
		return nil, err
	}

	result := data.Data.toSearchResult()
	// GetByID response might not have Type set correctly depending on endpoint
	if result.TvdbType == "" {
		result.TvdbType = endpoint
	}

	result.IsTV = endpoint == "series"
	if result.TitleType == mdb.TitleTypeUnknown {
		result.TitleType = mdb.TitleTypeMovie
		if result.IsTV {
			result.TitleType = mdb.TitleTypeTVSeries
		}
	}

	applyTranslation(ctx, &result, tvdbID, endpoint)

	applyExternalIDs(ctx, &result, tvdbID, endpoint)

	return &result, nil
}

type tvdbTranslationResponse struct {
	Status string `json:"status"`
	Data   struct {
		Name     string `json:"name"`
		Overview string `json:"overview"`
		Language string `json:"language"`
	} `json:"data"`
}

func getTranslation(ctx context.Context, tvdbID int, mediaType, lang string) (tvdbTranslationResponse, error) {
	tvdbType := toTvdbType(mediaType)
	iso3 := getISO3(lang)

	var data tvdbTranslationResponse

	endpoint := fmt.Sprintf("%s/%d/translations/%s", tvdbType, tvdbID, iso3)

	if err := get(ctx, endpoint, &data); err != nil {
		return tvdbTranslationResponse{}, err
	}

	return data, nil
}

func getTranslationFromCache(tvdbID int, mediaType, lang string) (tvdbTranslationResponse, bool) {
	tvdbType := toTvdbType(mediaType)
	iso3 := getISO3(lang)
	endpoint := fmt.Sprintf("%s/%d/translations/%s", tvdbType, tvdbID, iso3)

	prefLang := config.GetPreferredLanguage()
	cacheKey := fmt.Sprintf("tvdb:%s:%s", prefLang, endpoint)

	var data tvdbTranslationResponse

	ok, err := getFromCache(cacheKey, &data)
	if ok && err == nil && data.Data.Name != "" {
		return data, true
	}

	return tvdbTranslationResponse{}, false
}

func applyExternalIDs(ctx context.Context, result *mdb.SearchResult, tvdbID int, mediaType string) {
	externalIDs, err := getExternalIDs(ctx, tvdbID, mediaType)
	if err != nil {
		return
	}

	result.OriginalLanguage = externalIDs.Data.OriginalLanguage

	prefLang := config.GetPreferredLanguage()
	origLang := externalIDs.Data.OriginalLanguage

	for _, alias := range externalIDs.Data.Aliases {
		if isLanguageMatch(alias.Language, prefLang, "en", origLang) {
			result.AltTitle = append(result.AltTitle, alias.Name)
		}
	}

	refineTitleTypeFromGenres(result, externalIDs.Data.Genres)
	applyRemoteIDs(result, externalIDs.Data.RemoteIDs)
}

func refineTitleTypeFromGenres(result *mdb.SearchResult, genres []struct {
	Name string `json:"name"`
},
) {
	for _, g := range genres {
		if result.IsTV && (strings.EqualFold(g.Name, "Mini-Series") || strings.EqualFold(g.Name, "Miniseries")) {
			result.TitleType = mdb.TitleTypeTVMiniSeries

			return
		} else if !result.IsTV && (strings.EqualFold(g.Name, "Made for TV") || strings.EqualFold(g.Name, "TV Movie")) {
			result.TitleType = mdb.TitleTypeTVMovie

			return
		}
	}
}

func applyRemoteIDs(result *mdb.SearchResult, remoteIDs []remoteID) {
	for _, ext := range remoteIDs {
		switch ext.SourceName {
		case "IMDB":
			result.ImdbID = ext.ID
		case "TheMovieDB.com", "TMDB":
			tmdbID, _ := strconv.Atoi(ext.ID)

			result.TmdbID = tmdbID
			if result.IsTV {
				result.TmdbType = "tv"
			} else {
				result.TmdbType = "movie"
			}
		}
	}
}

func getExternalIDs(ctx context.Context, tvdbID int, mediaType string) (tvdbExternalIDsResponse, error) {
	endpoint := toTvdbType(mediaType)

	var data tvdbExternalIDsResponse
	if err := get(ctx, fmt.Sprintf("%s/%d/extended", endpoint, tvdbID), &data); err != nil {
		return tvdbExternalIDsResponse{}, err
	}

	return data, nil
}

func getEpisodes(ctx context.Context, seriesID, page int, lang string) (tvdbEpisodeResponse, error) {
	var data tvdbEpisodeResponse

	endpoint := fmt.Sprintf("series/%d/episodes/default", seriesID)
	if lang != "" {
		endpoint = fmt.Sprintf("%s/%s", endpoint, lang)
	}

	if err := get(ctx, fmt.Sprintf("%s?page=%d", endpoint, page), &data); err != nil {
		return tvdbEpisodeResponse{}, err
	}

	return data, nil
}

func getEnrichedFromMemory(seriesID int, lang string) ([]Episode, bool) {
	if config.NoCache {
		return nil, false
	}

	memKey := fmt.Sprintf("%d:%s", seriesID, lang)
	if val, ok := enrichedEpisodesMemoryCache.Load(memKey); ok {
		if eps, ok := val.([]Episode); ok {
			return slices.Clone(eps), true
		}
	}

	return nil, false
}

func getEnrichedFromDisk(seriesID int, lang string) ([]Episode, bool) {
	prefLang := config.GetPreferredLanguage()
	diskKey := fmt.Sprintf("tvdb:%s:series:%d:enriched_episodes:%s", prefLang, seriesID, lang)

	var cachedEpisodes []Episode

	ok, err := getFromCache(diskKey, &cachedEpisodes)
	if !ok || err != nil || len(cachedEpisodes) == 0 {
		return nil, false
	}

	if !config.NoCache {
		memKey := fmt.Sprintf("%d:%s", seriesID, lang)
		enrichedEpisodesMemoryCache.Store(memKey, slices.Clone(cachedEpisodes))
	}

	return cachedEpisodes, true
}

func saveEnrichedEpisodes(seriesID int, lang string, episodes []Episode) {
	memKey := fmt.Sprintf("%d:%s", seriesID, lang)
	if !config.NoCache {
		enrichedEpisodesMemoryCache.Store(memKey, slices.Clone(episodes))
	}

	data, err := json.Marshal(episodes)
	if err != nil {
		return
	}

	prefLang := config.GetPreferredLanguage()
	diskKey := fmt.Sprintf("tvdb:%s:series:%d:enriched_episodes:%s", prefLang, seriesID, lang)
	_ = cache.Set(diskKey, data)
}

func fetchRawEpisodes(ctx context.Context, seriesID int, lang string) ([]Episode, error) {
	var episodes []Episode

	for page := range 20 {
		data, err := getEpisodes(ctx, seriesID, page, lang)
		if err != nil {
			if page == 0 {
				return nil, err
			}

			break
		}

		episodes = append(episodes, data.Data.Episodes...)
		if data.Links.Next == "" {
			break
		}
	}

	return episodes, nil
}

// GetAllEpisodes retrieves all episodes for a given series from TVDB.
func GetAllEpisodes(ctx context.Context, seriesID int, lang string) ([]Episode, error) {
	if eps, ok := getEnrichedFromMemory(seriesID, lang); ok {
		return eps, nil
	}

	if eps, ok := getEnrichedFromDisk(seriesID, lang); ok {
		return eps, nil
	}

	episodes, err := fetchRawEpisodes(ctx, seriesID, lang)
	if err != nil {
		return nil, err
	}

	enrichFromCacheOnly(episodes, []string{lang}, 0)
	saveEnrichedEpisodes(seriesID, lang, episodes)

	return episodes, nil
}

// IdentifyEpisode attempts to find a specific episode in a TVDB series based on metadata.
func IdentifyEpisode(ctx context.Context, result mdb.SearchResult, meta *metadata.Metadata, allowSpecials bool) (mdb.EpisodeResult, error) {
	preferred := config.GetPreferredLanguage()
	langs := []string{preferred, result.OriginalLanguage, "en"}
	uniqueLangs := metadata.RemoveDuplicates(langs)

	normalizedQueryTitle := ""
	if len(meta.EpisodeTitles) > 0 {
		normalizedQueryTitle = metadata.Normalize(strings.Join(meta.EpisodeTitles, " / "))
	}

	for _, lang := range uniqueLangs {
		episodes, err := GetAllEpisodes(ctx, result.TvdbID, lang)
		if err != nil {
			continue
		}

		ui.PrintDebug(fmt.Sprintf("found %d episodes combined", len(episodes)))

		ep := findEpisodeInList(ctx, episodes, meta, normalizedQueryTitle, allowSpecials, uniqueLangs)
		if ep != nil {
			saveEnrichedEpisodes(result.TvdbID, lang, episodes)

			return finalizeEpisodeResult(ctx, ep, episodes, uniqueLangs), nil
		}
	}

	return mdb.EpisodeResult{}, errNotFound
}

func finalizeEpisodeResult(ctx context.Context, ep *Episode, episodes []Episode, uniqueLangs []string) mdb.EpisodeResult {
	res := ep.ToEpisodeResult()
	hasFinale := false
	totalEps := 0

	for _, e := range episodes {
		if e.SeasonNumber == res.Season {
			totalEps++

			if e.FinaleType == "season" || e.FinaleType == "series" {
				hasFinale = true
			}
		}
	}

	if hasFinale {
		res.TotalEpisodes = totalEps
	}

	ui.PrintDebug(fmt.Sprintf("found episode: %+v", res))

	for _, l := range uniqueLangs {
		if res.Name != "" && res.Overview != "" {
			break
		}

		fillEpisodeTranslation(ctx, &res, ep.ID, l)
	}

	fillEpisodeImdbID(ctx, &res, ep.ID)

	return res
}

func isPlaceholderSeasonEpisode(meta *metadata.Metadata) bool {
	return meta.Season == 0 && len(meta.Episodes) == 1 && meta.Episodes[0] == 0
}

func matchByNumberOrDate(episodes []Episode, meta *metadata.Metadata, allowSpecials bool) *Episode {
	if meta.Season >= 0 && len(meta.Episodes) > 0 && !isPlaceholderSeasonEpisode(meta) {
		if ep := matchBySeasonEpisode(episodes, meta.Season, meta.Episodes[0]); ep != nil {
			return ep
		}
	}

	if meta.Date != "" {
		if ep := matchByAirDate(episodes, meta.Date, allowSpecials); ep != nil {
			return ep
		}
	}

	return nil
}

func findEpisodeInList(ctx context.Context, episodes []Episode, meta *metadata.Metadata, normalizedQueryTitle string, allowSpecials bool, langs []string) *Episode {
	if ep := matchByNumberOrDate(episodes, meta, allowSpecials); ep != nil {
		return ep
	}

	if normalizedQueryTitle == "" {
		return nil
	}

	// 3. Lazy Title Matching
	// If a season is specified, try matching within that season first to avoid
	// collisions with similarly named episodes in other seasons.
	if meta.Season > 0 {
		if ep := matchByTitleLazy(ctx, episodes, normalizedQueryTitle, allowSpecials, langs, meta.Season); ep != nil {
			return ep
		}
	}

	// Fall back to searching all episodes across all seasons.
	return matchByTitleLazy(ctx, episodes, normalizedQueryTitle, allowSpecials, langs, 0)
}

func matchByTitleLazy(ctx context.Context, episodes []Episode, normTitle string, allowSpecials bool, langs []string, seasonFilter int) *Episode {
	// Pass 1: Try matching against existing names in the episode list first
	if ep := matchByTitle(episodes, normTitle, allowSpecials, seasonFilter); ep != nil {
		return ep
	}

	// Pass 2: Check local cache ONLY for missing episode names (zero network calls)
	enrichFromCacheOnly(episodes, langs, seasonFilter)

	if ep := matchByTitle(episodes, normTitle, allowSpecials, seasonFilter); ep != nil {
		return ep
	}

	// Pass 3: If still not found, fetch missing translations concurrently with bounded concurrency
	enrichMissingTitlesConcurrent(ctx, episodes, langs, normTitle, seasonFilter)

	if ep := matchByTitle(episodes, normTitle, allowSpecials, seasonFilter); ep != nil {
		return ep
	}

	// Pass 4: Fuzzy Match (Fallback)
	return matchByTitleFuzzy(episodes, normTitle, seasonFilter)
}

func isTitleMatch(epName, normTitle string) bool {
	cleaned := filename.ApplyTitleReplacements(epName)
	if metadata.Normalize(cleaned) == normTitle {
		return true
	}

	parts := strings.FieldsFunc(cleaned, func(r rune) bool {
		return r == '-' || r == ':' || r == '–'
	})
	for _, p := range parts {
		if metadata.Normalize(p) == normTitle {
			return true
		}
	}

	return false
}

func enrichFromCacheOnly(episodes []Episode, langs []string, seasonFilter int) {
	for i := range episodes {
		if seasonFilter > 0 && episodes[i].SeasonNumber != seasonFilter {
			continue
		}

		if episodes[i].Name != "" {
			continue
		}

		for _, l := range langs {
			if trans, ok := getTranslationFromCache(episodes[i].ID, "episodes", l); ok {
				episodes[i].Name = trans.Data.Name

				break
			}
		}
	}
}

const maxConcurrentTranslationRequests = 15

func getMissingIndices(episodes []Episode, seasonFilter int) []int {
	var missingIndices []int

	for i := range episodes {
		if seasonFilter > 0 && episodes[i].SeasonNumber != seasonFilter {
			continue
		}

		if episodes[i].Name == "" {
			missingIndices = append(missingIndices, i)
		}
	}

	return missingIndices
}

func enrichMissingTitlesConcurrent(ctx context.Context, episodes []Episode, langs []string, normTitle string, seasonFilter int) {
	missingIndices := getMissingIndices(episodes, seasonFilter)
	if len(missingIndices) == 0 {
		return
	}

	jobs := make(chan int, len(missingIndices))
	for _, idx := range missingIndices {
		jobs <- idx
	}

	close(jobs)

	numWorkers := min(maxConcurrentTranslationRequests, len(missingIndices))

	var (
		wg    sync.WaitGroup
		found atomic.Bool
	)

	for range numWorkers {
		wg.Go(func() {
			for idx := range jobs {
				if found.Load() || ctx.Err() != nil {
					return
				}

				if fetchAndSetTranslation(ctx, idx, episodes, langs, normTitle) {
					found.Store(true)

					return
				}
			}
		})
	}

	wg.Wait()
}

func fetchAndSetTranslation(ctx context.Context, idx int, episodes []Episode, langs []string, normTitle string) bool {
	var res mdb.EpisodeResult

	for _, l := range langs {
		fillEpisodeTranslation(ctx, &res, episodes[idx].ID, l)

		if res.Name != "" {
			episodes[idx].Name = res.Name

			return normTitle != "" && isTitleMatch(res.Name, normTitle)
		}
	}

	return false
}

func matchBySeasonEpisode(episodes []Episode, season, episode int) *Episode {
	for _, ep := range episodes {
		if ep.SeasonNumber == season && ep.Number == episode {
			return &ep
		}
	}

	return nil
}

func matchByAirDate(episodes []Episode, date string, allowSpecials bool) *Episode {
	for _, ep := range episodes {
		if ep.Aired == date {
			if ep.SeasonNumber == 0 && !allowSpecials {
				continue
			}

			return &ep
		}
	}

	return nil
}

func matchByTitle(episodes []Episode, normTitle string, allowSpecials bool, seasonFilter int) *Episode {
	for _, ep := range episodes {
		if seasonFilter > 0 && ep.SeasonNumber != seasonFilter {
			continue
		}

		if ep.Name == "" {
			continue
		}

		if ep.SeasonNumber == 0 && !allowSpecials {
			continue
		}

		if isTitleMatch(ep.Name, normTitle) {
			return &ep
		}
	}

	return nil
}

func matchByTitleFuzzy(episodes []Episode, normTitle string, seasonFilter int) *Episode {
	var bestMatch Episode

	maxSim := 0.0
	found := false

	for _, ep := range episodes {
		if seasonFilter > 0 && ep.SeasonNumber != seasonFilter {
			continue
		}

		epName := filename.ApplyTitleReplacements(ep.Name)

		sim := mdb.CalculateSimilarity(normTitle, metadata.Normalize(epName))
		if sim > maxSim {
			maxSim = sim
			bestMatch = ep
			found = true
		}
	}

	ui.PrintDebug(fmt.Sprintf("Best match for %s (sim = %f): %+v", normTitle, maxSim, bestMatch))

	if found && maxSim > 0.6 {
		return &bestMatch
	}

	return nil
}

func fillEpisodeTranslation(ctx context.Context, res *mdb.EpisodeResult, tvdbID int, lang string) {
	if lang == "" {
		ui.PrintDebug("no language specified, skipping translation")

		return
	}

	translation, err := getTranslation(ctx, tvdbID, "episodes", lang)
	if err != nil {
		ui.PrintDebug(fmt.Sprintf("failed to get translation (episode %d, %s): %v", tvdbID, lang, err))
	}

	if err == nil {
		if res.Name == "" && translation.Data.Name != "" {
			res.Name = translation.Data.Name
		}

		if res.Overview == "" && translation.Data.Overview != "" {
			res.Overview = translation.Data.Overview
		}
	}
}

func isLanguageMatch(lang string, targets ...string) bool {
	tag := language.Make(lang)

	for _, target := range targets {
		if target == "" {
			continue
		}

		if metadata.MatchLanguage(tag, language.Make(target)) {
			return true
		}
	}

	return false
}

type tvdbEpisodeExtendedResponse struct {
	Data struct {
		RemoteIDs []remoteID `json:"remoteIds"`
	} `json:"data"`
}

func fillEpisodeImdbID(ctx context.Context, res *mdb.EpisodeResult, tvdbID int) {
	if res.ImdbID != "" {
		return
	}

	var data tvdbEpisodeExtendedResponse
	if err := get(ctx, fmt.Sprintf("episodes/%d/extended", tvdbID), &data); err != nil {
		ui.PrintDebug(fmt.Sprintf("failed to fetch extended episode for remote ids: %v", err))

		return
	}

	for _, rid := range data.Data.RemoteIDs {
		// Type 2 is usually IMDB in TVDB v4
		if rid.SourceName == "Imdb" || rid.ID != "" && strings.HasPrefix(rid.ID, "tt") {
			res.ImdbID = rid.ID

			break
		}
	}
}
