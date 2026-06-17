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

func checkSubtitleFormat(track matroska.EbmlTrack) *CheckResult {
	codec := track.Codec
	if track.Type == "subtitles" && track.Properties.TextSubtitles && !strings.Contains(codec, "SRT") && !strings.Contains(codec, "SubStationAlpha") {
		warning := "text-based but codec is " + codec
		track.Codec = ui.Warning.Render(track.Codec)

		return newFailedTrackResult("matroska_subtitle_format", "Text subtitle track should be in SRT or SubStationAlpha format (convert others to SRT)", "warning", &track, warning)
	}

	return nil
}

func checkSubtitleFonts(filePath string, track matroska.EbmlTrack, attachments []matroska.EbmlAttachment) *CheckResult {
	if track.Type != "subtitles" || (!strings.Contains(track.Codec, "ASS") && !strings.Contains(track.Codec, "SSA") && !strings.Contains(track.Codec, "SubStationAlpha")) {
		return nil
	}

	content, err := matroska.ExtractTrack(filePath, track.ID)
	if err != nil {
		return nil
	}

	usedFonts := parseUsedFonts(content)
	if len(usedFonts) == 0 {
		return nil
	}

	missing := findMissingFonts(usedFonts, attachments)

	if len(missing) > 0 {
		warning := "missing fonts: " + strings.Join(missing, ", ")

		return newFailedTrackResult("matroska_subtitle_fonts", "SSA/ASS subtitle track uses fonts not included as attachments", "warning", &track, warning)
	}

	return nil
}

func findMissingFonts(usedFonts map[string]bool, attachments []matroska.EbmlAttachment) []string {
	var missing []string

	for font := range usedFonts {
		found := false

		for _, att := range attachments {
			if strings.Contains(strings.ToLower(att.FileName), strings.ToLower(font)) {
				found = true

				break
			}
		}

		if !found {
			missing = append(missing, font)
		}
	}

	return missing
}

func parseUsedFonts(content []byte) map[string]bool {
	fonts := make(map[string]bool)
	contentStr := string(content)
	lines := strings.Split(contentStr, "\n")

	parseFontsFromStyles(lines, fonts)
	parseFontsFromInlineTags(contentStr, fonts)

	return fonts
}

func parseFontsFromStyles(lines []string, fonts map[string]bool) {
	inStyles := false
	formatFields := []string{}

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "[V4+ Styles]" || line == "[V4 Styles]" {
			inStyles = true

			continue
		}

		if strings.HasPrefix(line, "[") {
			inStyles = false
		}

		if !inStyles {
			continue
		}

		if rest, ok := strings.CutPrefix(line, "Format:"); ok {
			formatFields = parseStyleFormat(rest)
		} else if rest, ok := strings.CutPrefix(line, "Style:"); ok {
			extractFontFromStyle(rest, formatFields, fonts)
		}
	}
}

func parseStyleFormat(rest string) []string {
	fields := strings.Split(rest, ",")
	for i := range fields {
		fields[i] = strings.TrimSpace(fields[i])
	}

	return fields
}

func extractFontFromStyle(rest string, formatFields []string, fonts map[string]bool) {
	values := strings.Split(rest, ",")

	for i, field := range formatFields {
		if i < len(values) && field == "Fontname" {
			fonts[strings.TrimSpace(values[i])] = true
		}
	}
}

func parseFontsFromInlineTags(content string, fonts map[string]bool) {
	re := regexp.MustCompile(`\\fn([^\\}]+)`)
	matches := re.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		fonts[strings.TrimSpace(match[1])] = true
	}
}

func checkZlibCompression(track matroska.EbmlTrack) *CheckResult {
	for algo := range strings.SplitSeq(track.Properties.ContentEncodingAlgorithms, ",") {
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
		return newFailedTrackResult("matroska_multi_lang", "Multi-language track must have a Name", "warning", &track, ui.Warning.Render("missing Name")+" for 'mul' language")
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

// trackDuplicateKey builds the identity used to detect duplicate tracks: two
// tracks sharing this key are considered duplicates.
func trackDuplicateKey(track matroska.EbmlTrack) string {
	props := track.Properties

	return fmt.Sprintf("%s-%s-%t-%t-%t-%t-%t-%t-%s",
		track.Type, props.Language, props.Default, props.Forced,
		props.HearingImpaired, props.VisualImpaired,
		props.Commentary, props.OriginalLanguage, props.Name)
}

func checkDuplicateTracks(track *matroska.EbmlTrack, seenTracks map[string]*matroska.EbmlTrack, reportedDuplicates map[string]bool) *CheckResult {
	trackKey := trackDuplicateKey(*track)
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
	langScore := calculateLangScore(track.Properties.Language, track.Properties.OriginalLanguage)
	propertyScore := calculatePropertyScore(track)

	return langScore + propertyScore
}

func calculateLangScore(lang string, isOriginal bool) int64 {
	tag := language.Make(lang)
	prefTag := language.Make(config.GetPreferredLanguage())

	var langScore int64

	switch {
	case tag == prefTag:
		langScore = priorityPreferred
	case isOriginal:
		langScore = priorityOriginal
	case tag == language.Make("mul"):
		langScore = priorityMul
	default:
		langScore = priorityOther
	}

	langName := metadata.LanguageName(lang)
	baseTag, _ := tag.Base()
	baseName := metadata.LanguageName(baseTag.String())

	langScore += calcScore(baseName) << 35
	if baseName != langName {
		langScore += calcScore(langName) << 10
	}

	return langScore
}

func calculatePropertyScore(track matroska.EbmlTrack) int64 {
	var propertyScore int64

	switch track.Type {
	case "audio":
		switch {
		case track.Properties.Commentary:
			propertyScore = propScoreCommentary
		case track.Properties.VisualImpaired:
			propertyScore = propScoreAD
		case track.Properties.TextDescriptions:
			propertyScore = propScoreDescription
		}
	case "subtitles":
		switch {
		case track.Properties.Forced:
			propertyScore = propScoreForced
		case track.Properties.HearingImpaired:
			propertyScore = propScoreSDH
		default:
			propertyScore = propScoreStandard
		}
		// text subs should be before image based subs
		if !track.Properties.TextSubtitles {
			propertyScore++
		}
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
