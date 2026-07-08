package correct

import (
	"fmt"
	"strconv"
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
	reviewStatisticsAndTags(plan, ebml, opts)
	reviewTrackEdits(plan, ebml, opts)
	reviewRemux(plan, ebml, opts)

	return !plan.IsEmpty()
}

func reviewContainerAndAttachments(plan *FixPlan, opts Options) {
	reviewContainerProperties(plan, opts)
	reviewAttachmentsToRemove(plan, opts)
	reviewAttachmentsToRename(plan, opts)
}

func reviewContainerProperties(plan *FixPlan, opts Options) {
	if len(plan.Metadata.Container.Properties) > 0 {
		ui.Println(ui.Muted.Render("Container Metadata:"))

		for _, p := range plan.Metadata.Container.Properties {
			ui.Println("  " + formatContainerChange(p.Key, p.OldValue, p.NewValue, p.Reason))
		}

		if !confirmApply(opts, "Apply these container fixes?", "Skipping container fixes...") {
			plan.Metadata.Container.Properties = nil
		}
	}
}

func reviewAttachmentsToRemove(plan *FixPlan, opts Options) {
	if len(plan.Metadata.Attachments.ToRemove) > 0 {
		ui.Println(ui.Muted.Render(fmt.Sprintf("Unused & Duplicate Fonts to Remove: %d attachments", len(plan.Metadata.Attachments.ToRemove))))

		headers := []string{"ID", "Attachment Name", "Full Name", "Size", "Reason"}
		rows := make([][]string, 0, len(plan.Metadata.Attachments.ToRemove))

		for _, f := range plan.Metadata.Attachments.ToRemove {
			sizeStr := formatSizeBytes(f.SizeBytes)
			rows = append(rows, []string{strconv.Itoa(f.ID), f.Name, f.FullName, sizeStr, f.Reason})
		}

		ui.Println("  " + strings.ReplaceAll(ui.DataTable(headers, rows), "\n", "\n  "))

		if !confirmApplyWithPolicy(opts, "Delete these unused/duplicate font attachments?", "Skipping unused font removal...", false) {
			plan.Metadata.Attachments.ToRemove = nil
		} else {
			// They accepted removal, so we shouldn't rename the ones being removed.
			var filtered []AttachmentRename

			removed := make(map[int]bool, len(plan.Metadata.Attachments.ToRemove))
			for _, f := range plan.Metadata.Attachments.ToRemove {
				removed[f.ID] = true
			}

			for _, r := range plan.Metadata.Attachments.Renames {
				if !removed[r.ID] {
					filtered = append(filtered, r)
				}
			}

			plan.Metadata.Attachments.Renames = filtered
		}
	}
}

func reviewAttachmentsToRename(plan *FixPlan, opts Options) {
	if len(plan.Metadata.Attachments.Renames) > 0 {
		ui.Println(ui.Muted.Render("Font Attachment Renames:"))

		headers := []string{"ID", "Old Name", "Full Name", "PostScript Name", "New Name"}
		rows := make([][]string, 0, len(plan.Metadata.Attachments.Renames))

		for _, r := range plan.Metadata.Attachments.Renames {
			rows = append(rows, []string{strconv.Itoa(r.ID), r.OldName, r.FullName, r.PostScriptName, r.NewName})
		}

		ui.Println("  " + strings.ReplaceAll(ui.DataTable(headers, rows), "\n", "\n  "))

		if !confirmApply(opts, "Rename these font attachments?", "Skipping font renames...") {
			plan.Metadata.Attachments.Renames = nil
		}
	}
}

func formatContainerChange(key, oldValue, newValue, reason string) string {
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
		label = "Date"
	}

	var res string

	switch {
	case oldValue == "set" && newValue == "":
		res = fmt.Sprintf("%s: %s %s %s", label, ui.Muted.Render("[present]"), arrow, ui.Muted.Render("[cleared]"))
	case newValue == "":
		res = fmt.Sprintf("%s: %s %s %s", label, quoteOrNone(oldValue), arrow, ui.Muted.Render("[cleared]"))
	default:
		res = fmt.Sprintf("%s: %s %s %s", label, quoteOrNone(oldValue), arrow, quoteOrNone(newValue))
	}

	if reason != "" {
		res += ui.Muted.Render(" (" + reason + ")")
	}

	return res
}

func reviewChaptersAndFonts(plan *FixPlan, opts Options) {
	reviewChapterSnaps(plan, opts)
	reviewMissingFonts(plan, opts)
}

func reviewChapterSnaps(plan *FixPlan, opts Options) {
	if plan.Metadata.Chapters.KeyframeSnaps.Changed <= 0 {
		return
	}

	ui.Println(ui.Muted.Render(fmt.Sprintf("Chapter Snaps: %d chapters to align to keyframes", plan.Metadata.Chapters.KeyframeSnaps.Changed)))

	if len(plan.Metadata.Chapters.KeyframeSnaps.Events) > 0 {
		headers := []string{"#", "Name", "Timestamp", "Seek Latency", "Previous KF", "Next KF"}
		tableRows := formatChapterEventsTable(plan.Metadata.Chapters.KeyframeSnaps.Events)
		ui.Println("  " + strings.ReplaceAll(ui.DataTable(headers, tableRows), "\n", "\n  "))
	}

	if !confirmApplyWithPolicy(opts, "Apply chapter keyframe alignment?", "Skipping chapter alignment...", false) {
		plan.Metadata.Chapters.KeyframeSnaps.Changed = 0
		plan.Metadata.Chapters.KeyframeSnaps.Times = nil
	}
}

func formatChapterEventsTable(events []ChapterSnapEvent) [][]string {
	tableRows := make([][]string, 0, len(events))

	for _, e := range events {
		latencyStr := formatSeekLatency(e.PreviousKeyframe, e.OriginalTime, e.DefaultDuration)

		prevStr := "-"
		if e.PreviousKeyframe != -1 {
			prevStr = formatNsToTime(e.PreviousKeyframe)
		}

		nextStr := formatNextKF(e.NextKeyframe, e.OriginalTime, e.DefaultDuration)

		diffPrev := int64(-1)
		if e.PreviousKeyframe != -1 {
			diffPrev = e.OriginalTime - e.PreviousKeyframe
		}

		diffNext := int64(-1)
		if e.NextKeyframe != -1 {
			diffNext = e.NextKeyframe - e.OriginalTime
		}

		var direction string

		switch {
		case e.PreviousKeyframe != -1 && (e.NextKeyframe == -1 || diffPrev <= diffNext):
			prevStr = ui.Success.Render(prevStr)
			direction = ui.Success.Render("<- Prev")
		case e.NextKeyframe != -1:
			nextStr = ui.Success.Render(nextStr)
			direction = ui.Success.Render("Next ->")
		default:
			direction = "-"
		}

		tableRows = append(tableRows, []string{
			strconv.Itoa(e.ChapterNum),
			e.Name,
			formatNsToTime(e.OriginalTime),
			latencyStr,
			prevStr,
			direction,
			nextStr,
		})
	}

	return tableRows
}

func reviewMissingFonts(plan *FixPlan, opts Options) {
	if len(plan.Metadata.Attachments.ToAdd) > 0 {
		ui.Println("  - Add missing fonts:")

		headers := []string{"Font Name", "File", "Source", "Requested By"}
		rows := make([][]string, 0, len(plan.Metadata.Attachments.ToAdd))

		for _, f := range plan.Metadata.Attachments.ToAdd {
			rows = append(rows, []string{
				f.FontName,
				f.AttachmentName,
				f.Source,
				strings.Join(f.RequestedBy, ", "),
			})
		}

		ui.Println("  " + strings.ReplaceAll(ui.DataTable(headers, rows), "\n", "\n  "))

		if !confirmApplyWithPolicy(opts, "Attach these missing subtitle fonts?", "Skipping missing font attachments...", false) {
			plan.Metadata.Attachments.ToAdd = nil
		}
	}
}

func getCreationTimeTags(ebml *matroska.EbmlMetadata) []string {
	var foundTags []string

	if ebml == nil {
		return foundTags
	}

	if ebml.Container.Properties.DateUtc != "" {
		foundTags = append(foundTags, "DateUTC: "+ebml.Container.Properties.DateUtc)
	}

	if ebml.Container.Properties.DateLocal != "" {
		foundTags = append(foundTags, "DateLocal: "+ebml.Container.Properties.DateLocal)
	}

	return foundTags
}

func reviewStatisticsAndTags(plan *FixPlan, ebml *matroska.EbmlMetadata, opts Options) {
	if plan.Metadata.Container.WriteStatistics {
		ui.Println(ui.Muted.Render("Track Statistics: [recompute and write tags]"))

		if !confirmApply(opts, "Add track statistics tags?", "Skipping track statistics tags...") {
			plan.Metadata.Container.WriteStatistics = false
		}
	}

	if plan.Metadata.Container.ClearCreationTime {
		ui.Println(ui.Muted.Render("Creation Time: [remove privacy-leaking tags]"))

		foundTags := getCreationTimeTags(ebml)
		if len(foundTags) > 0 {
			ui.Println("  Found: " + ui.Warning.Render(strings.Join(foundTags, "; ")))
		}

		if !confirmApplyWithPolicy(opts, "  Remove these creation/encode-time tags?", "Skipping creation-time tag removal...", false) {
			plan.Metadata.Container.ClearCreationTime = false
		}
	}
}

func reviewTrackEdits(plan *FixPlan, ebml *matroska.EbmlMetadata, opts Options) {
	if len(plan.Metadata.Tracks) > 0 {
		if ebml != nil {
			previewTrackEdits("Track Metadata", ebml, plan.Metadata.Tracks)
		} else {
			printTrackEditsFallback(plan.Metadata.Tracks)
		}

		if !confirmApply(opts, "Apply these track metadata fixes?", "Skipping track metadata fixes...") {
			plan.Metadata.Tracks = nil
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
	if !plan.Remux.Required {
		return
	}

	ui.Println(ui.Muted.Render("Remux Operations:"))

	reviewRemuxTrackOrder(plan, ebml, opts)
	reviewRemuxStripCompression(plan, ebml, opts)
	reviewRemuxRemoveTracks(plan, ebml, opts)

	// Re-evaluate if remux is still required after user choices
	plan.Remux.Required = len(plan.Remux.TrackOrder) > 0 || len(plan.Remux.StripCompression) > 0 || len(plan.Remux.RemoveTracks) > 0
}

func reviewRemuxTrackOrder(plan *FixPlan, ebml *matroska.EbmlMetadata, opts Options) {
	if len(plan.Remux.TrackOrder) > 0 {
		ui.Println("  - Reorder tracks")

		if ebml != nil {
			ui.Println(trackOrderTable(ebml, plan.Remux.TrackOrder))
		}

		if !confirmApply(opts, "Reorder tracks like this?", "Skipping track reordering...") {
			plan.Remux.TrackOrder = nil
		}
	}
}

func reviewRemuxStripCompression(plan *FixPlan, ebml *matroska.EbmlMetadata, opts Options) {
	if len(plan.Remux.StripCompression) > 0 {
		ui.Println(fmt.Sprintf("  - Strip compression from %d tracks:", len(plan.Remux.StripCompression)))

		for _, trackID := range plan.Remux.StripCompression {
			label := fmt.Sprintf("UID %d", trackID)

			if ebml != nil {
				if t := findTrackByID(ebml, trackID); t != nil {
					label = fmt.Sprintf("Track %d: %s (zlib compressed)", t.Properties.Number, trackLabel(t))
				}
			}

			ui.Println("      " + label)
		}

		if !confirmApply(opts, "Strip container compression from these tracks?", "Skipping compression strip...") {
			plan.Remux.StripCompression = nil
		}
	}
}

func reviewRemuxRemoveTracks(plan *FixPlan, ebml *matroska.EbmlMetadata, opts Options) {
	if len(plan.Remux.RemoveTracks) > 0 {
		ui.Println(fmt.Sprintf("  - Remove %d tracks:", len(plan.Remux.RemoveTracks)))

		headers := []string{"Track", "UID", "Type", "Lang", "Codec", "Name", "Flags", "Reason"}

		rows := make([][]string, 0, len(plan.Remux.RemoveTracks))

		for _, candidate := range plan.Remux.RemoveTracks {
			rows = append(rows, formatRemoveTrackRow(candidate, ebml))
		}

		if len(rows) > 0 {
			ui.Println(ui.TrackTable(headers, rows))
		}

		if !confirmApplyWithPolicy(opts, "Remove these tracks?", "Skipping track removal...", false) {
			plan.Remux.RemoveTracks = nil
		}
	}
}

func formatRemoveTrackRow(candidate RemovalCandidate, ebml *matroska.EbmlMetadata) []string {
	uidStr := strconv.Itoa(candidate.TrackID)
	trackNum, trackType, lang, codec, name, flags := "?", "?", "?", "?", "", ""

	var t *matroska.EbmlTrack
	if ebml != nil {
		t = findTrackByID(ebml, candidate.TrackID)
	}

	if t != nil {
		trackNum = strconv.Itoa(t.Properties.Number)
		trackType = t.Type
		lang = t.Properties.Language
		codec = t.Properties.CodecID
		name = t.Properties.Name

		var flagParts []string
		if t.Properties.Default {
			flagParts = append(flagParts, "Default")
		}

		if t.Properties.Forced {
			flagParts = append(flagParts, "Forced")
		}

		if t.Properties.HearingImpaired {
			flagParts = append(flagParts, "SDH")
		}

		if t.Properties.VisualImpaired {
			flagParts = append(flagParts, "AD")
		}

		flags = strings.Join(flagParts, " ")
	}

	return []string{
		trackNum,
		uidStr,
		trackType,
		lang,
		codec,
		name,
		flags,
		candidate.Reason,
	}
}

func formatSizeBytes(sizeBytes int64) string {
	const unit = 1024
	switch {
	case sizeBytes < unit:
		return fmt.Sprintf("%d B", sizeBytes)
	case sizeBytes < unit*unit:
		return fmt.Sprintf("%.1f KB", float64(sizeBytes)/float64(unit))
	default:
		return fmt.Sprintf("%.1f MB", float64(sizeBytes)/float64(unit*unit))
	}
}

func formatNsToTime(ns int64) string {
	ms := ns / 1000000
	hours := ms / 3600000
	ms %= 3600000
	minutes := ms / 60000
	ms %= 60000
	seconds := ms / 1000
	ms %= 1000

	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, ms)
}

func formatSeekLatency(prevKF, timeStart int64, defaultDuration int64) string {
	if prevKF == -1 {
		return "-"
	}

	diff := timeStart - prevKF
	latencyStr := fmt.Sprintf("%.3fs", float64(diff)/1e9)

	if defaultDuration > 0 {
		frames := float64(diff) / float64(defaultDuration)
		latencyStr += fmt.Sprintf(" (%.0ff)", frames)
	}

	return latencyStr
}

func formatNextKF(nextKF, timeStart int64, defaultDuration int64) string {
	if nextKF == -1 {
		return "-"
	}

	nextStr := formatNsToTime(nextKF)

	if defaultDuration > 0 {
		frames := float64(nextKF-timeStart) / float64(defaultDuration)
		nextStr += fmt.Sprintf(" (+%.0ff)", frames)
	}

	return nextStr
}
