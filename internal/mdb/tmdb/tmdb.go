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

type tmdbMedia struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`          // For movies
	Name          string  `json:"name"`           // For TV shows
	OriginalTitle string  `json:"original_title"` // For movies
	OriginalName  string  `json:"original_name"`  // For TV shows
	ReleaseDate   string  `json:"release_date"`   // For movies
	FirstAirDate  string  `json:"first_air_date"` // For TV shows
	Popularity    float64 `json:"popularity"`
	Overview      string  `json:"overview"`
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

	return mdb.SearchResult{
		TmdbID:        m.ID,
		TmdbType:      mediaType,
		Title:         title,
		OriginalTitle: originalTitle,
		Year:          resYear,
		IsTV:          mediaType == "tv",
		Popularity:    m.Popularity,
		Overview:      m.Overview,
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

func get(endpoint string, query url.Values, target interface{}) error {
	apiKey := config.GetTmdbApiKey()
	if apiKey == "" {
		return fmt.Errorf("TMDB API key not configured")
	}

	if query == nil {
		query = url.Values{}
	}
	query.Set("api_key", apiKey)
	if query.Get("language") == "" {
		query.Set("language", config.GetPreferredLanguage())
	}

	u := fmt.Sprintf("%s/%s?%s", baseURL, endpoint, query.Encode())
	resp, err := http.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("TMDB API returned status %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

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
		results = append(results, result)
	}

	return results, nil
}

func applyExternalIDs(result *mdb.SearchResult, mediaType string) {
	externalIDs, err := GetExternalIDs(result.TmdbID, mediaType)
	if err == nil {
		if externalIDs.Imdb != "" {
			result.ImdbID = externalIDs.Imdb
		}
		if externalIDs.Tvdb != 0 {
			result.TvdbID = externalIDs.Tvdb
			result.TvdbType = "series"
		}
	}
}

func GetByID(tmdbID int, mediaType string) (*mdb.SearchResult, error) {
	var r tmdbMedia
	if err := get(fmt.Sprintf("%s/%d", mediaType, tmdbID), nil, &r); err != nil {
		return nil, err
	}

	result := r.toSearchResult(mediaType)
	applyExternalIDs(&result, mediaType)

	return &result, nil
}

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

	if isTV && len(data.TVResults) > 0 {
		return finalizeImdbResult(data.TVResults[0], "tv", imdbID), nil
	}
	if !isTV && len(data.MovieResults) > 0 {
		return finalizeImdbResult(data.MovieResults[0], "movie", imdbID), nil
	}
	if len(data.MovieResults) > 0 {
		return finalizeImdbResult(data.MovieResults[0], "movie", imdbID), nil
	}
	if len(data.TVResults) > 0 {
		return finalizeImdbResult(data.TVResults[0], "tv", imdbID), nil
	}

	return nil, nil
}

func finalizeImdbResult(m tmdbMedia, mediaType, imdbID string) *mdb.SearchResult {
	result := m.toSearchResult(mediaType)
	result.ImdbID = imdbID
	applyExternalIDs(&result, mediaType)
	if result.IsTV {
		result.TvdbType = "series"
	} else {
		result.TvdbType = "movies"
	}
	return &result
}

func GetExternalIDs(tmdbID int, mediaType string) (tmdbExternalIDsResponse, error) {
	var data tmdbExternalIDsResponse
	if err := get(fmt.Sprintf("%s/%d/external_ids", mediaType, tmdbID), nil, &data); err != nil {
		return tmdbExternalIDsResponse{}, err
	}
	return data, nil
}

func GetEpisodeMetadata(seriesID int, season, episode int) (mdb.EpisodeResult, error) {
	var data tmdbEpisodeResponse
	endpoint := fmt.Sprintf("tv/%d/season/%d/episode/%d", seriesID, season, episode)
	if err := get(endpoint, nil, &data); err != nil {
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
