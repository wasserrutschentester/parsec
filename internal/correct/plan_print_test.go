package correct

import (
	"fmt"
	"testing"

	"codeberg.org/upPollo/parsec/internal/ui"
)

// merged into existing track 2
// appended as a new track

// Order is base order followed by new tracks from extra.

// Track 2 must carry both its base and extra properties.

// Track 2 has two mismatches (Forced + SDH) and must produce one consolidated edit.

func TestFormatContainerChange(t *testing.T) {
	t.Parallel()

	got := formatContainerChange("title", "Movie [1080p]", "")
	want := fmt.Sprintf("  Title: %q %s %s", "Movie [1080p]", ui.Muted.Render("->"), ui.Muted.Render("[cleared]"))

	if got != want {
		t.Errorf("formatContainerChange() = %q, want %q", got, want)
	}
}
