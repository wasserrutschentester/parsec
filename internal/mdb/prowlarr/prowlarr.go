package prowlarr

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"codeberg.org/upPollo/parsec/internal/cache"
	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/mdb"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/filename"
	"codeberg.org/upPollo/parsec/internal/ui"
)

type ReleaseResource struct {
	Title       string  `json:"title"`
	Size        int64   `json:"size"`
	Indexer     string  `json:"indexer"`
	Seeders     int     `json:"seeders"`
	Leechers    int     `json:"leechers"`
	PublishDate string  `json:"publishDate"`
	ImdbID      int64   `json:"imdbId"`
	TmdbID      int64   `json:"tmdbId"`
	TvdbID      int64   `json:"tvdbId"`
	IndexerId   int     `json:"indexerId"`
	InfoUrl     string  `json:"infoUrl"`
	DownloadUrl string  `json:"downloadUrl"`
	MagnetUrl   string  `json:"magnetUrl"`
	Protocol    string  `json:"protocol"`
	Age         int     `json:"age"`
	AgeHours    float64 `json:"ageHours"`
	AgeMinutes  float64 `json:"ageMinutes"`
	Guid        string  `json:"guid"`
}

func Search(imdbID string, tmdbID, tvdbID, season, episode int, isTV bool) ([]ReleaseResource, error) {
	queries := make([]string, 0)

	suffix := ""
	if season > 0 {
		suffix += fmt.Sprintf(" {Season:%d}", season)
	}

	if episode > 0 {
		suffix += fmt.Sprintf(" {Episode:%d}", episode)
	}

	if imdbID != "" {
		queries = append(queries, fmt.Sprintf("{ImdbId:%s}%s", imdbID, suffix))
	}

	if tmdbID > 0 {
		queries = append(queries, fmt.Sprintf("{TmdbId:%d}%s", tmdbID, suffix))
	}

	if tvdbID > 0 {
		queries = append(queries, fmt.Sprintf("{TvdbId:%d}%s", tvdbID, suffix))
	}

	if len(queries) == 0 {
		return nil, fmt.Errorf("at least one ID (IMDB, TMDB, or TVDB) is required for Prowlarr search")
	}

	mediaType := "movie"
	categories := config.GetProwlarrMovieCategories()

	if isTV {
		mediaType = "tvsearch"
		categories = config.GetProwlarrTvCategories()
	}

	allResults, err := performParallelSearch(queries, mediaType, categories, season, episode)
	if err != nil {
		return nil, err
	}

	return filterFalsePositives(allResults, imdbID, tmdbID, tvdbID), nil
}

func performParallelSearch(queries []string, mediaType string, categories []int, season, episode int) ([]ReleaseResource, error) {
	prowlarrUrl := config.GetProwlarrUrl()
	apiKey := config.GetProwlarrApiKey()
	indexerIds := config.GetProwlarrIndexers()

	if prowlarrUrl == "" || apiKey == "" {
		return nil, fmt.Errorf("prowlarr is not configured (url or api_key missing)")
	}

	var wg sync.WaitGroup

	resultsChan := make(chan []ReleaseResource, len(queries))
	errChan := make(chan error, len(queries))

	for _, query := range queries {
		wg.Add(1)
		go func(q string) {
			defer wg.Done()

			searchURL, err := buildSearchURL(prowlarrUrl, q, mediaType, categories, indexerIds)
			if err != nil {
				errChan <- err
				return
			}

			results, err := fetchReleases(searchURL, apiKey)
			if err != nil {
				errChan <- err
				return
			}

			resultsChan <- results
		}(query)
	}

	wg.Wait()
	close(resultsChan)
	close(errChan)

	if len(errChan) > 0 && len(resultsChan) == 0 {
		return nil, <-errChan
	}

	return processParallelResults(resultsChan), nil
}

func processParallelResults(resultsChan <-chan []ReleaseResource) []ReleaseResource {
	var allResults []ReleaseResource

	seenGuids := make(map[string]bool)
	duplicates := 0

	for results := range resultsChan {
		for _, r := range results {
			if !seenGuids[r.Guid] {
				allResults = append(allResults, r)
				seenGuids[r.Guid] = true
			} else {
				duplicates++
			}
		}
	}

	if duplicates > 0 {
		ui.PrintDebug(fmt.Sprintf("Removed %d duplicate Prowlarr results across multiple queries", duplicates))
	}

	return allResults
}

func buildSearchURL(baseURL, searchQuery, mediaType string, categories, indexerIds []int) (*url.URL, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid prowlarr url: %w", err)
	}

	u.Path = "/api/v1/search"

	q := u.Query()
	q.Set("type", mediaType)
	q.Set("query", searchQuery)

	for _, cat := range categories {
		q.Add("categories", strconv.Itoa(cat))
	}

	for _, id := range indexerIds {
		q.Add("indexerIds", strconv.Itoa(id))
	}

	ui.PrintDebug(fmt.Sprintf("query: %s", q))
	u.RawQuery = q.Encode()

	return u, nil
}

func fetchReleases(searchURL *url.URL, apiKey string) ([]ReleaseResource, error) {
	cacheKey := fmt.Sprintf("prowlarr:%s", searchURL.String())
	if cached, err := cache.Get(cacheKey); err == nil {
		var results []ReleaseResource
		if err := json.Unmarshal(cached, &results); err == nil {
			ui.PrintDebug(fmt.Sprintf("Prowlarr cache hit: %s", searchURL.String()))
			return results, nil
		}
	}

	ui.PrintDebug(fmt.Sprintf("Prowlarr search URL: %s", searchURL.String()))

	start := time.Now()

	req, err := http.NewRequest(http.MethodGet, searchURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("prowlarr request failed: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	duration := time.Since(start)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("prowlarr returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read prowlarr response: %w", err)
	}

	var results []ReleaseResource
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("failed to decode prowlarr response: %w", err)
	}

	ui.PrintDebug(fmt.Sprintf("Prowlarr request took %v, returned %d results", duration, len(results)))

	_ = cache.Set(cacheKey, body)

	return results, nil
}

func filterFalsePositives(results []ReleaseResource, imdbID string, tmdbID, tvdbID int) []ReleaseResource {
	filtered := make([]ReleaseResource, 0)

	var imdbInt int64

	if imdbID != "" {
		cleanImdb := strings.TrimPrefix(imdbID, "tt")
		imdbInt, _ = strconv.ParseInt(cleanImdb, 10, 64)
	}

	type indexerStat struct {
		total   int
		matched int
	}

	stats := make(map[string]indexerStat)

	for _, r := range results {
		match := false

		if imdbID != "" && r.ImdbID == imdbInt {
			match = true
		}

		if tmdbID > 0 && r.TmdbID == int64(tmdbID) {
			match = true
		}

		if tvdbID > 0 && r.TvdbID == int64(tvdbID) {
			match = true
		}

		s := stats[r.Indexer]
		s.total++

		if match {
			s.matched++

			filtered = append(filtered, r)
		} else {
			ui.PrintDebug(fmt.Sprintf("Prowlarr result '%s' from %s filtered out (IMDB: %d, TMDB: %d, TVDB: %d)", r.Title, r.Indexer, r.ImdbID, r.TmdbID, r.TvdbID))
		}

		stats[r.Indexer] = s
	}

	for indexer, s := range stats {
		ui.PrintDebug(fmt.Sprintf("Indexer '%s' stats: total=%d, matched=%d, false_positives=%d",
			indexer, s.total, s.matched, s.total-s.matched))
	}

	return filtered
}

func PrintReleases(result *mdb.SearchResult, meta *metadata.Metadata, filter bool) {
	ui.Println("\n" + ui.Header.Render("PROWLARR RELEASES:"))

	pResults, err := Search(result.ImdbID, result.TmdbID, result.TvdbID, meta.Season, meta.Episode, result.IsTV)
	if err != nil {
		ui.PrintError(fmt.Sprintf("Prowlarr search failed: %v", err))
		return
	}

	if len(pResults) == 0 {
		ui.Println(ui.Muted.Render("No releases found matching the IDs."))
		return
	}

	finalResults := pResults
	if filter {
		finalResults = FilterBestReleases(pResults, meta.Resolution)
	} else {
		sort.Slice(finalResults, func(i, j int) bool {
			return finalResults[i].Seeders > finalResults[j].Seeders
		})
	}

	if len(finalResults) == 0 {
		msg := "No releases found matching the IDs."
		if meta.Resolution != "" {
			msg = fmt.Sprintf("No releases found matching the IDs and resolution (%s).", meta.Resolution)
		}

		ui.Println(ui.Muted.Render(msg))

		return
	}

	ui.Println(RenderReleasesTable(finalResults))
}

func FilterBestReleases(results []ReleaseResource, targetRes string) []ReleaseResource {
	// 1. Try to find the best releases per indexer that match the target resolution
	finalResults := GetBestPerIndexer(results, targetRes)

	// 2. Fallback: If no exact resolution matches, return the best releases per indexer regardless of resolution
	if len(finalResults) == 0 && targetRes != "" {
		finalResults = GetBestPerIndexer(results, "")
	}

	sort.Slice(finalResults, func(i, j int) bool {
		return finalResults[i].Seeders > finalResults[j].Seeders
	})

	return finalResults
}

func GetBestPerIndexer(results []ReleaseResource, targetRes string) []ReleaseResource {
	bestPerIndexer := make(map[string]ReleaseResource)

	for _, r := range results {
		// If a target resolution is specified, skip results that don't match
		if targetRes != "" {
			rMeta := filename.Parse(r.Title)
			if rMeta.Resolution != targetRes {
				continue
			}
		}

		currentBest, exists := bestPerIndexer[r.Indexer]
		if !exists || r.Seeders > currentBest.Seeders {
			bestPerIndexer[r.Indexer] = r
		}
	}

	var best []ReleaseResource
	for _, r := range bestPerIndexer {
		best = append(best, r)
	}

	return best
}

func RenderReleasesTable(results []ReleaseResource) string {
	headers := []string{"Indexer", "Source", "Res", "Group", "Size", "Seeders", "Age", "Info"}

	var rows [][]string

	for _, r := range results {
		age := ""

		if r.PublishDate != "" {
			t, err := time.Parse(time.RFC3339, r.PublishDate)
			if err == nil {
				age = humanizeTime(t)
			}
		}

		// Parse the title to extract source, resolution, and group
		meta := filename.Parse(r.Title)

		rows = append(rows, []string{
			r.Indexer,
			meta.Source,
			meta.Resolution,
			meta.Group,
			humanizeBytes(r.Size),
			strconv.Itoa(r.Seeders),
			age,
			ui.Link.Render(r.InfoUrl),
		})
	}

	return ui.TrackTable(headers, rows)
}

func humanizeBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}

	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.2f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func humanizeTime(t time.Time) string {
	diff := time.Since(t)
	if diff < time.Hour {
		return fmt.Sprintf("%d min", int(diff.Minutes()))
	}

	if diff < 24*time.Hour {
		return fmt.Sprintf("%d hours", int(diff.Hours()))
	}

	return fmt.Sprintf("%d days", int(diff.Hours()/24))
}
