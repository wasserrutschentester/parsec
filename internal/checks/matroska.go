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
			Table:      res.Table,
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
		config.CheckMatroskaLanguageTag,
		config.CheckMatroskaMultiLang,
		config.CheckMatroskaNameQuality,
		config.CheckMatroskaNameCodecs,
		config.CheckMatroskaNameRedundantLang,
		config.CheckMatroskaOriginalLanguage,
		config.CheckMatroskaDuplicateTracks,
		config.CheckMatroskaNameKeywords,
		config.CheckMatroskaDefaultFlags,
		config.CheckMatroskaSubtitleFormat,
		config.CheckMatroskaSubtitleFonts,
		config.CheckMatroskaSubtitleInlineFonts,
		config.CheckMatroskaSrtValidation,
		config.CheckMatroskaUnusedFonts,
		config.CheckMatroskaAssScriptInfo,
		config.CheckMatroskaAssStyles,
		config.CheckMatroskaAssEvents,
		config.CheckMatroskaZlibCompression,
		config.CheckMatroskaTrackOrder,
		config.CheckMatroskaFontFilenameCompliance,
		config.CheckMatroskaTitleHygiene,
		config.CheckMatroskaVideoCropping,
		config.CheckMatroskaTrackDelay,
		config.CheckMatroskaTruehdCompatibility,
		config.CheckMatroskaCommentaryChannels,
		config.CheckMatroskaCommentaryBitrate,
		config.CheckMatroskaCommentaryPrefix,
		config.CheckMatroskaCommentaryPairing,
		config.CheckMatroskaChaptersStartNonZero,
		config.CheckMatroskaChaptersNonMonotonic,
		config.CheckMatroskaChaptersDuplicate,
		config.CheckMatroskaChaptersTooClose,
		config.CheckMatroskaChaptersExceedDuration,
		config.CheckMatroskaChaptersNameHygiene,
		config.CheckMatroskaChaptersLanguageHygiene,
		config.CheckMatroskaChaptersKeyframeAlignment,
		config.CheckMatroskaAppHygiene,
		config.CheckMatroskaCreationTimePrivacy,
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
func RunMatroskaChecks(filePath string, ebml *matroska.EbmlMetadata, ebmlErr error, meta *metadata.Metadata) []CheckResult {
	if ebmlErr != nil {
		return checkMatroskaFormat(ebmlErr)
	}

	var xmlChapters *matroska.Chapters

	if ebml.HasChapters() {
		var err error

		xmlChapters, err = matroska.ExtractChapters(filePath)
		if err != nil {
			ui.PrintDebug(fmt.Sprintf("Failed to extract chapters via mkvextract: %v", err))
		}
	}

	attachmentFonts := GetAttachmentFonts(filePath, ebml.Attachments)

	return runTrackChecks(filePath, ebml, xmlChapters, attachmentFonts, meta)
}

func checkMatroskaFormat(err error) []CheckResult {
	return []CheckResult{{
		Identifier: "matroska_ebml_error",
		Passed:     false,
		Severity:   "error",
		Warning:    fmt.Sprintf("%v", err),
	}}
}

func runTrackChecks(filePath string, ebml *matroska.EbmlMetadata, xmlChapters *matroska.Chapters, attachmentFonts []matroska.AttachmentFontInfo, meta *metadata.Metadata) []CheckResult {
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
	audioCounts, subCounts := GetTrackCounts(tracks)
	langHasOriginalFlag := GetOriginalLanguageMap(tracks)
	videoWidth, videoHeight := getVideoDimensions(tracks)

	extractedTracks := batchExtractTracksIfNeeded(filePath, tracks)

	var allUsedFonts map[FontStyle]bool
	if config.IsCheckEnabled(config.CheckMatroskaUnusedFonts) {
		allUsedFonts = ComputeAllUsedFonts(tracks, extractedTracks)
	}

	originalLang := ""
	if meta != nil {
		originalLang = meta.OriginalLanguage
	}

	for i := range tracks {
		runSingleIterationChecks(filePath, &tracks[i], agg,
			&lastAudioTrack, &lastSubTrack, &lastAudioPriority, &lastSubPriority,
			reportedOrderTracks, seenTracks, reportedDuplicates, seenAudioLangs, seenSubLangs,
			audioCounts, subCounts, langHasOriginalFlag, videoWidth, videoHeight, attachmentFonts,
			extractedTracks, originalLang)
	}

	runGlobalMatroskaChecks(filePath, ebml, meta, agg, allUsedFonts, attachmentFonts)
	runChaptersChecks(filePath, ebml, xmlChapters, agg)

	return agg.ToSlice()
}

func batchExtractTracksIfNeeded(filePath string, tracks []matroska.EbmlTrack) map[int][]byte {
	needsInlineFonts := config.IsCheckEnabled(config.CheckMatroskaSubtitleInlineFonts)
	needsASSEvents := config.IsCheckEnabled(config.CheckMatroskaAssEvents)
	needsSRTValidation := config.IsCheckEnabled(config.CheckMatroskaSrtValidation)

	includeASS := needsInlineFonts || needsASSEvents
	includeSRT := needsSRTValidation

	if !includeASS && !includeSRT {
		return nil
	}

	return ExtractSubtitleTracks(filePath, tracks, includeASS, includeSRT)
}

func runGlobalMatroskaChecks(filePath string, ebml *matroska.EbmlMetadata, meta *metadata.Metadata, agg *trackResultAggregator, allUsedFonts map[FontStyle]bool, attachmentFonts []matroska.AttachmentFontInfo) {
	if config.IsCheckEnabled(config.CheckMatroskaTitleHygiene) {
		agg.Add(checkTitleHygiene(ebml, meta))
	}

	if config.IsCheckEnabled(config.CheckMatroskaAppHygiene) {
		agg.Add(checkAppHygiene(ebml))
	}

	if config.IsCheckEnabled(config.CheckMatroskaCreationTimePrivacy) {
		agg.Add(checkCreationTimePrivacy(filePath, ebml))
	}

	if config.IsCheckEnabled(config.CheckMatroskaTruehdCompatibility) {
		agg.Add(checkTrueHDCompatibility(ebml.Tracks))
	}

	if config.IsCheckEnabled(config.CheckMatroskaUnusedFonts) {
		agg.Add(checkUnusedFonts(ebml.Attachments, attachmentFonts, allUsedFonts))
	}

	if config.IsCheckEnabled(config.CheckMatroskaFontFilenameCompliance) {
		agg.Add(checkFontFilenameCompliance(ebml.Attachments, attachmentFonts))
	}

	runCommentaryChecks(filePath, ebml.Tracks, agg)
}

func runCommentaryChecks(filePath string, tracks []matroska.EbmlTrack, agg *trackResultAggregator) {
	if config.IsCheckEnabled(config.CheckMatroskaCommentaryChannels) {
		agg.Add(checkCommentaryChannels(tracks))
	}

	if config.IsCheckEnabled(config.CheckMatroskaCommentaryBitrate) {
		agg.Add(checkCommentaryBitrate(filePath, tracks))
	}

	if config.IsCheckEnabled(config.CheckMatroskaCommentaryPrefix) {
		agg.Add(checkCommentaryPrefix(tracks))
	}

	if config.IsCheckEnabled(config.CheckMatroskaCommentaryPairing) {
		agg.Add(checkCommentaryPairing(tracks))
	}
}

func runChaptersChecks(filePath string, ebml *matroska.EbmlMetadata, xmlChapters *matroska.Chapters, agg *trackResultAggregator) {
	if xmlChapters.Empty() {
		return
	}

	if config.IsCheckEnabled(config.CheckMatroskaChaptersStartNonZero) {
		agg.Add(checkChaptersStartNonZero(xmlChapters))
	}

	if config.IsCheckEnabled(config.CheckMatroskaChaptersNonMonotonic) {
		agg.Add(checkChaptersNonMonotonic(xmlChapters))
	}

	if config.IsCheckEnabled(config.CheckMatroskaChaptersDuplicate) {
		agg.Add(checkChaptersDuplicate(xmlChapters))
	}

	if config.IsCheckEnabled(config.CheckMatroskaChaptersTooClose) {
		agg.Add(checkChaptersTooClose(xmlChapters))
	}

	if config.IsCheckEnabled(config.CheckMatroskaChaptersExceedDuration) {
		agg.Add(checkChaptersExceedDuration(ebml, xmlChapters))
	}

	if config.IsCheckEnabled(config.CheckMatroskaChaptersNameHygiene) {
		agg.Add(checkChaptersNameHygiene(xmlChapters))
	}

	if config.IsCheckEnabled(config.CheckMatroskaChaptersLanguageHygiene) {
		agg.Add(checkChaptersLanguageHygiene(xmlChapters))
	}

	if config.IsCheckEnabled(config.CheckMatroskaChaptersKeyframeAlignment) {
		agg.Add(checkChaptersKeyframeAlignment(filePath, ebml, xmlChapters))
	}
}

func runSingleIterationChecks(
	filePath string, track *matroska.EbmlTrack, agg *trackResultAggregator,
	lastAudioTrack, lastSubTrack **matroska.EbmlTrack, lastAudioPriority, lastSubPriority *int64,
	reportedOrderTracks map[int]bool, seenTracks map[string]*matroska.EbmlTrack, reportedDuplicates map[string]bool,
	seenAudioLangs, seenSubLangs map[string]bool, audioCounts, subCounts map[string]int,
	langHasOriginalFlag map[string]bool, videoWidth, videoHeight int, attachmentFonts []matroska.AttachmentFontInfo,
	extractedTracks map[int][]byte, originalLang string,
) {
	if config.IsCheckEnabled(config.CheckMatroskaTrackDelay) {
		agg.Add(checkTrackDelay(*track))
	}

	if track.Type == "video" && config.IsCheckEnabled(config.CheckMatroskaVideoCropping) {
		agg.Add(checkVideoCropping(*track))
	}

	if !IsRelevantTrack(*track) {
		return
	}

	agg.AddAll(runIndividualTrackChecks(filePath, *track, langHasOriginalFlag, videoWidth, videoHeight, attachmentFonts, extractedTracks))
	agg.AddAll(runStatefulTrackChecks(track, audioCounts, subCounts, seenTracks, reportedDuplicates, seenAudioLangs, seenSubLangs))

	if config.IsCheckEnabled(config.CheckMatroskaTrackOrder) {
		runTrackOrderCheck(track, lastAudioTrack, lastSubTrack, lastAudioPriority, lastSubPriority, reportedOrderTracks, agg, originalLang)
	}
}

// GetAttachmentFonts extracts font attachments and returns normalized name info.
func GetAttachmentFonts(filePath string, attachments []matroska.EbmlAttachment) []matroska.AttachmentFontInfo {
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
		if IsFontAttachment(att) {
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
	filePath string, track matroska.EbmlTrack,
	langHasOriginalFlag map[string]bool, videoWidth, videoHeight int,
	attachmentFonts []matroska.AttachmentFontInfo,
	extractedTracks map[int][]byte,
) []*CheckResult {
	var results []*CheckResult

	// Basic checks
	if config.IsCheckEnabled(config.CheckMatroskaLanguageTag) || config.IsCheckEnabled(config.CheckMatroskaMultiLang) {
		results = append(results, validateTrackBasics(track))
	}

	// Name-based checks
	results = append(results, runNameChecks(track)...)

	// Consistency and format checks
	if config.IsCheckEnabled(config.CheckMatroskaOriginalLanguage) {
		results = append(results, checkOriginalLanguageConsistency(track, langHasOriginalFlag))
	}

	results = append(results, runSubtitleSpecificChecks(filePath, track, videoWidth, videoHeight, attachmentFonts, extractedTracks)...)

	return results
}

func runSubtitleSpecificChecks(
	filePath string, track matroska.EbmlTrack, videoWidth, videoHeight int,
	attachmentFonts []matroska.AttachmentFontInfo,
	extractedTracks map[int][]byte,
) []*CheckResult {
	var results []*CheckResult

	if config.IsCheckEnabled(config.CheckMatroskaSubtitleFormat) {
		results = append(results, checkSubtitleFormat(track))
	}

	if config.IsCheckEnabled(config.CheckMatroskaSubtitleFonts) {
		results = append(results, checkSubtitleFonts(track, attachmentFonts))
	}

	// ASS specific checks
	if isASSSubtitles(track) {
		results = append(results, runASSSpecificChecks(filePath, track, videoWidth, videoHeight, attachmentFonts, extractedTracks)...)
	}

	// SRT specific checks
	if isSRTSubtitles(track) && config.IsCheckEnabled(config.CheckMatroskaSrtValidation) {
		content, err := getTrackContent(filePath, track.ID, extractedTracks)
		if err == nil {
			results = append(results, checkSRTValidation(track, content))
		}
	}

	if track.Type == "subtitles" && config.IsCheckEnabled(config.CheckMatroskaZlibCompression) {
		results = append(results, checkZlibCompression(track))
	}

	return results
}

func runASSSpecificChecks(
	filePath string, track matroska.EbmlTrack, videoWidth, videoHeight int,
	attachmentFonts []matroska.AttachmentFontInfo,
	extractedTracks map[int][]byte,
) []*CheckResult {
	var results []*CheckResult

	if config.IsCheckEnabled(config.CheckMatroskaAssScriptInfo) {
		results = append(results, checkASSScriptInfo(track, videoWidth, videoHeight))
	}

	if config.IsCheckEnabled(config.CheckMatroskaAssStyles) {
		results = append(results, checkASSStyles(track))
	}

	needsExtraction := config.IsCheckEnabled(config.CheckMatroskaSubtitleInlineFonts) || config.IsCheckEnabled(config.CheckMatroskaAssEvents)
	if !needsExtraction {
		return results
	}

	content, err := getTrackContent(filePath, track.ID, extractedTracks)
	if err != nil {
		return results
	}

	if config.IsCheckEnabled(config.CheckMatroskaSubtitleInlineFonts) {
		start := time.Now()
		res := checkSubtitleInlineFontsWithContent(track, attachmentFonts, content)
		ui.PrintDebug(fmt.Sprintf("checkSubtitleInlineFontsWithContent for track %d took %v", track.ID, time.Since(start)))

		results = append(results, res)
	}

	if config.IsCheckEnabled(config.CheckMatroskaAssEvents) {
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

	if config.IsCheckEnabled(config.CheckMatroskaNameQuality) {
		results = append(results, checkTrackNameQuality(track))
	}

	if config.IsCheckEnabled(config.CheckMatroskaNameCodecs) {
		results = append(results, checkTrackNameCodecs(track))
	}

	if config.IsCheckEnabled(config.CheckMatroskaNameRedundantLang) {
		results = append(results, checkTrackNameRedundantLang(track))
	}

	if config.IsCheckEnabled(config.CheckMatroskaNameKeywords) {
		results = append(results, checkNameKeywords(track))
	}

	return results
}

func runStatefulTrackChecks(track *matroska.EbmlTrack, audioCounts, subCounts map[string]int, seenTracks map[string]*matroska.EbmlTrack, reportedDuplicates, seenAudioLangs, seenSubLangs map[string]bool) []*CheckResult {
	var results []*CheckResult

	if config.IsCheckEnabled(config.CheckMatroskaDuplicateTracks) {
		results = append(results, checkDuplicateTracks(track, seenTracks, reportedDuplicates))
	}

	if config.IsCheckEnabled(config.CheckMatroskaDefaultFlags) {
		results = append(results, checkDefaultFlags(*track, audioCounts, subCounts, seenAudioLangs, seenSubLangs))
	}

	return results
}

func runTrackOrderCheck(track *matroska.EbmlTrack, lastAudioTrack, lastSubTrack **matroska.EbmlTrack, lastAudioPriority, lastSubPriority *int64, reportedOrderTracks map[int]bool, agg *trackResultAggregator, originalLang string) {
	priority := GetTrackPriority(*track, originalLang)
	switch track.Type {
	case "audio":
		agg.Add(checkTrackOrder(track, *lastAudioTrack, priority, lastAudioPriority, "some Audio tracks are out of order", reportedOrderTracks))
		*lastAudioTrack = track
	case "subtitles":
		agg.Add(checkTrackOrder(track, *lastSubTrack, priority, lastSubPriority, "some Subtitle tracks are out of order", reportedOrderTracks))
		*lastSubTrack = track
	}
}

// GetOriginalLanguageMap returns a map of languages that have an original flag set.
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

// GetTrackCounts counts the audio and subtitle tracks per language.
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

// IsRelevantTrack reports whether a track is an audio or subtitle track.
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
