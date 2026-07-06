package correct

import (
	"fmt"
	"testing"

	"codeberg.org/upPollo/parsec/internal/ui"
)

func TestFormatContainerChange(t *testing.T) {
	t.Parallel()

	got := formatContainerChange("title", "Movie [1080p]", "")
	want := fmt.Sprintf("  Title: %q %s %s", "Movie [1080p]", ui.Muted.Render("->"), ui.Muted.Render("[cleared]"))

	if got != want {
		t.Errorf("formatContainerChange() = %q, want %q", got, want)
	}
}
