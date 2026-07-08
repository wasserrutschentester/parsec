package correct

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"

	"codeberg.org/upPollo/parsec/internal/config"
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

func printGlobalSummary(plan *FixPlan) {
	var lines []string

	if summary := summarizeContainer(plan); summary != "" {
		lines = append(lines, " 1. Container:   "+summary)
	}

	if summary := summarizeAttachments(plan); summary != "" {
		lines = append(lines, " 2. Attachments: "+summary)
	}

	if len(plan.Metadata.Chapters.KeyframeSnaps.Events) > 0 {
		lines = append(lines, fmt.Sprintf(" 3. Chapters:    %d keyframe snaps", len(plan.Metadata.Chapters.KeyframeSnaps.Events)))
	}

	if summary := summarizeTracks(plan); summary != "" {
		lines = append(lines, " 4. Tracks:      "+summary)
	}

	if summary := summarizeRemux(plan); summary != "" {
		lines = append(lines, " 5. Remux:       "+summary)
	}

	ui.Println(ui.ReportSection("Fix Plan Summary"))

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
		parts = append(parts, fmt.Sprintf("%d name fixes", nameEdits))
	}

	if langEdits > 0 {
		parts = append(parts, fmt.Sprintf("%d lang fixes", langEdits))
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

	return strings.Join(parts, ", ")
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

func formatContainerPropertiesTable(plan *FixPlan) string {
	headers := []string{"Key", "Change", "Reason"}

	rows := make([][]string, 0, len(plan.Metadata.Container.Properties))

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

		var diffStr string

		switch {
		case p.OldValue == "set" && p.NewValue == "":
			diffStr = ui.Warning.Render("[cleared]")
		case p.NewValue == "":
			diffStr = ui.FormatStringDiff(p.OldValue, "")
		default:
			diffStr = ui.FormatStringDiff(p.OldValue, p.NewValue)
		}

		rows = append(rows, []string{
			label,
			diffStr,
			p.Reason,
		})
	}

	return ui.DataTable(headers, rows)
}

//nolint:cyclop // UI logic
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

//nolint:cyclop // UI logic
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

//nolint:gocognit,cyclop,funlen // UI logic
func formatAttachmentsTable(ebml *matroska.EbmlMetadata, plan *FixPlan) string {
	headers := []string{"Action", "#", "Current Filename", "New Filename", "Full Style Name", "Size", "Reason"}

	type op struct {
		Action    string
		ID        int
		Current   string
		New       string
		FullName  string
		Size      int64
		Reason    string
		SortOrder int
	}

	var allOps []*op

	// Add removals
	for _, r := range plan.Metadata.Attachments.ToRemove {
		allOps = append(allOps, &op{
			Action:    "Remove",
			ID:        r.ID,
			Current:   r.Name,
			New:       "-",
			FullName:  r.FullName,
			Size:      r.SizeBytes,
			Reason:    r.Reason,
			SortOrder: 1,
		})
	}

	// Add renames
	for _, r := range plan.Metadata.Attachments.Renames {
		size := int64(0)

		if ebml != nil {
			for _, a := range ebml.Attachments {
				if a.ID == r.ID {
					size = int64(a.Size)

					break
				}
			}
		}

		// Don't show rename if it's already marked for removal
		isRemoved := false

		for _, rem := range plan.Metadata.Attachments.ToRemove {
			if rem.ID == r.ID {
				isRemoved = true

				break
			}
		}

		if !isRemoved {
			allOps = append(allOps, &op{
				Action:    "Rename",
				ID:        r.ID,
				Current:   r.OldName,
				New:       r.NewName,
				FullName:  r.FullName,
				Size:      size,
				Reason:    "Rename required by ASS subtitles",
				SortOrder: 2,
			})
		}
	}

	// Add additions
	for _, a := range plan.Metadata.Attachments.ToAdd {
		size := int64(0)
		if stat, err := os.Stat(a.Path); err == nil {
			size = stat.Size()
		}

		allOps = append(allOps, &op{
			Action:    "Add",
			ID:        -1,
			Current:   "-",
			New:       a.AttachmentName,
			FullName:  a.FontName,
			Size:      size,
			Reason:    "Requested by " + strings.Join(a.RequestedBy, ", "),
			SortOrder: 3,
		})
	}

	sort.Slice(allOps, func(i, j int) bool {
		if allOps[i].ID == -1 && allOps[j].ID != -1 {
			return false
		}

		if allOps[j].ID == -1 && allOps[i].ID != -1 {
			return true
		}

		if allOps[i].ID == -1 && allOps[j].ID == -1 {
			return allOps[i].New < allOps[j].New
		}

		return allOps[i].ID < allOps[j].ID
	})

	rows := make([][]string, 0, len(allOps))

	for _, o := range allOps {
		currentName := o.Current
		newName := o.New
		actionStr := o.Action
		idStr := strconv.Itoa(o.ID)

		if o.ID == -1 {
			idStr = "-"
		}

		switch o.Action {
		case "Remove":
			actionStr = ui.Error.Render(actionStr)
			currentName = ui.Error.Render(currentName)
			newName = ui.Error.Render(newName)
		case "Rename":
			actionStr = ui.Warning.Render(actionStr)
			newName = ui.Success.Render(newName)
		case "Add":
			actionStr = ui.Success.Render(actionStr)
			newName = ui.Success.Render(newName)
		}

		rows = append(rows, []string{
			actionStr,
			idStr,
			currentName,
			newName,
			o.FullName,
			formatSizeBytes(o.Size),
			o.Reason,
		})
	}

	return ui.DataTable(headers, rows)
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

//nolint:cyclop // UI logic
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

func formatChapterEventsTable(events []ChapterSnapEvent) [][]string {
	tableRows := make([][]string, 0, len(events))

	for _, e := range events {
		latencyStr := formatSeekLatency(e.PreviousKeyframe, e.OriginalTime, e.DefaultDuration)

		diffPrev := int64(-1)
		if e.PreviousKeyframe != -1 {
			diffPrev = e.OriginalTime - e.PreviousKeyframe
		}

		diffNext := int64(-1)
		if e.NextKeyframe != -1 {
			diffNext = e.NextKeyframe - e.OriginalTime
		}

		var (
			direction  string
			newTimeStr string
			diffStr    string
		)

		switch {
		case e.PreviousKeyframe != -1 && (e.NextKeyframe == -1 || diffPrev <= diffNext):
			direction = ui.Success.Render("<- Prev")
			newTimeStr = ui.Success.Render(formatNsToTime(e.PreviousKeyframe))
			diffStr = fmt.Sprintf("-%.3fs", float64(diffPrev)/1e9)
		case e.NextKeyframe != -1:
			direction = ui.Success.Render("Next ->")
			newTimeStr = ui.Success.Render(formatNsToTime(e.NextKeyframe))
			diffStr = fmt.Sprintf("+%.3fs", float64(diffNext)/1e9)
		default:
			direction = "-"
			newTimeStr = formatNsToTime(e.OriginalTime)
			diffStr = "0.000s"
		}

		tableRows = append(tableRows, []string{
			strconv.Itoa(e.ChapterNum),
			e.Name,
			formatNsToTime(e.OriginalTime),
			direction,
			newTimeStr,
			diffStr,
			latencyStr,
		})
	}

	return tableRows
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

//nolint:cyclop // UI logic
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

//nolint:gocognit,cyclop,funlen // UI logic
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

//nolint:gocognit,nestif,cyclop,funlen // UI logic
func formatTracksTable(ebml *matroska.EbmlMetadata, edits []matroska.TrackEdit) string {
	sort.Slice(edits, func(i, j int) bool {
		return edits[i].Number < edits[j].Number
	})

	headers := []string{"ID", "Type", "#", "Codec", "Lang", "Name", "Flags", "Reason"}
	rows := make([][]string, 0, len(edits))

	for _, edit := range edits {
		track := findTrack(ebml, edit.Number)

		trackType, lang, codec, name, flags := "?", "?", "?", "?", ""
		typeOrder := "?"

		if track != nil {
			trackType, lang, codec, name, flags = trackColumns(track)

			typeCounts := make(map[string]int)
			for _, t := range ebml.Tracks {
				typeCounts[t.Type]++
				if t.ID == track.ID {
					typeOrder = strconv.Itoa(typeCounts[t.Type])

					break
				}
			}
		}

		for _, prop := range edit.Properties {
			if strings.HasPrefix(prop.Key, "flag-") {
				compact := compactFlagName(prop.Key)

				pretty := prettyFlag(prop.Key)
				if prop.Value == "1" {
					flags += " " + ui.Success.Render("[+] "+pretty)
				} else {
					// If it's present, replace it with the removed version. If not present, append it anyway so it's visible.
					if strings.Contains(flags, compact) {
						flags = strings.ReplaceAll(flags, compact, ui.Error.Render("[-] "+pretty))
					} else {
						flags += " " + ui.Error.Render("[-] "+pretty)
					}
				}

				continue
			}

			switch prop.Key {
			case "name":
				oldVal := ""
				if track != nil {
					oldVal = track.Properties.Name
				}

				if prop.Value == "" {
					name = ui.Warning.Render(quoteOrNone(oldVal) + " -> [cleared]")
				} else {
					name = ui.FormatStringDiff(oldVal, prop.Value)
				}
			case "language":
				oldVal := ""
				if track != nil {
					oldVal = track.Properties.Language
				}

				lang = ui.Warning.Render(fmt.Sprintf("%s -> %s", oldVal, prop.Value))
			}
		}

		// Clean up spacing in flags
		flags = strings.TrimSpace(strings.ReplaceAll(flags, "  ", " "))

		rows = append(rows, []string{
			strconv.Itoa(edit.Number),
			trackType,
			typeOrder,
			codec,
			lang,
			name,
			flags,
			joinReasons(edit.Properties),
		})
	}

	return ui.TrackTable(headers, rows)
}

func compactFlagName(key string) string {
	switch key {
	case "flag-default":
		return "D"
	case "flag-forced":
		return "Forced"
	case "flag-original":
		return "Orig"
	case "flag-commentary":
		return "Comm"
	case "flag-hearing-impaired":
		return "SDH"
	case "flag-visual-impaired":
		return "AD"
	}

	return strings.TrimPrefix(key, "flag-")
}

func joinReasons(props []matroska.TrackPropertyEdit) string {
	var parts []string

	for _, p := range props {
		if p.Reason != "" {
			parts = append(parts, p.Reason)
		}
	}

	if len(parts) == 0 {
		return "-"
	}

	return strings.Join(slices.Compact(parts), "; ")
}

//nolint:cyclop // UI logic
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

//nolint:cyclop,funlen // UI logic
func formatRemuxReorderTable(ebml *matroska.EbmlMetadata, plan *FixPlan) string {
	oldTracks := ebml.Tracks

	oldIndex := make(map[int]int)
	oldTypeOrder := make(map[int]int)
	typeCounts := make(map[string]int)

	for i, t := range oldTracks {
		oldIndex[t.ID] = i + 1
		typeCounts[t.Type]++
		oldTypeOrder[t.ID] = typeCounts[t.Type]
	}

	var newOrder []int
	if len(plan.Remux.TrackOrder) > 0 {
		newOrder = plan.Remux.TrackOrder
	} else {
		removedIDs := make(map[int]bool)
		for _, r := range plan.Remux.RemoveTracks {
			removedIDs[r.TrackID] = true
		}

		for _, t := range oldTracks {
			if !removedIDs[t.ID] {
				newOrder = append(newOrder, t.ID)
			}
		}
	}

	removedIDs := make(map[int]RemovalCandidate)
	for _, r := range plan.Remux.RemoveTracks {
		removedIDs[r.TrackID] = r
	}

	var retainedOldOrder []int

	for _, t := range oldTracks {
		if _, removed := removedIDs[t.ID]; !removed {
			retainedOldOrder = append(retainedOldOrder, t.ID)
		}
	}

	retainedOldIndex := make(map[int]int)
	for i, id := range retainedOldOrder {
		retainedOldIndex[id] = i
	}

	headers := []string{"ID", "oID", "Type", "#", "o#", "Codec", "Lang", "Name", "Action"}

	var rows [][]string

	newTypeCounts := make(map[string]int)
	newTypeCountsByID := make(map[int]int)
	gray := ui.Muted.Render

	// Pre-calculate new order indices
	newIndex := make(map[int]int)
	for i, id := range newOrder {
		newIndex[id] = i + 1

		track := findTrackByID(ebml, id)
		if track != nil {
			newTypeCounts[track.Type]++
			newTypeCountsByID[id] = newTypeCounts[track.Type]
		}
	}

	for _, t := range oldTracks {
		oID := oldIndex[t.ID]
		oTypeOrd := oldTypeOrder[t.ID]

		trackTypeStr, lang, codec, name, _ := trackColumns(&t)
		lang = formatLanguageName(lang, &t, plan)

		if _, removed := removedIDs[t.ID]; removed {
			trackTypeStr = ui.Error.Render(trackTypeStr)
			lang = ui.Error.Render(lang)
			codec = ui.Error.Render(codec)
			name = ui.Error.Render(name)
			action := ui.Error.Render("Removed")

			rows = append(rows, []string{
				"-", strconv.Itoa(oID), trackTypeStr, "-", strconv.Itoa(oTypeOrd), codec, lang, name, action,
			})

			continue
		}

		newID := newIndex[t.ID]
		newTypeOrd := newTypeCountsByID[t.ID]

		action := "Keep"

		// Calculate if shifted up/down among remaining tracks
		idx := newID - 1
		if oID != newID || oTypeOrd != newTypeOrd {
			switch {
			case retainedOldIndex[t.ID] > idx:
				action = ui.Success.Render("Moved Up (Active)")
			case retainedOldIndex[t.ID] < idx:
				action = "Shifted Down"
			default:
				action = "Shifted Up"
			}
		}

		idStr := strconv.Itoa(newID)

		oIDStr := strconv.Itoa(oID)
		if newID == oID {
			oIDStr = gray(oIDStr)
		}

		typeOrdStr := strconv.Itoa(newTypeOrd)

		oTypeOrdStr := strconv.Itoa(oTypeOrd)
		if newTypeOrd == oTypeOrd {
			oTypeOrdStr = gray(oTypeOrdStr)
		}

		if strings.Contains(action, "Moved Up") {
			trackTypeStr = ui.Success.Render(trackTypeStr)
			lang = ui.Success.Render(lang)
			codec = ui.Success.Render(codec)
			name = ui.Success.Render(name)
		}

		rows = append(rows, []string{
			idStr, oIDStr, trackTypeStr, typeOrdStr, oTypeOrdStr, codec, lang, name, action,
		})
	}

	return ui.TrackTable(headers, rows)
}

//nolint:nestif // UI logic
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

//nolint:cyclop // UI logic
func formatLanguageName(langCode string, track *matroska.EbmlTrack, plan *FixPlan) string {
	if langCode == "" {
		return ""
	}

	formatted := langCode

	tag, err := language.Parse(langCode)
	if err == nil {
		if name := display.English.Tags().Name(tag); name != "" {
			formatted = name
		}
	}

	// Use OriginalLanguage from plan, or fallback to the track flag
	isOriginal := false
	if plan != nil && plan.OriginalLanguage != "" && err == nil {
		isOriginal = tag == language.Make(plan.OriginalLanguage)
	} else if track != nil && track.Properties.OriginalLanguage {
		isOriginal = true
	}

	isPreferred := err == nil && tag == language.Make(config.GetPreferredLanguage())

	if isOriginal {
		formatted += " " + ui.Info.Render("[O]")
	} else if isPreferred {
		formatted += " " + ui.Info.Render("[P]")
	}

	return formatted
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
