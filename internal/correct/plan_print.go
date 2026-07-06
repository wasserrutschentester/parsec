package correct

import (
	"fmt"
	"strings"

	"codeberg.org/upPollo/parsec/internal/ui"
)

// PrintAndConfirmPlan displays the proposed fixes and asks the user for confirmation.
func PrintAndConfirmPlan(plan *FixPlan, opts Options) bool {
	if plan.IsEmpty() {
		ui.Println("No issues found. File is clean.")

		return false
	}

	ui.Println(ui.ReportSection("Proposed Corrections"))

	printContainerAndAttachments(plan)
	printChaptersAndFonts(plan)
	printStatisticsAndTags(plan)
	printTrackEditsAndRemux(plan)

	return confirmApply(opts, "Apply these fixes?", "Skipping fixes...")
}

func printContainerAndAttachments(plan *FixPlan) {
	if len(plan.ContainerProperties) > 0 {
		ui.Println(ui.Muted.Render("Container Properties:"))

		for k, v := range plan.ContainerProperties {
			if v == "" {
				ui.Println("  - " + k + ": [delete]")
			} else {
				ui.Println(fmt.Sprintf("  - %s: %s", k, quoteOrNone(v)))
			}
		}
	}

	if len(plan.AttachmentRenames) > 0 {
		ui.Println(ui.Muted.Render("Font Attachment Renames:"))

		for _, v := range plan.AttachmentRenames {
			ui.Println("  - rename to " + quoteOrNone(v))
		}
	}
}

func printChaptersAndFonts(plan *FixPlan) {
	if plan.ChapterKeyframeSnaps.Changed > 0 {
		ui.Println(ui.Muted.Render(fmt.Sprintf("Chapter Snaps: %d chapters to align to keyframes", plan.ChapterKeyframeSnaps.Changed)))
	}

	if len(plan.FontsToAdd) > 0 {
		ui.Println(ui.Muted.Render("Missing Fonts to Attach:"))

		for _, f := range plan.FontsToAdd {
			ui.Println("  + " + f.Name)
		}
	}

	if len(plan.FontsToRemove) > 0 {
		ui.Println(ui.Muted.Render(fmt.Sprintf("Unused Fonts to Remove: %d attachments", len(plan.FontsToRemove))))
	}
}

func printStatisticsAndTags(plan *FixPlan) {
	if plan.WriteStatistics {
		ui.Println(ui.Muted.Render("Track Statistics: [recompute and write tags]"))
	}

	if plan.ClearCreationTime {
		ui.Println(ui.Muted.Render("Creation Time: [remove privacy-leaking tags]"))
	}
}

func printTrackEditsAndRemux(plan *FixPlan) {
	if len(plan.TrackEdits) > 0 {
		ui.Println(ui.Muted.Render(fmt.Sprintf("Track Edits: %d properties to change", len(plan.TrackEdits))))

		for _, e := range plan.TrackEdits {
			var props []string

			for k, v := range e.Props {
				if v == "" {
					props = append(props, k+"=[delete]")
				} else {
					props = append(props, fmt.Sprintf("%s=%s", k, v))
				}
			}

			ui.Println(fmt.Sprintf("  Track %d: %s", e.Number, strings.Join(props, ", ")))
		}
	}

	if plan.RemuxRequired {
		ui.Println(ui.Muted.Render("Remux Operations:"))

		if len(plan.RemuxTrackOrder) > 0 {
			ui.Println("  - Reorder tracks")
		}

		if len(plan.RemuxStripCompression) > 0 {
			ui.Println(fmt.Sprintf("  - Strip compression from %d tracks", len(plan.RemuxStripCompression)))
		}

		if len(plan.RemuxRemoveTracks) > 0 {
			ui.Println(fmt.Sprintf("  - Remove %d tracks", len(plan.RemuxRemoveTracks)))
		}
	}
}
