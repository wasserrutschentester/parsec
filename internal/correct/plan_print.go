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

//nolint:cyclop // UI step has multiple branches
func reviewContainerAndAttachments(plan *FixPlan, opts Options) {
	if len(plan.ContainerProperties) > 0 {
		ui.Println(ui.Muted.Render("Container Metadata:"))

		for _, p := range plan.ContainerProperties {
			ui.Println("  " + formatContainerChange(p.Key, p.OldValue, p.NewValue, p.Reason))
		}

		if !confirmApply(opts, "Apply these container fixes?", "Skipping container fixes...") {
			plan.ContainerProperties = nil
		}
	}

	if len(plan.FontsToRemove) > 0 {
		ui.Println(ui.Muted.Render(fmt.Sprintf("Unused & Duplicate Fonts to Remove: %d attachments", len(plan.FontsToRemove))))

		headers := []string{"ID", "Attachment Name", "Full Name", "Size", "Reason"}
		rows := make([][]string, 0, len(plan.FontsToRemove))

		for _, f := range plan.FontsToRemove {
			rows = append(rows, []string{strconv.Itoa(f.ID), f.Name, f.FullName, f.Size, f.Reason})
		}

		ui.Println("  " + strings.ReplaceAll(ui.DataTable(headers, rows), "\n", "\n  "))

		if !confirmApplyWithPolicy(opts, "Delete these unused/duplicate font attachments?", "Skipping unused font removal...", false) {
			plan.FontsToRemove = nil
		} else {
			// They accepted removal, so we shouldn't rename the ones being removed.
			var filtered []AttachmentRename

			removed := make(map[int]bool, len(plan.FontsToRemove))
			for _, f := range plan.FontsToRemove {
				removed[f.ID] = true
			}

			for _, r := range plan.AttachmentRenames {
				if !removed[r.ID] {
					filtered = append(filtered, r)
				}
			}

			plan.AttachmentRenames = filtered
		}
	}

	if len(plan.AttachmentRenames) > 0 {
		ui.Println(ui.Muted.Render("Font Attachment Renames:"))

		headers := []string{"ID", "Old Name", "Full Name", "PostScript Name", "New Name"}
		rows := make([][]string, 0, len(plan.AttachmentRenames))

		for _, r := range plan.AttachmentRenames {
			rows = append(rows, []string{strconv.Itoa(r.ID), r.OldName, r.FullName, r.PostScriptName, r.NewName})
		}

		ui.Println("  " + strings.ReplaceAll(ui.DataTable(headers, rows), "\n", "\n  "))

		if !confirmApply(opts, "Rename these font attachments?", "Skipping font renames...") {
			plan.AttachmentRenames = nil
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
	if plan.ChapterKeyframeSnaps.Changed > 0 {
		ui.Println(ui.Muted.Render(fmt.Sprintf("Chapter Snaps: %d chapters to align to keyframes", plan.ChapterKeyframeSnaps.Changed)))

		if len(plan.ChapterKeyframeSnaps.TableRows) > 0 {
			headers := []string{"#", "Name", "Timestamp", "Seek Latency", "Previous KF", "Next KF"}
			ui.Println("  " + strings.ReplaceAll(ui.DataTable(headers, plan.ChapterKeyframeSnaps.TableRows), "\n", "\n  "))
		}

		if !confirmApplyWithPolicy(opts, "Apply chapter keyframe alignment?", "Skipping chapter alignment...", false) {
			plan.ChapterKeyframeSnaps.Changed = 0
			plan.ChapterKeyframeSnaps.Times = nil
		}
	}

	if len(plan.FontsToAdd) > 0 {
		ui.Println("  - Add missing fonts:")

		headers := []string{"Font Name", "File", "Source", "Requested By"}
		rows := make([][]string, 0, len(plan.FontsToAdd))

		for _, f := range plan.FontsToAdd {
			rows = append(rows, []string{
				f.FontName,
				f.AttachmentName,
				f.Source,
				strings.Join(f.RequestedBy, ", "),
			})
		}

		ui.Println("  " + strings.ReplaceAll(ui.DataTable(headers, rows), "\n", "\n  "))

		if !confirmApplyWithPolicy(opts, "Attach these missing subtitle fonts?", "Skipping missing font attachments...", false) {
			plan.FontsToAdd = nil
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
	if plan.WriteStatistics {
		ui.Println(ui.Muted.Render("Track Statistics: [recompute and write tags]"))

		if !confirmApply(opts, "Add track statistics tags?", "Skipping track statistics tags...") {
			plan.WriteStatistics = false
		}
	}

	if plan.ClearCreationTime {
		ui.Println(ui.Muted.Render("Creation Time: [remove privacy-leaking tags]"))

		foundTags := getCreationTimeTags(ebml)
		if len(foundTags) > 0 {
			ui.Println("  Found: " + ui.Warning.Render(strings.Join(foundTags, "; ")))
		}

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
	reviewRemuxStripCompression(plan, ebml, opts)
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

func reviewRemuxStripCompression(plan *FixPlan, ebml *matroska.EbmlMetadata, opts Options) {
	if len(plan.RemuxStripCompression) > 0 {
		ui.Println(fmt.Sprintf("  - Strip compression from %d tracks:", len(plan.RemuxStripCompression)))

		for _, trackID := range plan.RemuxStripCompression {
			label := fmt.Sprintf("UID %d", trackID)

			if ebml != nil {
				if t := findTrackByID(ebml, trackID); t != nil {
					label = fmt.Sprintf("Track %d: %s (zlib compressed)", t.Properties.Number, trackLabel(t))
				}
			}

			ui.Println("      " + label)
		}

		if !confirmApply(opts, "Strip container compression from these tracks?", "Skipping compression strip...") {
			plan.RemuxStripCompression = nil
		}
	}
}

//nolint:nestif // UI printing logic requires some nested checks
func reviewRemuxRemoveTracks(plan *FixPlan, ebml *matroska.EbmlMetadata, opts Options) {
	if len(plan.RemuxRemoveTracks) > 0 {
		ui.Println(fmt.Sprintf("  - Remove %d tracks:", len(plan.RemuxRemoveTracks)))

		headers := []string{"Track", "UID", "Type", "Lang", "Codec", "Name", "Flags", "Reason"}

		rows := make([][]string, 0, len(plan.RemuxRemoveTracks))

		for _, candidate := range plan.RemuxRemoveTracks {
			uidStr := strconv.Itoa(candidate.TrackID)
			trackNum, trackType, lang, codec, name, flags := "?", "?", "?", "?", "", ""

			if ebml != nil {
				if t := findTrackByID(ebml, candidate.TrackID); t != nil {
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
			}

			rows = append(rows, []string{
				trackNum,
				uidStr,
				trackType,
				lang,
				codec,
				name,
				flags,
				candidate.Reason,
			})
		}

		if len(rows) > 0 {
			ui.Println(ui.TrackTable(headers, rows))
		}

		if !confirmApplyWithPolicy(opts, "Remove these tracks?", "Skipping track removal...", false) {
			plan.RemuxRemoveTracks = nil
		}
	}
}
