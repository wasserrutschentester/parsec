package ui

import (
	"os"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

//nolint:paralleltest // depends on shared global state (IsDebug)
func TestPrintDebug(_ *testing.T) {
	// This is hard to test because it prints to stdout/stderr using lipgloss.
	// But we can at least check if it doesn't crash.
	IsDebug = true

	PrintDebug("test debug message")

	IsDebug = false

	PrintDebug("test debug message hidden")
}

func TestFormatDebug(t *testing.T) {
	t.Parallel()

	msg := "test message"

	formatted := formatDebug(msg)
	if formatted == "" {
		t.Error("formatDebug returned empty string")
	}
}

func TestFormatStringDiff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		old  string
		new  string
	}{
		{"Simple", "Hello World", "Hello User"},
		{"Insertion", "Hello", "Hello World"},
		{"Deletion", "Hello World", "Hello"},
		{"Replacement", "The quick brown fox", "The fast brown fox"},
		{"UTF8", "Hello 世界", "Hello Go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := formatStringDiff(tt.old, tt.new)
			if got == "" {
				t.Error("formatStringDiff returned empty string")
			}

			t.Logf("\n%s", got)
		})
	}
}

func TestFormatStringDiffAligned(t *testing.T) {
	t.Parallel()

	got := FormatStringDiffAligned("Expected", "The quick brown fox", "Actual", "The fast brown fox")
	if got == "" {
		t.Error("FormatStringDiffAligned returned empty string")
	}

	t.Logf("\n%s", got)
}

func TestCard(t *testing.T) {
	t.Parallel()

	title := "Test Title"
	subtitle := "Test Subtitle"
	body := "This is a long body that should be wrapped to the inner width of the card correctly."
	footer := "Test Footer"

	rendered := Card(title, subtitle, body, footer)
	if rendered == "" {
		t.Fatal("Card returned empty string")
	}

	lines := strings.Split(strings.TrimSpace(rendered), "\n")
	if len(lines) < 5 {
		t.Errorf("Expected at least 5 lines, got %d", len(lines))
	}

	// All lines should have the same visual width
	firstWidth := lipgloss.Width(lines[0])
	for i, line := range lines {
		w := lipgloss.Width(line)
		if w != firstWidth {
			t.Errorf("Line %d has width %d, expected %d", i, w, firstWidth)
		}
	}
}

func TestAnonymizePath(t *testing.T) {
	t.Parallel()
	// We can't easily mock os.UserHomeDir() without more complex setup,
	// but we can test it with the actual home dir.
	home, _ := os.UserHomeDir()
	if home == "" {
		t.Skip("Home directory not found")
	}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"Home path", home, "~"},
		{"Subdir path", home + "/some/path", "~/some/path"},
		{"Other path", "/tmp/path", "/tmp/path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := AnonymizePath(tt.path)
			if got != tt.expected {
				t.Errorf("AnonymizePath(%q) = %q, expected %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestRenderProgressBar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		percent  int
		contains string
	}{
		{0, "  0%"},
		{50, " 50%"},
		{100, "100%"},
	}

	for _, tt := range tests {
		got := RenderProgressBar(tt.percent)
		if !strings.Contains(got, tt.contains) {
			t.Errorf("RenderProgressBar(%d) = %q, expected to contain %q", tt.percent, got, tt.contains)
		}
	}
}
