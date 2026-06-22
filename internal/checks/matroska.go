package checks

import (
	"fmt"
	"strconv"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/ui"
)

type trackResultAggregator struct {
	aggregated map[string]*CheckResult
}

func newTrackResultAggregator() *trackResultAggregator {
	return &trackResultAggregator{
		aggregated: make(map[string]*CheckResult),
	}
}

func (a *trackResultAggregator) Add(res *CheckResult) {
	if res == nil {
		return
	}

	target, ok := a.aggregated[res.Identifier]
	if !ok {
		target = &CheckResult{
			Identifier: res.Identifier,
			Warning:    res.Warning,
			Passed:     true,
			Actual:     res.Actual,
			Expected:   res.Expected,
		}
		a.aggregated[res.Identifier] = target
	} else {
		a.mergeActualExpected(target, res)
	}

	target.Passed = false
	target.Severity = res.Severity
	target.Tracks = append(target.Tracks, res.Tracks...)
}

func (a *trackResultAggregator) mergeActualExpected(target, res *CheckResult) {
	if res.Actual != "" {
		if target.Actual == "" {
			target.Actual = res.Actual
		} else {
			target.Actual += "; " + res.Actual
		}
	}

	if res.Expected != "" {
		if target.Expected == "" {
			target.Expected = res.Expected
		} else {
			target.Expected += "; " + res.Expected
		}
	}
}

func (a *trackResultAggregator) AddAll(results []*CheckResult) {
	for _, res := range results {
		a.Add(res)
	}
}

func (a *trackResultAggregator) ToSlice() []CheckResult {
	var results []CheckResult

	// Convert aggregated map to slice in stable order
	ids := []string{
		"matroska_language_tag",
		"matroska_multi_lang",
		"matroska_name_quality",
		"matroska_name_codecs",
		"matroska_name_redundant_lang",
		"matroska_original_language",
		"matroska_duplicate_tracks",
		"matroska_name_keywords",
		"matroska_default_flags",
		"matroska_subtitle_format",
		"matroska_subtitle_fonts",
		"matroska_subtitle_inline_fonts",
		"matroska_unused_fonts",
		"matroska_ass_script_info",
		"matroska_ass_styles",
		"matroska_ass_events",
		"matroska_zlib_compression",
		"matroska_track_order",
		"matroska_font_filename_compliance",
		"matroska_title_hygiene",
		"matroska_video_cropping",
		"matroska_track_delay",
		"matroska_truehd_compatibility",
		"matroska_chapters_start_non_zero",
		"matroska_chapters_non_monotonic",
		"matroska_chapters_duplicate",
		"matroska_chapters_too_close",
		"matroska_chapters_exceed_duration",
		"matroska_chapters_name_hygiene",
		"matroska_chapters_language_hygiene",
		"matroska_chapters_keyframe_alignment",
		"matroska_app_hygiene",
	}

	for _, id := range ids {
		if res, ok := a.aggregated[id]; ok {
			results = append(results, *res)
		}
	}

	return results
}

func newFailedTrackResult(id, desc, severity string, track *matroska.EbmlTrack, warning string) *CheckResult {
	return &CheckResult{
		Identifier: id,
		Warning:    desc,
		Passed:     false,
		Severity:   severity,
		Tracks:     []TrackCheckResult{ebmlTrackToResult(track, false, warning)},
	}
}

// RunMatroskaChecks performs checks on the Matroska container and its tracks.
func RunMatroskaChecks(filePath string, meta *metadata.Metadata) []CheckResult {
	ebml, err := matroska.GetEbmlMetadata(filePath)
	if err != nil {
		return checkMatroskaFormat(err)
	}

	if ebml.HasChapters() {
		xmlChapters, err := matroska.ExtractChapters(filePath)
		if err == nil {
			if len(ebml.Chapters) == 0 {
				ebml.Chapters = append(ebml.Chapters, matroska.EbmlChapters{
					NumEntries: len(xmlChapters),
				})
			}

			ebml.Chapters[0].Editions = []matroska.EbmlEdition{
				{
					Chapters: xmlChapters,
				},
			}
		} else {
			ui.PrintDebug(fmt.Sprintf("Failed to extract chapters via mkvextract: %v", err))
		}
	}

	fontMap, attachmentNames := GetFontMapping(filePath, ebml.Attachments)

	return runTrackChecks(filePath, ebml, fontMap, attachmentNames, meta)
}

func checkMatroskaFormat(err error) []CheckResult {
	return []CheckResult{{
		Identifier: "matroska_ebml_error",
		Passed:     false,
		Severity:   "error",
		Warning:    fmt.Sprintf("%v", err),
	}}
}

func runTrackChecks(filePath string, ebml *matroska.EbmlMetadata, fontMap map[string]string, attachmentNames map[int][]string, meta *metadata.Metadata) []CheckResult {
	tracks := ebml.Tracks

	var (
		lastAudioPriority, lastSubPriority int64
		lastAudioTrack, lastSubTrack       *matroska.EbmlTrack
	)

	seenTracks := make(map[string]*matroska.EbmlTrack)
	reportedDuplicates := make(map[string]bool)
	reportedOrderTracks := make(map[int]bool)
	seenAudioLangs := make(map[string]bool)
	seenSubLangs := make(map[string]bool)

	agg := newTrackResultAggregator()
	allUsedFonts := make(map[string]bool)

	audioCounts, subCounts := GetTrackCounts(tracks)
	langHasOriginalFlag := GetOriginalLanguageMap(tracks)
	videoWidth, videoHeight := getVideoDimensions(tracks)

	if config.IsCheckEnabled("matroska_title_hygiene") {
		agg.Add(checkTitleHygiene(ebml, meta))
	}

	if config.IsCheckEnabled("matroska_app_hygiene") {
		agg.Add(checkAppHygiene(ebml))
	}

	for i := range tracks {
		runSingleIterationChecks(filePath, &tracks[i], agg,
			&lastAudioTrack, &lastSubTrack, &lastAudioPriority, &lastSubPriority,
			reportedOrderTracks, seenTracks, reportedDuplicates, seenAudioLangs, seenSubLangs,
			audioCounts, subCounts, langHasOriginalFlag, videoWidth, videoHeight, allUsedFonts, fontMap)
	}

	if config.IsCheckEnabled("matroska_truehd_compatibility") {
		agg.Add(checkTrueHDCompatibility(tracks))
	}

	if config.IsCheckEnabled("matroska_unused_fonts") {
		agg.Add(checkUnusedFonts(ebml.Attachments, attachmentNames, allUsedFonts))
	}

	if config.IsCheckEnabled("matroska_font_filename_compliance") {
		agg.Add(checkFontFilenameCompliance(ebml.Attachments, attachmentNames))
	}

	runChaptersChecks(filePath, ebml, agg)

	return agg.ToSlice()
}

func runChaptersChecks(filePath string, ebml *matroska.EbmlMetadata, agg *trackResultAggregator) {
	if len(ebml.Chapters) == 0 {
		return
	}

	if config.IsCheckEnabled("matroska_chapters_start_non_zero") {
		agg.Add(checkChaptersStartNonZero(ebml))
	}

	if config.IsCheckEnabled("matroska_chapters_non_monotonic") {
		agg.Add(checkChaptersNonMonotonic(ebml))
	}

	if config.IsCheckEnabled("matroska_chapters_duplicate") {
		agg.Add(checkChaptersDuplicate(ebml))
	}

	if config.IsCheckEnabled("matroska_chapters_too_close") {
		agg.Add(checkChaptersTooClose(ebml))
	}

	if config.IsCheckEnabled("matroska_chapters_exceed_duration") {
		agg.Add(checkChaptersExceedDuration(ebml))
	}

	if config.IsCheckEnabled("matroska_chapters_name_hygiene") {
		agg.Add(checkChaptersNameHygiene(ebml))
	}

	if config.IsCheckEnabled("matroska_chapters_language_hygiene") {
		agg.Add(checkChaptersLanguageHygiene(ebml))
	}

	if config.IsCheckEnabled("matroska_chapters_keyframe_alignment") {
		agg.Add(checkChaptersKeyframeAlignment(filePath, ebml))
	}
}

func runSingleIterationChecks(
	filePath string, track *matroska.EbmlTrack, agg *trackResultAggregator,
	lastAudioTrack, lastSubTrack **matroska.EbmlTrack, lastAudioPriority, lastSubPriority *int64,
	reportedOrderTracks map[int]bool, seenTracks map[string]*matroska.EbmlTrack, reportedDuplicates map[string]bool,
	seenAudioLangs, seenSubLangs map[string]bool, audioCounts, subCounts map[string]int,
	langHasOriginalFlag map[string]bool, videoWidth, videoHeight int, allUsedFonts map[string]bool, fontMap map[string]string,
) {
	if config.IsCheckEnabled("matroska_track_delay") {
		agg.Add(checkTrackDelay(*track))
	}

	if track.Type == "video" && config.IsCheckEnabled("matroska_video_cropping") {
		agg.Add(checkVideoCropping(*track))
	}

	if !IsRelevantTrack(*track) {
		return
	}

	agg.AddAll(runIndividualTrackChecks(filePath, *track, langHasOriginalFlag, videoWidth, videoHeight, allUsedFonts, fontMap))
	agg.AddAll(runStatefulTrackChecks(track, audioCounts, subCounts, seenTracks, reportedDuplicates, seenAudioLangs, seenSubLangs))

	if config.IsCheckEnabled("matroska_track_order") {
		runTrackOrderCheck(track, lastAudioTrack, lastSubTrack, lastAudioPriority, lastSubPriority, reportedOrderTracks, agg)
	}
}

// ComputeUsedFonts gathers the set of font names referenced by ASS/SSA
// subtitle tracks, mirroring the allUsedFonts side effects that
// checkSubtitleFonts and checkSubtitleInlineFontsWithContent produce during a
// normal check run so fix policy stays consistent with what check reports.
func ComputeUsedFonts(filePath string, tracks []matroska.EbmlTrack, fontMap map[string]string) map[string]bool {
	allUsedFonts := make(map[string]bool)

	for i := range tracks {
		track := tracks[i]
		if !isASSSubtitles(track) {
			continue
		}

		if config.IsCheckEnabled("matroska_subtitle_fonts") {
			checkSubtitleFonts(track, fontMap, allUsedFonts)
		}

		if config.IsCheckEnabled("matroska_subtitle_inline_fonts") {
			if content, err := matroska.ExtractTrack(filePath, track.ID); err == nil {
				checkSubtitleInlineFontsWithContent(track, fontMap, content, allUsedFonts)
			}
		}
	}

	return allUsedFonts
}

// GetFontMapping extracts the font attachments and returns a normalized
// font-name-to-attachment lookup plus the internal font names found per
// attachment ID. Shared between check and the fix policy in internal/fix.
func GetFontMapping(filePath string, attachments []matroska.EbmlAttachment) (map[string]string, map[int][]string) {
	fontMap := make(map[string]string)
	attachmentNames := make(map[int][]string)

	var fontIDs []int

	idToAtt := make(map[int]matroska.EbmlAttachment)

	for _, att := range attachments {
		if IsFontAttachment(att) {
			fontIDs = append(fontIDs, att.ID)
			idToAtt[att.ID] = att
		}
	}

	if len(fontIDs) == 0 {
		return fontMap, attachmentNames
	}

	extracted, err := matroska.ExtractAttachments(filePath, fontIDs)
	if err != nil {
		ui.PrintDebug(fmt.Sprintf("Failed to extract attachments: %v", err))

		return fontMap, attachmentNames
	}

	for id, data := range extracted {
		names, err := matroska.GetFontNames(data)
		if err != nil {
			ui.PrintDebug(fmt.Sprintf("Failed to parse font %s: %v", idToAtt[id].FileName, err))

			continue
		}

		attachmentNames[id] = names
		for _, name := range names {
			fontMap[normalizeFontName(name)] = name
		}
	}

	return fontMap, attachmentNames
}

func runIndividualTrackChecks(filePath string, track matroska.EbmlTrack, langHasOriginalFlag map[string]bool, videoWidth, videoHeight int, allUsedFonts map[string]bool, fontMap map[string]string) []*CheckResult {
	var results []*CheckResult

	// Basic checks
	if config.IsCheckEnabled("matroska_language_tag") || config.IsCheckEnabled("matroska_multi_lang") {
		results = append(results, validateTrackBasics(track))
	}

	// Name-based checks
	results = append(results, runNameChecks(track)...)

	// Consistency and format checks
	if config.IsCheckEnabled("matroska_original_language") {
		results = append(results, checkOriginalLanguageConsistency(track, langHasOriginalFlag))
	}

	if config.IsCheckEnabled("matroska_subtitle_format") {
		results = append(results, checkSubtitleFormat(track))
	}

	if config.IsCheckEnabled("matroska_subtitle_fonts") {
		results = append(results, checkSubtitleFonts(track, fontMap, allUsedFonts))
	}

	// ASS specific checks
	if isASSSubtitles(track) {
		results = append(results, runASSSpecificChecks(filePath, track, videoWidth, videoHeight, allUsedFonts, fontMap)...)
	}

	if track.Type == "subtitles" && config.IsCheckEnabled("matroska_zlib_compression") {
		results = append(results, checkZlibCompression(track))
	}

	return results
}

func runASSSpecificChecks(filePath string, track matroska.EbmlTrack, videoWidth, videoHeight int, allUsedFonts map[string]bool, fontMap map[string]string) []*CheckResult {
	var results []*CheckResult

	if config.IsCheckEnabled("matroska_ass_script_info") {
		results = append(results, checkASSScriptInfo(track, videoWidth, videoHeight))
	}

	if config.IsCheckEnabled("matroska_ass_styles") {
		results = append(results, checkASSStyles(track))
	}

	needsExtraction := config.IsCheckEnabled("matroska_subtitle_inline_fonts") || config.IsCheckEnabled("matroska_ass_events")
	if !needsExtraction {
		return results
	}

	content, err := matroska.ExtractTrack(filePath, track.ID)
	if err != nil {
		return results
	}

	if config.IsCheckEnabled("matroska_subtitle_inline_fonts") {
		results = append(results, checkSubtitleInlineFontsWithContent(track, fontMap, content, allUsedFonts))
	}

	if config.IsCheckEnabled("matroska_ass_events") {
		results = append(results, checkASSEvents(track, content))
	}

	return results
}

func runNameChecks(track matroska.EbmlTrack) []*CheckResult {
	var results []*CheckResult

	if config.IsCheckEnabled("matroska_name_quality") {
		results = append(results, checkTrackNameQuality(track))
	}

	if config.IsCheckEnabled("matroska_name_codecs") {
		results = append(results, checkTrackNameCodecs(track))
	}

	if config.IsCheckEnabled("matroska_name_redundant_lang") {
		results = append(results, checkTrackNameRedundantLang(track))
	}

	if config.IsCheckEnabled("matroska_name_keywords") {
		results = append(results, checkNameKeywords(track))
	}

	return results
}

func runStatefulTrackChecks(track *matroska.EbmlTrack, audioCounts, subCounts map[string]int, seenTracks map[string]*matroska.EbmlTrack, reportedDuplicates, seenAudioLangs, seenSubLangs map[string]bool) []*CheckResult {
	var results []*CheckResult

	if config.IsCheckEnabled("matroska_duplicate_tracks") {
		results = append(results, checkDuplicateTracks(track, seenTracks, reportedDuplicates))
	}

	if config.IsCheckEnabled("matroska_default_flags") {
		results = append(results, checkDefaultFlags(*track, audioCounts, subCounts, seenAudioLangs, seenSubLangs))
	}

	return results
}

func runTrackOrderCheck(track *matroska.EbmlTrack, lastAudioTrack, lastSubTrack **matroska.EbmlTrack, lastAudioPriority, lastSubPriority *int64, reportedOrderTracks map[int]bool, agg *trackResultAggregator) {
	priority := GetTrackPriority(*track)
	switch track.Type {
	case "audio":
		agg.Add(checkTrackOrder(track, *lastAudioTrack, priority, lastAudioPriority, "some Audio tracks are out of order", reportedOrderTracks))
		*lastAudioTrack = track
	case "subtitles":
		agg.Add(checkTrackOrder(track, *lastSubTrack, priority, lastSubPriority, "some Subtitle tracks are out of order", reportedOrderTracks))
		*lastSubTrack = track
	}
}

// GetOriginalLanguageMap returns the set of languages that already have at
// least one track flagged as original. Shared with the fix policy in
// internal/fix, which uses it to decide where flag-original is missing.
func GetOriginalLanguageMap(tracks []matroska.EbmlTrack) map[string]bool {
	langHasOriginalFlag := make(map[string]bool)

	for _, track := range tracks {
		if IsRelevantTrack(track) && track.Properties.OriginalLanguage {
			langHasOriginalFlag[track.Properties.Language] = true
		}
	}

	return langHasOriginalFlag
}

func ebmlTrackToResult(t *matroska.EbmlTrack, passed bool, warning string) TrackCheckResult {
	return TrackCheckResult{
		ID:        strconv.Itoa(t.Properties.Number),
		Type:      t.Type,
		TypeOrder: t.TypeOrder,
		Codec:     t.Codec,
		Name:      t.Properties.Name,
		Language:  metadata.LanguageName(t.Properties.Language),
		Flags:     ebmlGetFlagsSlice(t),
		Passed:    passed,
		Warning:   warning,
	}
}

func ebmlGetFlagsSlice(track *matroska.EbmlTrack) []string {
	var flags []string
	if track.Properties.Default {
		flags = append(flags, "Default")
	}

	if track.Properties.Forced {
		flags = append(flags, "Forced")
	}

	if track.Properties.HearingImpaired {
		flags = append(flags, "Hearing Impaired")
	}

	if track.Properties.VisualImpaired {
		flags = append(flags, "Visual Impaired")
	}

	if track.Properties.Commentary {
		flags = append(flags, "Commentary")
	}

	if track.Properties.OriginalLanguage {
		flags = append(flags, "Original")
	}

	return flags
}

// GetTrackCounts counts audio and subtitle tracks per language. Shared with
// the fix policy in internal/fix for its default-flag computation.
func GetTrackCounts(tracks []matroska.EbmlTrack) (audio, sub map[string]int) {
	audio = make(map[string]int)
	sub = make(map[string]int)

	for _, track := range tracks {
		if !IsRelevantTrack(track) {
			continue
		}

		switch track.Type {
		case "audio":
			audio[track.Properties.Language]++
		case "subtitles":
			sub[track.Properties.Language]++
		}
	}

	return audio, sub
}

// IsRelevantTrack reports whether a track is of type "audio" or "subtitles".
// Shared with the fix policy in internal/fix.
func IsRelevantTrack(track matroska.EbmlTrack) bool {
	return track.Type == "audio" || track.Type == "subtitles"
}

func getVideoDimensions(tracks []matroska.EbmlTrack) (int, int) {
	for _, track := range tracks {
		if track.Type == "video" {
			props := track.Properties

			// Try display_dimensions string
			dw, dh := matroska.ParseDimensions(props.DisplayDimensions)
			if dw > 0 && dh > 0 {
				return dw, dh
			}

			// Fallback to pixel_dimensions string
			pw, ph := matroska.ParseDimensions(props.PixelDimensions)
			if pw > 0 && ph > 0 {
				return pw, ph
			}
		}
	}

	return 0, 0
}
