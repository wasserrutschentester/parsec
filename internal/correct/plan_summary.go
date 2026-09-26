package correct

import (
	"fmt"
	"strings"

	"codeberg.org/upPollo/parsec/internal/ui"
)

func printGlobalSummary(plan *FixPlan) {
	var lines []string

	counter := 1

	if summary := summarizeContainer(plan); summary != "" {
		lines = append(lines, fmt.Sprintf(" %d. Container:   %s", counter, summary))
		counter++
	}

	if summary := summarizeAttachments(plan); summary != "" {
		lines = append(lines, fmt.Sprintf(" %d. Attachments: %s", counter, summary))
		counter++
	}

	if summary := summarizeChapters(plan); summary != "" {
		lines = append(lines, fmt.Sprintf(" %d. Chapters:    %s", counter, summary))
		counter++
	}

	if summary := summarizeTracks(plan); summary != "" {
		lines = append(lines, fmt.Sprintf(" %d. Tracks:      %s", counter, summary))
		counter++
	}

	if summary := summarizeRemux(plan); summary != "" {
		lines = append(lines, fmt.Sprintf(" %d. Remux:       %s", counter, summary))
	}

	ui.Println(ui.ReportSection("Correction Plan Summary"))

	for _, l := range lines {
		ui.Println(" " + l)
	}

	ui.Println()
}

func summarizeContainer(plan *FixPlan) string {
	var parts []string
	if len(plan.Metadata.Container.Properties) > 0 {
		parts = append(parts, fmt.Sprintf("%d property edits", len(plan.Metadata.Container.Properties)))
	}

	if plan.Metadata.Container.WriteStatistics {
		parts = append(parts, "add statistics")
	}

	if plan.Metadata.Container.ClearCreationTime {
		parts = append(parts, "clear creation time")
	}

	return strings.Join(parts, ", ")
}

func summarizeChapters(plan *FixPlan) string {
	if plan.Metadata.Chapters.LanguageFix.Changed == 0 && !plan.Metadata.Chapters.LanguageFix.NeedsPrompt && len(plan.Metadata.Chapters.KeyframeSnaps.Events) == 0 {
		return ""
	}

	var parts []string
	if plan.Metadata.Chapters.LanguageFix.Changed > 0 {
		parts = append(parts, fmt.Sprintf("%d language corrections", plan.Metadata.Chapters.LanguageFix.Changed))
	}

	if plan.Metadata.Chapters.LanguageFix.NeedsPrompt {
		parts = append(parts, "language input required")
	}

	if len(plan.Metadata.Chapters.KeyframeSnaps.Events) > 0 {
		parts = append(parts, fmt.Sprintf("%d keyframe snaps", len(plan.Metadata.Chapters.KeyframeSnaps.Events)))
	}

	return strings.Join(parts, ", ")
}

func summarizeAttachments(plan *FixPlan) string {
	var parts []string
	if len(plan.Metadata.Attachments.ToRemove) > 0 {
		parts = append(parts, fmt.Sprintf("%d removed", len(plan.Metadata.Attachments.ToRemove)))
	}

	if len(plan.Metadata.Attachments.Renames) > 0 {
		parts = append(parts, fmt.Sprintf("%d renamed", len(plan.Metadata.Attachments.Renames)))
	}

	if len(plan.Metadata.Attachments.ToAdd) > 0 {
		parts = append(parts, fmt.Sprintf("%d added", len(plan.Metadata.Attachments.ToAdd)))
	}

	return strings.Join(parts, ", ")
}

func summarizeTracks(plan *FixPlan) string {
	if len(plan.Metadata.Tracks) == 0 {
		return ""
	}

	flagEdits, nameEdits, langEdits := 0, 0, 0

	for _, e := range plan.Metadata.Tracks {
		for _, p := range e.Properties {
			switch {
			case strings.HasPrefix(p.Key, "flag-"):
				flagEdits++
			case p.Key == "name":
				nameEdits++
			case p.Key == "language":
				langEdits++
			}
		}
	}

	var parts []string
	if flagEdits > 0 {
		parts = append(parts, fmt.Sprintf("%d flag updates", flagEdits))
	}

	if nameEdits > 0 {
		parts = append(parts, fmt.Sprintf("%d name corrections", nameEdits))
	}

	if langEdits > 0 {
		parts = append(parts, fmt.Sprintf("%d lang corrections", langEdits))
	}

	return strings.Join(parts, ", ")
}

func summarizeRemux(plan *FixPlan) string {
	var parts []string
	if len(plan.Remux.TrackOrder) > 0 || len(plan.Remux.RemoveTracks) > 0 {
		parts = append(parts, "Reorder/remove tracks")
	}

	if len(plan.Remux.StripCompression) > 0 {
		parts = append(parts, "Strip compression")
	}

	if n := len(plan.Remux.TruncatedTracks); n > 0 {
		parts = append(parts, fmt.Sprintf("End %d tracks when video ends", n))
	}

	return strings.Join(parts, ", ")
}
