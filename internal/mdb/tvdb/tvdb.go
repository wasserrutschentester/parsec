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

type tvdbSearchResponse struct {
	Status string `json:"status"`
	Data   []struct {
		ID       string `json:"tvdb_id"`
		Slug     string `json:"slug"`
		Name     string `json:"name"`
		Year     string `json:"year"`
		Type     string `json:"type"`
		Overview string `json:"overview"`
	} `json:"data"`
}

type tvdbEpisodeResponse struct {
	Status string `json:"status"`
	Data   struct {
		ID           int    `json:"id"`
		Name         string `json:"name"`
		Aired        string `json:"aired"`
		SeasonNumber int    `json:"seasonNumber"`
		Number       int    `json:"number"`
		Overview     string `json:"overview"`
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

func Search(mediaType, query string, year int) ([]mdb.SearchResult, error) {
	token, err := login()
	if err != nil {
		return nil, err
	}

	tvdbType := mediaType
	if mediaType == "movie" {
		tvdbType = "movies"
	} else if mediaType == "tv" {
		tvdbType = "series"
	}

	u := fmt.Sprintf("%s/search?query=%s&type=%s", baseURL, query, tvdbType)
	if year > 0 {
		u = fmt.Sprintf("%s&year=%d", u, year)
	}

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TVDB API returned status %d", resp.StatusCode)
	}

	var data tvdbSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	results := make([]mdb.SearchResult, 0, len(data.Data))
	for _, r := range data.Data {
		tvdbID, _ := strconv.Atoi(r.ID)
		resYear := 0
		if len(r.Year) >= 4 {
			resYear, _ = strconv.Atoi(r.Year[:4])
		}

		result := mdb.SearchResult{
			TvdbID:   tvdbID,
			TvdbSlug: r.Slug,
			TvdbType: r.Type,
			Title:    r.Name,
			Year:     resYear,
			IsTV:     r.Type == "series",
			Overview: r.Overview,
		}

		// Try to get external IDs
		externalIDs, err := GetExternalIDs(tvdbID, r.Type, token)
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

		results = append(results, result)
	}

	return results, nil
}

func GetExternalIDs(tvdbID int, mediaType string, token string) (tvdbExternalIDsResponse, error) {
	var emptyResponse tvdbExternalIDsResponse
	if token == "" {
		var err error
		token, err = login()
		if err != nil {
			return emptyResponse, err
		}
	}

	endpoint := "series"
	if mediaType == "movie" {
		endpoint = "movies"
	}

	u := fmt.Sprintf("%s/%s/%d/extended", baseURL, endpoint, tvdbID)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return emptyResponse, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return emptyResponse, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return emptyResponse, fmt.Errorf("TVDB API returned status %d", resp.StatusCode)
	}

	var data tvdbExternalIDsResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return emptyResponse, err
	}

	return data, nil
}

func GetEpisodeMetadata(seriesID int, season, episode int) (mdb.EpisodeResult, error) {
	token, err := login()
	if err != nil {
		return mdb.EpisodeResult{}, err
	}

	u := fmt.Sprintf("%s/series/%d/episodes/default?season=%d", baseURL, seriesID, season)

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return mdb.EpisodeResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return mdb.EpisodeResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return mdb.EpisodeResult{}, fmt.Errorf("TVDB API returned status %d", resp.StatusCode)
	}

	var data struct {
		Data struct {
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

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
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
