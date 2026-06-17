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

	"codeberg.org/upPollo/parsec/internal/checks"
	"codeberg.org/upPollo/parsec/internal/config"
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
	originalLang, err := normalizeOriginalLanguageCode(opts.OriginalLanguage)
	if err != nil {
		return err
	}

	opts.OriginalLanguage = originalLang

	if err := fixMatroskaTracks(filePath, opts); err != nil {
		return err
	}

	if opts.Remux {
		return remuxMatroska(filePath, opts)
	}

	return nil
}

func fixMatroskaTracks(filePath string, opts Options) error {
	ebml, err := matroska.GetEbmlMetadata(filePath)
	if err != nil {
		// Not a Matroska file or unreadable: nothing to fix at container level.
		ui.PrintDebug(fmt.Sprintf("skipping track fixes for %s: %v", ui.AnonymizePath(filePath), err))

		return nil
	}

	edits := mergeTrackEdits(checks.ComputeMatroskaFixes(ebml.Tracks), promptManualTrackFixes(ebml, opts))
	if len(edits) == 0 {
		return nil
	}

	previewTrackEdits(ebml, edits)

	if opts.DryRun {
		ui.Println(ui.Muted.Render("Dry run: no changes made."))

		return nil
	}

	if !opts.Unattended && !ui.ConfirmContinue("Apply these track fixes?") {
		ui.Println(ui.Muted.Render("Skipping track fixes..."))

		return nil
	}

	if err := matroska.SetTrackProperties(filePath, edits); err != nil {
		ui.PrintError(fmt.Sprintf("Error fixing tracks for %s: %v", ui.AnonymizePath(filePath), err))

		return errTrackFix
	}

	ui.PrintSuccess("Track properties realigned.")

	return nil
}

func remuxMatroska(filePath string, opts Options) error {
	ebml, err := matroska.GetEbmlMetadata(filePath)
	if err != nil {
		ui.PrintDebug(fmt.Sprintf("skipping remux for %s: %v", ui.AnonymizePath(filePath), err))

		return nil
	}

	originalLang := lookupOriginalLanguage(filePath, ebml.Tracks, opts)

	plan := checks.ComputeMatroskaRemux(ebml.Tracks, originalLang)
	if plan.IsEmpty() {
		return nil
	}

	ui.Println(ui.ReportSection("Container Remux"))

	remuxOpts := matroska.RemuxOptions{
		TrackOrder:              plan.TrackOrder,
		StripCompressionIDs:     plan.StripCompressionIDs,
		DisableTrackCompression: config.IsCheckEnabled("matroska_zlib_compression"),
	}

	previewRemuxPlan(ebml, plan)

	remuxOpts.RemoveTrackIDs = selectRemovals(ebml, plan.RemovalCandidates, opts)
	if remuxOpts.IsEmpty() {
		ui.Println(ui.Muted.Render("Nothing selected to remux."))

		return nil
	}

	previewCompressionPolicy(plan, remuxOpts)

	if opts.DryRun {
		ui.Println(ui.Muted.Render("Dry run: no changes made."))

		return nil
	}

	if !opts.Unattended && !ui.ConfirmContinue("Remux now? (this rewrites the whole container)") {
		ui.Println(ui.Muted.Render("Skipping remux..."))

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

func previewRemuxPlan(ebml *matroska.EbmlMetadata, plan checks.MatroskaRemuxPlan) {
	if len(plan.TrackOrder) > 0 {
		labels := make([]string, 0, len(plan.TrackOrder))
		for _, id := range plan.TrackOrder {
			labels = append(labels, trackLabel(findTrackByID(ebml, id)))
		}

		ui.Println("  Reorder tracks: " + strings.Join(labels, ui.Muted.Render(" > ")))
	}

	for _, id := range plan.StripCompressionIDs {
		ui.Println("  Strip compression from " + trackLabel(findTrackByID(ebml, id)))
	}
}

func previewCompressionPolicy(plan checks.MatroskaRemuxPlan, opts matroska.RemuxOptions) {
	if !opts.DisableTrackCompression || len(plan.StripCompressionIDs) > 0 {
		return
	}

	ui.Println("  Write kept tracks without container compression.")
}

func verifyRemuxResult(filePath, originalLang string) {
	ebml, err := matroska.GetEbmlMetadata(filePath)
	if err != nil {
		ui.PrintWarning("Container remux applied, but verification failed.")

		return
	}

	if checks.ComputeMatroskaRemux(ebml.Tracks, originalLang).IsEmpty() {
		ui.PrintSuccess("Container remux fixes complete.")

		return
	}

	ui.PrintWarning("Container remux applied, but remaining remux issues were detected; run check for details.")
}

// selectRemovals prompts the user to confirm each proposed track removal and
// returns the IDs of the tracks they chose to drop. Removals are skipped in
// unattended mode because they are destructive and require human judgement.
func selectRemovals(ebml *matroska.EbmlMetadata, candidates []checks.RemovalCandidate, opts Options) []int {
	if len(candidates) == 0 {
		return nil
	}

	if opts.Unattended || !ui.IsTerminal() {
		ui.PrintWarning("Skipping track removals (destructive; requires confirmation).")

		return nil
	}

	var (
		ids           []int
		unwantedAudio []checks.RemovalCandidate
	)

	for _, candidate := range candidates {
		if candidate.Kind == checks.RemovalUnwantedAudioLang {
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

func selectUnwantedAudioRemovals(ebml *matroska.EbmlMetadata, candidates []checks.RemovalCandidate) []int {
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

func removalLanguages(candidates []checks.RemovalCandidate) []string {
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

// promptManualTrackFixes interactively gathers the track fixes whose correct
// value the tool cannot derive: a missing language tag, a missing multi-language
// name, and keyword/flag mismatches. It returns nothing in non-interactive or
// dry-run mode.
func promptManualTrackFixes(ebml *matroska.EbmlMetadata, opts Options) []matroska.TrackEdit {
	if opts.DryRun || opts.Unattended || !ui.IsTerminal() {
		return nil
	}

	var edits []matroska.TrackEdit

	for i := range ebml.Tracks {
		track := ebml.Tracks[i]

		if props := promptTrackFixes(track); len(props) > 0 {
			edits = append(edits, matroska.TrackEdit{Number: track.Properties.Number, Props: props})
		}
	}

	return edits
}

func promptTrackFixes(track matroska.EbmlTrack) map[string]string {
	props := make(map[string]string)

	if checks.NeedsLanguageFix(track) {
		ui.Println()
		ui.Println("  " + trackLabel(&track))

		if lang := ui.Prompt("  " + ui.Warning.Render("Missing language tag.") + " Enter a language code (blank to skip): "); lang != "" {
			props["language"] = lang
		}
	}

	if checks.NeedsMultiLangName(track) {
		ui.Println()
		ui.Println("  " + trackLabel(&track))

		if name := ui.Prompt("  " + ui.Warning.Render("Multi-language ('mul') track should list at least two languages in its Name.") + " Enter a name (blank to skip): "); name != "" {
			props["name"] = name
		}
	}

	for _, fix := range checks.ReverseKeywordFlagFixes(track) {
		ui.Println()
		ui.Println("  " + trackLabel(&track))

		if confirmPrompt(fmt.Sprintf("  Name mentions %q but the %q flag is unset. Set it?", fix.Keyword, fix.Property)) {
			props[fix.Property] = "1"
		}
	}

	return props
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

func previewTrackEdits(ebml *matroska.EbmlMetadata, edits []matroska.TrackEdit) {
	ui.Println(ui.ReportSection("Property Realignment"))

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

	return ui.LabelStyle.Render("Track "+strconv.Itoa(number)) + " " + ui.Muted.Render("("+context+")")
}

func formatEditChange(key, value string, track *matroska.EbmlTrack) string {
	arrow := ui.Muted.Render("->")

	switch {
	case key == "name":
		old := ""
		if track != nil {
			old = track.Properties.Name
		}

		return fmt.Sprintf("Name: %s %s %s", quoteOrNone(old), arrow, quoteOrNone(value))
	case key == "language":
		old := ""
		if track != nil {
			old = track.Properties.Language
		}

		return fmt.Sprintf("Language: %s %s %s", quoteOrNone(old), arrow, quoteOrNone(value))
	case strings.HasPrefix(key, "flag-"):
		return fmt.Sprintf("%s flag: %s %s %s", prettyFlag(key), flagState(currentFlag(key, track)), arrow, flagState(value == "1"))
	default:
		return fmt.Sprintf("%s: %s %s", key, arrow, strconv.Quote(value))
	}
}

func currentFlag(key string, track *matroska.EbmlTrack) bool {
	if track == nil {
		return false
	}

	switch key {
	case "flag-default":
		return track.Properties.Default
	case "flag-original":
		return track.Properties.OriginalLanguage
	case "flag-forced":
		return track.Properties.Forced
	case "flag-commentary":
		return track.Properties.Commentary
	case "flag-hearing-impaired":
		return track.Properties.HearingImpaired
	case "flag-visual-impaired":
		return track.Properties.VisualImpaired
	default:
		return false
	}
}

func prettyFlag(key string) string {
	name := strings.ReplaceAll(strings.TrimPrefix(key, "flag-"), "-", " ")
	if name == "" {
		return key
	}

	return strings.ToUpper(name[:1]) + name[1:]
}

func flagState(set bool) string {
	if set {
		return ui.Success.Render("set")
	}

	return ui.Muted.Render("unset")
}

func quoteOrNone(value string) string {
	if value == "" {
		return ui.Muted.Render("(none)")
	}

	return strconv.Quote(value)
}
