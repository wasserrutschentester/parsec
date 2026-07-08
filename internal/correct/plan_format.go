//nolint:gocognit,cyclop,nestif,funlen // UI logic is complex
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
