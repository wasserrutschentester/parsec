// Package checks provides validation logic for media files.
package checks

import (
	"codeberg.org/upPollo/parsec/internal/metadata"
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
	return metadata.Normalize(filename.RemoveDiacritics(s))
}
