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

		return newFailedTrackResult("matroska_subtitle_format", "Text subtitle track should converted to SRT", "warning", &track, warning)
	}

	return nil
}

func checkSubtitleFonts(track matroska.EbmlTrack, fontMap map[string]string, allUsedFonts map[string]bool) *CheckResult {
	if !isASSSubtitles(track) {
		return nil
	}

	privateBytes, err := track.Properties.DecodeCodecPrivate()
	if err != nil || len(privateBytes) == 0 {
		return nil
	}

	usedFonts := make(map[string]bool)
	lines := strings.Split(string(privateBytes), "\n")
	parseFontsFromStyles(lines, usedFonts)

	if len(usedFonts) == 0 {
		return nil
	}

	for font := range usedFonts {
		allUsedFonts[font] = true
	}

	missing := findMissingFonts(usedFonts, fontMap)

	if len(missing) > 0 {
		warning := "missing fonts (Styles): " + strings.Join(missing, ", ")

		return newFailedTrackResult("matroska_subtitle_fonts", "SSA/ASS subtitle track uses fonts in Styles not included as attachments", "warning", &track, warning)
	}

	return nil
}

func checkSubtitleInlineFontsWithContent(track matroska.EbmlTrack, fontMap map[string]string, content []byte, allUsedFonts map[string]bool) *CheckResult {
	usedFonts := make(map[string]bool)

	parseFontsFromInlineTags(string(content), usedFonts)

	if len(usedFonts) == 0 {
		return nil
	}

	for font := range usedFonts {
		allUsedFonts[font] = true
	}

	missing := findMissingFonts(usedFonts, fontMap)

	if len(missing) > 0 {
		warning := "missing fonts (Inline): " + strings.Join(missing, ", ")

		return newFailedTrackResult("matroska_subtitle_inline_fonts", "SSA/ASS subtitle track uses fonts in inline tags not included as attachments", "warning", &track, warning)
	}

	return nil
}

func checkASSScriptInfo(track matroska.EbmlTrack, videoWidth, videoHeight int) *CheckResult {
	privateBytes, err := track.Properties.DecodeCodecPrivate()
	if err != nil || len(privateBytes) == 0 {
		return nil
	}

	info := parseScriptInfo(string(privateBytes))
	errors := validateScriptInfo(info, videoWidth, videoHeight)

	if len(errors) > 0 {
		return newFailedTrackResult("matroska_ass_script_info", "ASS Script Info missing recommended headers", "info", &track, strings.Join(errors, "\n"))
	}

	return nil
}

func parseScriptInfo(content string) map[string]string {
	info := make(map[string]string)
	inScriptInfo := false

	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)

		if line == "[Script Info]" {
			inScriptInfo = true

			continue
		}

		if strings.HasPrefix(line, "[") {
			inScriptInfo = false
		}

		if inScriptInfo {
			if key, value, ok := strings.Cut(line, ":"); ok {
				info[strings.TrimSpace(key)] = strings.TrimSpace(value)
			}
		}
	}

	return info
}

func validateScriptInfo(info map[string]string, videoWidth, videoHeight int) []string {
	errors := make([]string, 0, 10)

	errors = append(errors, validateFunctionalHeaders(info)...)
	errors = append(errors, validateResolutionHeaders(info, videoWidth, videoHeight)...)

	return errors
}

func validateFunctionalHeaders(info map[string]string) []string {
	errors := make([]string, 0, 3)

	errors = append(errors, validateScriptType(info)...)
	errors = append(errors, validateScaledBorderAndShadow(info)...)
	errors = append(errors, validateYCbCrMatrix(info)...)

	return errors
}

func validateScriptType(info map[string]string) []string {
	val, ok := info["ScriptType"]
	if !ok || (val != "v4.00+" && val != "v4.00" && val != "v4.00++") {
		msg := "missing or invalid ScriptType"
		if ok {
			msg += " (Line: ScriptType: " + val + ")"
		}

		return []string{msg}
	}

	return nil
}

func validateScaledBorderAndShadow(info map[string]string) []string {
	val, ok := info["ScaledBorderAndShadow"]
	if !ok || strings.ToLower(val) != "yes" {
		msg := "ScaledBorderAndShadow should be 'yes'"
		if ok {
			msg += " (Line: ScaledBorderAndShadow: " + val + ")"
		}

		return []string{msg}
	}

	return nil
}

func validateYCbCrMatrix(info map[string]string) []string {
	val, ok := info["YCbCr Matrix"]
	if !ok {
		return []string{"missing YCbCr Matrix"}
	}

	if val == "" {
		return []string{"empty YCbCr Matrix (Line: YCbCr Matrix: )"}
	}

	return nil
}

func validateResolutionHeaders(info map[string]string, videoWidth, videoHeight int) []string {
	var errors []string

	headers := []string{"PlayResX", "PlayResY", "LayoutResX", "LayoutResY"}

	for _, key := range headers {
		val, ok := info[key]
		if !ok {
			errors = append(errors, "missing "+key)

			continue
		}

		target := videoWidth
		if strings.HasSuffix(key, "Y") {
			target = videoHeight
		}

		res := 0
		if n, _ := fmt.Sscanf(val, "%d", &res); n == 1 {
			if videoWidth > 0 && videoHeight > 0 && res != target {
				errors = append(errors, fmt.Sprintf("%s should be %d, got %s", key, target, val))
			}
		}
	}

	return errors
}

func checkASSStyles(track matroska.EbmlTrack) *CheckResult {
	privateBytes, err := track.Properties.DecodeCodecPrivate()
	if err != nil || len(privateBytes) == 0 {
		return nil
	}

	lines := strings.Split(string(privateBytes), "\n")
	errors := validateStyles(lines)

	if len(errors) > 0 {
		warning := strings.Join(errors, "\n")

		return newFailedTrackResult("matroska_ass_styles", "ASS Style validation failed", "warning", &track, warning)
	}

	return nil
}

func validateStyles(lines []string) []string {
	inStyles := false
	formatFields := []string{}

	var errors []string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "[V4+ Styles]" {
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
			for _, err := range validateStyleLine(rest, formatFields) {
				errors = append(errors, fmt.Sprintf("%s (Line: Style: %s)", err, rest))
			}
		}
	}

	return errors
}

func validateStyleLine(rest string, formatFields []string) []string {
	values := strings.Split(rest, ",")

	errors := make([]string, 0, len(formatFields))

	for i, field := range formatFields {
		if i >= len(values) {
			break
		}

		val := strings.TrimSpace(values[i])
		errors = append(errors, validateStyleField(field, val)...)
	}

	return errors
}

func validateStyleField(field, val string) []string {
	switch field {
	case "Name":
		return validateStyleName(val)
	case "Fontname":
		return validateStyleFontname(val)
	case "Fontsize":
		return validateStyleFontsize(val)
	case "BorderStyle":
		return validateStyleBorderStyle(val)
	case "Alignment":
		return validateStyleAlignment(val)
	case "Encoding":
		return validateStyleEncoding(val)
	}

	return nil
}

func validateStyleName(val string) []string {
	if val == "" {
		return []string{"empty Style Name"}
	}

	if strings.TrimSpace(val) != val {
		return []string{"Style Name has leading/trailing whitespace"}
	}

	return nil
}

func validateStyleFontname(val string) []string {
	if len(val) > 31 {
		return []string{"Fontname '" + val + "' too long (>31)"}
	}

	return nil
}

func validateStyleFontsize(val string) []string {
	fs := 0.0
	if n, _ := fmt.Sscanf(val, "%f", &fs); n == 1 {
		if fs < 0 || fs > 511 {
			return []string{"invalid Fontsize " + val}
		}
	}

	return nil
}

func validateStyleBorderStyle(val string) []string {
	if val != "1" && val != "3" {
		return []string{"invalid BorderStyle " + val}
	}

	return nil
}

func validateStyleAlignment(val string) []string {
	align := 0
	if n, _ := fmt.Sscanf(val, "%d", &align); n == 1 {
		if align < 1 || align > 9 {
			return []string{"invalid Alignment " + val}
		}
	}

	return nil
}

func validateStyleEncoding(val string) []string {
	if val != "1" {
		return []string{"Encoding should be 1, got " + val}
	}

	return nil
}

func checkASSEvents(track matroska.EbmlTrack, content []byte) *CheckResult {
	lines := strings.Split(string(content), "\n")
	definedStyles := parseDefinedStyles(lines)
	errors := validateEvents(lines, definedStyles)

	if len(errors) > 0 {
		warning := strings.Join(errors, "\n")

		return newFailedTrackResult("matroska_ass_events", "ASS Event validation failed", "warning", &track, warning)
	}

	return nil
}

func parseDefinedStyles(lines []string) map[string]bool {
	definedStyles := make(map[string]bool)
	inStyles := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "[V4+ Styles]" {
			inStyles = true

			continue
		}

		if strings.HasPrefix(line, "[") {
			inStyles = false
		}

		if inStyles {
			if rest, ok := strings.CutPrefix(line, "Style:"); ok {
				fields := strings.Split(rest, ",")
				if len(fields) > 0 {
					definedStyles[strings.TrimSpace(fields[0])] = true
				}
			}
		}
	}

	return definedStyles
}

func validateEvents(lines []string, definedStyles map[string]bool) []string {
	inEvents := false
	formatFields := []string{}

	var errors []string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "[Events]" {
			inEvents = true

			continue
		}

		if strings.HasPrefix(line, "[") && line != "[Events]" {
			inEvents = false
		}

		if !inEvents {
			continue
		}

		if rest, ok := strings.CutPrefix(line, "Format:"); ok {
			formatFields = parseStyleFormat(rest)
		} else if rest, ok := strings.CutPrefix(line, "Dialogue:"); ok {
			for _, err := range validateEventLine(rest, formatFields, definedStyles) {
				errors = append(errors, fmt.Sprintf("%s (Line: Dialogue: %s)", err, rest))
			}
		}
	}

	return errors
}

func validateEventLine(rest string, formatFields []string, definedStyles map[string]bool) []string {
	values := splitEventLine(rest, len(formatFields))

	errors := make([]string, 0, len(formatFields))

	for i, field := range formatFields {
		if i >= len(values) {
			break
		}

		val := strings.TrimSpace(values[i])
		errors = append(errors, validateEventField(field, val, definedStyles)...)
	}

	return errors
}

func validateEventField(field, val string, definedStyles map[string]bool) []string {
	var errors []string

	timeRegex := regexp.MustCompile(`^\d:\d\d:\d\d\.\d\d$`)

	switch field {
	case "Start", "End":
		if !timeRegex.MatchString(val) {
			errors = append(errors, "invalid time format '"+val+"'")
		}
	case "Style":
		if !definedStyles[val] {
			errors = append(errors, "undefined style '"+val+"'")
		}
	case "Text":
		errors = append(errors, validateEventText(val)...)
	case "Effect":
		if val != "" {
			errors = append(errors, validateEffect(val)...)
		}
	}

	return errors
}

func validateEventText(val string) []string {
	var errors []string

	if strings.Contains(val, "\\fe") {
		errors = append(errors, "forbidden tag \\fe used")
	}

	if strings.Contains(val, "\\be") && !strings.Contains(val, "\\blur") {
		errors = append(errors, "\\blur should be preferred over \\be")
	}

	return errors
}

func splitEventLine(line string, fieldCount int) []string {
	if fieldCount <= 1 {
		return []string{line}
	}

	parts := strings.SplitN(line, ",", fieldCount)

	return parts
}

func validateEffect(effect string) []string {
	parts := strings.Split(effect, ";")
	name := strings.ToLower(strings.TrimSpace(parts[0]))

	var errors []string

	if name == "banner" || name == "scroll up" || name == "scroll down" {
		errors = append(errors, validateStandardEffect(effect, parts)...)
	}

	return errors
}

func validateStandardEffect(effect string, parts []string) []string {
	var errors []string

	if len(parts) >= 2 {
		delay := 0
		if n, _ := fmt.Sscanf(parts[1], "%d", &delay); n == 1 {
			if delay < 1 || delay > 100 {
				errors = append(errors, "invalid delay "+parts[1]+" in effect")
			}
		}
	}

	if strings.Contains(strings.ToLower(effect), "fadeaway") {
		errors = append(errors, "unsupported effect parameter 'fadeaway'")
	}

	return errors
}

func isASSSubtitles(track matroska.EbmlTrack) bool {
	return track.Type == "subtitles" && (strings.Contains(track.Codec, "ASS") || strings.Contains(track.Codec, "SSA") || strings.Contains(track.Codec, "SubStationAlpha"))
}

// findMissingFonts checks if each used font has a matching attachment using robust internal name mapping.
func findMissingFonts(usedFonts map[string]bool, fontMap map[string]string) []string {
	var missing []string

	for font := range usedFonts {
		normalizedFont := normalizeFontName(font)
		if _, found := fontMap[normalizedFont]; !found {
			missing = append(missing, font)
		}
	}

	return missing
}

func normalizeFontName(name string) string {
	// Remove common separators and convert to lowercase for robust matching
	r := strings.NewReplacer(" ", "", "-", "", "_", "")

	return strings.ToLower(r.Replace(name))
}

func isFontAttachment(att matroska.EbmlAttachment) bool {
	lowerName := strings.ToLower(att.FileName)
	if strings.HasSuffix(lowerName, ".ttf") || strings.HasSuffix(lowerName, ".otf") || strings.HasSuffix(lowerName, ".ttc") {
		return true
	}

	lowerType := strings.ToLower(att.ContentType)

	return strings.HasPrefix(lowerType, "font/") ||
		strings.Contains(lowerType, "truetype") ||
		strings.Contains(lowerType, "opentype") ||
		strings.Contains(lowerType, "font-sfnt")
}

func checkUnusedFonts(attachments []matroska.EbmlAttachment, attachmentNames map[int][]string, allUsedFonts map[string]bool) *CheckResult {
	var unused []string

	normalizedUsedFonts := make(map[string]bool)
	for f := range allUsedFonts {
		normalizedUsedFonts[normalizeFontName(f)] = true
	}

	for _, att := range attachments {
		if !isFontAttachment(att) {
			continue
		}

		names := attachmentNames[att.ID]
		found := false

		for _, name := range names {
			if normalizedUsedFonts[normalizeFontName(name)] {
				found = true

				break
			}
		}

		if !found {
			unused = append(unused, att.FileName)
		}
	}

	if len(unused) > 0 {
		return &CheckResult{
			Identifier: "matroska_unused_fonts",
			Warning:    "Font attachments not used by any subtitle track: " + strings.Join(unused, ", "),
			Passed:     false,
			Severity:   "warning",
		}
	}

	return nil
}

func checkFontFilenameCompliance(attachments []matroska.EbmlAttachment, attachmentNames map[int][]string) *CheckResult {
	var nonCompliant []string

	for _, att := range attachments {
		if !isFontAttachment(att) {
			continue
		}

		names, ok := attachmentNames[att.ID]
		if !ok || len(names) == 0 {
			continue
		}

		// Get filename without extension
		baseName := att.FileName
		if idx := strings.LastIndex(baseName, "."); idx != -1 {
			baseName = baseName[:idx]
		}

		normalizedFileName := normalizeFontName(baseName)
		compliant := false

		for _, internalName := range names {
			if normalizeFontName(internalName) == normalizedFileName {
				compliant = true

				break
			}
		}

		if !compliant {
			nonCompliant = append(nonCompliant, fmt.Sprintf("%s (internal: %s)", att.FileName, strings.Join(names, ", ")))
		}
	}

	if len(nonCompliant) > 0 {
		return &CheckResult{
			Identifier: "matroska_font_filename_compliance",
			Warning:    "Font attachment filenames do not match internal font names:\n" + strings.Join(nonCompliant, "\n"),
			Passed:     false,
			Severity:   "info",
		}
	}

	return nil
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
