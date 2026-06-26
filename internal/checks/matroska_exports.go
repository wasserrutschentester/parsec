package checks

import (
	"slices"
	"strings"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

var (
	// ADRegex matches a standalone "AD" audio-description token in a track name.
	ADRegex = adRegex
	// JunkKeywords are stripped from track names by the name-quality fix policy.
	JunkKeywords = junkKeywords
	// SimpleCodecs are simple codec-name tokens stripped from track names.
	SimpleCodecs = simpleCodecs
)

// CountLanguagesInString counts recognizable language names found in a track name.
func CountLanguagesInString(name string) int {
	return countLanguagesInString(name)
}

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
	attachmentFonts := getFontMapping(filePath, attachments)
	fontMap := make(map[string]string)
	attachmentNames := make(map[int][]string)

	for _, font := range attachmentFonts {
		names := []string{font.FamilyName, font.PostScriptName}
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

// ComputeUsedFonts gathers font names referenced by ASS/SSA subtitle tracks.
func ComputeUsedFonts(filePath string, tracks []matroska.EbmlTrack, _ map[string]string) map[string]bool {
	allUsedFonts := make(map[string]bool)

	for _, track := range tracks {
		if !isASSSubtitles(track) {
			continue
		}

		if config.IsCheckEnabled("matroska_subtitle_fonts") {
			for font := range styleFontsFromTrack(track) {
				allUsedFonts[font.Family] = true
			}
		}

		if config.IsCheckEnabled("matroska_subtitle_inline_fonts") {
			if content, err := matroska.ExtractTrack(filePath, track.ID); err == nil {
				for font := range inlineFontsFromContent(track, content) {
					allUsedFonts[font.Family] = true
				}
			}
		}
	}

	return allUsedFonts
}

// ComputeMissingFonts gathers ASS/SSA font names that lack a matching attachment.
func ComputeMissingFonts(filePath string, tracks []matroska.EbmlTrack, fontMap map[string]string) []string {
	seen := make(map[string]bool)
	missing := make([]string, 0)

	for _, track := range tracks {
		if !isASSSubtitles(track) {
			continue
		}

		if config.IsCheckEnabled("matroska_subtitle_fonts") {
			missing = addMissingFonts(missing, seen, styleFontsFromTrack(track), fontMap)
		}

		if config.IsCheckEnabled("matroska_subtitle_inline_fonts") {
			if content, err := matroska.ExtractTrack(filePath, track.ID); err == nil {
				missing = addMissingFonts(missing, seen, inlineFontsFromContent(track, content), fontMap)
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

func addMissingFonts(missing []string, seen map[string]bool, usedFonts map[fontStyle]bool, fontMap map[string]string) []string {
	for font := range usedFonts {
		if fontMap[normalizeFontName(font.Family)] != "" {
			continue
		}

		desc := formatMissingFontDesc(font)
		normalized := normalizeFontName(desc)

		if seen[normalized] {
			continue
		}

		seen[normalized] = true

		missing = append(missing, desc)
	}

	return missing
}

// UnusedFontAttachments returns font attachments not referenced by subtitles.
func UnusedFontAttachments(attachments []matroska.EbmlAttachment, attachmentNames map[int][]string, allUsedFonts map[string]bool) []matroska.EbmlAttachment {
	normalizedUsedFonts := make(map[string]bool, len(allUsedFonts))
	for font := range allUsedFonts {
		normalizedUsedFonts[normalizeFontName(font)] = true
	}

	var unused []matroska.EbmlAttachment

	for _, att := range attachments {
		if !isFontAttachment(att) {
			continue
		}

		found := false

		for _, name := range attachmentNames[att.ID] {
			if normalizedUsedFonts[normalizeFontName(name)] {
				found = true

				break
			}
		}

		if !found {
			unused = append(unused, att)
		}
	}

	return unused
}

// FontFilenameCompliant reports whether a filename matches an internal font name.
func FontFilenameCompliant(fileName string, internalNames []string) bool {
	return isAttachmentNameCompliant(fileName, internalNames)
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

// GetVideoTrackNumberFromEBML returns the first video track's Matroska number.
func GetVideoTrackNumberFromEBML(ebml *matroska.EbmlMetadata) uint64 {
	return getVideoTrackNumberFromEBML(ebml)
}

// IsAligned reports whether a chapter timestamp is close enough to a keyframe.
func IsAligned(timeStart int64, keyframes []int64) (bool, int64) {
	return isAligned(timeStart, keyframes)
}
