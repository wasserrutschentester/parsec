// Package correct applies deterministic repairs for Matroska track metadata and
// container-level checks.
package correct

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"codeberg.org/upPollo/parsec/internal/checks"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/ui"
)

// Options controls which fixes are applied and how interactive decisions are
// handled.
type Options struct {
	DryRun     bool
	Remux      bool
	Unattended bool
	ImdbID     string
	TmdbID     int
	TvdbID     int
}

// AppendInteractiveTrackEdits prompts the user for manual fixes (like keyword/flag matching).
func AppendInteractiveTrackEdits(filePath string, plan *FixPlan, opts Options) error {
	if !canPrompt(opts) {
		return nil
	}

	ebml, err := matroska.GetEbmlMetadata(filePath)
	if err != nil {
		return nil // skip if we can't parse
	}

	// 1. Missing Font Interactive Resolution
	attachmentFonts := checks.GetAttachmentFonts(filePath, ebml.Attachments)

	missingPlan := ComputeMissingFontAttachments(filePath, ebml, attachmentFonts, false)
	if len(missingPlan.Unresolved) > 0 {
		printMissingFontPlan(missingPlan, false, false)

		if confirmApplyWithPolicy(opts, "Search for and download these missing subtitle fonts?", "Skipping missing font downloads...", false) {
			missingPlan = ComputeMissingFontAttachments(filePath, ebml, attachmentFonts, true)

			plan.FontsToAdd = nil
			for _, a := range missingPlan.Attachments {
				plan.FontsToAdd = append(plan.FontsToAdd, MissingFontAttachment{
					Path:           a.Path,
					AttachmentName: a.AttachmentName,
					MIMEType:       a.MIMEType,
					FontName:       a.FontName,
					Source:         a.Source,
				})
			}
		}
	}

	// 2. Track Edits
	keywordEdits := promptKeywordFlagFixes(ebml)
	multiLangEdits := promptMultiLangNameFixes(ebml)
	langEdits := promptLanguageFixes(ebml)

	plan.FlagEdits = mergeTrackEdits(plan.FlagEdits, keywordEdits)
	plan.NameEdits = mergeTrackEdits(plan.NameEdits, multiLangEdits)
	plan.LanguageEdits = mergeTrackEdits(plan.LanguageEdits, langEdits)

	// Recompute Remux Plan based on the final set of Track Edits
	// because interactive edits might have altered languages which changes sort order
	simulatedTracks := ApplyEditsToMemoryTracks(ebml.Tracks, plan.FlagEdits, plan.NameEdits, plan.LanguageEdits)

	originalLang := lookupOriginalLanguage(filePath, ebml.Tracks, opts)
	remuxPlan := ComputeMatroskaRemux(simulatedTracks, originalLang)
	plan.RemuxRequired = len(remuxPlan.TrackOrder) > 0 || len(remuxPlan.RemovalCandidates) > 0 || len(remuxPlan.StripCompressionIDs) > 0

	plan.RemuxTrackOrder = remuxPlan.TrackOrder
	plan.RemuxRemoveTracks = remuxPlan.RemovalCandidates
	plan.RemuxStripCompression = remuxPlan.StripCompressionIDs

	return nil
}

// ApplyFile applies enabled Matroska fixes to one file.
//
// Deprecated: This is the old monolithic entrypoint. Wait until cmd/correct.go is fully migrated before removing.
func confirmApply(opts Options, prompt, skipMsg string) bool {
	return confirmApplyWithPolicy(opts, prompt, skipMsg, true)
}

// canPrompt reports whether interactive prompts can be shown: not dry-run,
// not unattended, and a terminal is attached. Use this to gate prompt
// collection loops; use confirmApplyWithPolicy for single bulk confirmations.
func canPrompt(opts Options) bool {
	return !opts.DryRun && !opts.Unattended && ui.IsTerminal()
}

func confirmApplyWithPolicy(opts Options, prompt, skipMsg string, allowUnattended bool) bool {
	if opts.DryRun {
		ui.Println(ui.Muted.Render("Dry run: no changes made."))

		return false
	}

	if opts.Unattended {
		if allowUnattended {
			return true
		}

		ui.Println(ui.Muted.Render(skipMsg))

		return false
	}

	if !ui.IsTerminal() || !confirmPrompt(prompt) {
		ui.Println(ui.Muted.Render(skipMsg))

		return false
	}

	return true
}

// fixChapterAlignment snaps misaligned chapter start times to the nearest
// video keyframe. Re-timing chapters changes seek/navigation points, so it is
// always confirmed like the other container fixes above.
func previewFlagEdits(ebml *matroska.EbmlMetadata, edits []matroska.TrackEdit) {
	ui.Println(ui.ReportSection("Track Flags"))

	editByNumber := make(map[int]map[string]string, len(edits))
	hasDefaultEdit := false

	for _, edit := range edits {
		editByNumber[edit.Number] = edit.Props
		if _, ok := edit.Props["flag-default"]; ok {
			hasDefaultEdit = true
		}
	}

	headers := []string{"Track", "Type", "Lang", "Name", "Changes"}

	var rows [][]string

	if hasDefaultEdit {
		rows = defaultFlagRows(ebml, editByNumber)
	} else {
		rows = editedFlagRows(ebml, edits)
	}

	ui.Println(ui.TrackTable(headers, rows))
	ui.Println()
}

// defaultFlagRows returns one row per audio/subtitle track, showing pending
// changes for edited tracks and the unchanged Default state for others.
func defaultFlagRows(ebml *matroska.EbmlMetadata, editByNumber map[int]map[string]string) [][]string {
	var rows [][]string

	for i := range ebml.Tracks {
		track := &ebml.Tracks[i]
		if track.Type != "audio" && track.Type != "subtitles" {
			continue
		}

		props := editByNumber[track.Properties.Number]

		var changes string

		if props != nil {
			changes = flagChangeTags(props)
		} else if track.Properties.Default {
			changes = ui.Muted.Render("Default (unchanged)")
		}

		rows = append(rows, []string{
			strconv.Itoa(track.Properties.Number),
			track.Type,
			track.Properties.Language,
			track.Properties.Name,
			changes,
		})
	}

	return rows
}

// editedFlagRows returns one row per edited track only.
func editedFlagRows(ebml *matroska.EbmlMetadata, edits []matroska.TrackEdit) [][]string {
	rows := make([][]string, 0, len(edits))

	for _, edit := range edits {
		track := findTrack(ebml, edit.Number)

		trackType, lang, name := "?", "?", ""
		if track != nil {
			trackType, lang, name = track.Type, track.Properties.Language, track.Properties.Name
		}

		rows = append(rows, []string{strconv.Itoa(edit.Number), trackType, lang, name, flagChangeTags(edit.Props)})
	}

	return rows
}

func flagChangeTags(props map[string]string) string {
	keys := slices.Sorted(maps.Keys(props))
	tags := make([]string, 0, len(keys))

	for _, key := range keys {
		tags = append(tags, flagChangeTag(key, props[key] == "1"))
	}

	return strings.Join(tags, " ")
}

func flagChangeTag(key string, set bool) string {
	label := prettyFlag(key)
	if set {
		return ui.Success.Render("[+] " + label)
	}

	return ui.Muted.Render("[-] " + label)
}

// remuxMatroska previews the pending remux-only fixes (track order,
// compression stripping, track removals) regardless of --remux, so a plain
// `fix` run always shows what a rewrite would change. The actual mkvmerge
// rewrite only runs with --remux, and each of the three pieces is then
// confirmed independently, same as every other fix.
func trackOrderTable(ebml *matroska.EbmlMetadata, newOrder []int) string {
	oldOrder := make([]int, len(ebml.Tracks))
	for i, t := range ebml.Tracks {
		oldOrder[i] = t.ID
	}

	oldIndex := make(map[int]int, len(oldOrder))
	for i, id := range oldOrder {
		oldIndex[id] = i + 1
	}

	misplaced := misplacedTrackIDs(oldOrder, newOrder)

	headers := []string{"Old #", "New #", "Type", "Lang", "Codec", "Name", "Flags"}
	rows := make([][]string, 0, len(newOrder))

	for i, id := range newOrder {
		track := findTrackByID(ebml, id)

		trackType, lang, codec, name, flags := trackColumns(track)
		if misplaced[id] {
			trackType = ui.Warning.Render(trackType)
			lang = ui.Warning.Render(lang)
			codec = ui.Warning.Render(codec)
			name = ui.Warning.Render(name)
			flags = ui.Warning.Render(flags)
		}

		rows = append(rows, []string{strconv.Itoa(oldIndex[id]), strconv.Itoa(i + 1), trackType, lang, codec, name, flags})
	}

	return ui.TrackTable(headers, rows)
}

// misplacedTrackIDs returns the track IDs that genuinely need to move:
// everything in newOrder that isn't part of the longest common subsequence
// with oldOrder. LCS members are already in correct relative order and only
// change index because the misplaced ones move around them.
func misplacedTrackIDs(oldOrder, newOrder []int) map[int]bool {
	inLCS := make(map[int]bool)
	for _, id := range longestCommonSubsequence(oldOrder, newOrder) {
		inLCS[id] = true
	}

	misplaced := make(map[int]bool)

	for _, id := range newOrder {
		if !inLCS[id] {
			misplaced[id] = true
		}
	}

	return misplaced
}

// longestCommonSubsequence returns the longest common subsequence of a and b
// via the standard O(len(a)*len(b)) DP table.
func longestCommonSubsequence(a, b []int) []int {
	dp := lcsTable(a, b)
	lcs := backtrackLCS(a, b, dp)

	slices.Reverse(lcs)

	return lcs
}

func lcsTable(a, b []int) [][]int {
	n, m := len(a), len(b)
	dp := make([][]int, n+1)

	for i := range dp {
		dp[i] = make([]int, m+1)
	}

	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			switch {
			case a[i-1] == b[j-1]:
				dp[i][j] = dp[i-1][j-1] + 1
			case dp[i-1][j] >= dp[i][j-1]:
				dp[i][j] = dp[i-1][j]
			default:
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	return dp
}

func backtrackLCS(a, b []int, dp [][]int) []int {
	n, m := len(a), len(b)
	lcs := make([]int, 0, dp[n][m])

	for i, j := n, m; i > 0 && j > 0; {
		switch {
		case a[i-1] == b[j-1]:
			lcs = append(lcs, a[i-1])
			i--
			j--
		case dp[i-1][j] >= dp[i][j-1]:
			i--
		default:
			j--
		}
	}

	return lcs
}

// confirmCompressionStrip previews and confirms stripping zlib compression
// from the flagged tracks, independently of track order and removals.
func confirmPrompt(prompt string) bool {
	response := ui.Prompt(ui.Warning.Render(prompt + " [y/N] "))

	return response == "y" || response == "Y"
}

// promptLanguageFixes asks, per track, for a missing language tag. The
// correct value is free text, so unlike the keyword/flag mismatches it can't
// be batched into a single all/none choice and stays one prompt per track.
func promptLanguageFixes(ebml *matroska.EbmlMetadata) []matroska.TrackEdit {
	var edits []matroska.TrackEdit

	for i := range ebml.Tracks {
		track := ebml.Tracks[i]
		if !NeedsLanguageFix(track) {
			continue
		}

		ui.Println()
		ui.Println("  " + trackLabel(&track))

		if lang := ui.Prompt("  " + ui.Warning.Render("Missing language tag.") + " Enter a language code (blank to skip): "); lang != "" {
			edits = append(edits, matroska.TrackEdit{Number: track.Properties.Number, Props: map[string]string{"language": lang}})
		}
	}

	return edits
}

// promptMultiLangNameFixes asks, per track, for a missing or incomplete
// multi-language ('mul') Name. Free text, so it stays one prompt per track.
func promptMultiLangNameFixes(ebml *matroska.EbmlMetadata) []matroska.TrackEdit {
	var edits []matroska.TrackEdit

	for i := range ebml.Tracks {
		track := ebml.Tracks[i]
		if !NeedsMultiLangName(track) {
			continue
		}

		ui.Println()
		ui.Println("  " + trackLabel(&track))

		if name := ui.Prompt("  " + ui.Warning.Render("Multi-language ('mul') track should list at least two languages in its Name.") + " Enter a name (blank to skip): "); name != "" {
			edits = append(edits, matroska.TrackEdit{Number: track.Properties.Number, Props: map[string]string{"name": name}})
		}
	}

	return edits
}

// promptKeywordFlagFixes gathers every SDH/Forced/Commentary/Descriptive
// keyword-flag mismatch, consolidates them per track, then previews and
// confirms using previewFlagEdits — consistent with the auto-computed flag edits
// that follow. Multiple mismatches on the same track are batched into one edit.
func promptKeywordFlagFixes(ebml *matroska.EbmlMetadata) []matroska.TrackEdit {
	edits := buildKeywordFlagEdits(ebml)
	if len(edits) == 0 {
		return nil
	}

	total := countFlagProps(edits)

	ui.Println()
	previewFlagEdits(ebml, edits)

	switch promptBulkChoice(fmt.Sprintf("Set %d flag(s) to match track names?", total)) {
	case bulkAll:
		return edits
	case bulkSelect:
		return selectKeywordFlagEdits(ebml, edits)
	default:
		ui.Println(ui.Muted.Render("Skipping keyword/flag fixes..."))

		return nil
	}
}

// buildKeywordFlagEdits consolidates all keyword/flag mismatches per track into
// a slice of TrackEdits ready for previewFlagEdits and SetTrackProperties.
func buildKeywordFlagEdits(ebml *matroska.EbmlMetadata) []matroska.TrackEdit {
	var edits []matroska.TrackEdit

	for i := range ebml.Tracks {
		fixes := ReverseKeywordFlagFixes(ebml.Tracks[i])
		if len(fixes) == 0 {
			continue
		}

		props := make(map[string]string, len(fixes))
		for _, fix := range fixes {
			props[fix.Property] = "1"
		}

		edits = append(edits, matroska.TrackEdit{Number: ebml.Tracks[i].Properties.Number, Props: props})
	}

	return edits
}

func countFlagProps(edits []matroska.TrackEdit) int {
	n := 0

	for _, edit := range edits {
		n += len(edit.Props)
	}

	return n
}

// selectKeywordFlagEdits prompts once per track (showing its pending flag
// changes) and returns only the edits the user confirmed.
func selectKeywordFlagEdits(ebml *matroska.EbmlMetadata, edits []matroska.TrackEdit) []matroska.TrackEdit {
	var selected []matroska.TrackEdit

	for _, edit := range edits {
		track := findTrack(ebml, edit.Number)

		ui.Println()
		ui.Println("  " + trackLabel(track))
		ui.Println("  " + flagChangeTags(edit.Props))

		if confirmPrompt("  Apply these flag changes?") {
			selected = append(selected, edit)
		}
	}

	return selected
}

// bulkChoice is the user's answer to a batched all/none/select prompt.
type bulkChoice int

const (
	bulkNone bulkChoice = iota
	bulkAll
	bulkSelect
)

// promptBulkChoice asks prompt and reads an [a]ll/[n]one/[s]elect answer,
// defaulting to the safe bulkNone for anything else (including a blank Enter).
func promptBulkChoice(prompt string) bulkChoice {
	response := strings.ToLower(ui.Prompt(ui.Warning.Render(prompt) + " [a]ll / [n]one / [s]elect: "))

	switch response {
	case "a", "all":
		return bulkAll
	case "s", "select":
		return bulkSelect
	default:
		return bulkNone
	}
}

// mergeTrackEdits merges extra edits into base, combining property maps for
// tracks that appear in both.
func mergeTrackEdits(base, extra []matroska.TrackEdit) []matroska.TrackEdit {
	if len(extra) == 0 {
		return base
	}

	position := make(map[int]int, len(base))

	result := make([]matroska.TrackEdit, 0, len(base)+len(extra))
	for _, edit := range base {
		position[edit.Number] = len(result)
		result = append(result, edit)
	}

	for _, edit := range extra {
		if pos, ok := position[edit.Number]; ok {
			maps.Copy(result[pos].Props, edit.Props)

			continue
		}

		position[edit.Number] = len(result)
		result = append(result, edit)
	}

	return result
}

func findTrackByID(ebml *matroska.EbmlMetadata, id int) *matroska.EbmlTrack {
	for i := range ebml.Tracks {
		if ebml.Tracks[i].ID == id {
			return &ebml.Tracks[i]
		}
	}

	return nil
}

func trackLabel(track *matroska.EbmlTrack) string {
	if track == nil {
		return "?"
	}

	label := fmt.Sprintf("%s/%d (%s)", track.Type, track.TypeOrder, track.Properties.Language)
	if track.Properties.Name != "" {
		label += " " + strconv.Quote(track.Properties.Name)
	}

	return label
}

// trackColumns returns the individual display columns for a track used in the
// reorder table: type, language, codec, name, and a compact flags string.
func trackColumns(track *matroska.EbmlTrack) (trackType, lang, codec, name, flags string) {
	if track == nil {
		return "?", "", "", "", ""
	}

	return track.Type, track.Properties.Language, track.Codec, track.Properties.Name, trackFlagsCompact(track)
}

func trackFlagsCompact(track *matroska.EbmlTrack) string {
	p := track.Properties

	var parts []string

	if p.Default {
		parts = append(parts, "D")
	}

	if p.Forced {
		parts = append(parts, "Forced")
	}

	if p.OriginalLanguage {
		parts = append(parts, "Orig")
	}

	if p.Commentary {
		parts = append(parts, "Comm")
	}

	if p.HearingImpaired {
		parts = append(parts, "SDH")
	}

	if p.VisualImpaired {
		parts = append(parts, "AD")
	}

	return strings.Join(parts, " ")
}

func previewTrackEdits(section string, ebml *matroska.EbmlMetadata, edits []matroska.TrackEdit) {
	ui.Println(ui.ReportSection(section))

	for _, edit := range edits {
		track := findTrack(ebml, edit.Number)
		ui.Println(formatEditHeader(edit.Number, track))

		for _, key := range slices.Sorted(maps.Keys(edit.Props)) {
			ui.Println("  " + formatEditChange(key, edit.Props[key], track))
		}
	}

	ui.Println()
}

func findTrack(ebml *matroska.EbmlMetadata, number int) *matroska.EbmlTrack {
	for i := range ebml.Tracks {
		if ebml.Tracks[i].Properties.Number == number {
			return &ebml.Tracks[i]
		}
	}

	return nil
}

func formatEditHeader(number int, track *matroska.EbmlTrack) string {
	if track == nil {
		return ui.LabelStyle.Render("Track " + strconv.Itoa(number))
	}

	context := fmt.Sprintf("%s / %s", track.Type, track.Properties.Language)
	if track.Properties.Name != "" {
		context += " " + strconv.Quote(track.Properties.Name)
	}

	return ui.LabelStyle.Render("Track "+strconv.Itoa(number)) + " " + ui.Muted.Render("("+context+")")
}

// formatEditChange renders a name or language property edit as "Label: old
// -> new". Flag edits are rendered separately by previewFlagEdits/
// flagChangeTag, which use a compact [+]/[-] tag instead of a full sentence.
func formatEditChange(key, value string, track *matroska.EbmlTrack) string {
	arrow := ui.Muted.Render("->")

	switch key {
	case "name":
		old := ""
		if track != nil {
			old = track.Properties.Name
		}

		return fmt.Sprintf("Name: %s %s %s", quoteOrNone(old), arrow, quoteOrNone(value))
	case "language":
		old := ""
		if track != nil {
			old = track.Properties.Language
		}

		return fmt.Sprintf("Language: %s %s %s", quoteOrNone(old), arrow, quoteOrNone(value))
	default:
		return fmt.Sprintf("%s: %s %s", key, arrow, strconv.Quote(value))
	}
}

func prettyFlag(key string) string {
	name := strings.ReplaceAll(strings.TrimPrefix(key, "flag-"), "-", " ")
	if name == "" {
		return key
	}

	return strings.ToUpper(name[:1]) + name[1:]
}

func quoteOrNone(value string) string {
	if value == "" {
		return ui.Muted.Render("(none)")
	}

	return strconv.Quote(value)
}
