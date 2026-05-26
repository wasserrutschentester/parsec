package mdb

import "fmt"

type SearchResult struct {
	TmdbID           int
	TmdbType         string
	ImdbID           string
	TvdbID           int
	TvdbType         string
	TvdbSlug         string
	Title            string
	OriginalTitle    string
	OriginalLanguage string
	AltTitle         []string
	Year             int
	IsTV             bool
	Popularity       float64
	Similarity       float64
	Overview         string
}

type EpisodeResult struct {
	Name     string
	Airdate  string
	Overview string
	Season   int
	Episode  int
	TvdbID   int
}

type MatroskaTags struct {
	Title string
	Imdb  string
	Tmdb  string
	Tvdb  int
	Tvdb2 string
}

func PrintResult(result SearchResult) {
	fmt.Println()
	if result.Similarity > 0 {
		fmt.Printf("Found: %s (%d) [Match: %.0f%%]\n", result.Title, result.Year, result.Similarity*100)
	} else {
		fmt.Printf("Found: %s (%d)\n", result.Title, result.Year)
	}
	if result.OriginalTitle != "" && result.OriginalTitle != result.Title {
		fmt.Printf("Original Title: %s\n", result.OriginalTitle)
	}
	if result.OriginalLanguage != "" {
		fmt.Printf("Original Language: %s\n", result.OriginalLanguage)
	}
	if len(result.AltTitle) > 0 {
		fmt.Printf("Alternative Titles: %v\n", result.AltTitle)
	}
	if result.Overview != "" {
		fmt.Printf("Overview: %s\n", result.Overview)
	}

	if result.TmdbID > 0 && result.TmdbType != "" {
		fmt.Printf("TMDB: https://tmdb.org/%s/%d\n", result.TmdbType, result.TmdbID)
	}
	if result.ImdbID != "" {
		fmt.Printf("IMDB: https://imdb.com/title/%s\n", result.ImdbID)
	}
	if result.TvdbSlug != "" && result.TvdbType != "" {
		fmt.Printf("TVDB: https://thetvdb.com/%s/%s\n", result.TvdbType, result.TvdbSlug)
	} else if result.TvdbID > 0 && result.TvdbType != "" {
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

func GetMatroskaTags(result SearchResult) MatroskaTags {
	tags := MatroskaTags{}
	if result.Title != "" {
		tags.Title = result.Title
	}
	if result.ImdbID != "" {
		tags.Imdb = result.ImdbID
	}
	if result.TmdbID > 0 && result.TmdbType != "" {
		tags.Tmdb = fmt.Sprintf("%s/%d", result.TmdbType, result.TmdbID)
	}
	if result.TvdbID > 0 {
		if result.IsTV {
			tags.Tvdb = result.TvdbID
		}

		if result.TvdbType != "" {
			tags.Tvdb2 = fmt.Sprintf("%s/%d", result.TvdbType, result.TvdbID)
		}
	}
	return tags
}

func (tags *MatroskaTags) SetEpisodeTags(result EpisodeResult) {
	if result.Name != "" {
		tags.Title = result.Name
	}
	if result.TvdbID > 0 {
		tags.Tvdb2 = fmt.Sprintf("episodes/%d", result.TvdbID)
	}
}
