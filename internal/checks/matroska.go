package checks

import (
	"fmt"
	"strconv"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
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
		}
		a.aggregated[res.Identifier] = target
	}

	target.Passed = false
	target.Severity = res.Severity
	target.Tracks = append(target.Tracks, res.Tracks...)
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
	}

	for _, id := range ids {
		if res, ok := a.aggregated[id]; ok && !res.Passed {
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
func RunMatroskaChecks(filePath string) []CheckResult {
	ebml, err := matroska.GetEbmlMetadata(filePath)
	if err != nil {
		return checkMatroskaFormat(err)
	}

	return runTrackChecks(filePath, ebml)
}

func checkMatroskaFormat(err error) []CheckResult {
	return []CheckResult{{
		Identifier: "matroska_ebml_error",
		Passed:     false,
		Severity:   "error",
		Warning:    fmt.Sprintf("%v", err),
	}}
}

func runTrackChecks(filePath string, ebml *matroska.EbmlMetadata) []CheckResult {
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

	audioCounts, subCounts := getTrackCounts(tracks)
	langHasOriginalFlag := getOriginalLanguageMap(tracks)
	videoWidth, videoHeight := getVideoDimensions(tracks)

	for i := range tracks {
		track := &tracks[i]
		if !isRelevantTrack(*track) {
			continue
		}

		agg.AddAll(runIndividualTrackChecks(filePath, *track, langHasOriginalFlag, ebml.Attachments, videoWidth, videoHeight, allUsedFonts))
		agg.AddAll(runStatefulTrackChecks(track, audioCounts, subCounts, seenTracks, reportedDuplicates, seenAudioLangs, seenSubLangs))

		if config.IsCheckEnabled("matroska_track_order") {
			runTrackOrderCheck(track, &lastAudioTrack, &lastSubTrack, &lastAudioPriority, &lastSubPriority, reportedOrderTracks, agg)
		}
	}

	if config.IsCheckEnabled("matroska_unused_fonts") {
		agg.Add(checkUnusedFonts(ebml.Attachments, allUsedFonts))
	}

	return agg.ToSlice()
}

func runIndividualTrackChecks(filePath string, track matroska.EbmlTrack, langHasOriginalFlag map[string]bool, attachments []matroska.EbmlAttachment, videoWidth, videoHeight int, allUsedFonts map[string]bool) []*CheckResult {
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
		results = append(results, checkSubtitleFonts(track, attachments, allUsedFonts))
	}

	// ASS specific checks
	if isASSSubtitles(track) {
		results = append(results, runASSSpecificChecks(filePath, track, attachments, videoWidth, videoHeight, allUsedFonts)...)
	}

	if config.IsCheckEnabled("matroska_zlib_compression") {
		results = append(results, checkZlibCompression(track))
	}

	return results
}

func runASSSpecificChecks(filePath string, track matroska.EbmlTrack, attachments []matroska.EbmlAttachment, videoWidth, videoHeight int, allUsedFonts map[string]bool) []*CheckResult {
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
		results = append(results, checkSubtitleInlineFontsWithContent(track, attachments, content, allUsedFonts))
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
			if props.DisplayWidth > 0 && props.DisplayHeight > 0 {
				return props.DisplayWidth, props.DisplayHeight
			}

			return props.PixelWidth, props.PixelHeight
		}
	}

	return 0, 0
}
