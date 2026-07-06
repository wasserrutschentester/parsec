package correct

import (
	"fmt"
	"strings"

	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/ui"
)

// ReviewPlan displays the proposed fixes group-by-group, prompting the user for
// confirmation on each. If the user declines a group, it is cleared from the plan.
// It returns true if there are any approved changes remaining to execute.
func ReviewPlan(plan *FixPlan, ebml *matroska.EbmlMetadata, opts Options) bool {
	if plan.IsEmpty() {
		ui.Println("No issues found. File is clean.")

		return false
	}

	ui.Println(ui.ReportSection("Proposed Corrections"))

	reviewContainerAndAttachments(plan, opts)
	reviewChaptersAndFonts(plan, opts)
	reviewStatisticsAndTags(plan, opts)
	reviewTrackEdits(plan, ebml, opts)
	reviewRemux(plan, ebml, opts)

	return !plan.IsEmpty()
}

func reviewContainerAndAttachments(plan *FixPlan, opts Options) {
	if len(plan.ContainerProperties) > 0 {
		ui.Println(ui.Muted.Render("Container Metadata:"))

		for _, p := range plan.ContainerProperties {
			ui.Println("  " + formatContainerChange(p.Key, p.OldValue, p.NewValue))
		}

		if !confirmApply(opts, "Apply these container fixes?", "Skipping container fixes...") {
			plan.ContainerProperties = nil
		}
	}

	if len(plan.AttachmentRenames) > 0 {
		ui.Println(ui.Muted.Render("Font Attachment Renames:"))

		headers := []string{"Old Name", "Internal Name", "New Name"}
		rows := make([][]string, 0, len(plan.AttachmentRenames))

		for _, r := range plan.AttachmentRenames {
			rows = append(rows, []string{r.OldName, r.InternalName, r.NewName})
		}

		ui.Println("  " + strings.ReplaceAll(ui.DataTable(headers, rows), "\n", "\n  "))

		if !confirmApply(opts, "Rename these font attachments?", "Skipping font renames...") {
			plan.AttachmentRenames = nil
		}
	}
}

func formatContainerChange(key, oldValue, newValue string) string {
	arrow := ui.Muted.Render("->")
	label := key

	switch key {
	case "title":
		label = "Title"
	case "writing-application":
		label = "WritingApp"
	case "muxing-application":
		label = "MuxingApp"
	case "date":
		label = "CreationTime"

		return fmt.Sprintf("  %s: %s %s %s", label, ui.Warning.Render("[present]"), arrow, ui.Muted.Render("[cleared]"))
	}

	if newValue == "" {
		return fmt.Sprintf("  %s: %s %s %s", label, quoteOrNone(oldValue), arrow, ui.Muted.Render("[cleared]"))
	}

	return fmt.Sprintf("  %s: %s %s %s", label, quoteOrNone(oldValue), arrow, quoteOrNone(newValue))
}

func reviewChaptersAndFonts(plan *FixPlan, opts Options) {
	if plan.ChapterKeyframeSnaps.Changed > 0 {
		ui.Println(ui.Muted.Render(fmt.Sprintf("Chapter Snaps: %d chapters to align to keyframes", plan.ChapterKeyframeSnaps.Changed)))

		if !confirmApplyWithPolicy(opts, "Apply chapter keyframe alignment?", "Skipping chapter alignment...", false) {
			plan.ChapterKeyframeSnaps.Changed = 0
			plan.ChapterKeyframeSnaps.Times = nil
		}
	}

	if len(plan.FontsToAdd) > 0 {
		ui.Println(ui.Muted.Render("Missing Fonts to Attach:"))

		headers := []string{"Font Name", "Sourced From"}
		rows := make([][]string, 0, len(plan.FontsToAdd))

		for _, f := range plan.FontsToAdd {
			rows = append(rows, []string{f.FontName, f.Source})
		}

		ui.Println("  " + strings.ReplaceAll(ui.DataTable(headers, rows), "\n", "\n  "))

		if !confirmApplyWithPolicy(opts, "Attach these missing subtitle fonts?", "Skipping missing font attachments...", false) {
			plan.FontsToAdd = nil
		}
	}

	if len(plan.FontsToRemove) > 0 {
		ui.Println(ui.Muted.Render(fmt.Sprintf("Unused Fonts to Remove: %d attachments", len(plan.FontsToRemove))))

		headers := []string{"Attachment Name", "Full Name", "Size"}
		rows := make([][]string, 0, len(plan.FontsToRemove))

		for _, f := range plan.FontsToRemove {
			rows = append(rows, []string{f.Name, f.FullName, f.Size})
		}

		ui.Println("  " + strings.ReplaceAll(ui.DataTable(headers, rows), "\n", "\n  "))

		if !confirmApplyWithPolicy(opts, "Delete these unused font attachments?", "Skipping unused font removal...", false) {
			plan.FontsToRemove = nil
		}
	}
}

func reviewStatisticsAndTags(plan *FixPlan, opts Options) {
	if plan.WriteStatistics {
		ui.Println(ui.Muted.Render("Track Statistics: [recompute and write tags]"))

		if !confirmApply(opts, "Add track statistics tags?", "Skipping track statistics tags...") {
			plan.WriteStatistics = false
		}
	}

	if plan.ClearCreationTime {
		ui.Println(ui.Muted.Render("Creation Time: [remove privacy-leaking tags]"))

		if !confirmApplyWithPolicy(opts, "  Remove these creation/encode-time tags?", "Skipping creation-time tag removal...", false) {
			plan.ClearCreationTime = false
		}
	}
}

func reviewTrackEdits(plan *FixPlan, ebml *matroska.EbmlMetadata, opts Options) {
	if len(plan.FlagEdits) > 0 {
		if ebml != nil {
			previewFlagEdits(ebml, plan.FlagEdits)
		} else {
			printTrackEditsFallback(plan.FlagEdits)
		}

		if !confirmApply(opts, "Apply these flag fixes?", "Skipping flag fixes...") {
			plan.FlagEdits = nil
		}
	}

	if len(plan.NameEdits) > 0 {
		if ebml != nil {
			previewTrackEdits("Track Names", ebml, plan.NameEdits)
		} else {
			printTrackEditsFallback(plan.NameEdits)
		}

		if !confirmApply(opts, "Apply these name fixes?", "Skipping name fixes...") {
			plan.NameEdits = nil
		}
	}

	if len(plan.LanguageEdits) > 0 {
		if ebml != nil {
			previewTrackEdits("Language Tags", ebml, plan.LanguageEdits)
		} else {
			printTrackEditsFallback(plan.LanguageEdits)
		}

		if !confirmApply(opts, "Apply these language tag fixes?", "Skipping language tag fixes...") {
			plan.LanguageEdits = nil
		}
	}
}

func printTrackEditsFallback(edits []matroska.TrackEdit) {
	ui.Println(ui.Muted.Render(fmt.Sprintf("Track Edits: %d properties to change", len(edits))))

	for _, e := range edits {
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

func reviewRemux(plan *FixPlan, ebml *matroska.EbmlMetadata, opts Options) {
	if !plan.RemuxRequired {
		return
	}

	ui.Println(ui.Muted.Render("Remux Operations:"))

	reviewRemuxTrackOrder(plan, ebml, opts)
	reviewRemuxStripCompression(plan, opts)
	reviewRemuxRemoveTracks(plan, ebml, opts)

	// Re-evaluate if remux is still required after user choices
	plan.RemuxRequired = len(plan.RemuxTrackOrder) > 0 || len(plan.RemuxStripCompression) > 0 || len(plan.RemuxRemoveTracks) > 0
}

func reviewRemuxTrackOrder(plan *FixPlan, ebml *matroska.EbmlMetadata, opts Options) {
	if len(plan.RemuxTrackOrder) > 0 {
		ui.Println("  - Reorder tracks")

		if ebml != nil {
			ui.Println(trackOrderTable(ebml, plan.RemuxTrackOrder))
		}

		if !confirmApply(opts, "Reorder tracks like this?", "Skipping track reordering...") {
			plan.RemuxTrackOrder = nil
		}
	}
}

func reviewRemuxStripCompression(plan *FixPlan, opts Options) {
	if len(plan.RemuxStripCompression) > 0 {
		ui.Println(fmt.Sprintf("  - Strip compression from %d tracks", len(plan.RemuxStripCompression)))

		if !confirmApply(opts, "Strip container compression from these tracks?", "Skipping compression strip...") {
			plan.RemuxStripCompression = nil
		}
	}
}

func reviewRemuxRemoveTracks(plan *FixPlan, ebml *matroska.EbmlMetadata, opts Options) {
	if len(plan.RemuxRemoveTracks) > 0 {
		ui.Println(fmt.Sprintf("  - Remove %d tracks:", len(plan.RemuxRemoveTracks)))

		for _, r := range plan.RemuxRemoveTracks {
			// Try to find the track to get more info for the UI
			if ebml != nil {
				if t := findTrackByID(ebml, r.TrackID); t != nil {
					ui.Println(fmt.Sprintf("      %d: %s (%s)", t.Properties.Number, trackLabel(t), r.Reason))

					continue
				}
			}

			ui.Println(fmt.Sprintf("      UID %d (%s)", r.TrackID, r.Reason))
		}

		if !confirmApplyWithPolicy(opts, "Remove these tracks?", "Skipping track removal...", false) {
			plan.RemuxRemoveTracks = nil
		}
	}
}
