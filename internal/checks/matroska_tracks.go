package checks

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/ui"
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
	// Bit positions for layout packing
	bitCommentary = 62
	bitCategory   = 60
	bitBaseLang   = 35
	bitFullLang   = 10

	// Bit masks for properties (ensures values don't overflow their allocated bits)
	maskCategory = int64(0x3)       // 2 bits (0..3)
	maskLangName = int64(0x1FFFFFF) // 25 bits
	maskProperty = int64(0x3FF)     // 10 bits

	// Language/Category values (bits 60..61)
	categoryPreferred = int64(0)
	categoryOriginal  = int64(1)
	categoryMul       = int64(2)
	categoryOther     = int64(3)

	priorityPreferred = categoryPreferred << bitCategory
	priorityOriginal  = categoryOriginal << bitCategory
	priorityMul       = categoryMul << bitCategory
	priorityOther     = categoryOther << bitCategory

	priorityCommentaryBit = int64(1) << bitCommentary

	// Property scores (bits 0..9)
	propScoreForced      = int64(0)
	propScoreStandard    = int64(2)
	propScoreAD          = int64(4)
	propScoreSDH         = int64(4)
	propScoreDescription = int64(6)
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
			warning := "score: " + formatPriority(*lastPriority)
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
	commentary := (p >> bitCommentary) & 1
	cat := (p >> bitCategory) & maskCategory
	catField := (commentary << 2) | cat
	base := (p >> bitBaseLang) & maskLangName
	full := (p >> bitFullLang) & maskLangName
	typ := p & maskProperty

	return fmt.Sprintf("0x%X:%07X:%07X:%03X", catField, base, full, typ)
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

// CountLanguagesInString counts recognizable language names found in a track name.
func CountLanguagesInString(name string) int {
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

func isSpecializedTrack(track matroska.EbmlTrack) bool {
	props := track.Properties

	return props.Forced || props.Commentary || props.VisualImpaired || props.HearingImpaired || props.TextDescriptions
}

func checkDefaultFlags(track matroska.EbmlTrack, audioCounts, subCounts map[string]int, seenAudioLangs, seenSubLangs map[string]bool) *CheckResult {
	props := track.Properties
	isSpecialized := isSpecializedTrack(track)

	shouldBeDefault := false
	if !isSpecialized {
		shouldBeDefault = determineShouldBeDefault(track, audioCounts, subCounts, seenAudioLangs, seenSubLangs)
	}

	if props.Default != shouldBeDefault {
		var warning string

		switch {
		case shouldBeDefault:
			warning = fmt.Sprintf("%s first standard track for %s", ui.Success.Render("[+]"), ui.Warning.Render(props.Language))
		case isSpecialized:
			warning = fmt.Sprintf("%s %s", ui.Error.Render("[-]"), ui.Warning.Render("specialized track"))
		default:
			return nil
		}

		return newFailedTrackResult("matroska_default_flags", "Default flags aren't correctly Assigned", "warning", &track, warning)
	}

	return nil
}

func checkTrackDelay(track matroska.EbmlTrack) *CheckResult {
	if track.Properties.CodecDelay == 0 {
		return nil
	}

	absDelay := track.Properties.CodecDelay
	if absDelay < 0 {
		absDelay = -absDelay
	}

	// mkvmerge -J output packet_delay is in nanoseconds.
	// 1001ms = 1,001,000,000 ns.
	const maxDelayNs = 1001 * 1000 * 1000

	if strings.Contains(strings.ToUpper(track.Codec), "A_TRUEHD") {
		return nil
	}

	if absDelay > maxDelayNs {
		warning := fmt.Sprintf("delay of %dms exceeds ±1001ms", track.Properties.CodecDelay/1000000)

		return newFailedTrackResult("matroska_track_delay", "Excessive Container Delay", "warning", &track, warning)
	}

	return nil
}

func determineShouldBeDefault(track matroska.EbmlTrack, audioCounts, subCounts map[string]int, seenAudioLangs, seenSubLangs map[string]bool) bool {
	switch track.Type {
	case "audio":
		return shouldTrackBeDefault(track, audioCounts, seenAudioLangs)
	case "subtitles":
		return shouldTrackBeDefault(track, subCounts, seenSubLangs)
	default:
		return false
	}
}

func shouldTrackBeDefault(track matroska.EbmlTrack, counts map[string]int, seenLangs map[string]bool) bool {
	props := track.Properties

	shouldBeDefault := props.Default

	// If this is the first track of this language we've encountered
	if !seenLangs[props.Language] {
		seenLangs[props.Language] = true
		shouldBeDefault = true
	}

	// For a single track, the default flag is optional (Relaxation).
	if counts[props.Language] == 1 && !props.Default {
		shouldBeDefault = false
	}

	return shouldBeDefault
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
		if CountLanguagesInString(props.Name) < 2 {
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

var (
	directorKeywords = []string{"director", "filmmaker", "producer", "writer"}
	dpKeywords       = []string{"cinematographer", "photography", " dp "}
	actorKeywords    = []string{"actor", "cast", "crew", "lead"}
)

func containsAny(s string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}

	return false
}

func getCommentarySubPriority(name string) int64 {
	nameLower := strings.ToLower(name)

	switch {
	case containsAny(nameLower, directorKeywords):
		return 0
	case containsAny(nameLower, dpKeywords):
		return 1
	case containsAny(nameLower, actorKeywords):
		return 2
	default:
		return 3
	}
}

func getTrackPriority(track matroska.EbmlTrack) int64 {
	langScore := calculateLangScore(track.Properties.Language, track.Properties.OriginalLanguage)
	propertyScore := calculatePropertyScore(track) & maskProperty

	if track.Properties.Commentary {
		commentaryScore := getCommentaryPriority(track)
		if track.Type == "audio" {
			// Audio commentaries: Grouped at the absolute end, ordered strictly by notability & properties (ignoring language)
			return commentaryScore + propertyScore
		}
		// Subtitle commentaries: Grouped at the absolute end, ordered by language, then notability & subtitle properties
		return commentaryScore + langScore + propertyScore
	}

	return langScore + propertyScore
}

func getCommentaryPriority(track matroska.EbmlTrack) int64 {
	return priorityCommentaryBit + (getCommentarySubPriority(track.Properties.Name) << 3)
}

func calculateLangScore(lang string, isOriginal bool) int64 {
	tag := language.Make(lang)
	prefTag := language.Make(config.GetPreferredLanguage())

	var langScore int64

	switch {
	case metadata.MatchLanguage(tag, prefTag):
		langScore = priorityPreferred
	case isOriginal:
		langScore = priorityOriginal
	case metadata.MatchLanguage(tag, language.Make("mul")):
		langScore = priorityMul
	default:
		langScore = priorityOther
	}

	langName := metadata.LanguageName(lang)
	baseTag, _ := tag.Base()
	baseName := metadata.LanguageName(baseTag.String())

	langScore += (calcScore(baseName) & maskLangName) << bitBaseLang
	if baseName != langName {
		langScore += (calcScore(langName) & maskLangName) << bitFullLang
	}

	return langScore
}

func calculateAudioPropertyScore(props matroska.EbmlTrackProperties) int64 {
	switch {
	case props.VisualImpaired:
		return propScoreAD
	case props.TextDescriptions:
		return propScoreDescription
	default:
		return propScoreStandard
	}
}

func calculateSubtitlePropertyScore(props matroska.EbmlTrackProperties) int64 {
	var score int64

	switch {
	case props.Forced:
		score = propScoreForced
	case props.HearingImpaired:
		score = propScoreSDH
	default:
		score = propScoreStandard
	}

	if !props.TextSubtitles {
		score++
	}

	return score
}

func calculatePropertyScore(track matroska.EbmlTrack) int64 {
	var propertyScore int64

	switch track.Type {
	case "audio":
		propertyScore = calculateAudioPropertyScore(track.Properties)
	case "subtitles":
		propertyScore = calculateSubtitlePropertyScore(track.Properties)
	}

	return propertyScore
}

func calcScore(name string) int64 {
	s := int64(0)

	for i := range 5 {
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

func validateTrackBasics(track matroska.EbmlTrack) *CheckResult {
	lang := track.Properties.Language

	tag := language.Make(lang)
	if config.IsCheckEnabled("matroska_language_tag") && tag == language.Und {
		return newFailedTrackResult("matroska_language_tag", "Track doesn't have valid language tag", "warning", &track, ui.Error.Render("missing or invalid language tag"))
	}

	if config.IsCheckEnabled("matroska_multi_lang") && metadata.MatchLanguage(tag, language.Make("mul")) && track.Properties.Name == "" {
		return newFailedTrackResult("matroska_multi_lang", "Multi-language track must have a Name", "warning", &track, ui.Warning.Render("missing Name")+" for 'mul' language")
	}

	return nil
}
