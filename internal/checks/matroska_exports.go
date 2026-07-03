package checks

import (
	"slices"
	"strings"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/ui"
)

// FontStyle identifies a font by family, weight and italic flag, the same
// granularity matroska_unused_fonts and matroska_subtitle_fonts match on.
type FontStyle = fontStyle

var (
	// ADRegex matches a standalone "AD" audio-description token in a track name.
	ADRegex = adRegex
	// JunkKeywords are stripped from track names by the name-quality fix policy.
	JunkKeywords = junkKeywords
	// SimpleCodecs are simple codec-name tokens stripped from track names.
	SimpleCodecs = simpleCodecs
)

// GetLanguageCodeFromName maps a language word to its BCP-47 base code.
func GetLanguageCodeFromName(word string) string {
	return getLanguageCodeFromName(word)
}

// DetermineShouldBeDefault decides the default-flag policy for a track.
func DetermineShouldBeDefault(track matroska.EbmlTrack, audioCounts, subCounts map[string]int, seenAudioLangs, seenSubLangs map[string]bool) bool {
	return determineShouldBeDefault(track, audioCounts, subCounts, seenAudioLangs, seenSubLangs)
}

// GetTrackPriority computes a sort priority for audio/subtitle tracks.
func GetTrackPriority(track matroska.EbmlTrack) int64 {
	return getTrackPriority(track)
}

// GetTrackCounts counts audio and subtitle tracks per language.
func GetTrackCounts(tracks []matroska.EbmlTrack) (audio, sub map[string]int) {
	return getTrackCounts(tracks)
}

// GetOriginalLanguageMap returns languages that already have an original flag.
func GetOriginalLanguageMap(tracks []matroska.EbmlTrack) map[string]bool {
	return getOriginalLanguageMap(tracks)
}

// IsRelevantTrack reports whether a track is audio or subtitles.
func IsRelevantTrack(track matroska.EbmlTrack) bool {
	return isRelevantTrack(track)
}

// IsFontAttachment detects font attachments by file extension or MIME type.
func IsFontAttachment(att matroska.EbmlAttachment) bool {
	return isFontAttachment(att)
}

// GetFontMapping extracts font attachments and returns normalized name and
// attachment-ID lookups for fix policy code.
func GetFontMapping(filePath string, attachments []matroska.EbmlAttachment) (map[string]string, map[int][]string) {
	return FontMappingFromFonts(getFontMapping(filePath, attachments))
}

// FontMappingFromFonts derives the same normalized name and attachment-ID
// lookups as GetFontMapping, but from an already-parsed font list. Callers
// that already hold a GetAttachmentFonts result (the common case: parsing
// every attachment's font binary is the expensive part, and its result stays
// valid across a whole correct run since renaming/attaching/removing
// attachments never changes another attachment's font binary) should use
// this instead of calling GetFontMapping again, which would reparse from
// scratch after any mkvpropedit edit bumps the file's mtime and busts
// getFontMapping's cache key.
func FontMappingFromFonts(attachmentFonts []matroska.AttachmentFontInfo) (map[string]string, map[int][]string) {
	fontMap := make(map[string]string)
	attachmentNames := make(map[int][]string)

	for _, font := range attachmentFonts {
		names := []string{font.PostScriptName, font.FamilyName}
		names = append(names, font.FullNames...)
		attachmentNames[font.AttachmentID] = uniqueStrings(append(attachmentNames[font.AttachmentID], names...))
	}

	for _, names := range attachmentNames {
		for _, name := range names {
			fontMap[normalizeFontName(name)] = name
		}
	}

	return fontMap, attachmentNames
}

// extractASSTrackContents extracts every ASS/SSA subtitle track in one
// mkvextract invocation (matroska.ExtractTracks) rather than one process per
// track, mirroring the check pipeline's batchExtractTracksIfNeeded. Tracks
// missing from the result (extraction failed, or wasn't attempted because no
// ASS tracks exist) simply contribute no inline fonts.
func extractASSTrackContents(filePath string, tracks []matroska.EbmlTrack) map[int][]byte {
	var ids []int

	for _, track := range tracks {
		if isASSSubtitles(track) {
			ids = append(ids, track.ID)
		}
	}

	if len(ids) == 0 {
		return nil
	}

	extracted, err := matroska.ExtractTracks(filePath, ids)
	if err != nil {
		ui.PrintDebug("Failed to extract tracks for inline font scan: " + err.Error())

		return nil
	}

	return extracted
}

// ComputeUsedFonts gathers the fonts (family, weight, italic) referenced by
// ASS/SSA subtitle tracks, at the same granularity the matroska_unused_fonts
// and matroska_subtitle_fonts checks match on (see findUnusedFontAttachments
// and findMissingFonts) so fix and check can never disagree about which
// fonts are in use. Both style-block and inline tag sources are always
// scanned regardless of check config, so the fix never incorrectly flags a
// font as unused because a check happens to be disabled. A track whose
// content can't be extracted contributes no inline fonts for that track
// only; it does not affect the rest.
func ComputeUsedFonts(filePath string, tracks []matroska.EbmlTrack) map[FontStyle]bool {
	allUsedFonts := make(map[FontStyle]bool)
	extracted := extractASSTrackContents(filePath, tracks)

	for _, track := range tracks {
		if !isASSSubtitles(track) {
			continue
		}

		for font := range styleFontsFromTrack(track) {
			allUsedFonts[font] = true
		}

		content, ok := extracted[track.ID]
		if !ok {
			continue
		}

		for font := range inlineFontsFromContent(track, content) {
			allUsedFonts[font] = true
		}
	}

	return allUsedFonts
}

// ComputeMissingFonts gathers ASS/SSA font descriptions that lack a matching
// attachment, matching by family+weight+italic via findMissingFonts (the
// same function matroska_subtitle_fonts/matroska_subtitle_inline_fonts use).
func ComputeMissingFonts(filePath string, tracks []matroska.EbmlTrack, attachmentFonts []matroska.AttachmentFontInfo) []string {
	seen := make(map[string]bool)
	missing := make([]string, 0)
	extracted := extractASSTrackContents(filePath, tracks)

	addMissing := func(descs []string) {
		for _, desc := range descs {
			normalized := normalizeFontName(desc)
			if seen[normalized] {
				continue
			}

			seen[normalized] = true

			missing = append(missing, desc)
		}
	}

	for _, track := range tracks {
		if !isASSSubtitles(track) {
			continue
		}

		if config.IsCheckEnabled("matroska_subtitle_fonts") {
			addMissing(findMissingFonts(styleFontsFromTrack(track), attachmentFonts))
		}

		if config.IsCheckEnabled("matroska_subtitle_inline_fonts") {
			if content, ok := extracted[track.ID]; ok {
				addMissing(findMissingFonts(inlineFontsFromContent(track, content), attachmentFonts))
			}
		}
	}

	slices.Sort(missing)

	return missing
}

func styleFontsFromTrack(track matroska.EbmlTrack) map[fontStyle]bool {
	privateBytes, err := track.Properties.DecodeCodecPrivate()
	if err != nil || len(privateBytes) == 0 {
		return nil
	}

	usedFonts := make(map[fontStyle]bool)
	parseFontsFromStyles(strings.Split(string(privateBytes), "\n"), usedFonts)

	return usedFonts
}

func inlineFontsFromContent(track matroska.EbmlTrack, content []byte) map[fontStyle]bool {
	usedFonts := make(map[fontStyle]bool)
	styleConfigs := parseStyleConfigsFromTrack(track)
	parseFontsFromInlineTagsWithState(content, styleConfigs, usedFonts)

	return usedFonts
}

func parseStyleConfigsFromTrack(track matroska.EbmlTrack) map[string]fontStyle {
	privateBytes, err := track.Properties.DecodeCodecPrivate()
	if err != nil || len(privateBytes) == 0 {
		return nil
	}

	return parseStyleConfigs(privateBytes)
}

// GetAttachmentFonts returns the parsed font info (family, weight, italic,
// internal names) for each font attachment in the file. It is the same data
// matroska_unused_fonts and matroska_subtitle_fonts match against, so fix
// computations that need family/weight/italic fidelity (as opposed to
// GetFontMapping's flattened name lookup) should use this instead.
func GetAttachmentFonts(filePath string, attachments []matroska.EbmlAttachment) []matroska.AttachmentFontInfo {
	return getFontMapping(filePath, attachments)
}

// UnusedFontAttachments returns font attachments not referenced by any
// subtitle track, matching by PostScript name or by family+italic+weight via
// findUnusedFontAttachments, the same matching matroska_unused_fonts uses.
func UnusedFontAttachments(attachments []matroska.EbmlAttachment, attachmentFonts []matroska.AttachmentFontInfo, allUsedFonts map[FontStyle]bool) []matroska.EbmlAttachment {
	return findUnusedFontAttachments(attachments, attachmentFonts, allUsedFonts)
}

// FontFilenameCompliant reports whether a filename matches an internal font name.
func FontFilenameCompliant(fileName string, internalNames []string) bool {
	return isAttachmentNameCompliant(fileName, internalNames)
}

// ProposedFontFilename returns the compliant filename matroska_font_filename_compliance
// proposes for a non-compliant font attachment: PostScript name, then the
// first full name, then the family name (each cleaned via
// cleanFallbackFontName), keeping attFileName's original extension. Returns
// "" if attID has no usable internal name. correct's rename fix must use
// this rather than deriving its own target, so the name it actually writes
// can never drift from what the check displays as "Proposed Name".
func ProposedFontFilename(attFileName string, attID int, attachmentFonts []matroska.AttachmentFontInfo) string {
	return getProposedFontFilename(attFileName, attID, attachmentFonts)
}

// FontNameMatches reports whether name matches one of a font file's internal names.
func FontNameMatches(name string, internalNames []string) bool {
	normalizedName := normalizeFontName(name)
	for _, internalName := range internalNames {
		if normalizeFontName(internalName) == normalizedName {
			return true
		}
	}

	return false
}

// ExtractCommentaryCoreOriginalCase returns the core identifying part of a
// commentary track name with original case preserved and SDH tokens intact.
func ExtractCommentaryCoreOriginalCase(name string) string {
	return extractCommentaryCoreRaw(name)
}

// ExtractCommentaryCore returns the normalised core of a commentary name:
// SDH decorations removed, trimmed, and lowercased.
func ExtractCommentaryCore(name string) string {
	return extractCoreCommentaryName(name)
}

// GetVideoTrackNumberFromEBML returns the first video track's Matroska
// number, or 0 when there is no video track.
func GetVideoTrackNumberFromEBML(ebml *matroska.EbmlMetadata) uint64 {
	track := getVideoTrackFromEBML(ebml)
	if track == nil {
		return 0
	}

	return uint64(track.Properties.Number)
}

// IsAligned reports whether a chapter timestamp is close enough to a keyframe.
func IsAligned(timeStart int64, keyframes []int64) (bool, int64) {
	aligned, closestDiff, _, _ := isAligned(timeStart, keyframes)

	return aligned, closestDiff
}
