package tvdb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"codeberg.org/n0ne/parsec/internal/config"
	"codeberg.org/n0ne/parsec/internal/mdb"
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
	TvdbID   string      `json:"tvdb_id"` // Reliable in Search results (numeric string)
	ID       interface{} `json:"id"`      // Integer in GetByID, String in Search (e.g. "series-123")
	Slug     string      `json:"slug"`
	Name     string      `json:"name"`
	Year     string      `json:"year"`
	Type     string      `json:"type"`
	Overview string      `json:"overview"`
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

	return mdb.SearchResult{
		TvdbID:   tvdbID,
		TvdbSlug: m.Slug,
		TvdbType: m.Type,
		Title:    m.Name,
		Year:     resYear,
		IsTV:     m.Type == "series",
		Overview: m.Overview,
	}
}

type tvdbSearchResponse struct {
	Status string      `json:"status"`
	Data   []tvdbMedia `json:"data"`
}

type tvdbEpisodeResponse struct {
	Status string `json:"status"`
	Data   struct {
		Episodes []struct {
			ID           int    `json:"id"`
			Name         string `json:"name"`
			Aired        string `json:"aired"`
			SeasonNumber int    `json:"seasonNumber"`
			Number       int    `json:"number"`
			Overview     string `json:"overview"`
		} `json:"episodes"`
	} `json:"data"`
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("TVDB login failed with status %d", resp.StatusCode)
	}

	var data loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	return data.Data.Token, nil
}

func get(endpoint string, target interface{}) error {
	token, err := login()
	if err != nil {
		return err
	}

	u := fmt.Sprintf("%s/%s", BaseURL, endpoint)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("TVDB API returned status %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(target)
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

	applyExternalIDs(&result, tvdbID, endpoint)

	return &result, nil
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
			if ext.SourceName == "IMDB" {
				result.ImdbID = ext.ID
			} else if ext.SourceName == "TheMovieDB.com" || ext.SourceName == "TMDB" {
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

func GetEpisodeMetadata(seriesID int, season, episode int, lang string) (mdb.EpisodeResult, error) {
	var data tvdbEpisodeResponse
	endpoint := fmt.Sprintf("series/%d/episodes/default", seriesID)
	if lang != "" {
		endpoint = fmt.Sprintf("%s/%s", endpoint, lang)
	}

	if err := get(fmt.Sprintf("%s?season=%d", endpoint, season), &data); err != nil {
		return mdb.EpisodeResult{}, err
	}

	for _, ep := range data.Data.Episodes {
		if ep.Number == episode {
			return mdb.EpisodeResult{
				Name:     ep.Name,
				Airdate:  ep.Aired,
				Overview: ep.Overview,
				Season:   ep.SeasonNumber,
				Episode:  ep.Number,
				TvdbID:   ep.ID,
			}, nil
		}
	}

	return mdb.EpisodeResult{}, fmt.Errorf("episode not found")
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
