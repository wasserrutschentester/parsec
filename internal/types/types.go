package types

type CheckResult struct {
	Identifier string             `json:"identifier"`
	Passed     bool               `json:"passed"`
	Severity   string             `json:"severity,omitempty"` // "info", "warning", "error"
	Warning    string             `json:"warning,omitempty"`
	Tracks     []TrackCheckResult `json:"tracks,omitempty"`
	Expected   string             `json:"expected,omitempty"`
	Actual     string             `json:"actual,omitempty"`
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
