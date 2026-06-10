package update

import "testing"

func TestIsNewer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		latest  string
		current string
		want    bool
	}{
		// Basic comparisons
		{"v0.1.1", "v0.1.0", true},
		{"v0.2.0", "v0.1.9", true},
		{"v1.0.0", "v0.9.9", true},
		{"v1.10.0", "v1.2.0", true}, // Multi-digit parts

		// Older version
		{"v0.1.0", "v0.1.1", false},
		{"v0.0.9", "v0.1.0", false},

		// Different lengths
		{"v0.1.0", "v0.1", false},
		{"v1.0.0", "v1", false},

		// Pre-releases and commit hashes
		{"v0.1.0", "v0.1.0-abcdef1", true},        // Stable > pre-release
		{"v0.1.1", "v0.1.0-abcdef1", true},        // Newer stable > older pre-release
		{"v0.1.0-beta.1", "v0.1.0-alpha.1", true}, // Beta > Alpha
		{"v0.1.0-alpha.2", "v0.1.0-alpha.1", true},

		// Initial version
		{"v0.0.1", "v0.0.0", true},
		{"v0.0.1", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.latest+" vs "+tt.current, func(t *testing.T) {
			t.Parallel()

			if got := IsNewer(tt.latest, tt.current); got != tt.want {
				t.Errorf("IsNewer(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
			}
		})
	}
}
