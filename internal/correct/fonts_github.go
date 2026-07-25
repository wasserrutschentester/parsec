package correct

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"codeberg.org/upPollo/parsec/internal/checks"
	"codeberg.org/upPollo/parsec/internal/ui"
)

var fallbackGitHubRepos = []string{
	"extsalt/10000-font-collection",
	"serendipious/every-font",
}

type gitTreeResponse struct {
	Tree []struct {
		Path string `json:"path"`
		Type string `json:"type"`
	} `json:"tree"`
}

type githubFontCandidate struct {
	Repo  string
	Path  string
	Score int
}

func (r defaultFontResolver) resolveGitHubFallbacks(fontName string) (ResolvedFont, bool) {
	var candidates []githubFontCandidate

	normalizedTarget := checks.NormalizeFontName(fontName)

	for _, repo := range fallbackGitHubRepos {
		cacheKey := "github-tree:" + repo
		url := fmt.Sprintf("https://api.github.com/repos/%s/git/trees/master?recursive=1", repo)

		var tree gitTreeResponse
		if err := r.getJSON(cacheKey, url, &tree); err != nil {
			ui.PrintDebug(fmt.Sprintf("failed to fetch GitHub tree for %s: %v", repo, err))

			continue
		}

		for _, item := range tree.Tree {
			if item.Type != "blob" || !isFontFile(item.Path) {
				continue
			}

			base := filepath.Base(item.Path)
			nameOnly := strings.TrimSuffix(base, filepath.Ext(base))
			normalizedRepo := checks.NormalizeFontName(nameOnly)

			score := calculateFontMatchScore(normalizedTarget, normalizedRepo)
			if score > 0 {
				candidates = append(candidates, githubFontCandidate{
					Repo:  repo,
					Path:  item.Path,
					Score: score,
				})
			}
		}
	}

	// Sort candidates by score descending
	slices.SortFunc(candidates, func(a, b githubFontCandidate) int {
		return b.Score - a.Score
	})

	// Try up to 10 candidates
	maxAttempts := min(len(candidates), 10)

	for i := range maxAttempts {
		candidate := candidates[i]
		rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/master/%s", candidate.Repo, candidate.Path)

		ui.PrintDebug(fmt.Sprintf("Testing fallback font %s (score: %d) from %s", candidate.Path, candidate.Score, rawURL))

		if resolved, ok := r.downloadAndMatchFont(rawURL, filepath.Base(candidate.Path), fontName, "github:"+candidate.Repo); ok {
			return resolved, true
		}
	}

	return ResolvedFont{}, false
}

func calculateFontMatchScore(target, repo string) int {
	if target == repo {
		return 100
	}

	if strings.HasPrefix(repo, target) || strings.HasPrefix(target, repo) {
		return 80
	}

	if strings.Contains(repo, target) || strings.Contains(target, repo) {
		return 60
	}

	// Levenshtein distance for fuzzy matching
	dist := levenshteinDistance(target, repo)

	// If the distance is too large, don't consider it a candidate
	maxDist := max(len(target)/3, 3)

	if dist <= maxDist {
		return 50 - dist
	}

	return 0
}

func levenshteinDistance(s, t string) int {
	if len(s) == 0 {
		return len(t)
	}

	if len(t) == 0 {
		return len(s)
	}

	d := make([][]int, len(s)+1)
	for i := range d {
		d[i] = make([]int, len(t)+1)
		d[i][0] = i
	}

	for j := range d[0] {
		d[0][j] = j
	}

	for j := 1; j <= len(t); j++ {
		for i := 1; i <= len(s); i++ {
			if s[i-1] == t[j-1] {
				d[i][j] = d[i-1][j-1]
			} else {
				d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+1)
			}
		}
	}

	return d[len(s)][len(t)]
}
