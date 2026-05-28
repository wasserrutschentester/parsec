package mdb

import (
	"fmt"
	"strings"

	"codeberg.org/n0ne/parsec/internal/ui"
)

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
	title := fmt.Sprintf("%s (%d)", result.Title, result.Year)
	subtitle := ""
	if result.Similarity > 0 {
		subtitle = fmt.Sprintf("[ %.0f%% MATCH ]", result.Similarity*100)
	}

	var props [][2]string
	if result.OriginalTitle != "" && result.OriginalTitle != result.Title {
		props = append(props, [2]string{"Original Title", result.OriginalTitle})
	}
	if result.OriginalLanguage != "" {
		props = append(props, [2]string{"Language", result.OriginalLanguage})
	}
	if len(result.AltTitle) > 0 {
		props = append(props, [2]string{"Alt Titles", strings.Join(result.AltTitle, ", ")})
	}

	body := ui.PropertyLayout(props)
	if result.Overview != "" {
		if body != "" {
			body += "\n\n"
		}
		body += ui.LabelStyle.Render("OVERVIEW") + "\n" + result.Overview
	}

	var links []string
	if result.TmdbID > 0 && result.TmdbType != "" {
		links = append(links, ui.Link.Render(fmt.Sprintf("https://tmdb.org/%s/%d", result.TmdbType, result.TmdbID)))
	}
	if result.ImdbID != "" {
		links = append(links, ui.Link.Render(fmt.Sprintf("https://imdb.com/title/%s", result.ImdbID)))
	}
	if result.TvdbSlug != "" && result.TvdbType != "" {
		links = append(links, ui.Link.Render(fmt.Sprintf("https://thetvdb.com/%s/%s", result.TvdbType, result.TvdbSlug)))
	} else if result.TvdbID > 0 && result.TvdbType != "" {
		links = append(links, ui.Link.Render(fmt.Sprintf("https://thetvdb.com/?tab=%s&id=%d", result.TvdbType, result.TvdbID)))
	}

	footer := strings.Join(links, "  ")

	ui.Println(ui.Card(title, subtitle, body, footer))
}

func PrintEpisodeResult(result EpisodeResult) {
	title := fmt.Sprintf("%s (S%02dE%02d)", result.Name, result.Season, result.Episode)
	subtitle := fmt.Sprintf("Aired: %s", result.Airdate)

	body := ""
	if result.Overview != "" {
		body = ui.LabelStyle.Render("OVERVIEW") + "\n" + result.Overview
	}

	footer := ""
	if result.TvdbID > 0 {
		footer = ui.Link.Render(fmt.Sprintf("https://thetvdb.com/?tab=episode&id=%d", result.TvdbID))
	}

	ui.Println(ui.Card(title, subtitle, body, footer))
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
