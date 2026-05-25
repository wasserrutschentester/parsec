package tmdb

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"codeberg.org/n0ne/parsec/internal/config"
	"codeberg.org/n0ne/parsec/internal/mdb"
)

const (
	baseURL = "https://api.themoviedb.org/3"
)

type tmdbSearchResponse struct {
	Results []struct {
		ID            int     `json:"id"`
		Title         string  `json:"title"`          // For movies
		Name          string  `json:"name"`           // For TV shows
		OriginalTitle string  `json:"original_title"` // For movies
		OriginalName  string  `json:"original_name"`  // For TV shows
		ReleaseDate   string  `json:"release_date"`   // For movies
		FirstAirDate  string  `json:"first_air_date"` // For TV shows
		Popularity    float64 `json:"popularity"`
		Overview      string  `json:"overview"`
	} `json:"results"`
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

func Search(mediaType, query string, year int) ([]mdb.SearchResult, error) {
	apiKey := config.GetTmdbApiKey()
	if apiKey == "" {
		return nil, fmt.Errorf("TMDB API key not configured")
	}

	params := url.Values{}
	params.Add("api_key", apiKey)
	params.Add("query", query)
	params.Add("language", config.GetPreferredLanguage())

	if year > 0 {
		if mediaType == "movie" {
			params.Add("primary_release_year", strconv.Itoa(year))
		} else {
			params.Add("first_air_date_year", strconv.Itoa(year))
		}
	}

	u := fmt.Sprintf("%s/search/%s?%s", baseURL, mediaType, params.Encode())
	resp, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB API returned status %d", resp.StatusCode)
	}

	var data tmdbSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	results := make([]mdb.SearchResult, 0, len(data.Results))
	for _, r := range data.Results {
		title := r.Title
		originalTitle := r.OriginalTitle
		date := r.ReleaseDate

		if mediaType == "tv" {
			title = r.Name
			originalTitle = r.OriginalName
			date = r.FirstAirDate
		}

		resYear := 0
		if len(date) >= 4 {
			resYear, _ = strconv.Atoi(date[:4])
		}

		result := mdb.SearchResult{
			TmdbID:        r.ID,
			TmdbType:      mediaType,
			Title:         title,
			OriginalTitle: originalTitle,
			Year:          resYear,
			IsTV:          mediaType == "tv",
			Popularity:    r.Popularity,
			Overview:      r.Overview,
		}

		externalIDs, err := GetExternalIDs(r.ID, mediaType)
		if err != nil {
			fmt.Println(err)
			continue
		} else {
			if externalIDs.Imdb != "" {
				result.ImdbID = externalIDs.Imdb
			}
			if externalIDs.Tvdb != 0 {
				result.TvdbID = externalIDs.Tvdb
				result.TvdbType = "series"
			}
		}

		results = append(results, result)
	}

	return results, nil
}

func GetExternalIDs(tmdbID int, mediaType string) (tmdbExternalIDsResponse, error) {
	apiKey := config.GetTmdbApiKey()
	emptyResponse := tmdbExternalIDsResponse{}
	if apiKey == "" {
		return emptyResponse, fmt.Errorf("TMDB API key not configured")
	}

	u := fmt.Sprintf("%s/%s/%d/external_ids?api_key=%s", baseURL, mediaType, tmdbID, apiKey)
	resp, err := http.Get(u)
	if err != nil {
		return emptyResponse, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return emptyResponse, fmt.Errorf("TMDB API returned status %d", resp.StatusCode)
	}

	var data tmdbExternalIDsResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return emptyResponse, err
	}

	return data, nil
}

func GetEpisodeMetadata(seriesID int, season, episode int) (mdb.EpisodeResult, error) {
	apiKey := config.GetTmdbApiKey()
	emptyResult := mdb.EpisodeResult{}
	if apiKey == "" {
		return emptyResult, fmt.Errorf("TMDB API key not configured")
	}

	u := fmt.Sprintf("%s/tv/%d/season/%d/episode/%d?api_key=%s&language=%s", baseURL, seriesID, season, episode, apiKey, config.GetPreferredLanguage())
	resp, err := http.Get(u)
	if err != nil {
		return emptyResult, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return emptyResult, fmt.Errorf("TMDB API returned status %d", resp.StatusCode)
	}

	var data tmdbEpisodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return emptyResult, err
	}

	return mdb.EpisodeResult{
		Name:     data.Name,
		Airdate:  data.AirDate,
		Overview: data.Overview,
		Season:   season,
		Episode:  episode,
	}, nil
}
