//nolint:gocognit,cyclop,nestif,funlen // UI logic is complex
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

	printGlobalSummary(plan)

	if canPrompt(opts) {
		resp := ui.PromptYNI("Apply all changes without review? (y)es, (n)o, (I)nspect", "i")
		switch resp {
		case "y":
			return true
		case "n":
			*plan = FixPlan{}

			return false
		}
	}

	reviewContainerAndAttachments(plan, ebml, opts)
	reviewChaptersAndFonts(plan, opts)
	reviewStatisticsAndTags(plan, ebml, opts)
	reviewTrackEdits(plan, ebml, opts)
	reviewRemux(plan, ebml, opts)

	return !plan.IsEmpty()
}

func reviewContainerAndAttachments(plan *FixPlan, ebml *matroska.EbmlMetadata, opts Options) {
	reviewContainerProperties(plan, opts)
	reviewAttachmentsGroup(plan, ebml, opts)
}

func reviewContainerProperties(plan *FixPlan, opts Options) {
	if len(plan.Metadata.Container.Properties) == 0 {
		return
	}

	for {
		ui.Println(ui.ReportSection("Container Properties"))

		ui.Println("  " + strings.ReplaceAll(formatContainerPropertiesTable(plan), "\n", "\n  "))
		ui.Println()

		if !canPrompt(opts) {
			if !confirmApplyWithPolicy(opts, "Apply these container fixes?", "Skipping container fixes...") {
				plan.Metadata.Container.Properties = nil
			}

			return
		}

		resp := ui.PromptYNI("Apply these container fixes?", "y")
		switch resp {
		case "y":
			return
		case "n":
			plan.Metadata.Container.Properties = nil

			return
		case "i":
			ui.Println(ui.Muted.Render("\nReviewing individual container properties..."))
			reviewIndividualContainerProperties(plan)

			if len(plan.Metadata.Container.Properties) == 0 {
				return
			}
		}
	}
}

func reviewIndividualContainerProperties(plan *FixPlan) {
	var kept []ContainerPropertyEdit

	for _, p := range plan.Metadata.Container.Properties {
		label := p.Key
		switch label {
		case "title":
			label = "Title"
		case "writing-application":
			label = "WritingApp"
		case "muxing-application":
			label = "MuxingApp"
		case "date":
			label = "Date"
		}

		var msg string

		switch {
		case p.OldValue == "set" && p.NewValue == "":
			msg = fmt.Sprintf("  %s: Clear existing value. Apply?", label)
		case p.NewValue == "":
			msg = fmt.Sprintf("  %s: Clear %s. Apply?", label, quoteOrNone(p.OldValue))
		default:
			msg = fmt.Sprintf("  %s: Change %s -> %s. Apply?", label, quoteOrNone(p.OldValue), quoteOrNone(p.NewValue))
		}

		resp := ui.PromptYNE(msg, "y")
		switch resp {
		case "y":
			kept = append(kept, p)
		case "e":
			newVal := ui.Prompt(fmt.Sprintf("  Enter new %s: ", p.Key))
			p.NewValue = newVal
			kept = append(kept, p)
		}
	}

	plan.Metadata.Container.Properties = kept
}

func reviewAttachmentsGroup(plan *FixPlan, ebml *matroska.EbmlMetadata, opts Options) {
	if len(plan.Metadata.Attachments.ToRemove) == 0 && len(plan.Metadata.Attachments.Renames) == 0 && len(plan.Metadata.Attachments.ToAdd) == 0 {
		return
	}

	defaultOpt := "n"

loop:
	for {
		ui.Println(ui.ReportSection("Attachments"))

		tableStr := formatAttachmentsTable(ebml, plan)
		ui.Println("  " + strings.ReplaceAll(tableStr, "\n", "\n  "))
		ui.Println()

		if !canPrompt(opts) {
			if !confirmApplyWithPolicy(opts, "Apply these attachment changes?", "Skipping attachments...") {
				plan.Metadata.Attachments.ToRemove = nil
				plan.Metadata.Attachments.Renames = nil
				plan.Metadata.Attachments.ToAdd = nil
			}

			return
		}

		resp := ui.PromptYNI("Apply these attachment changes?", defaultOpt)
		switch resp {
		case "y":
			break loop
		case "n":
			plan.Metadata.Attachments.ToRemove = nil
			plan.Metadata.Attachments.Renames = nil
			plan.Metadata.Attachments.ToAdd = nil

			break loop
		case "i":
			ui.Println(ui.Muted.Render("\nReviewing individual attachment changes..."))
			reviewIndividualAttachments(plan, ebml)
			filterRenames(plan)

			if len(plan.Metadata.Attachments.ToRemove) == 0 && len(plan.Metadata.Attachments.Renames) == 0 && len(plan.Metadata.Attachments.ToAdd) == 0 {
				break loop
			}

			defaultOpt = "y"
		}
	}
}

func filterRenames(plan *FixPlan) {
	removed := make(map[int]bool)
	for _, f := range plan.Metadata.Attachments.ToRemove {
		removed[f.ID] = true
	}

	var filtered []AttachmentRename

	for _, r := range plan.Metadata.Attachments.Renames {
		if !removed[r.ID] {
			filtered = append(filtered, r)
		}
	}

	plan.Metadata.Attachments.Renames = filtered
}

func reviewIndividualAttachments(plan *FixPlan, ebml *matroska.EbmlMetadata) {
	reviewAttachmentRemovals(plan, ebml)
	reviewAttachmentRenames(plan, ebml)
	reviewAttachmentAdditions(plan, ebml)
}

func reviewAttachmentRemovals(plan *FixPlan, ebml *matroska.EbmlMetadata) {
	if len(plan.Metadata.Attachments.ToRemove) == 0 {
		return
	}

	ui.Println(ui.ReportSection("Attachments - Removals"))

	tempPlan := &FixPlan{}
	tempPlan.Metadata.Attachments.ToRemove = plan.Metadata.Attachments.ToRemove

	tableStr := formatAttachmentsTable(ebml, tempPlan)
	ui.Println("  " + strings.ReplaceAll(tableStr, "\n", "\n  "))

	input := ui.Prompt("  Enter IDs to keep (comma-separated, blank to remove all): ")
	keepIDs := parseIDs(input)

	var keptRemovals []AttachmentRemove

	for _, r := range plan.Metadata.Attachments.ToRemove {
		if !keepIDs[r.ID] {
			keptRemovals = append(keptRemovals, r)
		}
	}

	plan.Metadata.Attachments.ToRemove = keptRemovals
}

func reviewAttachmentRenames(plan *FixPlan, ebml *matroska.EbmlMetadata) {
	filterRenames(plan)

	if len(plan.Metadata.Attachments.Renames) == 0 {
		return
	}

	defaultOpt := "y"

loopRenames:
	for {
		ui.Println()
		ui.Println(ui.ReportSection("Attachments - Renames"))

		tempPlan := &FixPlan{}
		tempPlan.Metadata.Attachments.Renames = plan.Metadata.Attachments.Renames

		tableStr := formatAttachmentsTable(ebml, tempPlan)
		ui.Println("  " + strings.ReplaceAll(tableStr, "\n", "\n  "))

		resp := ui.PromptYNI("Apply attachment renames?", defaultOpt)
		switch resp {
		case "y":
			break loopRenames
		case "n":
			plan.Metadata.Attachments.Renames = nil

			break loopRenames
		case "i":
			var keptRenames []AttachmentRename

			for _, r := range plan.Metadata.Attachments.Renames {
				msg := fmt.Sprintf("  Attachment %d: Rename '%s' -> '%s'?", r.ID, r.OldName, r.NewName)

				resp := ui.PromptYNE(msg, "y")
				switch resp {
				case "y":
					keptRenames = append(keptRenames, r)
				case "e":
					newVal := ui.Prompt(fmt.Sprintf("  Enter new filename for '%s': ", r.OldName))
					if newVal != "" {
						r.NewName = newVal
						keptRenames = append(keptRenames, r)
					}
				}
			}

			plan.Metadata.Attachments.Renames = keptRenames
			if len(plan.Metadata.Attachments.Renames) == 0 {
				break loopRenames
			}

			defaultOpt = "y"
		}
	}
}

func reviewAttachmentAdditions(plan *FixPlan, ebml *matroska.EbmlMetadata) {
	if len(plan.Metadata.Attachments.ToAdd) == 0 {
		return
	}

	defaultOpt := "y"

loopAdditions:
	for {
		ui.Println()
		ui.Println(ui.ReportSection("Attachments - Additions"))

		tempPlan := &FixPlan{}
		tempPlan.Metadata.Attachments.ToAdd = plan.Metadata.Attachments.ToAdd

		tableStr := formatAttachmentsTable(ebml, tempPlan)
		ui.Println("  " + strings.ReplaceAll(tableStr, "\n", "\n  "))

		resp := ui.PromptYNI("Attach missing fonts?", defaultOpt)
		switch resp {
		case "y":
			break loopAdditions
		case "n":
			plan.Metadata.Attachments.ToAdd = nil

			break loopAdditions
		case "i":
			var keptAdditions []MissingFontAttachment

			for _, a := range plan.Metadata.Attachments.ToAdd {
				msg := fmt.Sprintf("  Attach missing font '%s'? (Source: %s)", a.AttachmentName, a.Source)
				if ui.PromptYN(msg, "y") {
					keptAdditions = append(keptAdditions, a)
				}
			}

			plan.Metadata.Attachments.ToAdd = keptAdditions
			if len(plan.Metadata.Attachments.ToAdd) == 0 {
				break loopAdditions
			}

			defaultOpt = "y"
		}
	}
}

func parseIDs(input string) map[int]bool {
	ids := make(map[int]bool)

	for p := range strings.SplitSeq(input, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		if id, err := strconv.Atoi(p); err == nil {
			ids[id] = true
		}
	}

	return ids
}

func reviewChaptersAndFonts(plan *FixPlan, opts Options) {
	reviewChapterSnaps(plan, opts)
}

func reviewChapterSnaps(plan *FixPlan, opts Options) {
	if plan.Metadata.Chapters.KeyframeSnaps.Changed <= 0 {
		return
	}

	defaultOpt := "n"

loopChapters:
	for {
		ui.Println(ui.Muted.Render(fmt.Sprintf("Chapter Snaps: %d chapters to align to keyframes", plan.Metadata.Chapters.KeyframeSnaps.Changed)))

		if len(plan.Metadata.Chapters.KeyframeSnaps.Events) > 0 {
			headers := []string{"#", "Name", "Old Timestamp", "Direction", "New Timestamp", "Diff", "Old Latency"}
			tableRows := formatChapterEventsTable(plan.Metadata.Chapters.KeyframeSnaps.Events)
			ui.Println("  " + strings.ReplaceAll(ui.DataTable(headers, tableRows), "\n", "\n  "))
		}

		ui.Println()

		if !canPrompt(opts) {
			if !confirmApplyWithPolicy(opts, "Apply chapter keyframe alignment?", "Skipping chapter alignment...") {
				plan.Metadata.Chapters.KeyframeSnaps.Changed = 0
				plan.Metadata.Chapters.KeyframeSnaps.Times = nil
			}

			return
		}

		resp := ui.PromptYNI("Apply chapter keyframe alignment?", defaultOpt)
		switch resp {
		case "y":
			break loopChapters
		case "n":
			plan.Metadata.Chapters.KeyframeSnaps.Changed = 0
			plan.Metadata.Chapters.KeyframeSnaps.Times = nil

			break loopChapters
		case "i":
			ui.Println(ui.Muted.Render("\nReviewing individual chapter alignments..."))
			reviewIndividualChapterSnaps(plan)

			if plan.Metadata.Chapters.KeyframeSnaps.Changed <= 0 {
				break loopChapters
			}

			defaultOpt = "y"
		}
	}
}

func reviewIndividualChapterSnaps(plan *FixPlan) {
	var finalEvents []ChapterSnapEvent

	changed := 0

	for _, e := range plan.Metadata.Chapters.KeyframeSnaps.Events {
		diffPrev := int64(-1)
		if e.PreviousKeyframe != -1 {
			diffPrev = e.OriginalTime - e.PreviousKeyframe
		}

		diffNext := int64(-1)
		if e.NextKeyframe != -1 {
			diffNext = e.NextKeyframe - e.OriginalTime
		}

		var (
			newTime   int64
			direction string
		)

		switch {
		case e.PreviousKeyframe != -1 && (e.NextKeyframe == -1 || diffPrev <= diffNext):
			newTime = e.PreviousKeyframe
			direction = ui.Success.Render("<- Prev")
		case e.NextKeyframe != -1:
			newTime = e.NextKeyframe
			direction = ui.Success.Render("Next ->")
		default:
			newTime = e.OriginalTime
			direction = "-"
		}

		msg := fmt.Sprintf("  Chapter %d: Align %s %s %s. Apply?", e.ChapterNum, formatNsToTime(e.OriginalTime), direction, formatNsToTime(newTime))
		if ui.PromptYN(msg, "y") {
			finalEvents = append(finalEvents, e)
			changed++
		} else {
			plan.Metadata.Chapters.KeyframeSnaps.Times[e.ChapterNum-1] = e.OriginalTime
		}
	}

	plan.Metadata.Chapters.KeyframeSnaps.Events = finalEvents
	plan.Metadata.Chapters.KeyframeSnaps.Changed = changed
}

func reviewStatisticsAndTags(plan *FixPlan, ebml *matroska.EbmlMetadata, opts Options) {
	if !plan.Metadata.Container.WriteStatistics && !plan.Metadata.Container.ClearCreationTime {
		return
	}

	defaultOpt := "n"

	for {
		ui.Println(ui.ReportSection("Container Metadata"))

		if plan.Metadata.Container.WriteStatistics {
			ui.Println("  " + ui.Success.Render("[+]") + " Track Statistics: " + ui.Muted.Render("recompute and write tags"))
		}

		if plan.Metadata.Container.ClearCreationTime {
			ui.Println("  " + ui.Error.Render("[-]") + " Creation Time: " + ui.Muted.Render("remove privacy-leaking tags"))

			foundTags := getCreationTimeTags(ebml)
			if len(foundTags) > 0 {
				ui.Println("      Found: " + ui.Warning.Render(strings.Join(foundTags, "; ")))
			}
		}

		ui.Println()

		if !canPrompt(opts) {
			if !confirmApplyWithPolicy(opts, "Apply container metadata changes?", "Skipping container metadata...") {
				plan.Metadata.Container.WriteStatistics = false
				plan.Metadata.Container.ClearCreationTime = false
			}

			return
		}

		resp := ui.PromptYNI("Apply container metadata changes?", defaultOpt)
		switch resp {
		case "y":
			return
		case "n":
			plan.Metadata.Container.WriteStatistics = false
			plan.Metadata.Container.ClearCreationTime = false

			return
		case "i":
			ui.Println(ui.Muted.Render("\nReviewing individual container edits..."))
			reviewIndividualContainerEdits(plan, ebml)

			if !plan.Metadata.Container.WriteStatistics && !plan.Metadata.Container.ClearCreationTime {
				return
			}

			defaultOpt = "y"
		}
	}
}

func reviewIndividualContainerEdits(plan *FixPlan, ebml *matroska.EbmlMetadata) {
	if plan.Metadata.Container.WriteStatistics {
		if !ui.PromptYN("  Add track statistics tags?", "y") {
			plan.Metadata.Container.WriteStatistics = false
		}
	}

	if plan.Metadata.Container.ClearCreationTime {
		foundTags := getCreationTimeTags(ebml)

		msg := "  Remove creation-time tags?"
		if len(foundTags) > 0 {
			msg = "  Remove creation-time tags? (" + strings.Join(foundTags, ", ") + ")"
		}

		// Defaults to "n" because privacy removal was traditionally false-default
		if !ui.PromptYN(msg, "n") {
			plan.Metadata.Container.ClearCreationTime = false
		}
	}
}

func reviewTrackEdits(plan *FixPlan, ebml *matroska.EbmlMetadata, opts Options) {
	if len(plan.Metadata.Tracks) == 0 {
		return
	}

	defaultOpt := "n"

	for {
		ui.Println(ui.ReportSection("Tracks"))

		if ebml != nil {
			tableStr := formatTracksTable(ebml, plan.Metadata.Tracks)
			ui.Println("  " + strings.ReplaceAll(tableStr, "\n", "\n  "))
		} else {
			printTrackEditsFallback(plan.Metadata.Tracks)
		}

		ui.Println()

		if !canPrompt(opts) {
			if !confirmApplyWithPolicy(opts, "Apply all track changes?", "Skipping track edits...") {
				plan.Metadata.Tracks = nil
			}

			return
		}

		resp := ui.PromptYNI("Apply all track changes?", defaultOpt)
		switch resp {
		case "y":
			return
		case "n":
			plan.Metadata.Tracks = nil

			return
		case "i":
			ui.Println(ui.Muted.Render("\nReviewing track changes by type..."))

			plan.Metadata.Tracks = reviewTrackEditsByType(plan.Metadata.Tracks, ebml)

			if len(plan.Metadata.Tracks) == 0 {
				return
			}

			defaultOpt = "y"
		}
	}
}

func reviewTrackEditsByType(edits []matroska.TrackEdit, ebml *matroska.EbmlMetadata) []matroska.TrackEdit {
	edits = reviewTrackEditsSubgroup(edits, ebml, "Name Edits", func(key string) bool { return key == "name" })
	edits = reviewTrackEditsSubgroup(edits, ebml, "Language Edits", func(key string) bool { return key == "language" })
	edits = reviewTrackEditsSubgroup(edits, ebml, "Default Flag Assignment", func(key string) bool { return key == "flag-default" })
	edits = reviewTrackEditsSubgroup(edits, ebml, "Other Flag Edits", func(key string) bool { return strings.HasPrefix(key, "flag-") && key != "flag-default" })

	return edits
}

func reviewTrackEditsSubgroup(edits []matroska.TrackEdit, ebml *matroska.EbmlMetadata, title string, keyFilter func(string) bool) []matroska.TrackEdit {
	var subgroupEdits []matroska.TrackEdit

	for _, e := range edits {
		var matchProps []matroska.TrackPropertyEdit

		for _, p := range e.Properties {
			if keyFilter(p.Key) {
				matchProps = append(matchProps, p)
			}
		}

		if len(matchProps) > 0 {
			subgroupEdits = append(subgroupEdits, matroska.TrackEdit{
				Number:     e.Number,
				Properties: matchProps,
			})
		}
	}

	if len(subgroupEdits) == 0 {
		return edits
	}

	var finalSubgroup []matroska.TrackEdit

	defaultOpt := "n"

loopSubgroup:
	for {
		ui.Println(ui.ReportSection("Tracks - " + title))

		tableStr := formatTracksTable(ebml, subgroupEdits)
		ui.Println("  " + strings.ReplaceAll(tableStr, "\n", "\n  "))
		ui.Println()

		resp := ui.PromptYNI(fmt.Sprintf("Apply %s?", strings.ToLower(title)), defaultOpt)
		switch resp {
		case "y":
			finalSubgroup = subgroupEdits

			break loopSubgroup
		case "n":
			finalSubgroup = nil

			break loopSubgroup
		case "i":
			subgroupEdits = reviewIndividualTrackEdits(subgroupEdits, ebml)

			finalSubgroup = subgroupEdits
			if len(subgroupEdits) == 0 {
				break loopSubgroup
			}

			defaultOpt = "y"
		}
	}

	keptProps := make(map[int]map[string]matroska.TrackPropertyEdit)
	for _, e := range finalSubgroup {
		keptProps[e.Number] = make(map[string]matroska.TrackPropertyEdit)
		for _, p := range e.Properties {
			keptProps[e.Number][p.Key] = p
		}
	}

	var finalEdits []matroska.TrackEdit

	for _, e := range edits {
		var newProps []matroska.TrackPropertyEdit

		for _, p := range e.Properties {
			if keyFilter(p.Key) {
				if kept, ok := keptProps[e.Number][p.Key]; ok {
					newProps = append(newProps, kept)
				}
			} else {
				newProps = append(newProps, p)
			}
		}

		if len(newProps) > 0 {
			finalEdits = append(finalEdits, matroska.TrackEdit{
				Number:     e.Number,
				Properties: newProps,
			})
		}
	}

	return finalEdits
}

func reviewIndividualTrackEdits(edits []matroska.TrackEdit, ebml *matroska.EbmlMetadata) []matroska.TrackEdit {
	var finalEdits []matroska.TrackEdit

	for _, edit := range edits {
		track := findTrack(ebml, edit.Number)
		trackContext := fmt.Sprintf("Track %d", edit.Number)

		if track != nil {
			trackTypeStr, lang, _, name, _ := trackColumns(track)
			trackContext = fmt.Sprintf("Track %d [%s, %s, %s]", edit.Number, trackTypeStr, lang, quoteOrNone(name))
		}

		var finalProps []matroska.TrackPropertyEdit

		for _, prop := range edit.Properties {
			var msg string

			switch prop.Key {
			case "name", "language":
				oldVal := ""

				if track != nil {
					if prop.Key == "name" {
						oldVal = track.Properties.Name
					} else {
						oldVal = track.Properties.Language
					}
				}

				msg = fmt.Sprintf("%s: Change %s %s -> %s. Apply?", trackContext, prop.Key, quoteOrNone(oldVal), quoteOrNone(prop.Value))

				resp := ui.PromptYNE(msg, "y")
				switch resp {
				case "y":
					finalProps = append(finalProps, prop)
				case "e":
					newVal := ui.Prompt(fmt.Sprintf("Enter new %s: ", prop.Key))
					prop.Value = newVal
					finalProps = append(finalProps, prop)
				}
			default:
				msg = fmt.Sprintf("%s: Set %s to %s. Apply?", trackContext, prop.Key, prop.Value)
				if ui.PromptYN(msg, "y") {
					finalProps = append(finalProps, prop)
				}
			}
		}

		if len(finalProps) > 0 {
			finalEdits = append(finalEdits, matroska.TrackEdit{
				Number:     edit.Number,
				Properties: finalProps,
			})
		}
	}

	return finalEdits
}

func printTrackEditsFallback(edits []matroska.TrackEdit) {
	ui.Println(ui.Muted.Render(fmt.Sprintf("Track Edits: %d properties to change", len(edits))))

	for _, e := range edits {
		var props []string

		for _, p := range e.Properties {
			if p.Value == "" {
				props = append(props, p.Key+"=[delete]")
			} else {
				props = append(props, fmt.Sprintf("%s=%s", p.Key, p.Value))
			}
		}

		ui.Println(fmt.Sprintf("  Track %d: %s", e.Number, strings.Join(props, ", ")))
	}
}

func reviewRemux(plan *FixPlan, ebml *matroska.EbmlMetadata, opts Options) {
	if !plan.Remux.Required {
		return
	}

	reviewRemuxTrackOrderAndRemoval(plan, ebml, opts)
	reviewRemuxStripCompression(plan, ebml, opts)

	// Re-evaluate if remux is still required after user choices
	plan.Remux.Required = len(plan.Remux.TrackOrder) > 0 || len(plan.Remux.StripCompression) > 0 || len(plan.Remux.RemoveTracks) > 0
}

func reviewRemuxTrackOrderAndRemoval(plan *FixPlan, ebml *matroska.EbmlMetadata, opts Options) {
	if len(plan.Remux.TrackOrder) == 0 && len(plan.Remux.RemoveTracks) == 0 {
		return
	}

	if ebml == nil {
		return
	}

	ui.Println(ui.ReportSection("Remux: Track Reordering"))

	tableStr := formatRemuxReorderTable(ebml, plan)
	ui.Println("  " + strings.ReplaceAll(tableStr, "\n", "\n  "))
	ui.Println()

	if !canPrompt(opts) {
		if !confirmApplyWithPolicy(opts, "Apply remux operations?", "Skipping remux operations...") {
			plan.Remux.TrackOrder = nil
			plan.Remux.RemoveTracks = nil
		}

		return
	}

	if !ui.PromptYN("Apply remux operations?", "n") {
		plan.Remux.TrackOrder = nil
		plan.Remux.RemoveTracks = nil
	}
}

func reviewRemuxStripCompression(plan *FixPlan, ebml *matroska.EbmlMetadata, opts Options) {
	if len(plan.Remux.StripCompression) > 0 {
		ui.Println(ui.ReportSection("Remux: Compression Stripping"))
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

		ui.Println()

		if !canPrompt(opts) {
			if !confirmApplyWithPolicy(opts, "Strip compression from tracks?", "Skipping compression stripping...") {
				plan.Remux.StripCompression = nil
			}

			return
		}

		if !ui.PromptYN("Strip compression from tracks?", "n") {
			plan.Remux.StripCompression = nil
		}
	}
}
