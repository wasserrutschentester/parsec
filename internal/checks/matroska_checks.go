package checks

import (
	"fmt"
	"regexp"
	"strings"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/ui"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

var (
	dtsRegex        = regexp.MustCompile(`\bDTS\b`)
	adRegex         = regexp.MustCompile(`\bAD\b`)
	wordSplitRegex  = regexp.MustCompile(`[\s/.,;()]+`)
	commonLangNames = map[string]string{
		"english": "en", "german": "de", "french": "fr", "spanish": "es",
		"italian": "it", "japanese": "ja", "chinese": "zh", "korean": "ko",
		"russian": "ru", "portuguese": "pt", "dutch": "nl", "polish": "pl",
		"swedish": "sv", "danish": "da", "norwegian": "no", "finnish": "fi",
		"arabic": "ar", "hindi": "hi", "turkish": "tr", "thai": "th",
		"vietnamese": "vi", "indonesian": "id", "hebrew": "he", "czech": "cs",
		"hungarian": "hu", "romanian": "ro", "greek": "el", "bulgarian": "bg",
		"deutsch": "de", "français": "fr", "francais": "fr", "español": "es",
		"espanol": "es", "italiano": "it", "日本語": "ja", "中文": "zh",
		"한국어": "ko", "русский": "ru",
	}

	junkKeywords = []string{"STEREO", "SURROUND", "EXTERNAL", "UPLOADED", "ENCODED"}
	simpleCodecs = []string{"AC3", "AAC", "E-AC3", "EAC3", "FLAC"}
)

const (
	priorityPreferred = int64(1) << 60
	priorityOriginal  = int64(2) << 60
	priorityMul       = int64(3) << 60
	priorityOther     = int64(4) << 60

	propScoreCommentary  = int64(30)
	propScoreAD          = int64(10)
	propScoreDescription = int64(20)
	propScoreForced      = int64(0)
	propScoreSDH         = int64(20)
	propScoreStandard    = int64(10)
)

func checkTrackOrder(track, prevTrack *matroska.EbmlTrack, priority int64, lastPriority *int64, description string, reportedTracks map[int]bool) *CheckResult {
	if priority < *lastPriority {
		res := &CheckResult{
			Identifier: "matroska_track_order",
			Warning:    description,
			Passed:     false,
			Severity:   "warning",
		}

		if prevTrack != nil && !reportedTracks[prevTrack.Properties.Number] {
			warning := fmt.Sprintf("score: %s", formatPriority(*lastPriority))
			res.Tracks = append(res.Tracks, ebmlTrackToResult(prevTrack, true, warning))
			reportedTracks[prevTrack.Properties.Number] = true
		}

		if !reportedTracks[track.Properties.Number] {
			warning := ui.Warning.Render(fmt.Sprintf("score: %s (out of order)", formatPriority(priority)))
			res.Tracks = append(res.Tracks, ebmlTrackToResult(track, false, warning))
			reportedTracks[track.Properties.Number] = true
		}

		*lastPriority = priority

		return res
	}

	*lastPriority = priority

	return nil
}

func formatPriority(p int64) string {
	cat := (p >> 60) & 0xF
	base := (p >> 35) & 0x1FFFFFF
	full := (p >> 10) & 0x1FFFFFF
	typ := p & 0x3FF

	return fmt.Sprintf("0x%X:%07X:%07X:%03X", cat, base, full, typ)
}

func checkTrackNameQuality(track matroska.EbmlTrack) *CheckResult {
	nameUpper := strings.ToUpper(track.Properties.Name)
	for _, junk := range junkKeywords {
		if strings.Contains(nameUpper, junk) {
			warning := fmt.Sprintf("junk keyword '%s' in Name", ui.Warning.Render(junk))
			return newFailedTrackResult("matroska_name_quality", "Track Name contains junk keywords", "info", &track, warning)
		}
	}

	return nil
}

func checkTrackNameCodecs(track matroska.EbmlTrack) *CheckResult {
	nameUpper := strings.ToUpper(track.Properties.Name)
	for _, codec := range simpleCodecs {
		if strings.Contains(nameUpper, codec) {
			warning := fmt.Sprintf("simple codec '%s' in Name", ui.Warning.Render(codec))
			return newFailedTrackResult("matroska_name_codecs", "Track Name contains simple codec", "info", &track, warning)
		}
	}
	// Special handling for DTS (allow DTS-HD, DTS:X etc)
	if strings.Contains(nameUpper, "DTS") && !strings.Contains(nameUpper, "DTS-HD") && !strings.Contains(nameUpper, "DTS:X") && !strings.Contains(nameUpper, "DTS-ES") {
		if dtsRegex.MatchString(nameUpper) {
			warning := fmt.Sprintf("simple codec '%s' in Name", ui.Warning.Render("DTS"))
			return newFailedTrackResult("matroska_name_codecs", "Track Name contains simple codec", "info", &track, warning)
		}
	}

	return nil
}

func checkTrackNameRedundantLang(track matroska.EbmlTrack) *CheckResult {
	if isRedundantLanguageName(track.Properties.Name, track.Properties.Language) {
		return newFailedTrackResult("matroska_name_redundant_lang", "Track Name contains redundant language name", "info", &track, ui.Warning.Render("redundant language in Name"))
	}

	return nil
}

func isRedundantLanguageName(name, trackLang string) bool {
	tag := language.Make(trackLang)
	base, _ := tag.Base()
	target := base.String()

	for _, word := range tokenizeTrackName(name) {
		if getLanguageCodeFromName(word) == target {
			return true
		}
	}

	return false
}

func countLanguagesInString(name string) int {
	count := 0

	for _, word := range tokenizeTrackName(name) {
		if isLanguageName(word) {
			count++
		}
	}

	return count
}

func tokenizeTrackName(name string) []string {
	if name == "" {
		return nil
	}

	return wordSplitRegex.Split(name, -1)
}

func isLanguageName(word string) bool {
	return getLanguageCodeFromName(word) != ""
}

func getLanguageCodeFromName(word string) string {
	if len(word) <= 3 {
		return ""
	}

	wordLower := strings.ToLower(word)

	if base := commonLangNames[wordLower]; base != "" {
		return base
	}

	tag, err := language.Parse(word)
	if err == nil {
		langName := display.English.Languages().Name(tag)
		if strings.EqualFold(langName, word) {
			base, _ := tag.Base()
			return base.String()
		}
	}

	return ""
}

func checkDefaultFlags(track matroska.EbmlTrack, audioCounts, subCounts map[string]int, seenAudioLangs, seenSubLangs map[string]bool) *CheckResult {
	props := track.Properties
	shouldBeDefault := false

	// Specialized tracks MUST NEVER be default
	isSpecialized := props.Forced || props.Commentary || props.VisualImpaired || props.HearingImpaired || props.TextDescriptions

	if !isSpecialized {
		switch track.Type {
		case "audio":
			if !seenAudioLangs[props.Language] {
				shouldBeDefault = true
				seenAudioLangs[props.Language] = true
			}
			// Relaxation: if only one track in this language, it is fine to not set the default flag
			if audioCounts[props.Language] == 1 && !props.Default {
				shouldBeDefault = false
			}
		case "subtitles":
			if !seenSubLangs[props.Language] {
				shouldBeDefault = true
				seenSubLangs[props.Language] = true
			}
			// Relaxation: if only one track in this language, it is fine to not set the default flag
			if subCounts[props.Language] == 1 && !props.Default {
				shouldBeDefault = false
			}
		}
	}

	if props.Default != shouldBeDefault {
		var warning string
		if shouldBeDefault {
			warning = fmt.Sprintf("%s first standard track for %s", ui.Success.Render("[+]"), ui.Warning.Render(props.Language))
		} else if isSpecialized {
			warning = fmt.Sprintf("%s %s", ui.Error.Render("[-]"), ui.Warning.Render("specialized track"))
		} else {
			warning = fmt.Sprintf("%s redundant standard track for %s", ui.Error.Render("[-]"), ui.Warning.Render(props.Language))
		}

		return newFailedTrackResult("matroska_default_flags", "Default flags aren't correctly Assigned", "warning", &track, warning)
	}

	return nil
}

func checkSubtitleFormat(track matroska.EbmlTrack) *CheckResult {
	codec := track.Codec
	if track.Type == "subtitles" && track.Properties.TextSubtitles && !strings.Contains(codec, "SRT") {
		warning := fmt.Sprintf("text-based but codec is %s", codec)
		track.Codec = ui.Warning.Render(track.Codec)

		return newFailedTrackResult("matroska_subtitle_format", "Text subtitle track isn't in SRT format", "warning", &track, warning)
	}

	return nil
}

func checkZlibCompression(track matroska.EbmlTrack) *CheckResult {
	algos := strings.Split(track.Properties.ContentEncodingAlgorithms, ",")
	for _, algo := range algos {
		if algo == "0" { // 0 = zlib
			return newFailedTrackResult("matroska_zlib_compression", "Track uses zlib compression", "warning", &track, ui.Warning.Render("zlib compression enabled"))
		}
	}

	return nil
}

func validateTrackBasics(track matroska.EbmlTrack) *CheckResult {
	lang := track.Properties.Language

	tag := language.Make(lang)
	if config.IsCheckEnabled("matroska_language_tag") && tag == language.Und {
		return newFailedTrackResult("matroska_language_tag", "Track doesn't have valid language tag", "warning", &track, ui.Error.Render("missing or invalid language tag"))
	}

	if config.IsCheckEnabled("matroska_multi_lang") && tag == language.Make("mul") && track.Properties.Name == "" {
		return newFailedTrackResult("matroska_multi_lang", "Multi-language track must have a Name", "warning", &track, fmt.Sprintf("%s for 'mul' language", ui.Warning.Render("missing Name")))
	}

	return nil
}

func checkOriginalLanguageConsistency(track matroska.EbmlTrack, langHasOriginalFlag map[string]bool) *CheckResult {
	lang := track.Properties.Language
	if langHasOriginalFlag[lang] && !track.Properties.OriginalLanguage {
		return newFailedTrackResult("matroska_original_language", "Some but not all Original language tracks have the flag set", "info", &track, "missing Original flag")
	}

	return nil
}

func checkDuplicateTracks(track *matroska.EbmlTrack, seenTracks map[string]*matroska.EbmlTrack, reportedDuplicates map[string]bool) *CheckResult {
	props := track.Properties

	trackKey := fmt.Sprintf("%s-%s-%t-%t-%t-%t-%t-%t-%s",
		track.Type, props.Language, props.Default, props.Forced,
		props.HearingImpaired, props.VisualImpaired,
		props.Commentary, props.OriginalLanguage, props.Name)
	if firstTrack, ok := seenTracks[trackKey]; ok {
		res := &CheckResult{
			Identifier: "matroska_duplicate_tracks",
			Warning:    "Duplicate tracks (same Language, Flags and Name) were found",
			Passed:     false,
			Severity:   "warning",
		}
		if !reportedDuplicates[trackKey] {
			res.Tracks = append(res.Tracks, ebmlTrackToResult(firstTrack, true, "original track"))
			reportedDuplicates[trackKey] = true
		}

		res.Tracks = append(res.Tracks, ebmlTrackToResult(track, false, ui.Warning.Render("duplicate track")))

		return res
	}

	seenTracks[trackKey] = track

	return nil
}

func checkNameKeywords(track matroska.EbmlTrack) *CheckResult {
	props := track.Properties
	nameUpper := strings.ToUpper(props.Name)

	if res := checkFlagKeywordResult(track, props.HearingImpaired, "Hearing Impaired", "SDH", strings.Contains(nameUpper, "SDH")); res != nil {
		return res
	}

	if res := checkFlagKeywordResult(track, props.Forced, "Forced", "Forced", strings.Contains(nameUpper, "FORCED")); res != nil {
		return res
	}

	if res := checkFlagKeywordResult(track, props.Commentary, "Commentary", "Commentary", strings.Contains(nameUpper, "COMMENTARY")); res != nil {
		return res
	}

	hasVIKeyword := strings.Contains(nameUpper, "DESCRIPTIVE") || strings.Contains(nameUpper, "DESCRIPTION") || adRegex.MatchString(nameUpper)
	if res := checkFlagKeywordResult(track, props.VisualImpaired, "Visual Impaired", "Descriptive', 'Description', or 'AD", hasVIKeyword); res != nil {
		return res
	}

	if props.Language == "mul" {
		if countLanguagesInString(props.Name) < 2 {
			return newFailedTrackResult("matroska_name_keywords", "Track Name and Flags don't match", "warning", &track, fmt.Sprintf("'mul' but Name has %s names", ui.Warning.Render("<2 language")))
		}
	}

	return nil
}

func checkFlagKeywordResult(track matroska.EbmlTrack, flag bool, flagName, keywordStr string, hasKeyword bool) *CheckResult {
	identifier := "matroska_name_keywords"
	checkWarning := "Track Name and Flags don't match"

	if flag && !hasKeyword {
		warning := fmt.Sprintf("is %s but Name missing '%s'", ui.Warning.Render(flagName), ui.Warning.Render(keywordStr))
		return newFailedTrackResult(identifier, checkWarning, "info", &track, warning)
	}

	if !flag && hasKeyword {
		warning := fmt.Sprintf("'%s' in Name but no %s flag", ui.Warning.Render(keywordStr), ui.Warning.Render(flagName))
		return newFailedTrackResult(identifier, checkWarning, "info", &track, warning)
	}

	return nil
}

func getTrackPriority(track matroska.EbmlTrack) int64 {
	lang := track.Properties.Language
	tag := language.Make(lang)
	prefTag := language.Make(config.GetPreferredLanguage())

	var langScore int64
	if tag == prefTag {
		langScore = priorityPreferred
	} else if track.Properties.OriginalLanguage {
		langScore = priorityOriginal
	} else if tag == language.Make("mul") {
		langScore = priorityMul
	} else {
		langScore = priorityOther
	}

	langName := metadata.LanguageName(lang)
	baseTag, _ := tag.Base()
	baseName := metadata.LanguageName(baseTag.String())

	langScore += calcScore(baseName) << 35
	if baseName != langName {
		langScore += calcScore(langName) << 10
	}

	var propertyScore int64

	switch track.Type {
	case "audio":
		if track.Properties.Commentary {
			propertyScore = propScoreCommentary
		} else if track.Properties.VisualImpaired {
			propertyScore = propScoreAD
		} else if track.Properties.TextDescriptions {
			propertyScore = propScoreDescription
		}
	case "subtitles":
		if track.Properties.Forced {
			propertyScore = propScoreForced
		} else if track.Properties.HearingImpaired {
			propertyScore = propScoreSDH
		} else {
			propertyScore = propScoreStandard
		}
		// text subs should be before image based subs
		if !track.Properties.TextSubtitles {
			propertyScore += 1
		}
	}

	return langScore + propertyScore
}

func calcScore(name string) int64 {
	s := int64(0)

	for i := 0; i < 5; i++ {
		val := int64(0)

		if i < len(name) {
			c := name[i]
			if c >= 'A' && c <= 'Z' {
				val = int64(c - 'A' + 1)
			}
		}

		s = (s << 5) | val
	}

	return s
}
