package checks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"codeberg.org/upPollo/parsec/internal/cache"
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

	attachmentFonts := getFontMapping(filePath, ebml.Attachments)

	return runTrackChecks(filePath, ebml, attachmentFonts, meta)
}

func checkMatroskaFormat(err error) []CheckResult {
	return []CheckResult{{
		Identifier: "matroska_ebml_error",
		Passed:     false,
		Severity:   "error",
		Warning:    fmt.Sprintf("%v", err),
	}}
}

func runTrackChecks(filePath string, ebml *matroska.EbmlMetadata, attachmentFonts []matroska.AttachmentFontInfo, meta *metadata.Metadata) []CheckResult {
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
	allUsedFonts := make(map[fontStyle]bool)

	audioCounts, subCounts := getTrackCounts(tracks)
	langHasOriginalFlag := getOriginalLanguageMap(tracks)
	videoWidth, videoHeight := getVideoDimensions(tracks)

	extractedTracks := batchExtractTracksIfNeeded(filePath, tracks)

	for i := range tracks {
		runSingleIterationChecks(filePath, &tracks[i], agg,
			&lastAudioTrack, &lastSubTrack, &lastAudioPriority, &lastSubPriority,
			reportedOrderTracks, seenTracks, reportedDuplicates, seenAudioLangs, seenSubLangs,
			audioCounts, subCounts, langHasOriginalFlag, videoWidth, videoHeight, allUsedFonts, attachmentFonts,
			extractedTracks)
	}

	runGlobalMatroskaChecks(ebml, meta, agg, allUsedFonts, attachmentFonts)
	runChaptersChecks(filePath, ebml, agg)

	return agg.ToSlice()
}

func batchExtractTracksIfNeeded(filePath string, tracks []matroska.EbmlTrack) map[int][]byte {
	needsExtraction := config.IsCheckEnabled("matroska_subtitle_inline_fonts") || config.IsCheckEnabled("matroska_ass_events")
	if !needsExtraction {
		return nil
	}

	var extractTrackIDs []int

	for _, track := range tracks {
		if isRelevantTrack(track) && isASSSubtitles(track) {
			extractTrackIDs = append(extractTrackIDs, track.ID)
		}
	}

	if len(extractTrackIDs) == 0 {
		return nil
	}

	extractedTracks, extractErr := matroska.ExtractTracks(filePath, extractTrackIDs)
	if extractErr != nil {
		ui.PrintDebug(fmt.Sprintf("Failed to extract tracks: %v", extractErr))
	}

	return extractedTracks
}

func runGlobalMatroskaChecks(ebml *matroska.EbmlMetadata, meta *metadata.Metadata, agg *trackResultAggregator, allUsedFonts map[fontStyle]bool, attachmentFonts []matroska.AttachmentFontInfo) {
	if config.IsCheckEnabled("matroska_title_hygiene") {
		agg.Add(checkTitleHygiene(ebml, meta))
	}

	if config.IsCheckEnabled("matroska_app_hygiene") {
		agg.Add(checkAppHygiene(ebml))
	}

	if config.IsCheckEnabled("matroska_truehd_compatibility") {
		agg.Add(checkTrueHDCompatibility(ebml.Tracks))
	}

	if config.IsCheckEnabled("matroska_unused_fonts") {
		agg.Add(checkUnusedFonts(ebml.Attachments, attachmentFonts, allUsedFonts))
	}

	if config.IsCheckEnabled("matroska_font_filename_compliance") {
		agg.Add(checkFontFilenameCompliance(ebml.Attachments, attachmentFonts))
	}
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
	langHasOriginalFlag map[string]bool, videoWidth, videoHeight int, allUsedFonts map[fontStyle]bool, attachmentFonts []matroska.AttachmentFontInfo,
	extractedTracks map[int][]byte,
) {
	if config.IsCheckEnabled("matroska_track_delay") {
		agg.Add(checkTrackDelay(*track))
	}

	if track.Type == "video" && config.IsCheckEnabled("matroska_video_cropping") {
		agg.Add(checkVideoCropping(*track))
	}

	if !isRelevantTrack(*track) {
		return
	}

	agg.AddAll(runIndividualTrackChecks(filePath, *track, langHasOriginalFlag, videoWidth, videoHeight, allUsedFonts, attachmentFonts, extractedTracks))
	agg.AddAll(runStatefulTrackChecks(track, audioCounts, subCounts, seenTracks, reportedDuplicates, seenAudioLangs, seenSubLangs))

	if config.IsCheckEnabled("matroska_track_order") {
		runTrackOrderCheck(track, lastAudioTrack, lastSubTrack, lastAudioPriority, lastSubPriority, reportedOrderTracks, agg)
	}
}

func getFontMapping(filePath string, attachments []matroska.EbmlAttachment) []matroska.AttachmentFontInfo {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	cacheKey := fmt.Sprintf("font_mapping:%s:%d:%d", absPath, info.Size(), info.ModTime().UnixNano())

	if cachedData, err := cache.GetPersistent(cacheKey); err == nil {
		var fontFonts []matroska.AttachmentFontInfo
		if err := json.Unmarshal(cachedData, &fontFonts); err == nil {
			return fontFonts
		}
	}

	var fontIDs []int

	idToAtt := make(map[int]matroska.EbmlAttachment)

	for _, att := range attachments {
		if isFontAttachment(att) {
			fontIDs = append(fontIDs, att.ID)
			idToAtt[att.ID] = att
		}
	}

	if len(fontIDs) == 0 {
		return nil
	}

	fontFonts := extractAndParseFonts(filePath, fontIDs, idToAtt)

	if serialized, err := json.Marshal(fontFonts); err == nil {
		_ = cache.SetPersistent(cacheKey, serialized)
	}

	return fontFonts
}

func extractAndParseFonts(filePath string, fontIDs []int, idToAtt map[int]matroska.EbmlAttachment) []matroska.AttachmentFontInfo {
	var fontFonts []matroska.AttachmentFontInfo

	extracted, err := matroska.ExtractAttachments(filePath, fontIDs)
	if err != nil {
		ui.PrintDebug(fmt.Sprintf("Failed to extract attachments: %v", err))

		return fontFonts
	}

	for id, data := range extracted {
		fonts, err := matroska.GetAttachmentFonts(id, idToAtt[id].FileName, data)
		if err != nil {
			ui.PrintDebug(fmt.Sprintf("Failed to parse font %s: %v", idToAtt[id].FileName, err))

			continue
		}

		fontFonts = append(fontFonts, fonts...)
	}

	return fontFonts
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]bool)

	var out []string

	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}

	return out
}

func runIndividualTrackChecks(
	filePath string, track matroska.EbmlTrack, langHasOriginalFlag map[string]bool,
	videoWidth, videoHeight int, allUsedFonts map[fontStyle]bool, attachmentFonts []matroska.AttachmentFontInfo,
	extractedTracks map[int][]byte,
) []*CheckResult {
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
		results = append(results, checkSubtitleFonts(track, attachmentFonts, allUsedFonts))
	}

	// ASS specific checks
	if isASSSubtitles(track) {
		results = append(results, runASSSpecificChecks(filePath, track, videoWidth, videoHeight, allUsedFonts, attachmentFonts, extractedTracks)...)
	}

	if track.Type == "subtitles" && config.IsCheckEnabled("matroska_zlib_compression") {
		results = append(results, checkZlibCompression(track))
	}

	return results
}

func runASSSpecificChecks(
	filePath string, track matroska.EbmlTrack, videoWidth, videoHeight int,
	allUsedFonts map[fontStyle]bool, attachmentFonts []matroska.AttachmentFontInfo,
	extractedTracks map[int][]byte,
) []*CheckResult {
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

	content, err := getTrackContent(filePath, track.ID, extractedTracks)
	if err != nil {
		return results
	}

	if config.IsCheckEnabled("matroska_subtitle_inline_fonts") {
		start := time.Now()
		res := checkSubtitleInlineFontsWithContent(track, attachmentFonts, content, allUsedFonts)
		ui.PrintDebug(fmt.Sprintf("checkSubtitleInlineFontsWithContent for track %d took %v", track.ID, time.Since(start)))

		results = append(results, res)
	}

	if config.IsCheckEnabled("matroska_ass_events") {
		start := time.Now()
		res := checkASSEvents(track, content)
		ui.PrintDebug(fmt.Sprintf("checkASSEvents for track %d took %v", track.ID, time.Since(start)))

		results = append(results, res)
	}

	return results
}

func getTrackContent(filePath string, trackID int, extractedTracks map[int][]byte) ([]byte, error) {
	if extractedTracks != nil {
		if content, exists := extractedTracks[trackID]; exists {
			return content, nil
		}
	}

	return matroska.ExtractTrack(filePath, trackID)
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
	priority := getTrackPriority(*track)
	switch track.Type {
	case "audio":
		agg.Add(checkTrackOrder(track, *lastAudioTrack, priority, lastAudioPriority, "some Audio tracks are out of order", reportedOrderTracks))
		*lastAudioTrack = track
	case "subtitles":
		agg.Add(checkTrackOrder(track, *lastSubTrack, priority, lastSubPriority, "some Subtitle tracks are out of order", reportedOrderTracks))
		*lastSubTrack = track
	}
}

func getOriginalLanguageMap(tracks []matroska.EbmlTrack) map[string]bool {
	langHasOriginalFlag := make(map[string]bool)

	for _, track := range tracks {
		if isRelevantTrack(track) && track.Properties.OriginalLanguage {
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

func getTrackCounts(tracks []matroska.EbmlTrack) (audio, sub map[string]int) {
	audio = make(map[string]int)
	sub = make(map[string]int)

	for _, track := range tracks {
		if !isRelevantTrack(track) {
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

func isRelevantTrack(track matroska.EbmlTrack) bool {
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
