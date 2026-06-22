package checks

import (
	"cmp"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/text/language"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

// cleanSplitRegex tokenizes a track name for cleaning. Unlike wordSplitRegex it
// does not split on dots, so channel notations such as "5.1" stay intact.
var cleanSplitRegex = regexp.MustCompile(`[\s/,;()]+`)

// ComputeMatroskaFlagFixes inspects the tracks of a Matroska file and returns
// the track property edits required to satisfy the auto-fixable flag checks:
// default-flag and original-language assignment. Checks that are disabled in
// the configuration are skipped, mirroring RunMatroskaChecks. Kept separate
// from ComputeMatroskaNameFixes so the two can be previewed and confirmed
// independently.
func ComputeMatroskaFlagFixes(tracks []matroska.EbmlTrack) []matroska.TrackEdit {
	builder := newFixBuilder()
	builder.computeDefaultFlagFixes(tracks)
	builder.computeOriginalFlagFixes(tracks)

	return builder.edits()
}

// ComputeMatroskaNameFixes inspects the tracks of a Matroska file and returns
// the track name edits required to satisfy the auto-fixable name-quality
// checks: junk keyword, codec and redundant-language removal, plus missing
// keyword appension. Checks that are disabled in the configuration are
// skipped, mirroring RunMatroskaChecks.
func ComputeMatroskaNameFixes(tracks []matroska.EbmlTrack) []matroska.TrackEdit {
	builder := newFixBuilder()
	builder.computeNameFixes(tracks)

	return builder.edits()
}

// ComputeContainerFixes returns the segment-level ("info") property edits
// needed to satisfy the title- and writing-application-hygiene checks. A
// matching property is cleared rather than rewritten with a guessed
// replacement, mirroring the conservative junk-removal approach used for
// track names. Checks that are disabled in the configuration are skipped.
func ComputeContainerFixes(ebml *matroska.EbmlMetadata) map[string]string {
	props := make(map[string]string)

	if config.IsCheckEnabled("matroska_title_hygiene") && matchesAnyPattern(ebml.Container.Properties.Title, titleJunkPatterns) {
		props["title"] = ""
	}

	if config.IsCheckEnabled("matroska_app_hygiene") && matchesAnyPattern(ebml.Container.Properties.WritingApplication, appJunkPatterns) {
		props["writing-application"] = ""
	}

	return props
}

// ChapterAlignmentFix describes the timestamp corrections needed to align
// chapters with video keyframes, satisfying matroska_chapters_keyframe_alignment.
// Times is the full, ordered list of chapter start times (ns) to write back:
// mkvpropedit rewrites chapters by document position, so already-aligned
// chapters pass their original time through unchanged alongside the snapped
// ones. Changed counts how many entries actually differ from the original.
type ChapterAlignmentFix struct {
	Times   []int64
	Changed int
}

// ComputeChapterKeyframeSnaps returns the chapter timestamp corrections that
// align each misaligned chapter with its nearest video keyframe. Returns a
// zero-value ChapterAlignmentFix (Changed == 0) when the check is disabled,
// the file has no chapters, the video keyframe index can't be read, or
// nothing needs to change. Only the first edition is considered, matching the
// check's own getChapters(); a file with additional editions is left alone.
func ComputeChapterKeyframeSnaps(filePath string, ebml *matroska.EbmlMetadata) ChapterAlignmentFix {
	if !config.IsCheckEnabled("matroska_chapters_keyframe_alignment") {
		return ChapterAlignmentFix{}
	}

	chapters := getChapters(ebml)
	if len(chapters) == 0 || len(ebml.Chapters) != 1 || len(ebml.Chapters[0].Editions) != 1 {
		return ChapterAlignmentFix{}
	}

	videoTrackNum := getVideoTrackNumberFromEBML(ebml)
	if videoTrackNum == 0 {
		return ChapterAlignmentFix{}
	}

	keyframes, err := matroska.ReadKeyframeTimestamps(filePath, videoTrackNum, ebml.Container.Properties.TimestampScale)
	if err != nil || len(keyframes) == 0 {
		return ChapterAlignmentFix{}
	}

	times, changed := snapChaptersToKeyframes(chapters, keyframes)
	if changed == 0 {
		return ChapterAlignmentFix{}
	}

	return ChapterAlignmentFix{Times: times, Changed: changed}
}

// snapChaptersToKeyframes returns, in chapter order, each chapter's start
// time snapped to its nearest keyframe when misaligned, or unchanged when
// already aligned, plus how many entries were actually snapped.
func snapChaptersToKeyframes(chapters []matroska.EbmlChapterAtom, keyframes []int64) ([]int64, int) {
	times := make([]int64, len(chapters))
	changed := 0

	for i, ch := range chapters {
		if aligned, _ := isAligned(ch.TimeStart, keyframes); aligned {
			times[i] = ch.TimeStart

			continue
		}

		times[i] = nearestKeyframe(ch.TimeStart, keyframes)
		changed++
	}

	return times, changed
}

// nearestKeyframe returns the keyframe timestamp closest to timeStart.
// keyframes must be non-empty.
func nearestKeyframe(timeStart int64, keyframes []int64) int64 {
	best := keyframes[0]
	bestDiff := absInt64(timeStart - best)

	for _, kf := range keyframes[1:] {
		if diff := absInt64(timeStart - kf); diff < bestDiff {
			bestDiff = diff
			best = kf
		}
	}

	return best
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}

	return v
}

// ComputeUnusedFontAttachments returns the font attachments that satisfy the
// matroska_unused_fonts check's removal criteria: not referenced by any
// subtitle track's Styles or inline tags. Returns nil when the check is
// disabled in the configuration.
func ComputeUnusedFontAttachments(filePath string, ebml *matroska.EbmlMetadata) []matroska.EbmlAttachment {
	if !config.IsCheckEnabled("matroska_unused_fonts") {
		return nil
	}

	fontMap, attachmentNames := getFontMapping(filePath, ebml.Attachments)
	allUsedFonts := computeUsedFonts(filePath, ebml.Tracks, fontMap)

	return unusedFontAttachments(ebml.Attachments, attachmentNames, allUsedFonts)
}

// FontRename describes a font attachment filename correction needed to
// satisfy the matroska_font_filename_compliance check.
type FontRename struct {
	ID      int
	OldName string
	NewName string
}

// ComputeFontRenames returns the font attachments whose filename should be
// renamed to match the font's internal name. Returns nil when the check is
// disabled or no font attachment carries a usable internal name.
func ComputeFontRenames(filePath string, ebml *matroska.EbmlMetadata) []FontRename {
	if !config.IsCheckEnabled("matroska_font_filename_compliance") {
		return nil
	}

	_, attachmentNames := getFontMapping(filePath, ebml.Attachments)

	return computeFontRenames(ebml.Attachments, attachmentNames)
}

// computeFontRenames is the pure font-rename policy, shared with tests so
// font extraction (and therefore real font files) is not required to verify it.
func computeFontRenames(attachments []matroska.EbmlAttachment, attachmentNames map[int][]string) []FontRename {
	var renames []FontRename

	for _, att := range attachments {
		if !isFontAttachment(att) {
			continue
		}

		names := attachmentNames[att.ID]
		if len(names) == 0 || fontFilenameCompliant(att.FileName, names) {
			continue
		}

		renames = append(renames, FontRename{ID: att.ID, OldName: att.FileName, NewName: fontRenameTarget(att.FileName, names[0])})
	}

	return renames
}

// fontRenameTarget builds the compliant filename for a font attachment,
// keeping the original extension and using the font's primary internal name.
func fontRenameTarget(oldName, internalName string) string {
	ext := ""
	if idx := strings.LastIndex(oldName, "."); idx != -1 {
		ext = oldName[idx:]
	}

	return internalName + ext
}

// fixBuilder accumulates property edits per track while preserving the order in
// which tracks are first touched, so the resulting edit list is deterministic.
type fixBuilder struct {
	order []int
	byNum map[int]map[string]string
}

func newFixBuilder() *fixBuilder {
	return &fixBuilder{byNum: make(map[int]map[string]string)}
}

func (b *fixBuilder) set(number int, key, value string) {
	props, ok := b.byNum[number]
	if !ok {
		props = make(map[string]string)
		b.byNum[number] = props
		b.order = append(b.order, number)
	}

	props[key] = value
}

func (b *fixBuilder) edits() []matroska.TrackEdit {
	edits := make([]matroska.TrackEdit, 0, len(b.order))
	for _, number := range b.order {
		edits = append(edits, matroska.TrackEdit{Number: number, Props: b.byNum[number]})
	}

	return edits
}

func (b *fixBuilder) computeDefaultFlagFixes(tracks []matroska.EbmlTrack) {
	if !config.IsCheckEnabled("matroska_default_flags") {
		return
	}

	audioCounts, subCounts := getTrackCounts(tracks)
	seenAudioLangs := make(map[string]bool)
	seenSubLangs := make(map[string]bool)

	for i := range tracks {
		track := tracks[i]
		if !isRelevantTrack(track) {
			continue
		}

		props := track.Properties
		isSpecialized := props.Forced || props.Commentary || props.VisualImpaired || props.HearingImpaired || props.TextDescriptions

		shouldBeDefault := false
		if !isSpecialized {
			shouldBeDefault = determineShouldBeDefault(track, audioCounts, subCounts, seenAudioLangs, seenSubLangs)
		}

		if props.Default != shouldBeDefault {
			b.set(props.Number, "flag-default", boolFlag(shouldBeDefault))
		}
	}
}

func (b *fixBuilder) computeOriginalFlagFixes(tracks []matroska.EbmlTrack) {
	if !config.IsCheckEnabled("matroska_original_language") {
		return
	}

	langHasOriginalFlag := getOriginalLanguageMap(tracks)

	for i := range tracks {
		track := tracks[i]
		if !isRelevantTrack(track) {
			continue
		}

		props := track.Properties
		if langHasOriginalFlag[props.Language] && !props.OriginalLanguage {
			b.set(props.Number, "flag-original", "1")
		}
	}
}

func (b *fixBuilder) computeNameFixes(tracks []matroska.EbmlTrack) {
	for i := range tracks {
		track := tracks[i]
		if !isRelevantTrack(track) {
			continue
		}

		if newName := fixedTrackName(track); newName != track.Properties.Name {
			// An empty value instructs SetTrackProperties to delete the name.
			b.set(track.Properties.Number, "name", newName)
		}
	}
}

// fixedTrackName returns the cleaned track name, removing junk keywords, simple
// codec names and redundant language names, then appending any keyword that a
// set flag requires. It returns the original name unchanged when nothing needs
// fixing, so cosmetic separator differences never trigger an edit.
func fixedTrackName(track matroska.EbmlTrack) string {
	original := track.Properties.Name
	if original == "" {
		return ""
	}

	kept, removed := cleanNameTokens(original, track.Properties.Language)

	result, appended := maybeAppendKeywords(strings.Join(kept, " "), track.Properties)
	if !removed && !appended {
		return original
	}

	return strings.TrimSpace(result)
}

// cleanNameTokens splits a track name and drops the tokens flagged by the
// enabled name checks, reporting whether anything was removed.
func cleanNameTokens(name, lang string) (kept []string, removed bool) {
	removeJunk := config.IsCheckEnabled("matroska_name_quality")
	removeCodecs := config.IsCheckEnabled("matroska_name_codecs")
	removeLang := config.IsCheckEnabled("matroska_name_redundant_lang")

	for _, token := range cleanSplitRegex.Split(name, -1) {
		switch {
		case token == "":
			continue
		case removeJunk && slices.Contains(junkKeywords, strings.ToUpper(token)):
			removed = true
		case removeCodecs && isSimpleCodecToken(token):
			removed = true
		case removeLang && isLanguageMatch(token, lang):
			removed = true
		default:
			kept = append(kept, token)
		}
	}

	return kept, removed
}

// maybeAppendKeywords appends the keywords required by the track's flags when
// the name-keyword check is enabled, reporting whether the name changed.
func maybeAppendKeywords(name string, props matroska.EbmlTrackProperties) (string, bool) {
	if !config.IsCheckEnabled("matroska_name_keywords") {
		return name, false
	}

	withKeywords := appendFlagKeywords(name, props)

	return withKeywords, withKeywords != name
}

func isSimpleCodecToken(token string) bool {
	upper := strings.ToUpper(token)
	if slices.Contains(simpleCodecs, upper) {
		return true
	}

	// Standalone "DTS" only; "DTS-HD", "DTS:X" and "DTS-ES" are single tokens
	// and are intentionally preserved.
	return upper == "DTS"
}

func isLanguageMatch(word, trackLang string) bool {
	tag := language.Make(trackLang)

	base, _ := tag.Base()

	return getLanguageCodeFromName(word) == base.String()
}

func appendFlagKeywords(name string, props matroska.EbmlTrackProperties) string {
	appendKeyword := func(keyword string) {
		name = strings.TrimSpace(name + " " + keyword)
	}

	if props.HearingImpaired && !strings.Contains(strings.ToUpper(name), "SDH") {
		appendKeyword("SDH")
	}

	if props.Forced && !strings.Contains(strings.ToUpper(name), "FORCED") {
		appendKeyword("Forced")
	}

	if props.Commentary && !strings.Contains(strings.ToUpper(name), "COMMENTARY") {
		appendKeyword("Commentary")
	}

	if props.VisualImpaired && !hasVisualImpairedKeyword(strings.ToUpper(name)) {
		appendKeyword("Descriptive")
	}

	return name
}

func hasVisualImpairedKeyword(upper string) bool {
	return strings.Contains(upper, "DESCRIPTIVE") || strings.Contains(upper, "DESCRIPTION") || adRegex.MatchString(upper)
}

func boolFlag(value bool) string {
	if value {
		return "1"
	}

	return "0"
}

// NeedsLanguageFix reports whether a track is missing a valid language tag and
// the corresponding check is enabled. The correct value is unknown, so the
// caller must obtain it from the user.
func NeedsLanguageFix(track matroska.EbmlTrack) bool {
	if !config.IsCheckEnabled("matroska_language_tag") || !isRelevantTrack(track) {
		return false
	}

	return language.Make(track.Properties.Language) == language.Und
}

// NeedsMultiLangName reports whether a multi-language ("mul") track needs a
// descriptive Name that the tool cannot derive, so the caller must obtain it
// from the user. It covers two enabled-check conditions: matroska_multi_lang (a
// 'mul' track must have a Name at all) and matroska_name_keywords (a 'mul'
// track's Name must list at least two languages).
func NeedsMultiLangName(track matroska.EbmlTrack) bool {
	if !isRelevantTrack(track) || language.Make(track.Properties.Language) != language.Make("mul") {
		return false
	}

	name := track.Properties.Name

	if config.IsCheckEnabled("matroska_multi_lang") && name == "" {
		return true
	}

	return config.IsCheckEnabled("matroska_name_keywords") && countLanguagesInString(name) < 2
}

// KeywordFlagFix describes a track whose name contains a keyword whose matching
// flag is not set. The resolution (set the flag or drop the word) is ambiguous,
// so the caller must ask the user.
type KeywordFlagFix struct {
	// Property is the mkvpropedit flag to set, e.g. "flag-commentary".
	Property string
	// Keyword is the keyword found in the track name, e.g. "Commentary".
	Keyword string
}

// ReverseKeywordFlagFixes returns the keyword/flag mismatches where the track
// name advertises a property (SDH, Forced, Commentary, descriptive) whose flag
// is not actually set. It returns nil when the name-keywords check is disabled.
func ReverseKeywordFlagFixes(track matroska.EbmlTrack) []KeywordFlagFix {
	if !config.IsCheckEnabled("matroska_name_keywords") || !isRelevantTrack(track) {
		return nil
	}

	props := track.Properties
	upper := strings.ToUpper(props.Name)

	keywordFlags := []struct {
		flagSet  bool
		inName   bool
		property string
		keyword  string
	}{
		{props.HearingImpaired, strings.Contains(upper, "SDH"), "flag-hearing-impaired", "SDH"},
		{props.Forced, strings.Contains(upper, "FORCED"), "flag-forced", "Forced"},
		{props.Commentary, strings.Contains(upper, "COMMENTARY"), "flag-commentary", "Commentary"},
		{props.VisualImpaired, hasVisualImpairedKeyword(upper), "flag-visual-impaired", "Descriptive"},
	}

	var fixes []KeywordFlagFix

	for _, kf := range keywordFlags {
		if !kf.flagSet && kf.inName {
			fixes = append(fixes, KeywordFlagFix{Property: kf.property, Keyword: kf.keyword})
		}
	}

	return fixes
}

// RemovalKind identifies why a track is proposed for removal.
type RemovalKind string

const (
	// RemovalDuplicateTrack identifies exact duplicate track removal.
	RemovalDuplicateTrack RemovalKind = "duplicate_track"
	// RemovalUnwantedAudioLang identifies non-preferred, non-original audio removal.
	RemovalUnwantedAudioLang RemovalKind = "unwanted_audio_language"
	// RemovalEmptyTrack identifies an audio track carrying no channels.
	RemovalEmptyTrack RemovalKind = "empty_track"
)

// RemovalCandidate describes a track proposed for removal during a remux,
// together with a human-readable reason. Removals are destructive, so the
// caller is expected to confirm each candidate with the user.
type RemovalCandidate struct {
	TrackID int
	Kind    RemovalKind
	Reason  string
	Track   matroska.EbmlTrack
}

// MatroskaRemuxPlan describes the lossless remux operations needed to satisfy
// the remux-only Matroska checks. Track IDs are mkvmerge track IDs (the "id"
// field), as required by mkvmerge.
type MatroskaRemuxPlan struct {
	// TrackOrder is the desired output order of track IDs. It is nil when the
	// tracks are already correctly ordered.
	TrackOrder []int
	// StripCompressionIDs lists track IDs whose container compression should be
	// removed.
	StripCompressionIDs []int
	// RemovalCandidates lists tracks proposed for removal (requires confirmation).
	RemovalCandidates []RemovalCandidate
}

// IsEmpty reports whether the plan contains no work.
func (p MatroskaRemuxPlan) IsEmpty() bool {
	return len(p.TrackOrder) == 0 && len(p.StripCompressionIDs) == 0 && len(p.RemovalCandidates) == 0
}

// ComputeMatroskaRemux inspects the tracks of a Matroska file and returns the
// remux operations needed to satisfy the checks that cannot be fixed in place:
// track ordering, container compression and removal of duplicate, redundant and
// unwanted-language tracks. originalLang is the MDB original language (may be
// empty); without it, unwanted-language pruning is skipped so the original
// track is never proposed for removal. Disabled checks are skipped.
func ComputeMatroskaRemux(tracks []matroska.EbmlTrack, originalLang string) MatroskaRemuxPlan {
	return MatroskaRemuxPlan{
		TrackOrder:          computeTrackOrder(tracks),
		StripCompressionIDs: computeCompressionStrips(tracks),
		RemovalCandidates:   computeRemovalCandidates(tracks, originalLang),
	}
}

// computeTrackOrder returns the desired output order (by track ID), grouping
// tracks as video, audio, subtitles and other while sorting audio and subtitle
// tracks by priority. It returns nil when the current order is already correct.
func computeTrackOrder(tracks []matroska.EbmlTrack) []int {
	if !config.IsCheckEnabled("matroska_track_order") {
		return nil
	}

	var video, audio, subs, other []matroska.EbmlTrack

	for _, track := range tracks {
		switch track.Type {
		case "audio":
			audio = append(audio, track)
		case "subtitles":
			subs = append(subs, track)
		case "video":
			video = append(video, track)
		default:
			other = append(other, track)
		}
	}

	byPriority := func(a, b matroska.EbmlTrack) int {
		return cmp.Compare(getTrackPriority(a), getTrackPriority(b))
	}
	slices.SortStableFunc(audio, byPriority)
	slices.SortStableFunc(subs, byPriority)

	ordered := slices.Concat(video, audio, subs, other)

	desired := make([]int, len(ordered))
	for i, track := range ordered {
		desired[i] = track.ID
	}

	current := make([]int, len(tracks))
	for i, track := range tracks {
		current[i] = track.ID
	}

	if slices.Equal(desired, current) {
		return nil
	}

	return desired
}

func computeCompressionStrips(tracks []matroska.EbmlTrack) []int {
	if !config.IsCheckEnabled("matroska_zlib_compression") {
		return nil
	}

	var ids []int

	for _, track := range tracks {
		for algo := range strings.SplitSeq(track.Properties.ContentEncodingAlgorithms, ",") {
			if algo == "0" { // 0 = zlib
				ids = append(ids, track.ID)

				break
			}
		}
	}

	return ids
}

// removalCollector accumulates removal candidates, keeping the first reason
// recorded for a track so the more specific signal (e.g. exact duplicate) wins.
type removalCollector struct {
	seen       map[int]bool
	candidates []RemovalCandidate
}

func (c *removalCollector) add(track matroska.EbmlTrack, kind RemovalKind, reason string) {
	if c.seen[track.ID] {
		return
	}

	c.seen[track.ID] = true
	c.candidates = append(c.candidates, RemovalCandidate{TrackID: track.ID, Kind: kind, Reason: reason, Track: track})
}

func computeRemovalCandidates(tracks []matroska.EbmlTrack, originalLang string) []RemovalCandidate {
	collector := &removalCollector{seen: make(map[int]bool)}

	collectDuplicateTracks(collector, tracks)
	collectUnwantedLanguageAudio(collector, tracks, originalLang)
	collectEmptyAudioTracks(collector, tracks)

	return collector.candidates
}

// collectEmptyAudioTracks proposes removal of audio tracks reporting zero
// channels, satisfying mediainfo_empty_tracks for the part derivable from the
// EBML track properties alone (mkvmerge always reports audio_channels for a
// genuine audio track).
func collectEmptyAudioTracks(collector *removalCollector, tracks []matroska.EbmlTrack) {
	if !config.IsCheckEnabled("mediainfo_empty_tracks") {
		return
	}

	for _, track := range tracks {
		if track.Type == "audio" && track.Properties.AudioChannels <= 0 {
			collector.add(track, RemovalEmptyTrack, "audio track has zero channels")
		}
	}
}

func collectDuplicateTracks(collector *removalCollector, tracks []matroska.EbmlTrack) {
	if !config.IsCheckEnabled("matroska_duplicate_tracks") {
		return
	}

	seen := make(map[string]bool)

	for _, track := range tracks {
		if !isRelevantTrack(track) {
			continue
		}

		key := trackDuplicateKey(track)
		if seen[key] {
			collector.add(track, RemovalDuplicateTrack, "duplicate of an earlier track (same language, flags and name)")

			continue
		}

		seen[key] = true
	}
}

func collectUnwantedLanguageAudio(collector *removalCollector, tracks []matroska.EbmlTrack, originalLang string) {
	// Without the original language we cannot tell which non-preferred track is
	// the legitimate original, so we skip pruning entirely to stay safe.
	if originalLang == "" || !config.IsCheckEnabled("mdb_unwanted_audio_lang") {
		return
	}

	wanted := map[language.Tag]bool{
		language.Make(config.GetPreferredLanguage()): true,
		language.Make(originalLang):                  true,
		language.Und:                                 true,
		language.Make("mul"):                         true,
		language.Make("zxx"):                         true, // no linguistic content (e.g. music-only); never unwanted
	}

	for _, track := range tracks {
		if track.Type != "audio" {
			continue
		}

		if !wanted[language.Make(track.Properties.Language)] {
			collector.add(track, RemovalUnwantedAudioLang, "unwanted audio language '"+track.Properties.Language+"'")
		}
	}
}
