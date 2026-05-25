package mdb

import "fmt"

type SearchResult struct {
	TmdbID        int
	TmdbType      string
	ImdbID        string
	TvdbID        int
	TvdbType      string
	WikiDataID    string
	Title         string
	OriginalTitle string
	AltTitle      []string
	Year          int
	IsTV          bool
	Popularity    float64
	Overview      string
}

type EpisodeResult struct {
	Name     string
	Airdate  string
	Overview string
	Season   int
	Episode  int
	TvdbID   int
}

func PrintResult(result SearchResult) {
	fmt.Println()
	fmt.Printf("Found: %s (%d)\n", result.Title, result.Year)
	if result.Overview != "" {
		fmt.Printf("Overview: %s\n", result.Overview)
	}

	if result.TmdbID > 0 && result.TmdbType != "" {
		fmt.Printf("TMDB: https://tmdb.org/%s/%d\n", result.TmdbType, result.TmdbID)
	}
	if result.ImdbID != "" {
		fmt.Printf("IMDB: https://imdb.com/title/%s\n", result.ImdbID)
	}
	if result.TvdbID > 0 && result.TvdbType != "" {
		fmt.Printf("TVDB: https://thetvdb.com/?tab=%s&id=%d\n", result.TvdbType, result.TvdbID)
	}
}

func PrintEpisodeResult(result EpisodeResult) {
	fmt.Println()
	fmt.Printf("%s (S%02dE%02d) Aired on %s\n", result.Name, result.Season, result.Episode, result.Airdate)
	if result.Overview != "" {
		fmt.Printf("Overview: %s\n", result.Overview)
	}
	if result.TvdbID > 0 {
		fmt.Printf("TVDB: https://thetvdb.com/?tab=episode&id=%d\n", result.TvdbID)
	}

}
