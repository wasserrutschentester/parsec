// Package imdb provides a client for IMDb's GraphQL API.
package imdb

// LanguageItem represents a spoken language with its normalized ISO code and display text.
type LanguageItem struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// EpisodeDetail represents an individual episode item in an IMDb series listing.
type EpisodeDetail struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	ReleaseDate string `json:"releaseDate"`
	Season      int    `json:"season"`
	Episode     int    `json:"episode"`
	Runtime     int    `json:"runtime"`
	Overview    string `json:"overview"`
}

// TitleDetails contains the title metadata collected from IMDb.
type TitleDetails struct {
	IMDbID           string          `json:"imdbId"`
	Title            string          `json:"title"`
	OriginalTitle    string          `json:"originalTitle"`
	OriginalLanguage string          `json:"originalLanguage"`
	SpokenLanguages  []LanguageItem  `json:"spokenLanguages"`
	Year             int             `json:"year"`
	EndYear          int             `json:"endYear"`
	IsTV             bool            `json:"isTv"`
	Type             string          `json:"type"`
	RuntimeMinutes   int             `json:"runtimeMinutes"`
	Overview         string          `json:"overview"`
	Genres           []string        `json:"genres"`
	Countries        []string        `json:"countries"`
	AltTitles        []string        `json:"altTitles"`
	Status           string          `json:"status"`
	Episodes         []EpisodeDetail `json:"episodes"`
}

// Internal GraphQL protocol types

type graphQLRequest struct {
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
	Query         string         `json:"query"`
}

type graphQLErrorEnvelope struct {
	Errors []struct {
		Message    string `json:"message"`
		Extensions struct {
			Code string `json:"code"`
		} `json:"extensions"`
	} `json:"errors"`
}
