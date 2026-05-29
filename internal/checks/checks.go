package checks

import (
	"regexp"
	"strings"

	"codeberg.org/n0ne/parsec/internal/types"
)

type TrackCheckResult = types.TrackCheckResult
type CheckResult = types.CheckResult

func normalizeForComparison(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, ".", " ")
	s = strings.ReplaceAll(s, "-", " ")
	// remove all non-alphanumeric chars (except spaces)
	re := regexp.MustCompile(`[^a-z0-9 ]`)
	s = re.ReplaceAllString(s, "")
	// collapse multiple spaces
	reSpaces := regexp.MustCompile(`\s+`)
	s = reSpaces.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
