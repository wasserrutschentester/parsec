// Package mdb provides interfaces and utilities for interacting with media databases.
package mdb

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"text/template"

	"charm.land/lipgloss/v2"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/ui"
)

// SearchResult represents a media item found in an online database.
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
	Genres           []string
}

// ErrNotFound is returned when no results are found in the database.
var ErrNotFound = errors.New("no result found")

// ErrInvalidTargetValue is returned when a target_value template resolves to a non-integer.
var ErrInvalidTargetValue = errors.New("target_value evaluated to non-integer")

// ErrEmptyTarget is returned when a target_value template evaluates to an empty string.
var ErrEmptyTarget = errors.New("target_value is empty")

// EpisodeResult represents a TV episode found in an online database.
type EpisodeResult struct {
	Name          string
	Airdate       string
	Overview      string
	Season        int
	Episode       int
	TvdbID        int
	TotalEpisodes int
	ImdbID        string
	IsFinale      bool
}

// TagTemplateContext provides metadata to the tag rendering engine.
type TagTemplateContext struct {
	Media       SearchResult
	Episode     *EpisodeResult
	Episodes    []EpisodeResult
	Comment     string
	ReleaseName string
}

// MatroskaTagSet represents metadata tags that can be written to a Matroska file for a specific target.
type MatroskaTagSet struct {
	TargetTypeValue int
	Fields          map[string]string
}

// FormatLanguage returns a human-readable language string.
func FormatLanguage(lang string) string {
	if lang == "zxx" {
		return "zxx (No Dialogue)"
	}

	return lang
}

// PrintResult prints a detailed SearchResult to the UI.
func PrintResult(result SearchResult) {
	title := fmt.Sprintf("%s (%d)", result.Title, result.Year)

	subtitle := ""
	if result.Similarity > 0 {
		subtitle = fmt.Sprintf("[ %.0f%% MATCH ]", result.Similarity*100)
	}

	body := getResultBody(result)
	footer := getResultFooter(result)

	ui.Println(ui.Card(title, subtitle, body, footer))
}

func getResultBody(result SearchResult) string {
	var props [][2]string
	if result.OriginalTitle != "" && result.OriginalTitle != result.Title {
		props = append(props, [2]string{"Origin Title", result.OriginalTitle})
	}

	if result.OriginalLanguage != "" {
		props = append(props, [2]string{"Origin Lang", FormatLanguage(result.OriginalLanguage)})
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

	return body
}

type footerLine struct {
	label string
	id    string
	url   string
}

func getResultFooter(result SearchResult) string {
	items := getFooterItems(result)
	if len(items) == 0 {
		return ""
	}

	maxLen := 0

	for _, item := range items {
		l := len(item.label) + len(item.id) + 2 // "Label: ID"
		if l > maxLen {
			maxLen = l
		}
	}

	var lines []string

	idStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11"))

	for _, item := range items {
		labelPart := ui.Muted.Render(item.label + ":")
		idPart := idStyle.Render(item.id)
		urlLabel := ui.Muted.Render("URL:")
		padding := strings.Repeat(" ", maxLen-(len(item.label)+len(item.id)+2)+3)
		line := fmt.Sprintf("%s %s%s%s %s", labelPart, idPart, padding, urlLabel, ui.Link.Render(item.url))
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func getFooterItems(result SearchResult) []footerLine {
	var items []footerLine

	if result.TmdbID > 0 && result.TmdbType != "" {
		items = append(items, footerLine{
			label: "TMDB ID",
			id:    fmt.Sprintf("%s/%d", result.TmdbType, result.TmdbID),
			url:   fmt.Sprintf("https://tmdb.org/%s/%d", result.TmdbType, result.TmdbID),
		})
	}

	if result.ImdbID != "" {
		items = append(items, footerLine{
			label: "IMDB ID",
			id:    result.ImdbID,
			url:   "https://imdb.com/title/" + result.ImdbID,
		})
	}

	if result.TvdbSlug != "" && result.TvdbType != "" {
		items = append(items, footerLine{
			label: "TVDB ID",
			id:    fmt.Sprintf("%s/%d", result.TvdbType, result.TvdbID),
			url:   fmt.Sprintf("https://thetvdb.com/%s/%s", result.TvdbType, result.TvdbSlug),
		})
	} else if result.TvdbID > 0 && result.TvdbType != "" {
		items = append(items, footerLine{
			label: "TVDB ID",
			id:    fmt.Sprintf("%s/%d", result.TvdbType, result.TvdbID),
			url:   fmt.Sprintf("https://thetvdb.com/?tab=%s&id=%d", result.TvdbType, result.TvdbID),
		})
	}

	return items
}

// PrintEpisodeResult prints a detailed EpisodeResult to the UI.
func PrintEpisodeResult(result EpisodeResult) {
	title := fmt.Sprintf("%s (S%02dE%02d)", result.Name, result.Season, result.Episode)
	subtitle := "Transmission Date: " + result.Airdate

	body := ""
	if result.Overview != "" {
		body = ui.LabelStyle.Render("OVERVIEW") + "\n" + result.Overview
	}

	footer := ""

	var footerParts []string

	idStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11"))

	if result.ImdbID != "" {
		labelPart := ui.Muted.Render("IMDb ID:")
		idPart := idStyle.Render(result.ImdbID)
		urlLabel := ui.Muted.Render("URL:")
		link := ui.Link.Render("https://www.imdb.com/title/" + result.ImdbID)
		footerParts = append(footerParts, fmt.Sprintf("%s %s   %s %s", labelPart, idPart, urlLabel, link))
	}

	if result.TvdbID > 0 {
		labelPart := ui.Muted.Render("TVDB ID:")
		idPart := idStyle.Render(strconv.Itoa(result.TvdbID))
		urlLabel := ui.Muted.Render("URL:")
		link := ui.Link.Render(fmt.Sprintf("https://thetvdb.com/?tab=episode&id=%d", result.TvdbID))
		footerParts = append(footerParts, fmt.Sprintf("%s %s   %s %s", labelPart, idPart, urlLabel, link))
	}

	footer = strings.Join(footerParts, "\n")

	ui.Println(ui.Card(title, subtitle, body, footer))
}

// PrintCompactResult prints a one-line summary of a SearchResult to the UI.
func PrintCompactResult(result SearchResult) {
	title := fmt.Sprintf("%s (%d) [OV: %s]", result.Title, result.Year, FormatLanguage(result.OriginalLanguage))

	ids := ""
	if result.TmdbID > 0 && result.TmdbType != "" {
		ids = fmt.Sprintf("TMDB: %s/%d,", result.TmdbType, result.TmdbID)
	}

	if result.ImdbID != "" {
		ids += fmt.Sprintf(" IMDb: %s,", result.ImdbID)
	}

	if result.TvdbID > 0 {
		ids += fmt.Sprintf(" TVDB: %d", result.TvdbID)
	}

	ids = strings.TrimSuffix(ids, ",")
	ids = strings.TrimSpace(ids)
	ui.Println("Match:", title, ids)
}

// PrintCompactEpisodeResult prints a one-line summary of an EpisodeResult to the UI.
func PrintCompactEpisodeResult(result EpisodeResult) {
	// indented to align to Match:
	ui.Println(fmt.Sprintf("       %s (S%02dE%02d) %s", result.Name, result.Season, result.Episode, result.Airdate))
}

// CombineEpisodeNames joins the names of a slice of EpisodeResult with a slash.
func CombineEpisodeNames(episodes []EpisodeResult) string {
	if len(episodes) == 0 {
		return ""
	}

	titles := make([]string, 0, len(episodes))
	for _, ep := range episodes {
		titles = append(titles, ep.Name)
	}

	return strings.Join(titles, " / ")
}

// ExtractEpisodeNumbers extracts all episode numbers from a slice of EpisodeResult.
func ExtractEpisodeNumbers(episodes []EpisodeResult) []int {
	if len(episodes) == 0 {
		return nil
	}

	nums := make([]int, 0, len(episodes))
	for _, ep := range episodes {
		nums = append(nums, ep.Episode)
	}

	return nums
}

// GetMatroskaTags creates a slice of MatroskaTagSet from a TagTemplateContext using templates.
func GetMatroskaTags(ctx TagTemplateContext) ([]MatroskaTagSet, error) {
	configs, err := config.GetTagProfile()
	if err != nil {
		return nil, err
	}

	var tagSets []MatroskaTagSet

	for _, cfg := range configs {
		tagSetsFromCfg, err := evaluateTagConfig(cfg, ctx)
		if err != nil {
			if errors.Is(err, ErrEmptyTarget) {
				continue
			}

			return nil, err
		}

		tagSets = append(tagSets, tagSetsFromCfg...)
	}

	return tagSets, nil
}

var templateFuncs = template.FuncMap{
	"join":    strings.Join,
	"upper":   strings.ToUpper,
	"lower":   strings.ToLower,
	"trim":    strings.TrimSpace,
	"replace": strings.ReplaceAll,
	"title":   cases.Title(language.Und).String,
	"add":     func(a, b int) int { return a + b },
	"sub":     func(a, b int) int { return a - b },
	"mul":     func(a, b int) int { return a * b },
	"div":     func(a, b int) int { return a / b },
}

func evaluateTagConfig(cfg config.TagConfig, ctx TagTemplateContext) ([]MatroskaTagSet, error) {
	if cfg.Iterator == "Episodes" && len(ctx.Episodes) > 0 {
		var allSets []MatroskaTagSet

		for i := range ctx.Episodes {
			ctxCopy := ctx
			ctxCopy.Episode = &ctx.Episodes[i]

			tagSet, err := evaluateSingleContext(cfg, ctxCopy)
			if err != nil {
				if errors.Is(err, ErrEmptyTarget) {
					continue
				}

				return nil, err
			}

			allSets = append(allSets, *tagSet)
		}

		if len(allSets) == 0 {
			return nil, ErrEmptyTarget
		}

		return allSets, nil
	}

	// Default behavior (single context)
	tagSet, err := evaluateSingleContext(cfg, ctx)
	if err != nil {
		return nil, err
	}

	return []MatroskaTagSet{*tagSet}, nil
}

func evaluateSingleContext(cfg config.TagConfig, ctx TagTemplateContext) (*MatroskaTagSet, error) {
	tmplTarget, err := template.New("target_value").Funcs(templateFuncs).Parse(cfg.TargetValue)
	if err != nil {
		return nil, fmt.Errorf("invalid template for target_value: %w", err)
	}

	var targetBuf bytes.Buffer
	if err := tmplTarget.Execute(&targetBuf, ctx); err != nil {
		return nil, fmt.Errorf("failed to execute target_value template: %w", err)
	}

	targetStr := targetBuf.String()
	if targetStr == "" {
		return nil, ErrEmptyTarget
	}

	targetVal, err := strconv.Atoi(targetStr)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidTargetValue, targetStr)
	}

	tagSet := &MatroskaTagSet{TargetTypeValue: targetVal, Fields: make(map[string]string)}
	for k, v := range cfg.Fields {
		tmpl, err := template.New(k).Funcs(templateFuncs).Parse(v)
		if err != nil {
			return nil, fmt.Errorf("invalid tag template for %s: %w", k, err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, ctx); err != nil {
			return nil, fmt.Errorf("failed to execute tag template for %s: %w", k, err)
		}

		if buf.String() != "" {
			tagSet.Fields[k] = buf.String()
		}
	}

	return tagSet, nil
}

// CalculateSimilarity calculates the Levenshtein similarity between two strings (0.0 to 1.0).
func CalculateSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}

	if len(s1) == 0 || len(s2) == 0 {
		return 0.0
	}

	dist := levenshteinDistance(s1, s2)
	maxLen := math.Max(float64(len(s1)), float64(len(s2)))

	return 1.0 - (float64(dist) / maxLen)
}

func levenshteinDistance(s1, s2 string) int {
	r1, r2 := []rune(s1), []rune(s2)
	n, m := len(r1), len(r2)

	if n > m {
		r1, r2 = r2, r1
		n, m = m, n
	}

	row := make([]int, n+1)
	for i := 0; i <= n; i++ {
		row[i] = i
	}

	for j := 1; j <= m; j++ {
		prev := j

		for i := 1; i <= n; i++ {
			var cost int
			if r1[i-1] != r2[j-1] {
				cost = 1
			}

			newVal := min(row[i]+1, prev+1, row[i-1]+cost)
			row[i-1] = prev
			prev = newVal
		}

		row[n] = prev
	}

	return row[n]
}
