// Package checks provides validation logic for media files.
package checks

import (
	"regexp"
	"strings"

	"codeberg.org/upPollo/parsec/internal/metadata/filename"
	"codeberg.org/upPollo/parsec/internal/types"
)

type (
	// TrackCheckResult is an alias for types.TrackCheckResult.
	TrackCheckResult = types.TrackCheckResult
	// CheckResult is an alias for types.CheckResult.
	CheckResult = types.CheckResult
)

func normalizeForComparison(s string) string {
	s = strings.ToLower(s)
	s = filename.RemoveDiacritics(s)
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
