package imdb

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"codeberg.org/upPollo/parsec/internal/mdb"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/ui"
)

const (
	getTitleInfoQuery = `query GetTitleInfo($id: ID!) {
  title(id: $id) {
    id
    titleText { text }
    originalTitleText { text }
    releaseYear { year endYear }
    titleType { id isSeries }
    runtime { seconds }
    plot { plotText { plainText } }
    ratingsSummary { aggregateRating voteCount }
    certificate { rating }
    principalCredits {
      category { id text }
      credits {
        ... on Cast {
          characters {
            name
          }
        }
        name {
          id
          nameText { text }
        }
      }
    }
    titleGenres { genres { genre { text } } }
    countriesOfOrigin { countries { id text } }
    spokenLanguages { spokenLanguages { id text } }
    akas(first: 50) {
      edges {
        node {
          text
        }
      }
    }
    episodes {
      episodes(first: 500) {
        total
        edges {
          node {
            id
            titleText { text }
            releaseDate { year month day }
            plot { plotText { plainText } }
            runtime { seconds }
            series {
              displayableEpisodeNumber {
                displayableSeason { season }
                episodeNumber { text }
              }
            }
          }
        }
      }
    }
  }
}`

	searchTitlesQuery = `query SearchTitles($constraints: AdvancedTitleSearchConstraints!) {
  advancedTitleSearch(first: 10, constraints: $constraints) {
    total
    edges {
      node {
        title {
          id
          titleText { text }
          originalTitleText { text }
          titleType { id isSeries }
          releaseYear { year }
          plot { plotText { plainText } }
          ratingsSummary { voteCount }
        }
      }
    }
  }
}`
)

type titleData struct {
	ID        string `json:"id"`
	TitleText struct {
		Text string `json:"text"`
	} `json:"titleText"`
	OriginalTitleText struct {
		Text string `json:"text"`
	} `json:"originalTitleText"`
	ReleaseYear struct {
		Year    int `json:"year"`
		EndYear int `json:"endYear"`
	} `json:"releaseYear"`
	TitleType struct {
		ID       string `json:"id"`
		IsSeries bool   `json:"isSeries"`
	} `json:"titleType"`
	Runtime struct {
		Seconds int `json:"seconds"`
	} `json:"runtime"`
	Plot struct {
		PlotText struct {
			PlainText string `json:"plainText"`
		} `json:"plotText"`
	} `json:"plot"`
	RatingsSummary struct {
		AggregateRating float64 `json:"aggregateRating"`
		VoteCount       int     `json:"voteCount"`
	} `json:"ratingsSummary"`
	Certificate struct {
		Rating string `json:"rating"`
	} `json:"certificate"`
	PrincipalCredits []struct {
		Category struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"category"`
		Credits []struct {
			Characters []struct {
				Name string `json:"name"`
			} `json:"characters"`
			Name struct {
				ID       string `json:"id"`
				NameText struct {
					Text string `json:"text"`
				} `json:"nameText"`
			} `json:"name"`
		} `json:"credits"`
	} `json:"principalCredits"`
	TitleGenres struct {
		Genres []struct {
			Genre struct {
				Text string `json:"text"`
			} `json:"genre"`
		} `json:"genres"`
	} `json:"titleGenres"`
	CountriesOfOrigin struct {
		Countries []struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"countries"`
	} `json:"countriesOfOrigin"`
	SpokenLanguages struct {
		SpokenLanguages []struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"spokenLanguages"`
	} `json:"spokenLanguages"`
	Akas struct {
		Edges []struct {
			Node struct {
				Text string `json:"text"`
			} `json:"node"`
		} `json:"edges"`
	} `json:"akas"`
	Episodes struct {
		Episodes struct {
			Total int `json:"total"`
			Edges []struct {
				Node struct {
					ID        string `json:"id"`
					TitleText struct {
						Text string `json:"text"`
					} `json:"titleText"`
					ReleaseDate struct {
						Year  int `json:"year"`
						Month int `json:"month"`
						Day   int `json:"day"`
					} `json:"releaseDate"`
					Plot struct {
						PlotText struct {
							PlainText string `json:"plainText"`
						} `json:"plotText"`
					} `json:"plot"`
					Runtime struct {
						Seconds int `json:"seconds"`
					} `json:"runtime"`
					Series struct {
						DisplayableEpisodeNumber struct {
							DisplayableSeason struct {
								Season string `json:"season"`
							} `json:"displayableSeason"`
							EpisodeNumber struct {
								Text string `json:"text"`
							} `json:"episodeNumber"`
						} `json:"displayableEpisodeNumber"`
					} `json:"series"`
				} `json:"node"`
			} `json:"edges"`
		} `json:"episodes"`
	} `json:"episodes"`
}

func (t *titleData) computeStatus() string {
	if t.TitleType.IsSeries {
		if t.ReleaseYear.EndYear > 0 {
			return "Ended"
		}

		return "Continuing"
	}

	if t.ReleaseYear.Year > 0 {
		return "Released"
	}

	return ""
}

func appendCastMember(cast []mdb.CastMember, name string, chars []struct {
	Name string `json:"name"`
},
) []mdb.CastMember {
	if slices.ContainsFunc(cast, func(cm mdb.CastMember) bool { return cm.Name == name }) {
		return cast
	}

	role := ""
	if len(chars) > 0 {
		role = strings.TrimSpace(chars[0].Name)
	}

	return append(cast, mdb.CastMember{Name: name, Role: role})
}

func (t *titleData) extractCredits() (directors, writers []string, cast []mdb.CastMember) {
	for _, pc := range t.PrincipalCredits {
		cat := strings.ToLower(pc.Category.ID)
		for _, c := range pc.Credits {
			name := strings.TrimSpace(c.Name.NameText.Text)
			if name == "" {
				continue
			}

			switch cat {
			case "director":
				if !slices.Contains(directors, name) {
					directors = append(directors, name)
				}
			case "writer", "creator":
				if !slices.Contains(writers, name) {
					writers = append(writers, name)
				}
			case "cast", "actor", "actress":
				cast = appendCastMember(cast, name, c.Characters)
			}
		}
	}

	return directors, writers, cast
}

func (t *titleData) extractGenres() []string {
	var genres []string

	for _, g := range t.TitleGenres.Genres {
		if g.Genre.Text != "" {
			genres = append(genres, g.Genre.Text)
		}
	}

	return genres
}

func (t *titleData) extractCountries() []string {
	var countries []string

	for _, c := range t.CountriesOfOrigin.Countries {
		code := c.ID
		if code == "" {
			code = c.Text
		}

		if code != "" {
			countries = append(countries, strings.ToUpper(code))
		}
	}

	return countries
}

func (t *titleData) extractLanguages() ([]LanguageItem, string) {
	var spokenLanguages []LanguageItem

	primaryLang := ""

	for _, l := range t.SpokenLanguages.SpokenLanguages {
		iso := NormalizeLanguage(l.ID, l.Text)
		if iso != "" {
			spokenLanguages = append(spokenLanguages, LanguageItem{
				ID:   iso,
				Text: l.Text,
			})

			if primaryLang == "" {
				primaryLang = iso
			}
		}
	}

	return spokenLanguages, primaryLang
}

func (t *titleData) extractAltTitles() []string {
	var altTitles []string

	for _, edge := range t.Akas.Edges {
		text := strings.TrimSpace(edge.Node.Text)
		if text != "" && text != t.TitleText.Text && !slices.Contains(altTitles, text) {
			altTitles = append(altTitles, text)
		}
	}

	return altTitles
}

func (t *titleData) extractEpisodes() []EpisodeDetail {
	episodes := make([]EpisodeDetail, 0, len(t.Episodes.Episodes.Edges))

	for _, edge := range t.Episodes.Episodes.Edges {
		epNode := edge.Node
		seasonVal, _ := strconv.Atoi(epNode.Series.DisplayableEpisodeNumber.DisplayableSeason.Season)
		epVal, _ := strconv.Atoi(epNode.Series.DisplayableEpisodeNumber.EpisodeNumber.Text)

		episodes = append(episodes, EpisodeDetail{
			ID:          epNode.ID,
			Title:       epNode.TitleText.Text,
			ReleaseDate: formatDate(epNode.ReleaseDate.Year, epNode.ReleaseDate.Month, epNode.ReleaseDate.Day),
			Season:      seasonVal,
			Episode:     epVal,
			Runtime:     epNode.Runtime.Seconds / 60,
			Overview:    epNode.Plot.PlotText.PlainText,
		})
	}

	if total := t.Episodes.Episodes.Total; total > 0 && len(episodes) < total {
		ui.PrintDebug(fmt.Sprintf("imdb: %s has %d episodes total but only %d fetched (pagination limit of 500)", t.ID, total, len(episodes)))
	}

	return episodes
}

type titleInfoResponse struct {
	Data struct {
		Title titleData `json:"title"`
	} `json:"data"`
}

type searchTitleNode struct {
	ID        string `json:"id"`
	TitleText struct {
		Text string `json:"text"`
	} `json:"titleText"`
	OriginalTitleText struct {
		Text string `json:"text"`
	} `json:"originalTitleText"`
	TitleType struct {
		ID       string `json:"id"`
		IsSeries bool   `json:"isSeries"`
	} `json:"titleType"`
	ReleaseYear struct {
		Year int `json:"year"`
	} `json:"releaseYear"`
	Plot struct {
		PlotText struct {
			PlainText string `json:"plainText"`
		} `json:"plotText"`
	} `json:"plot"`
	RatingsSummary struct {
		VoteCount int `json:"voteCount"`
	} `json:"ratingsSummary"`
}

type searchResponse struct {
	Data struct {
		AdvancedTitleSearch struct {
			Total int `json:"total"`
			Edges []struct {
				Node struct {
					Title searchTitleNode `json:"title"`
				} `json:"node"`
			} `json:"edges"`
		} `json:"advancedTitleSearch"`
	} `json:"data"`
}

// GetByID retrieves a media item by IMDb ID and maps it to mdb.SearchResult.
func GetByID(ctx context.Context, imdbID string) (*mdb.SearchResult, error) {
	details, err := GetTitleDetails(ctx, imdbID)
	if err != nil {
		return nil, err
	}

	result := details.ToSearchResult()

	return &result, nil
}

// GetTitleDetails retrieves full title metadata from IMDb.
func GetTitleDetails(ctx context.Context, imdbID string) (*TitleDetails, error) {
	id := FormatIMDbID(imdbID)
	if id == "" {
		return nil, ErrNotFound
	}

	var resp titleInfoResponse
	if err := executeGraphQL(ctx, "GetTitleInfo", getTitleInfoQuery, map[string]any{"id": id}, &resp); err != nil {
		return nil, err
	}

	t := &resp.Data.Title
	if t.ID == "" || t.TitleText.Text == "" {
		return nil, ErrNotFound
	}

	origText := t.OriginalTitleText.Text
	if origText == "" {
		origText = t.TitleText.Text
	}

	spokenLanguages, primaryLang := t.extractLanguages()
	directors, writers, cast := t.extractCredits()

	return &TitleDetails{
		IMDbID:           t.ID,
		Title:            t.TitleText.Text,
		OriginalTitle:    origText,
		OriginalLanguage: primaryLang,
		SpokenLanguages:  spokenLanguages,
		Year:             t.ReleaseYear.Year,
		EndYear:          t.ReleaseYear.EndYear,
		IsTV:             t.TitleType.IsSeries,
		Type:             mdb.TitleType(t.TitleType.ID),
		RuntimeMinutes:   t.Runtime.Seconds / 60,
		Overview:         t.Plot.PlotText.PlainText,
		Rating:           t.RatingsSummary.AggregateRating,
		Votes:            t.RatingsSummary.VoteCount,
		Certificate:      t.Certificate.Rating,
		Genres:           t.extractGenres(),
		Countries:        t.extractCountries(),
		AltTitles:        t.extractAltTitles(),
		Status:           t.computeStatus(),
		Directors:        directors,
		Writers:          writers,
		Cast:             cast,
		Episodes:         t.extractEpisodes(),
	}, nil
}

func parseSearchTitle(t searchTitleNode, isTV bool) (mdb.SearchResult, bool) {
	if t.ID == "" {
		return mdb.SearchResult{}, false
	}

	titleType := mdb.TitleType(t.TitleType.ID)
	if titleType.IsExcludedFromSearch(isTV) {
		return mdb.SearchResult{}, false
	}

	if isTV != t.TitleType.IsSeries && (isTV || t.TitleType.IsSeries) {
		return mdb.SearchResult{}, false
	}

	orig := t.OriginalTitleText.Text
	if orig == "" {
		orig = t.TitleText.Text
	}

	return mdb.SearchResult{
		ImdbID:        t.ID,
		Title:         t.TitleText.Text,
		OriginalTitle: orig,
		Year:          t.ReleaseYear.Year,
		IsTV:          t.TitleType.IsSeries,
		TitleType:     titleType,
		Overview:      t.Plot.PlotText.PlainText,
		Votes:         t.RatingsSummary.VoteCount,
	}, true
}

// Search searches for titles by query string and optional year/category.
func Search(ctx context.Context, query string, year int, isTV bool) ([]mdb.SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	constraints := map[string]any{
		"titleTextConstraint": map[string]any{"searchTerm": query},
	}
	if year > 0 {
		constraints["releaseDateConstraint"] = map[string]any{
			"releaseDateRange": map[string]any{
				"start": fmt.Sprintf("%d-01-01", year-1),
				"end":   fmt.Sprintf("%d-12-31", year+1),
			},
		}
	}

	var resp searchResponse
	if err := executeGraphQL(ctx, "SearchTitles", searchTitlesQuery, map[string]any{"constraints": constraints}, &resp); err != nil {
		return nil, err
	}

	var results []mdb.SearchResult

	for _, edge := range resp.Data.AdvancedTitleSearch.Edges {
		if res, ok := parseSearchTitle(edge.Node.Title, isTV); ok {
			results = append(results, res)
		}
	}

	return results, nil
}

// GetSeasonEpisodes returns all episodes for a specific season from a series.
func GetSeasonEpisodes(ctx context.Context, imdbID string, season int) ([]mdb.EpisodeResult, error) {
	details, err := GetTitleDetails(ctx, imdbID)
	if err != nil {
		return nil, err
	}

	totalInSeason := countSeasonEpisodes(details.Episodes, season)

	var results []mdb.EpisodeResult

	for _, ep := range details.Episodes {
		if ep.Season == season {
			results = append(results, ep.ToEpisodeResult(totalInSeason))
		}
	}

	return results, nil
}

func matchEpisodeByNumber(episodes []EpisodeDetail, season, epNum int) *EpisodeDetail {
	if season <= 0 || epNum <= 0 {
		return nil
	}

	for i := range episodes {
		if episodes[i].Season == season && episodes[i].Episode == epNum {
			return &episodes[i]
		}
	}

	return nil
}

func matchEpisodeByDate(episodes []EpisodeDetail, date string, allowSpecials bool) *EpisodeDetail {
	if date == "" {
		return nil
	}

	for i := range episodes {
		if episodes[i].ReleaseDate == date {
			if !allowSpecials && episodes[i].Season == 0 {
				continue
			}

			return &episodes[i]
		}
	}

	return nil
}

func matchEpisodeByTitle(episodes []EpisodeDetail, titles []string, allowSpecials bool) *EpisodeDetail {
	if len(titles) == 0 {
		return nil
	}

	normQuery := metadata.Normalize(strings.Join(titles, " / "))

	for i := range episodes {
		if !allowSpecials && episodes[i].Season == 0 {
			continue
		}

		if metadata.Normalize(episodes[i].Title) == normQuery {
			return &episodes[i]
		}
	}

	return nil
}

// IdentifyEpisode finds a matching episode for a series using metadata.
func IdentifyEpisode(ctx context.Context, result mdb.SearchResult, meta *metadata.Metadata, allowSpecials bool) (mdb.EpisodeResult, error) {
	if result.ImdbID == "" {
		return mdb.EpisodeResult{}, ErrNotFound
	}

	details, err := GetTitleDetails(ctx, result.ImdbID)
	if err != nil {
		return mdb.EpisodeResult{}, err
	}

	epNum := 0
	if len(meta.Episodes) > 0 {
		epNum = meta.Episodes[0]
	}

	match := matchEpisodeByNumber(details.Episodes, meta.Season, epNum)
	if match == nil {
		match = matchEpisodeByDate(details.Episodes, meta.Date, allowSpecials)
	}

	if match == nil {
		match = matchEpisodeByTitle(details.Episodes, meta.EpisodeTitles, allowSpecials)
	}

	if match != nil {
		epRes := match.ToEpisodeResult(countSeasonEpisodes(details.Episodes, match.Season))
		enrichEpisodeDetails(ctx, &epRes, match.ID)

		return epRes, nil
	}

	return mdb.EpisodeResult{}, ErrNotFound
}

func enrichEpisodeDetails(ctx context.Context, epRes *mdb.EpisodeResult, imdbID string) {
	epDetails, err := GetTitleDetails(ctx, imdbID)
	if err != nil || epDetails == nil {
		return
	}

	epRes.Directors = epDetails.Directors
	epRes.Writers = epDetails.Writers
	epRes.Genres = epDetails.Genres
	epRes.Rating = epDetails.Rating
	epRes.Votes = epDetails.Votes

	if epRes.Overview == "" {
		epRes.Overview = epDetails.Overview
	}

	if epRes.Runtime == 0 {
		epRes.Runtime = epDetails.RuntimeMinutes
	}
}

func countSeasonEpisodes(episodes []EpisodeDetail, season int) int {
	count := 0

	for _, ep := range episodes {
		if ep.Season == season {
			count++
		}
	}

	return count
}

// ToEpisodeResult converts an EpisodeDetail to an mdb.EpisodeResult.
func (e EpisodeDetail) ToEpisodeResult(totalSeasonEpisodes int) mdb.EpisodeResult {
	return mdb.EpisodeResult{
		Name:          e.Title,
		Airdate:       e.ReleaseDate,
		Overview:      e.Overview,
		Season:        e.Season,
		Episode:       e.Episode,
		Runtime:       e.Runtime,
		ImdbID:        e.ID,
		TotalEpisodes: totalSeasonEpisodes,
	}
}

// ToSearchResult maps TitleDetails to mdb.SearchResult.
func (t *TitleDetails) ToSearchResult() mdb.SearchResult {
	return mdb.SearchResult{
		ImdbID:           t.IMDbID,
		Title:            t.Title,
		OriginalTitle:    t.OriginalTitle,
		OriginalLanguage: t.OriginalLanguage,
		AltTitle:         t.AltTitles,
		Year:             t.Year,
		Runtime:          t.RuntimeMinutes,
		IsTV:             t.IsTV,
		TitleType:        t.Type,
		Rating:           t.Rating,
		Votes:            t.Votes,
		Certificate:      t.Certificate,
		Overview:         t.Overview,
		Genres:           t.Genres,
		Countries:        t.Countries,
		Status:           t.Status,
		Directors:        t.Directors,
		Writers:          t.Writers,
		Cast:             t.Cast,
	}
}

func formatDate(year, month, day int) string {
	if year <= 0 {
		return ""
	}

	if month <= 0 || day <= 0 {
		return fmt.Sprintf("%04d", year)
	}

	return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
}
