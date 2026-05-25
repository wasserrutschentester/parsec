package tvdb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"codeberg.org/n0ne/parsec/internal/config"
	"codeberg.org/n0ne/parsec/internal/mdb"
)

const (
	baseURL = "https://api4.thetvdb.com/v4"
)

type loginResponse struct {
	Status string `json:"status"`
	Data   struct {
		Token string `json:"token"`
	} `json:"data"`
}

type tvdbMedia struct {
	ID       string `json:"tvdb_id"` // Note: Search returns string, but some extended calls return int.
	IDInt    int    `json:"id"`      // For extended/GetByID calls
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Year     string `json:"year"`
	Type     string `json:"type"`
	Overview string `json:"overview"`
}

func (m *tvdbMedia) toSearchResult() mdb.SearchResult {
	tvdbID := m.IDInt
	if tvdbID == 0 {
		tvdbID, _ = strconv.Atoi(m.ID)
	}
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

	resp, err := http.Post(baseURL+"/login", "application/json", bytes.NewBuffer(jsonData))
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

	u := fmt.Sprintf("%s/%s", baseURL, endpoint)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
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
	if mediaType == "movie" {
		return "movies"
	} else if mediaType == "tv" {
		return "series"
	}
	return mediaType
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

func GetEpisodeMetadata(seriesID int, season, episode int) (mdb.EpisodeResult, error) {
	var data tvdbEpisodeResponse
	if err := get(fmt.Sprintf("series/%d/episodes/default?season=%d", seriesID, season), &data); err != nil {
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
