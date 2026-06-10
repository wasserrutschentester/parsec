package types

// CheckResult represents the result of a single check.
type CheckResult struct {
	Identifier string             `json:"identifier"`
	Passed     bool               `json:"passed"`
	Severity   string             `json:"severity,omitempty"` // "info", "warning", "error"
	Warning    string             `json:"warning,omitempty"`
	Tracks     []TrackCheckResult `json:"tracks,omitempty"`
	Expected   string             `json:"expected,omitempty"`
	Actual     string             `json:"actual,omitempty"`
}

// TrackCheckResult represents the result of a check on a specific track.
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

// IssueGroup represents a group of check results under a specific category.
type IssueGroup struct {
	Category string        `json:"category"`
	Results  []CheckResult `json:"results"`
}

// CheckReport represents a complete report of all checks performed on a file.
type CheckReport struct {
	File          string       `json:"file"`
	Passed        bool         `json:"passed"`
	ReleaseName   string       `json:"filename"`
	GeneratedName string       `json:"generated_name"`
	Version       string       `json:"version"`
	Issues        []IssueGroup `json:"issues"`
}
