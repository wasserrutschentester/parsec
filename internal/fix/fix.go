// Package fix applies deterministic repairs for Matroska track metadata and
// container-level checks.
package fix

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/filename"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/ui"
)

var (
	errTrackFix = errors.New("applying track fixes failed")
	errRemux    = errors.New("remuxing failed")
)

// Options controls which fixes are applied and how interactive decisions are
// handled.
type Options struct {
	DryRun           bool
	Remux            bool
	Unattended       bool
	OriginalLanguage string
	ImdbID           string
	TmdbID           int
	TvdbID           int
}

// ApplyFile applies enabled Matroska fixes to one file.
func ApplyFile(filePath string, opts Options) error {
	ui.Println(ui.LabelValue("Target Name:", filename.GetBaseName(filePath)))

	originalLang, err := normalizeOriginalLanguageCode(opts.OriginalLanguage)
	if err != nil {
		return err
	}

	opts.OriginalLanguage = originalLang

	if err := fixMatroskaTracks(filePath, opts); err != nil {
		return err
	}

	if err := fixContainerMetadata(filePath, opts); err != nil {
		return err
	}

	return remuxMatroska(filePath, opts)
}

// fixContainerMetadata applies the in-place, mkvpropedit-based container fixes
// that don't touch individual tracks: clearing junk title/writing-application
// fields, attaching missing subtitle fonts, and removing font attachments
// unused by any subtitle track. Unlike the --remux fixes, these never rewrite
// the container, but attachment removal is destructive, so it is always
// prompted and skipped unattended.
func fixContainerMetadata(filePath string, opts Options) error {
	ebml, err := matroska.GetEbmlMetadata(filePath)
	if err != nil {
		ui.PrintDebug(fmt.Sprintf("skipping container fixes for %s: %v", ui.AnonymizePath(filePath), err))

		return nil
	}

	meta := buildFixMetadata(filePath, opts)
	if err := fixContainerProperties(filePath, ebml, meta, opts); err != nil {
		return err
	}

	if err := renameNonCompliantFonts(filePath, ebml, opts); err != nil {
		return err
	}

	if err := fixChapterAlignment(filePath, ebml, opts); err != nil {
		return err
	}

	if err := attachMissingFonts(filePath, ebml, opts); err != nil {
		return err
	}

	return removeUnusedFonts(filePath, ebml, opts)
}

// confirmApply prints the dry-run notice and reports false when opts.DryRun is
// set, otherwise prompts with prompt (defaulting to "no", like every other fix
// confirmation). Unattended mode auto-applies callers that opt into it via the
// policy helper below and skips callers that need explicit confirmation.
func confirmApply(opts Options, prompt, skipMsg string) bool {
	return confirmApplyWithPolicy(opts, prompt, skipMsg, true)
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
func fixChapterAlignment(filePath string, ebml *matroska.EbmlMetadata, opts Options) error {
	fix := ComputeChapterKeyframeSnaps(filePath, ebml)
	if fix.Changed == 0 {
		return nil
	}

	ui.Println(ui.ReportSection("Chapter Keyframe Alignment"))
	ui.Println(fmt.Sprintf("  Snap %d of %d chapter(s) to the nearest video keyframe.", fix.Changed, len(fix.Times)))

	if !confirmApplyWithPolicy(opts, "Apply chapter keyframe alignment?", "Skipping chapter alignment...", false) {
		return nil
	}

	if err := matroska.RewriteChapterTimestamps(filePath, fix.Times); err != nil {
		ui.PrintError(fmt.Sprintf("Error aligning chapters for %s: %v", ui.AnonymizePath(filePath), err))

		return errTrackFix
	}

	ui.PrintSuccess("Chapters aligned to keyframes.")

	return nil
}

// renameNonCompliantFonts renames font attachments whose filename doesn't
// match their internal font name, so naming tools (e.g. fonts that won't
// match a subtitle's \fn reference by filename) stay consistent with the
// font's actual name. Content is untouched, so this is non-destructive.
func renameNonCompliantFonts(filePath string, ebml *matroska.EbmlMetadata, opts Options) error {
	renames := ComputeFontRenames(filePath, ebml)
	if len(renames) == 0 {
		return nil
	}

	ui.Println(ui.ReportSection("Font Filename Compliance"))

	ids := make(map[int]string, len(renames))

	for _, r := range renames {
		ui.Println(fmt.Sprintf("  %s -> %s %s", quoteOrNone(r.OldName), quoteOrNone(r.NewName),
			ui.Muted.Render("(internal: "+strings.Join(r.InternalNames, ", ")+")")))
		ids[r.ID] = r.NewName
	}

	if !confirmApply(opts, "Rename these font attachments?", "Skipping font renames...") {
		return nil
	}

	if err := matroska.RenameAttachments(filePath, ids); err != nil {
		ui.PrintError(fmt.Sprintf("Error renaming font attachments for %s: %v", ui.AnonymizePath(filePath), err))

		return errTrackFix
	}

	ui.PrintSuccess("Font attachment filenames aligned.")

	return nil
}

// fixContainerProperties applies each pending container property fix (title
// hygiene, writing-application hygiene) as its own independently confirmable
// change, since they come from unrelated checks and one being declined
// shouldn't block the other.
func fixContainerProperties(filePath string, ebml *matroska.EbmlMetadata, meta *metadata.Metadata, opts Options) error {
	props := ComputeContainerFixes(ebml, meta)

	for _, key := range slices.Sorted(maps.Keys(props)) {
		if err := applyContainerProperty(filePath, ebml, opts, key, props[key]); err != nil {
			return err
		}
	}

	return nil
}

func applyContainerProperty(filePath string, ebml *matroska.EbmlMetadata, opts Options, key, newValue string) error {
	label := containerKeyLabel(key)

	ui.Println(ui.ReportSection(label))
	ui.Println("  " + formatContainerChange(key, containerPropertyValue(ebml, key), newValue))
	ui.Println()

	if !confirmApplyWithPolicy(opts, "Clear "+label+"?", "Skipping "+label+" fix...", false) {
		return nil
	}

	if err := matroska.SetContainerProperties(filePath, map[string]string{key: newValue}); err != nil {
		ui.PrintError(fmt.Sprintf("Error fixing %s for %s: %v", label, ui.AnonymizePath(filePath), err))

		return errTrackFix
	}

	ui.PrintSuccess(label + " cleaned up.")

	return nil
}

func containerPropertyValue(ebml *matroska.EbmlMetadata, key string) string {
	switch key {
	case "title":
		return ebml.Container.Properties.Title
	case "writing-application":
		return ebml.Container.Properties.WritingApplication
	default:
		return ""
	}
}

func formatContainerChange(key, oldValue, newValue string) string {
	arrow := ui.Muted.Render("->")

	return fmt.Sprintf("%s: %s %s %s", containerKeyLabel(key), quoteOrNone(oldValue), arrow, quoteOrNone(newValue))
}

func containerKeyLabel(key string) string {
	switch key {
	case "title":
		return "Title"
	case "writing-application":
		return "Writing Application"
	default:
		return key
	}
}

// removeUnusedFonts prompts to delete font attachments unused by any
// subtitle track. It is skipped in dry-run, unattended, or non-interactive
// runs since attachment removal is destructive and requires confirmation.
func removeUnusedFonts(filePath string, ebml *matroska.EbmlMetadata, opts Options) error {
	unused := ComputeUnusedFontAttachments(filePath, ebml)
	if len(unused) == 0 {
		return nil
	}

	ui.Println(ui.ReportSection("Unused Font Attachments"))

	for _, att := range unused {
		ui.Println("  " + att.FileName)
	}

	if opts.DryRun {
		ui.Println(ui.Muted.Render("Dry run: no changes made."))

		return nil
	}

	if opts.Unattended || !ui.IsTerminal() {
		ui.PrintWarning("Skipping unused font removal (destructive; requires confirmation).")

		return nil
	}

	if !confirmPrompt("  Delete these unused font attachments?") {
		ui.Println(ui.Muted.Render("Skipping unused font removal..."))

		return nil
	}

	ids := make([]int, 0, len(unused))
	for _, att := range unused {
		ids = append(ids, att.ID)
	}

	if err := matroska.DeleteAttachments(filePath, ids); err != nil {
		ui.PrintError(fmt.Sprintf("Error removing unused fonts for %s: %v", ui.AnonymizePath(filePath), err))

		return errTrackFix
	}

	ui.PrintSuccess("Unused font attachments removed.")

	return nil
}

// fixMatroskaTracks applies track property edits in three independently
// confirmable groups (flags, names, language tags) rather than one combined
// batch, so declining one group never blocks the others.
func fixMatroskaTracks(filePath string, opts Options) error {
	ebml, err := matroska.GetEbmlMetadata(filePath)
	if err != nil {
		// Not a Matroska file or unreadable: nothing to fix at container level.
		ui.PrintDebug(fmt.Sprintf("skipping track fixes for %s: %v", ui.AnonymizePath(filePath), err))

		return nil
	}

	var keywordFlagEdits, multiLangNameEdits, languageEdits []matroska.TrackEdit

	if !opts.DryRun && !opts.Unattended && ui.IsTerminal() {
		keywordFlagEdits = promptKeywordFlagFixes(ebml)
		multiLangNameEdits = promptMultiLangNameFixes(ebml)
		languageEdits = promptLanguageFixes(ebml)
	}

	flagEdits := mergeTrackEdits(ComputeMatroskaFlagFixes(ebml.Tracks), keywordFlagEdits)
	if err := applyTrackEditGroup(filePath, opts, flagEdits, func() { previewFlagEdits(ebml, flagEdits) },
		"Apply these flag fixes?", "Skipping flag fixes...", "Track flags realigned."); err != nil {
		return err
	}

	nameEdits := mergeTrackEdits(ComputeMatroskaNameFixes(ebml.Tracks), multiLangNameEdits)
	if err := applyTrackEditGroup(filePath, opts, nameEdits, func() { previewTrackEdits("Track Names", ebml, nameEdits) },
		"Apply these name fixes?", "Skipping name fixes...", "Track names realigned."); err != nil {
		return err
	}

	return applyTrackEditGroup(filePath, opts, languageEdits, func() { previewTrackEdits("Language Tags", ebml, languageEdits) },
		"Apply these language tag fixes?", "Skipping language tag fixes...", "Language tags updated.")
}

// applyTrackEditGroup calls preview, confirms and applies one batch of track
// property edits via a single mkvpropedit call. Returns nil without writing
// anything when edits is empty or the user declines.
func applyTrackEditGroup(filePath string, opts Options, edits []matroska.TrackEdit, preview func(), prompt, skipMsg, successMsg string) error {
	if len(edits) == 0 {
		return nil
	}

	preview()

	if !confirmApply(opts, prompt, skipMsg) {
		return nil
	}

	if err := matroska.SetTrackProperties(filePath, edits); err != nil {
		ui.PrintError(fmt.Sprintf("Error fixing tracks for %s: %v", ui.AnonymizePath(filePath), err))

		return errTrackFix
	}

	ui.PrintSuccess(successMsg)

	return nil
}

// previewFlagEdits renders pending flag edits as a track table, matching the
// layout check uses for track issues. Each flag is shown as a compact
// "[+] Label" (being set) / "[-] Label" (being cleared) tag instead of a full
// sentence per flag, since a table cell has little room.
func previewFlagEdits(ebml *matroska.EbmlMetadata, edits []matroska.TrackEdit) {
	ui.Println(ui.ReportSection("Track Flags"))

	headers := []string{"Track", "Type", "Lang", "Name", "Changes"}
	rows := make([][]string, 0, len(edits))

	for _, edit := range edits {
		track := findTrack(ebml, edit.Number)

		trackType, lang, name := "?", "?", ""
		if track != nil {
			trackType, lang, name = track.Type, track.Properties.Language, track.Properties.Name
		}

		rows = append(rows, []string{strconv.Itoa(edit.Number), trackType, lang, name, flagChangeTags(edit.Props)})
	}

	ui.Println(ui.TrackTable(headers, rows))
	ui.Println()
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
func remuxMatroska(filePath string, opts Options) error {
	ebml, err := matroska.GetEbmlMetadata(filePath)
	if err != nil {
		ui.PrintDebug(fmt.Sprintf("skipping remux for %s: %v", ui.AnonymizePath(filePath), err))

		return nil
	}

	// The MDB original-language lookup is a real network call (and may prompt
	// for disambiguation), so it only runs when --remux is actually going to
	// apply something; a plain preview must stay free of side effects.
	var originalLang string
	if opts.Remux {
		originalLang = lookupOriginalLanguage(filePath, ebml.Tracks, opts)
	}

	plan := ComputeMatroskaRemux(ebml.Tracks, originalLang)
	if plan.IsEmpty() {
		return nil
	}

	ui.Println(ui.ReportSection("Container Remux"))

	if !opts.Remux {
		previewRemuxWithoutApplying(ebml, plan)

		return nil
	}

	remuxOpts := matroska.RemuxOptions{}

	if confirmTrackOrder(ebml, plan, opts) {
		remuxOpts.TrackOrder = plan.TrackOrder
	}

	if confirmCompressionStrip(ebml, plan, opts) {
		remuxOpts.StripCompressionIDs = plan.StripCompressionIDs
		remuxOpts.DisableTrackCompression = config.IsCheckEnabled("matroska_zlib_compression")
	}

	remuxOpts.RemoveTrackIDs = selectRemovals(ebml, plan.RemovalCandidates, opts)

	if remuxOpts.IsEmpty() {
		ui.Println(ui.Muted.Render("Nothing selected to remux."))

		return nil
	}

	ui.Println(ui.Muted.Render("Remuxing... this may take a while for large files."))

	if err := matroska.RemuxTracks(filePath, remuxOpts); err != nil {
		ui.PrintError(fmt.Sprintf("Error remuxing %s: %v", ui.AnonymizePath(filePath), err))

		return errRemux
	}

	verifyRemuxResult(filePath, originalLang)

	return nil
}

// previewRemuxWithoutApplying shows the full remux plan and tells the user
// how to apply it, without performing the MDB lookup or touching the file.
func previewRemuxWithoutApplying(ebml *matroska.EbmlMetadata, plan MatroskaRemuxPlan) {
	previewRemuxPlan(ebml, plan)

	if needsOriginalLanguageForUnwantedAudio(ebml.Tracks) {
		ui.Println(ui.Muted.Render("  (unwanted-language audio pruning needs --remux to resolve the MDB original language)"))
	}

	ui.Println(ui.Muted.Render("Run with --remux to apply these changes."))
}

// previewRemuxPlan lists every pending remux-only change (track order,
// compression stripping, removal candidates with their reasons) without
// modifying anything, so it's safe to print whether or not --remux was given.
func previewRemuxPlan(ebml *matroska.EbmlMetadata, plan MatroskaRemuxPlan) {
	if len(plan.TrackOrder) > 0 {
		ui.Println(trackOrderTable(ebml, plan.TrackOrder))
	}

	for _, id := range plan.StripCompressionIDs {
		ui.Println("  Strip compression from " + trackLabel(findTrackByID(ebml, id)))
	}

	for _, candidate := range plan.RemovalCandidates {
		ui.Println("  Remove " + trackLabel(findTrackByID(ebml, candidate.TrackID)) + ": " + ui.Warning.Render(candidate.Reason))
	}
}

// confirmTrackOrder previews and confirms the track-reordering part of the
// remux plan independently of compression and removals.
func confirmTrackOrder(ebml *matroska.EbmlMetadata, plan MatroskaRemuxPlan, opts Options) bool {
	if len(plan.TrackOrder) == 0 {
		return false
	}

	ui.Println(trackOrderTable(ebml, plan.TrackOrder))

	return confirmApply(opts, "Reorder tracks like this?", "Skipping track reordering...")
}

// trackOrderTable renders a before/after table for the reorder: only the
// tracks that were genuinely out of place (not part of the longest common
// subsequence between the current and desired order) are highlighted; the
// rest keep their relative order and just shift index as a side effect.
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
func confirmCompressionStrip(ebml *matroska.EbmlMetadata, plan MatroskaRemuxPlan, opts Options) bool {
	if len(plan.StripCompressionIDs) == 0 {
		return false
	}

	for _, id := range plan.StripCompressionIDs {
		ui.Println("  Strip compression from " + trackLabel(findTrackByID(ebml, id)))
	}

	return confirmApply(opts, "Strip container compression from these tracks?", "Skipping compression strip...")
}

func verifyRemuxResult(filePath, originalLang string) {
	ebml, err := matroska.GetEbmlMetadata(filePath)
	if err != nil {
		ui.PrintWarning("Container remux applied, but verification failed.")

		return
	}

	if ComputeMatroskaRemux(ebml.Tracks, originalLang).IsEmpty() {
		ui.PrintSuccess("Container remux fixes complete.")

		return
	}

	ui.PrintWarning("Container remux applied, but remaining remux issues were detected; run check for details.")
}

// selectRemovals prompts the user to confirm each proposed track removal and
// returns the IDs of the tracks they chose to drop. Removals are skipped in
// unattended mode because they are destructive and require human judgement.
func selectRemovals(ebml *matroska.EbmlMetadata, candidates []RemovalCandidate, opts Options) []int {
	if len(candidates) == 0 {
		return nil
	}

	if opts.Unattended || !ui.IsTerminal() {
		ui.PrintWarning("Skipping track removals (destructive; requires confirmation).")

		return nil
	}

	var (
		ids           []int
		unwantedAudio []RemovalCandidate
	)

	for _, candidate := range candidates {
		if candidate.Kind == RemovalUnwantedAudioLang {
			unwantedAudio = append(unwantedAudio, candidate)

			continue
		}

		ui.Println()
		ui.Println("  " + trackLabel(findTrackByID(ebml, candidate.TrackID)))
		ui.Println("  " + ui.Warning.Render(candidate.Reason))

		if confirmPrompt("  Remove this track?") {
			ids = append(ids, candidate.TrackID)
		}
	}

	ids = append(ids, selectUnwantedAudioRemovals(ebml, unwantedAudio)...)

	return ids
}

func selectUnwantedAudioRemovals(ebml *matroska.EbmlMetadata, candidates []RemovalCandidate) []int {
	if len(candidates) == 0 {
		return nil
	}

	ui.Println()
	ui.Println("  " + ui.Warning.Render("Unwanted audio languages: ") + strings.Join(removalLanguages(candidates), ", "))

	for _, candidate := range candidates {
		ui.Println("  " + trackLabel(findTrackByID(ebml, candidate.TrackID)))
	}

	if !confirmPrompt("  Remove audio tracks in these languages?") {
		return nil
	}

	ids := make([]int, 0, len(candidates))
	for _, candidate := range candidates {
		ids = append(ids, candidate.TrackID)
	}

	return ids
}

func removalLanguages(candidates []RemovalCandidate) []string {
	seen := make(map[string]bool, len(candidates))

	var langs []string

	for _, candidate := range candidates {
		lang := candidate.Track.Properties.Language
		if lang == "" {
			lang = "und"
		}

		if !seen[lang] {
			seen[lang] = true
			langs = append(langs, lang)
		}
	}

	slices.Sort(langs)

	return langs
}

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

// keywordMismatch pairs a track with one keyword/flag mismatch it needs
// resolved, e.g. its name mentions "Commentary" but flag-commentary is unset.
type keywordMismatch struct {
	track matroska.EbmlTrack
	fix   KeywordFlagFix
}

// promptKeywordFlagFixes gathers every SDH/Forced/Commentary/Descriptive
// keyword-flag mismatch across all tracks into one overview table (the same
// columns check uses for track issues) instead of one confirmation per track,
// then lets the user apply all of them, none, or pick individually.
func promptKeywordFlagFixes(ebml *matroska.EbmlMetadata) []matroska.TrackEdit {
	var mismatches []keywordMismatch

	for i := range ebml.Tracks {
		track := ebml.Tracks[i]
		for _, fix := range ReverseKeywordFlagFixes(track) {
			mismatches = append(mismatches, keywordMismatch{track: track, fix: fix})
		}
	}

	if len(mismatches) == 0 {
		return nil
	}

	ui.Println()
	ui.Println(ui.ReportSection("Keyword/Flag Mismatches"))
	ui.Println(keywordMismatchTable(mismatches))

	switch promptBulkChoice(fmt.Sprintf("Set %d flag(s) to match the track names?", len(mismatches))) {
	case bulkAll:
		return keywordMismatchEdits(mismatches, nil)
	case bulkSelect:
		selected := make(map[int]bool, len(mismatches))

		for i, m := range mismatches {
			if confirmPrompt(fmt.Sprintf("  Set %q on %s?", m.fix.Property, trackLabel(&m.track))) {
				selected[i] = true
			}
		}

		return keywordMismatchEdits(mismatches, selected)
	default:
		ui.Println(ui.Muted.Render("Skipping keyword/flag fixes..."))

		return nil
	}
}

func keywordMismatchTable(mismatches []keywordMismatch) string {
	headers := []string{"Track", "Type", "Lang", "Name", "Keyword", "Flag"}
	rows := make([][]string, 0, len(mismatches))

	for _, m := range mismatches {
		rows = append(rows, []string{
			strconv.Itoa(m.track.Properties.Number),
			m.track.Type,
			m.track.Properties.Language,
			m.track.Properties.Name,
			m.fix.Keyword,
			m.fix.Property,
		})
	}

	return ui.TrackTable(headers, rows)
}

// keywordMismatchEdits builds the track edits for the given mismatches. A nil
// selected applies all of them; otherwise only the indices marked true are
// included.
func keywordMismatchEdits(mismatches []keywordMismatch, selected map[int]bool) []matroska.TrackEdit {
	props := make(map[int]map[string]string)

	var order []int

	for i, m := range mismatches {
		if selected != nil && !selected[i] {
			continue
		}

		number := m.track.Properties.Number

		p, ok := props[number]
		if !ok {
			p = make(map[string]string)
			props[number] = p

			order = append(order, number)
		}

		p[m.fix.Property] = "1"
	}

	edits := make([]matroska.TrackEdit, 0, len(order))
	for _, number := range order {
		edits = append(edits, matroska.TrackEdit{Number: number, Props: props[number]})
	}

	return edits
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
