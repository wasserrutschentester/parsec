package tvdb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"codeberg.org/upPollo/parsec/internal/cache"
	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/mdb"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/ui"
	"golang.org/x/text/language"
)

var (
	BaseURL    = "https://api4.thetvdb.com/v4"
	HTTPClient = http.DefaultClient
)

type loginResponse struct {
	Status string `json:"status"`
	Data   struct {
		Token string `json:"token"`
	} `json:"data"`
}

type tvdbMedia struct {
	TvdbID             string      `json:"tvdb_id"` // Reliable in Search results (numeric string)
	ID                 interface{} `json:"id"`      // Integer in GetByID, String in Search (e.g. "series-123")
	Slug               string      `json:"slug"`
	Name               string      `json:"name"`
	NameTranslated     string      `json:"name_translated"`
	Year               string      `json:"year"`
	Type               string      `json:"type"`
	Overview           string      `json:"overview"`
	OverviewTranslated []string    `json:"overview_translated"`
	Language           string      `json:"language"`         // language of this record
	PrimaryLanguage    string      `json:"primary_language"` // search results
	OriginalLanguage   string      `json:"originalLanguage"` // direct lookups
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

	return mdb.SearchResult{
		TvdbID:           tvdbID,
		TvdbSlug:         m.Slug,
		TvdbType:         m.Type,
		Title:            title,
		Year:             resYear,
		IsTV:             m.Type == "series",
		Overview:         overview,
		OriginalLanguage: origLang,
	}
}

type tvdbSearchResponse struct {
	Status string      `json:"status"`
	Data   []tvdbMedia `json:"data"`
}

type TvdbEpisode struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Aired        string `json:"aired"`
	SeasonNumber int    `json:"seasonNumber"`
	Number       int    `json:"number"`
	Overview     string `json:"overview"`
}

func (e *TvdbEpisode) toEpisodeResult() mdb.EpisodeResult {
	return mdb.EpisodeResult{
		Name:     e.Name,
		Airdate:  e.Aired,
		Overview: e.Overview,
		Season:   e.SeasonNumber,
		Episode:  e.Number,
		TvdbID:   e.ID,
	}
}

type tvdbEpisodeResponse struct {
	Status string `json:"status"`
	Data   struct {
		Episodes []TvdbEpisode `json:"episodes"`
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
		RemoteIds []remoteID `json:"remoteIds"`
	} `json:"data"`
}

func login() (string, error) {
	// Try to get cached token
	tokenKey := "tvdb_token"
	if cached, err := cache.Get(tokenKey); err == nil {
		return string(cached), nil
	}

	apiKey := config.GetTvdbApiKey()
	if apiKey == "" {
		return "", fmt.Errorf("TVDB API key not configured")
	}

	authData := map[string]string{"apikey": apiKey}

	jsonData, err := json.Marshal(authData)
	if err != nil {
		return "", err
	}

	resp, err := HTTPClient.Post(BaseURL+"/login", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("TVDB login failed with status %d", resp.StatusCode)
	}

	var data loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	// Cache token (cache.Get already handles 6h expiration, but token might be shorter or we want to be safe)
	_ = cache.Set(tokenKey, []byte(data.Data.Token))

	return data.Data.Token, nil
}

func getISO3(lang string) string {
	tag := language.Make(lang)
	base, _ := tag.Base()

	return base.ISO3()
}

func get(endpoint string, target interface{}) error {
	return getWithRetry(endpoint, target, true)
}

func getWithRetry(endpoint string, target interface{}, allowRetry bool) error {
	prefLang := config.GetPreferredLanguage()

	cacheKey := fmt.Sprintf("tvdb:%s:%s", prefLang, endpoint)
	if cached, err := cache.Get(cacheKey); err == nil {
		return json.Unmarshal(cached, target)
	}

	token, err := login()
	if err != nil {
		return err
	}

	u := fmt.Sprintf("%s/%s", BaseURL, endpoint)

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	if prefLang != "" {
		req.Header.Set("Accept-Language", getISO3(prefLang))
	}

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized {
			_ = cache.Remove("tvdb_token")

			if allowRetry {
				return getWithRetry(endpoint, target, false)
			}
		}

		return fmt.Errorf("TVDB API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	_ = cache.Set(cacheKey, body)

	return json.Unmarshal(body, target)
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

func Search(mediaType, query string, year int) ([]mdb.SearchResult, error) {
	tvdbType := toTvdbType(mediaType)

	endpoint := fmt.Sprintf("search?query=%s&type=%s", query, tvdbType)
	if year > 0 {
		endpoint = fmt.Sprintf("%s&year=%d", endpoint, year)
	}

	var data tvdbSearchResponse
	if err := get(endpoint, &data); err != nil {
		return nil, err
	}

	results := make([]mdb.SearchResult, 0, len(data.Data))
	for _, r := range data.Data {
		result := r.toSearchResult()
		applyExternalIDs(&result, result.TvdbID, r.Type)
		results = append(results, result)
	}

	return results, nil
}

type tvdbRemoteMatch struct {
	Series *tvdbMedia `json:"series"`
	Movie  *tvdbMedia `json:"movie"`
}

type tvdbRemoteIdResponse struct {
	Status string            `json:"status"`
	Data   []tvdbRemoteMatch `json:"data"`
}

func GetByRemoteID(remoteID, mediaType string) (*mdb.SearchResult, error) {
	endpoint := fmt.Sprintf("search/remoteid/%s", remoteID)

	var data tvdbRemoteIdResponse
	if err := get(endpoint, &data); err != nil {
		return nil, err
	}

	if len(data.Data) == 0 {
		return nil, nil
	}

	r, actualType := selectBestRemoteMatch(data.Data, mediaType)
	if r == nil {
		return nil, nil
	}

	// Manually set type for toSearchResult
	r.Type = actualType

	result := r.toSearchResult()

	// TVDB ID is sometimes nested under 'id' in these responses rather than 'tvdb_id'
	tvdbID := parseTvdbID(r)
	applyExternalIDs(&result, tvdbID, actualType)

	applyTranslation(&result, tvdbID, actualType)

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

func applyTranslation(result *mdb.SearchResult, tvdbID int, mediaType string) {
	prefLang := config.GetPreferredLanguage()
	if prefLang == "" {
		return
	}

	translation, err := GetTranslation(tvdbID, mediaType, prefLang)
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

func GetByID(tvdbID int, mediaType string) (*mdb.SearchResult, error) {
	endpoint := toTvdbType(mediaType)

	var data struct {
		Data tvdbMedia `json:"data"`
	}

	if err := get(fmt.Sprintf("%s/%d", endpoint, tvdbID), &data); err != nil {
		return nil, err
	}

	result := data.Data.toSearchResult()
	// GetByID response might not have Type set correctly depending on endpoint
	if result.TvdbType == "" {
		result.TvdbType = endpoint
	}

	result.IsTV = endpoint == "series"

	applyTranslation(&result, tvdbID, endpoint)

	applyExternalIDs(&result, tvdbID, endpoint)

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

func GetTranslation(tvdbID int, mediaType, lang string) (tvdbTranslationResponse, error) {
	tvdbType := toTvdbType(mediaType)
	iso3 := getISO3(lang)

	var data tvdbTranslationResponse

	endpoint := fmt.Sprintf("%s/%d/translations/%s", tvdbType, tvdbID, iso3)

	if err := get(endpoint, &data); err != nil {
		return tvdbTranslationResponse{}, err
	}

	return data, nil
}

func applyExternalIDs(result *mdb.SearchResult, tvdbID int, mediaType string) {
	externalIDs, err := GetExternalIDs(tvdbID, mediaType)
	if err == nil {
		result.OriginalLanguage = externalIDs.Data.OriginalLanguage

		prefLang := config.GetPreferredLanguage()
		origLang := externalIDs.Data.OriginalLanguage

		for _, alias := range externalIDs.Data.Aliases {
			if isLanguageMatch(alias.Language, prefLang, "en", origLang) {
				result.AltTitle = append(result.AltTitle, alias.Name)
			}
		}

		for _, ext := range externalIDs.Data.RemoteIds {
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
}

func GetExternalIDs(tvdbID int, mediaType string) (tvdbExternalIDsResponse, error) {
	endpoint := toTvdbType(mediaType)

	var data tvdbExternalIDsResponse
	if err := get(fmt.Sprintf("%s/%d/extended", endpoint, tvdbID), &data); err != nil {
		return tvdbExternalIDsResponse{}, err
	}

	return data, nil
}

func GetEpisodes(seriesID, page int, lang string) (tvdbEpisodeResponse, error) {
	var data tvdbEpisodeResponse

	endpoint := fmt.Sprintf("series/%d/episodes/default", seriesID)
	if lang != "" {
		endpoint = fmt.Sprintf("%s/%s", endpoint, lang)
	}

	if err := get(fmt.Sprintf("%s?page=%d", endpoint, page), &data); err != nil {
		return tvdbEpisodeResponse{}, err
	}

	return data, nil
}

func GetAllEpisodes(seriesID int, lang string) ([]TvdbEpisode, error) {
	var episodes []TvdbEpisode

	for page := 0; page < 20; page++ {
		data, err := GetEpisodes(seriesID, page, lang)
		if err != nil {
			break
		}

		episodes = append(episodes, data.Data.Episodes...)
		if data.Links.Next == "" {
			break
		}
	}

	return episodes, nil
}

func IdentifyEpisode(result mdb.SearchResult, meta *metadata.Metadata, allowSpecials bool) (mdb.EpisodeResult, error) {
	preferred := config.GetPreferredLanguage()
	langs := []string{preferred, result.OriginalLanguage, "en"}
	uniqueLangs := metadata.RemoveDuplicates(langs)

	normalizedQueryTitle := metadata.Normalize(meta.EpisodeTitle)

	for _, lang := range uniqueLangs {
		episodes, err := GetAllEpisodes(result.TvdbID, lang)
		if err != nil {
			continue
		}

		ui.PrintDebug(fmt.Sprintf("found %d episodes combined", len(episodes)))

		var ep *TvdbEpisode
		// 1. Season/Episode Number Match
		if (meta.Season > 0 && meta.Episode > 0) || allowSpecials {
			ep = matchBySeasonEpisode(episodes, meta.Season, meta.Episode)
		}
		// 2. Air Date Match
		if meta.Date != "" && ep == nil {
			ep = matchByAirDate(episodes, meta.Date, allowSpecials)
		}
		// 3. Normalized Title Match
		if normalizedQueryTitle != "" && ep == nil {
			ep = matchByTitle(episodes, normalizedQueryTitle, allowSpecials)
		}
		// 4. Fuzzy Match (Fallback)
		if normalizedQueryTitle != "" && ep == nil {
			ep = matchByTitleFuzzy(episodes, normalizedQueryTitle)
		}

		if ep != nil {
			res := ep.toEpisodeResult()
			ui.PrintDebug(fmt.Sprintf("found episode: %+v", res))
			fillEpisodeTranslation(&res, ep.ID, lang)

			return res, nil
		}
	}

	return mdb.EpisodeResult{}, fmt.Errorf("no episode found")
}

func matchBySeasonEpisode(episodes []TvdbEpisode, season, episode int) *TvdbEpisode {
	for _, ep := range episodes {
		if ep.SeasonNumber == season && ep.Number == episode {
			return &ep
		}
	}

	return nil
}

func matchByAirDate(episodes []TvdbEpisode, date string, allowSpecials bool) *TvdbEpisode {
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

func matchByTitle(episodes []TvdbEpisode, normTitle string, allowSpecials bool) *TvdbEpisode {
	for _, ep := range episodes {
		if metadata.Normalize(ep.Name) == normTitle {
			if ep.SeasonNumber == 0 && !allowSpecials {
				continue
			}

			return &ep
		}
	}

	return nil
}

func matchByTitleFuzzy(episodes []TvdbEpisode, normTitle string) *TvdbEpisode {
	var bestMatch TvdbEpisode

	maxSim := 0.0
	found := false

	for _, ep := range episodes {
		sim := mdb.CalculateSimilarity(normTitle, metadata.Normalize(ep.Name))
		if sim > maxSim {
			maxSim = sim
			bestMatch = ep
			found = true
		}
	}

	if found && maxSim > 0.8 {
		return &bestMatch
	}

	return nil
}

func fillEpisodeTranslation(res *mdb.EpisodeResult, tvdbID int, lang string) {
	if lang == "" {
		ui.PrintDebug("no language specified, skipping translation")
		return
	}

	translation, err := GetTranslation(tvdbID, "episodes", lang)
	ui.PrintDebug(fmt.Sprintf("translation: %+v, err: %v", translation, err))

	if err == nil {
		if translation.Data.Name != "" {
			res.Name = translation.Data.Name
		}

		if translation.Data.Overview != "" {
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

		if tag == language.Make(target) {
			return true
		}
	}

	return false
}
