package checks

import (
	"regexp"
	"strings"
)

type CheckResult struct {
	Identifier  string             `json:"identifier"`
	Description string             `json:"description"`
	Passed      bool               `json:"passed"`
	Severity    string             `json:"severity,omitempty"` // "info", "warning", "error"
	Warning     string             `json:"warning,omitempty"`
	Tracks      []TrackCheckResult `json:"tracks,omitempty"`
	// For non-track checks (e.g. MDB diffs)
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
}

type TrackCheckResult struct {
	ID        string   `json:"id"`
	Type      string   `json:"type"`
	Passed    bool     `json:"passed"`
	TypeOrder int      `json:"type_order"`
	Codec     string   `json:"codec,omitempty"`
	Name      string   `json:"name,omitempty"`
	Language  string   `json:"language,omitempty"`
	Flags     []string `json:"flags,omitempty"`
	Warning   string   `json:"warning,omitempty"`
}

func NormalizeForComparison(s string) string {
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
