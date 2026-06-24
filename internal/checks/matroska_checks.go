package checks

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/metadata/mediainfo"
	"codeberg.org/upPollo/parsec/internal/ui"
)

var getMediaInfo = mediainfo.Get

var (
	dtsRegex              = regexp.MustCompile(`\bDTS\b`)
	adRegex               = regexp.MustCompile(`\bAD\b`)
	wordSplitRegex        = regexp.MustCompile(`[\s/.,;()]+`)
	assTimeRegex          = regexp.MustCompile(`^\d:\d\d:\d\d\.\d\d$`)
	fontSeparatorReplacer = strings.NewReplacer(" ", "", "-", "", "_", "")
	commonLangNames       = map[string]string{
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

func checkVideoCropping(track matroska.EbmlTrack) *CheckResult {
	if track.Type != "video" {
		return nil
	}

	props := track.Properties

	pixelWidth, pixelHeight := matroska.ParseDimensions(props.PixelDimensions)
	if pixelWidth == 0 || pixelHeight == 0 {
		return nil
	}

	displayWidth, displayHeight := matroska.ParseDimensions(props.DisplayDimensions)
	if displayWidth <= 0 || displayHeight <= 0 {
		return nil
	}

	pixelAR := float64(pixelWidth) / float64(pixelHeight)
	displayAR := float64(displayWidth) / float64(displayHeight)

	// If display AR is wider than pixel AR, but no crop values are set,
	// it might be a "fake" crop or black bars that should be cropped.
	if displayAR > pixelAR+0.01 {
		warning := fmt.Sprintf("resolution-based black bars detected but no MKV crop values set (AR %.2f vs Display AR %.2f)", pixelAR, displayAR)

		return newFailedTrackResult("matroska_video_cropping", "Missing MKV Cropping", "warning", &track, warning)
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

func checkSubtitleFormat(track matroska.EbmlTrack) *CheckResult {
	codec := track.Codec
	if track.Type == "subtitles" && track.Properties.TextSubtitles && !strings.Contains(codec, "SRT") && !strings.Contains(codec, "SubStationAlpha") {
		warning := "text-based but codec is " + codec
		track.Codec = ui.Warning.Render(track.Codec)

		return newFailedTrackResult("matroska_subtitle_format", "Text subtitle track should converted to SRT", "warning", &track, warning)
	}

	return nil
}

func checkSubtitleFonts(track matroska.EbmlTrack, attachmentFonts []matroska.AttachmentFontInfo, allUsedFonts map[fontStyle]bool) *CheckResult {
	if !isASSSubtitles(track) {
		return nil
	}

	privateBytes, err := track.Properties.DecodeCodecPrivate()
	if err != nil || len(privateBytes) == 0 {
		return nil
	}

	usedFonts := make(map[fontStyle]bool)
	lines := strings.Split(string(privateBytes), "\n")
	parseFontsFromStyles(lines, usedFonts)

	if len(usedFonts) == 0 {
		return nil
	}

	for font := range usedFonts {
		allUsedFonts[font] = true
	}

	missing := findMissingFonts(usedFonts, attachmentFonts)

	if len(missing) > 0 {
		warning := "missing fonts (Styles): " + strings.Join(missing, ", ")

		return newFailedTrackResult("matroska_subtitle_fonts", "SSA/ASS subtitle track uses fonts in Styles not included as attachments", "warning", &track, warning)
	}

	return nil
}

func checkSubtitleInlineFontsWithContent(track matroska.EbmlTrack, attachmentFonts []matroska.AttachmentFontInfo, content []byte, allUsedFonts map[fontStyle]bool) *CheckResult {
	usedFonts := make(map[fontStyle]bool)
	styleConfigs := parseStyleConfigs(content)

	parseFontsFromInlineTagsWithState(content, styleConfigs, usedFonts)

	if len(usedFonts) == 0 {
		return nil
	}

	for font := range usedFonts {
		allUsedFonts[font] = true
	}

	missing := findMissingFonts(usedFonts, attachmentFonts)

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

	for i, line := range lines {
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
				errors = append(errors, fmt.Sprintf("%s (Line %d: Style: %s )", ui.Warning.Render(err), i+1, rest))
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

	for i, line := range lines {
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
				errors = append(errors, fmt.Sprintf("%s (Line %d: Dialogue: %s)", err, i+1, rest))
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

	switch field {
	case "Start", "End":
		if !assTimeRegex.MatchString(val) {
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

type fontStyle struct {
	Family string
	Weight int
	Italic bool
}

type normalizedAttachmentFont struct {
	normalizedPostScript string
	normalizedFamily     string
	italic               bool
	weight               int
	isVariable           bool
}

func hasMatchingAttachmentNorm(font fontStyle, normalizedFamily string, normAtts []normalizedAttachmentFont) bool {
	for _, att := range normAtts {
		if att.normalizedPostScript == normalizedFamily {
			return true
		}

		if att.normalizedFamily == normalizedFamily && att.italic == font.Italic {
			if att.isVariable || matchWeight(att.weight, font.Weight) {
				return true
			}
		}
	}

	return false
}

func formatMissingFontDesc(font fontStyle) string {
	styleDesc := ""

	switch {
	case font.Weight > 400 && font.Italic:
		styleDesc = " Bold Italic"
	case font.Weight > 400:
		styleDesc = " Bold"
	case font.Italic:
		styleDesc = " Italic"
	case font.Weight < 400:
		styleDesc = fmt.Sprintf(" Weight %d", font.Weight)
	}

	return fmt.Sprintf("%s%s", font.Family, styleDesc)
}

// findMissingFonts checks if each used font has a matching attachment using robust internal name mapping.
func findMissingFonts(usedFonts map[fontStyle]bool, attachmentFonts []matroska.AttachmentFontInfo) []string {
	var missing []string

	normAtts := make([]normalizedAttachmentFont, len(attachmentFonts))
	for i, att := range attachmentFonts {
		normAtts[i] = normalizedAttachmentFont{
			normalizedPostScript: normalizeFontName(att.PostScriptName),
			normalizedFamily:     normalizeFontName(att.FamilyName),
			italic:               att.Italic,
			weight:               att.Weight,
			isVariable:           att.IsVariable,
		}
	}

	for font := range usedFonts {
		normalizedFamily := normalizeFontName(font.Family)
		if !hasMatchingAttachmentNorm(font, normalizedFamily, normAtts) {
			missing = append(missing, formatMissingFontDesc(font))
		}
	}

	slices.Sort(missing)

	return missing
}

func matchWeight(attWeight, requestedWeight int) bool {
	if requestedWeight >= 600 {
		return attWeight >= 600
	}

	if requestedWeight <= 300 {
		return attWeight <= 300
	}

	return attWeight > 300 && attWeight < 600
}

func normalizeFontName(name string) string {
	// Remove common separators and convert to lowercase for robust matching
	return strings.ToLower(fontSeparatorReplacer.Replace(name))
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

type normalizedUsedFont struct {
	family string
	italic bool
	weight int
}

type normalizedAttachmentFontID struct {
	attachmentID         int
	normalizedPostScript string
	normalizedFamily     string
	italic               bool
	weight               int
	isVariable           bool
}

func isAttachmentUsedNorm(attID int, normAtts []normalizedAttachmentFontID, normalizedUsed []normalizedUsedFont) bool {
	for _, font := range normalizedUsed {
		for _, fInfo := range normAtts {
			if fInfo.attachmentID == attID {
				if fInfo.normalizedPostScript == font.family {
					return true
				}

				if fInfo.normalizedFamily == font.family && fInfo.italic == font.italic {
					if fInfo.isVariable || matchWeight(fInfo.weight, font.weight) {
						return true
					}
				}
			}
		}
	}

	return false
}

func checkUnusedFonts(attachments []matroska.EbmlAttachment, attachmentFonts []matroska.AttachmentFontInfo, allUsedFonts map[fontStyle]bool) *CheckResult {
	if len(attachments) == 0 {
		return nil
	}

	var unused []string

	normalizedUsed := make([]normalizedUsedFont, 0, len(allUsedFonts))
	for font := range allUsedFonts {
		normalizedUsed = append(normalizedUsed, normalizedUsedFont{
			family: normalizeFontName(font.Family),
			italic: font.Italic,
			weight: font.Weight,
		})
	}

	normAtts := make([]normalizedAttachmentFontID, len(attachmentFonts))
	for i, att := range attachmentFonts {
		normAtts[i] = normalizedAttachmentFontID{
			attachmentID:         att.AttachmentID,
			normalizedPostScript: normalizeFontName(att.PostScriptName),
			normalizedFamily:     normalizeFontName(att.FamilyName),
			italic:               att.Italic,
			weight:               att.Weight,
			isVariable:           att.IsVariable,
		}
	}

	for _, att := range attachments {
		if isFontAttachment(att) && !isAttachmentUsedNorm(att.ID, normAtts, normalizedUsed) {
			unused = append(unused, att.FileName)
		}
	}

	if len(unused) > 0 {
		warning := "Font attachments not used by any subtitle track"
		if !config.IsCheckEnabled("matroska_subtitle_inline_fonts") {
			warning += " (some might be used by inline styles since matroska_subtitle_inline_fonts is disabled)"
		}

		warning += ": " + strings.Join(unused, ", ")

		return &CheckResult{
			Identifier: "matroska_unused_fonts",
			Warning:    warning,
			Passed:     false,
			Severity:   "warning",
		}
	}

	return nil
}

func getAttachmentFontNames(attID int, attachmentFonts []matroska.AttachmentFontInfo) []string {
	var names []string

	for _, fInfo := range attachmentFonts {
		if fInfo.AttachmentID == attID {
			names = append(names, fInfo.FamilyName, fInfo.PostScriptName)
			names = append(names, fInfo.FullNames...)
		}
	}

	return uniqueStrings(names)
}

func getProposedFontFilename(attFileName string, attID int, attachmentFonts []matroska.AttachmentFontInfo) string {
	var ext string

	if idx := strings.LastIndex(attFileName, "."); idx != -1 {
		ext = attFileName[idx:]
	}

	for _, fInfo := range attachmentFonts {
		if fInfo.AttachmentID == attID {
			if fInfo.PostScriptName != "" {
				return fInfo.PostScriptName + ext
			}

			if len(fInfo.FullNames) > 0 && fInfo.FullNames[0] != "" {
				return fInfo.FullNames[0] + ext
			}

			if fInfo.FamilyName != "" {
				return fInfo.FamilyName + ext
			}
		}
	}

	return ""
}

func isAttachmentNameCompliant(attFileName string, names []string) bool {
	baseName := attFileName

	if idx := strings.LastIndex(baseName, "."); idx != -1 {
		baseName = baseName[:idx]
	}

	normalizedFileName := normalizeFontName(baseName)

	for _, internalName := range names {
		if normalizeFontName(internalName) == normalizedFileName {
			return true
		}
	}

	return false
}

func checkFontFilenameCompliance(attachments []matroska.EbmlAttachment, attachmentFonts []matroska.AttachmentFontInfo) *CheckResult {
	type complianceRow struct {
		current  string
		proposed string
		internal string
	}

	var nonCompliant []complianceRow

	for _, att := range attachments {
		if !isFontAttachment(att) {
			continue
		}

		names := getAttachmentFontNames(att.ID, attachmentFonts)
		if len(names) == 0 {
			continue
		}

		if !isAttachmentNameCompliant(att.FileName, names) {
			proposed := getProposedFontFilename(att.FileName, att.ID, attachmentFonts)
			if proposed == "" {
				proposed = "-"
			}

			nonCompliant = append(nonCompliant, complianceRow{
				current:  att.FileName,
				proposed: proposed,
				internal: strings.Join(names, ", "),
			})
		}
	}

	if len(nonCompliant) > 0 {
		headers := []string{"Current Filename", "Internal Fonts", "Proposed Filename"}
		rows := make([][]string, 0, len(nonCompliant))

		for _, row := range nonCompliant {
			rows = append(rows, []string{row.current, row.internal, row.proposed})
		}

		tableStr := ui.FontComplianceTable(headers, rows)

		return &CheckResult{
			Identifier: "matroska_font_filename_compliance",
			Warning:    "Font attachment filenames do not match internal font names:\n" + tableStr,
			Passed:     false,
			Severity:   "info",
		}
	}

	return nil
}

func parseSingleStyleLine(line string, formatFields []string) (string, fontStyle, bool) {
	rest, ok := strings.CutPrefix(line, "Style:")
	if !ok {
		return "", fontStyle{}, false
	}

	values := strings.Split(rest, ",")

	var name, family string

	boldVal := "0"
	italicVal := "0"

	for i, field := range formatFields {
		if i < len(values) {
			val := strings.TrimSpace(values[i])

			switch field {
			case "Name":
				name = val
			case "Fontname":
				family = val
			case "Bold":
				boldVal = val
			case "Italic":
				italicVal = val
			}
		}
	}

	if name != "" && family != "" {
		return name, fontStyle{
			Family: family,
			Weight: parseBoldWeight(boldVal),
			Italic: parseItalic(italicVal),
		}, true
	}

	return "", fontStyle{}, false
}

func parseStyleConfigs(codecPrivate []byte) map[string]fontStyle {
	styleMap := make(map[string]fontStyle)
	lines := strings.Split(string(codecPrivate), "\n")

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
		} else if name, config, ok := parseSingleStyleLine(line, formatFields); ok {
			styleMap[name] = config
		}
	}

	return styleMap
}

func parseFontsFromStyles(lines []string, fonts map[fontStyle]bool) {
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

func extractFontFromStyle(rest string, formatFields []string, fonts map[fontStyle]bool) {
	values := strings.Split(rest, ",")

	var family string

	boldVal := "0"
	italicVal := "0"

	for i, field := range formatFields {
		if i < len(values) {
			val := strings.TrimSpace(values[i])

			switch field {
			case "Fontname":
				family = val
			case "Bold":
				boldVal = val
			case "Italic":
				italicVal = val
			}
		}
	}

	if family != "" {
		weight := parseBoldWeight(boldVal)
		isItalic := parseItalic(italicVal)
		fonts[fontStyle{Family: family, Weight: weight, Italic: isItalic}] = true
	}
}

func parseBoldWeight(valStr string) int {
	valStr = strings.TrimSpace(valStr)
	if valStr == "" {
		return 400
	}

	if valStr == "0" {
		return 400
	}

	if valStr == "1" || valStr == "-1" {
		return 700
	}

	var w int
	if _, err := fmt.Sscanf(valStr, "%d", &w); err == nil {
		if w > 0 {
			return w
		}
	}

	return 400
}

func parseItalic(valStr string) bool {
	valStr = strings.TrimSpace(valStr)

	return valStr == "1" || valStr == "-1"
}

func parseFontsFromInlineTagsWithState(content []byte, styleConfigs map[string]fontStyle, usedFonts map[fontStyle]bool) {
	lines := strings.Split(string(content), "\n")
	eventFormatFields := []string{}
	inEvents := false

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
			eventFormatFields = parseStyleFormat(rest)
		} else if rest, ok := strings.CutPrefix(line, "Dialogue:"); ok {
			parseDialogueLine(rest, eventFormatFields, styleConfigs, usedFonts)
		}
	}
}

func parseDialogueLine(rest string, formatFields []string, styleConfigs map[string]fontStyle, usedFonts map[fontStyle]bool) {
	values := splitDialogueEventLine(rest, len(formatFields))

	var styleName, textVal string

	for i, field := range formatFields {
		if i < len(values) {
			val := strings.TrimSpace(values[i])

			switch field {
			case "Style":
				styleName = val
			case "Text":
				textVal = values[i]
			}
		}
	}

	initialStyle, ok := styleConfigs[styleName]
	if !ok {
		initialStyle = fontStyle{Family: "Arial", Weight: 400, Italic: false}
	}

	parseInlineTagsAndText(textVal, initialStyle, styleConfigs, usedFonts)
}

// splitDialogueEventLine is a helper to split dialogue fields by comma.
func splitDialogueEventLine(line string, fieldCount int) []string {
	if fieldCount <= 1 {
		return []string{line}
	}

	return strings.SplitN(line, ",", fieldCount)
}

func parseInlineTagsAndText(text string, initialStyle fontStyle, styleConfigs map[string]fontStyle, usedFonts map[fontStyle]bool) {
	active := initialStyle
	inTag := false
	tagStartIndex := -1
	hasTextRunContent := false

	for i := 0; i < len(text); i++ {
		c := text[i]
		switch c {
		case '{':
			inTag = true

			if hasTextRunContent {
				usedFonts[active] = true
				hasTextRunContent = false
			}

			tagStartIndex = i + 1
		case '}':
			inTag, active = handleTagEnd(inTag, tagStartIndex, text[:i], active, initialStyle, styleConfigs)
		default:
			if !inTag && !isWhitespace(c) {
				hasTextRunContent = true
			}
		}
	}

	if hasTextRunContent {
		usedFonts[active] = true
	}
}

func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

func handleTagEnd(inTag bool, tagStartIndex int, textPrefix string, active, initialStyle fontStyle, styleConfigs map[string]fontStyle) (bool, fontStyle) {
	if inTag && tagStartIndex >= 0 {
		parseTagsBlock(textPrefix[tagStartIndex:], &active, initialStyle, styleConfigs)

		return false, active
	}

	return inTag, active
}

func parseTagsBlock(tagContent string, active *fontStyle, lineStyle fontStyle, styleConfigs map[string]fontStyle) {
	parts := strings.SplitSeq(tagContent, "\\")
	for part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		switch {
		case strings.HasPrefix(part, "fn"):
			name := strings.TrimSpace(part[2:])
			if name != "" {
				active.Family = name
			}
		case strings.HasPrefix(part, "b"):
			val := strings.TrimSpace(part[1:])
			active.Weight = parseBoldWeight(val)
		case strings.HasPrefix(part, "i"):
			val := strings.TrimSpace(part[1:])
			active.Italic = parseItalic(val)
		case strings.HasPrefix(part, "r"):
			styleName := strings.TrimSpace(part[1:])
			if styleName != "" {
				if rStyle, ok := styleConfigs[styleName]; ok {
					*active = rStyle
				} else {
					*active = lineStyle
				}
			} else {
				*active = lineStyle
			}
		}
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

func checkTitleHygiene(ebml *matroska.EbmlMetadata, meta *metadata.Metadata) *CheckResult {
	title := ebml.Container.Properties.Title
	if title == "" {
		return nil
	}

	officialTitle := meta.Title
	if officialTitle != "" {
		if normalizeForComparison(title) == normalizeForComparison(officialTitle) {
			return nil
		}
	}

	junkPatterns := []string{
		`\[.*\]`, // Bracketed info
		`\(.*\)`, // Parenthesized info
		`\b1080p\b`, `\b720p\b`, `\b2160p\b`,
		`\bWEB-DL\b`, `\bBlu-ray\b`, `\bBD\b`,
		`\bx264\b`, `\bx265\b`, `\bHEVC\b`,
	}

	for _, p := range junkPatterns {
		re := regexp.MustCompile("(?i)" + p)
		if re.MatchString(title) {
			return &CheckResult{
				Identifier: "matroska_title_hygiene",
				Warning:    "Global Title contains technical metadata",
				Passed:     false,
				Severity:   "warning",
				Actual:     title,
			}
		}
	}

	return nil
}

func checkAppHygiene(ebml *matroska.EbmlMetadata) *CheckResult {
	app := ebml.Container.Properties.WritingApplication
	if app == "" {
		return nil
	}

	junkPatterns := []string{
		`[a-zA-Z]:\\`,            // Windows paths
		`/(home|Users|var|tmp)/`, // Unix paths
		`\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`, // UUID
	}

	for _, p := range junkPatterns {
		re := regexp.MustCompile("(?i)" + p)
		if re.MatchString(app) {
			return &CheckResult{
				Identifier: "matroska_app_hygiene",
				Warning:    "Writing Application metadata contains potentially identifiable information",
				Passed:     false,
				Severity:   "warning",
				Actual:     app,
			}
		}
	}

	return nil
}

// checkTrueHDCompatibility checks if a Dolby TrueHD audio track is followed by a lossy compatibility track (AC3/EAC3) in the same language.
func checkTrueHDCompatibility(tracks []matroska.EbmlTrack) *CheckResult {
	res := &CheckResult{
		Identifier: "matroska_truehd_compatibility",
		Warning:    "TrueHD track is not followed by a lossy compatibility track",
		Passed:     true,
	}

	for i := range tracks {
		track := &tracks[i]
		if track.Type != "audio" {
			continue
		}

		if !strings.Contains(strings.ToUpper(track.Codec), "A_TRUEHD") {
			continue
		}

		// TrueHD track found. Check next track.
		if i+1 >= len(tracks) {
			res.Passed = false
			res.Severity = "warning"
			res.Tracks = append(res.Tracks, ebmlTrackToResult(track, false, "TrueHD track is the last track and has no lossy compatibility track"))

			continue
		}

		nextTrack := &tracks[i+1]
		if nextTrack.Type != "audio" {
			res.Passed = false
			res.Severity = "warning"
			res.Tracks = append(res.Tracks, ebmlTrackToResult(track, false, "TrueHD track is followed by a non-audio track of type "+nextTrack.Type))

			continue
		}

		if nextTrack.Properties.Language != track.Properties.Language {
			res.Passed = false
			res.Severity = "warning"
			res.Tracks = append(res.Tracks, ebmlTrackToResult(track, false, fmt.Sprintf("TrueHD track is followed by a track with different language: %s (expected %s)", nextTrack.Properties.Language, track.Properties.Language)))

			continue
		}

		nextCodec := strings.ToUpper(nextTrack.Codec)
		if !strings.Contains(nextCodec, "A_AC3") && !strings.Contains(nextCodec, "A_EAC3") {
			res.Passed = false
			res.Severity = "warning"
			res.Tracks = append(res.Tracks, ebmlTrackToResult(track, false, fmt.Sprintf("TrueHD track is followed by an incompatible codec: %s (expected AC3 or EAC3)", nextTrack.Codec)))

			continue
		}
	}

	if !res.Passed {
		return res
	}

	return nil
}

func formatNsToTime(ns int64) string {
	ms := ns / 1000000
	hours := ms / 3600000
	ms %= 3600000
	minutes := ms / 60000
	ms %= 60000
	seconds := ms / 1000
	ms %= 1000

	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, ms)
}

func checkChaptersStartNonZero(chapters *matroska.Chapters) *CheckResult {
	if chapters == nil || len(chapters.Atoms) == 0 {
		return nil
	}

	firstChapter := chapters.Atoms[0]
	if firstChapter.TimeStart != 0 {
		timeStr := formatNsToTime(firstChapter.TimeStart)

		return &CheckResult{
			Identifier: "matroska_chapters_start_non_zero",
			Warning:    "First chapter does not start at 00:00:00",
			Passed:     false,
			Severity:   "warning",
			Actual:     timeStr,
			Expected:   "00:00:00.000",
		}
	}

	return nil
}

func checkChaptersNonMonotonic(chapters *matroska.Chapters) *CheckResult {
	if chapters == nil || len(chapters.Atoms) == 0 {
		return nil
	}

	var lastTime int64 = -1
	for _, ch := range chapters.Atoms {
		if lastTime >= 0 && ch.TimeStart < lastTime {
			return &CheckResult{
				Identifier: "matroska_chapters_non_monotonic",
				Warning:    "Chapter times are not strictly increasing",
				Passed:     false,
				Severity:   "error",
				Actual:     fmt.Sprintf("chapter starts at %s after %s", formatNsToTime(ch.TimeStart), formatNsToTime(lastTime)),
			}
		}

		lastTime = ch.TimeStart
	}

	return nil
}

func checkChaptersDuplicate(chapters *matroska.Chapters) *CheckResult {
	if chapters == nil || len(chapters.Atoms) == 0 {
		return nil
	}

	seenTimes := make(map[int64]bool)
	for _, ch := range chapters.Atoms {
		if seenTimes[ch.TimeStart] {
			return &CheckResult{
				Identifier: "matroska_chapters_duplicate",
				Warning:    "Duplicate chapter timestamps found",
				Passed:     false,
				Severity:   "error",
				Actual:     "duplicate timestamp at " + formatNsToTime(ch.TimeStart),
			}
		}

		seenTimes[ch.TimeStart] = true
	}

	return nil
}

func checkChaptersTooClose(chapters *matroska.Chapters) *CheckResult {
	if chapters == nil || len(chapters.Atoms) == 0 {
		return nil
	}

	var lastTime int64 = -1
	for _, ch := range chapters.Atoms {
		if lastTime >= 0 {
			diff := ch.TimeStart - lastTime
			if diff < 10000000000 {
				return &CheckResult{
					Identifier: "matroska_chapters_too_close",
					Warning:    "Chapter interval is too short (< 10 seconds)",
					Passed:     false,
					Severity:   "warning",
					Actual:     fmt.Sprintf("interval is %.1fs between %s and %s", float64(diff)/1000000000.0, formatNsToTime(lastTime), formatNsToTime(ch.TimeStart)),
				}
			}
		}

		lastTime = ch.TimeStart
	}

	return nil
}

func checkChaptersExceedDuration(ebml *matroska.EbmlMetadata, chapters *matroska.Chapters) *CheckResult {
	if chapters == nil || len(chapters.Atoms) == 0 {
		return nil
	}

	duration := ebml.Container.Properties.Duration
	if duration <= 0 {
		return nil
	}

	for _, ch := range chapters.Atoms {
		if ch.TimeStart > duration {
			return &CheckResult{
				Identifier: "matroska_chapters_exceed_duration",
				Warning:    "Chapter timestamp exceeds video duration",
				Passed:     false,
				Severity:   "error",
				Actual:     fmt.Sprintf("chapter at %s (video duration is %s)", formatNsToTime(ch.TimeStart), formatNsToTime(duration)),
			}
		}
	}

	return nil
}

func checkSingleChapterNameHygiene(ch matroska.EbmlChapterAtom) (bool, []string) {
	hasNonEmpty := false

	var currentNames []string

	for _, display := range ch.Display {
		name := strings.TrimSpace(display.String)
		if name != "" {
			hasNonEmpty = true

			currentNames = append(currentNames, name)
		}
	}

	return hasNonEmpty, currentNames
}

func checkConsecutiveDuplicateNames(currentNames, lastNames []string, timeStart int64) *CheckResult {
	for _, currentName := range currentNames {
		for _, lastName := range lastNames {
			if strings.EqualFold(currentName, lastName) {
				return &CheckResult{
					Identifier: "matroska_chapters_name_hygiene",
					Warning:    "Consecutive duplicate chapter names found",
					Passed:     false,
					Severity:   "warning",
					Actual:     fmt.Sprintf("consecutive chapters have name %q at %s", currentName, formatNsToTime(timeStart)),
				}
			}
		}
	}

	return nil
}

func checkChaptersNameHygiene(chapters *matroska.Chapters) *CheckResult {
	if chapters == nil || len(chapters.Atoms) == 0 {
		return nil
	}

	var lastNames []string

	for i, ch := range chapters.Atoms {
		if len(ch.Display) == 0 {
			return &CheckResult{
				Identifier: "matroska_chapters_name_hygiene",
				Warning:    "Chapter has no display name entry",
				Passed:     false,
				Severity:   "warning",
				Actual:     fmt.Sprintf("chapter %d (starts at %s)", i+1, formatNsToTime(ch.TimeStart)),
			}
		}

		hasNonEmpty, currentNames := checkSingleChapterNameHygiene(ch)

		if !hasNonEmpty {
			return &CheckResult{
				Identifier: "matroska_chapters_name_hygiene",
				Warning:    "Chapter display name is empty or only whitespace",
				Passed:     false,
				Severity:   "warning",
				Actual:     fmt.Sprintf("chapter %d (starts at %s)", i+1, formatNsToTime(ch.TimeStart)),
			}
		}

		if i > 0 {
			if res := checkConsecutiveDuplicateNames(currentNames, lastNames, ch.TimeStart); res != nil {
				return res
			}
		}

		if len(currentNames) > 0 {
			lastNames = currentNames
		}
	}

	return nil
}

func getChapterLanguages(ch matroska.EbmlChapterAtom) (map[string]bool, *CheckResult) {
	currentLangs := make(map[string]bool)

	for _, display := range ch.Display {
		lang := strings.TrimSpace(display.Language)
		if lang == "" || strings.EqualFold(lang, "und") {
			chapterName := display.String
			if chapterName == "" {
				chapterName = "(no name)"
			}

			return nil, &CheckResult{
				Identifier: "matroska_chapters_language_hygiene",
				Warning:    "Chapter display entry has undetermined or missing language",
				Passed:     false,
				Severity:   "warning",
				Actual:     fmt.Sprintf("chapter %q (starts at %s) language is %q", chapterName, formatNsToTime(ch.TimeStart), lang),
			}
		}

		currentLangs[strings.ToLower(lang)] = true
	}

	return currentLangs, nil
}

func checkLanguagesInconsistent(firstLangs, currentLangs map[string]bool, timeStart int64) *CheckResult {
	if len(currentLangs) == 0 || len(firstLangs) == 0 {
		return nil
	}

	matches := true
	if len(currentLangs) != len(firstLangs) {
		matches = false
	} else {
		for l := range currentLangs {
			if !firstLangs[l] {
				matches = false

				break
			}
		}
	}

	if !matches {
		var list1, list2 []string
		for l := range firstLangs {
			list1 = append(list1, l)
		}

		for l := range currentLangs {
			list2 = append(list2, l)
		}

		return &CheckResult{
			Identifier: "matroska_chapters_language_hygiene",
			Warning:    "Inconsistent chapter languages in edition",
			Passed:     false,
			Severity:   "warning",
			Actual:     fmt.Sprintf("languages %v vs %v at chapter starting at %s", list1, list2, formatNsToTime(timeStart)),
		}
	}

	return nil
}

func checkChaptersLanguageHygiene(chapters *matroska.Chapters) *CheckResult {
	if chapters == nil || len(chapters.Atoms) == 0 {
		return nil
	}

	var firstLangs map[string]bool

	for _, ch := range chapters.Atoms {
		currentLangs, res := getChapterLanguages(ch)
		if res != nil {
			return res
		}

		if firstLangs == nil {
			if len(currentLangs) > 0 {
				firstLangs = currentLangs
			}
		} else {
			if res := checkLanguagesInconsistent(firstLangs, currentLangs, ch.TimeStart); res != nil {
				return res
			}
		}
	}

	return nil
}

func isAligned(timeStart int64, keyframes []int64) (bool, int64) {
	closestDiff := int64(-1)

	for _, kf := range keyframes {
		var (
			currentDiff int64
			inRange     bool
		)

		diff := timeStart - kf

		if diff >= 0 {
			currentDiff = diff
			inRange = diff <= 8_000_000
		} else {
			currentDiff = -diff
			inRange = (-diff) <= 1_000_000 // Allow up to 1ms negative tolerance for rounding errors
		}

		if closestDiff == -1 || currentDiff < closestDiff {
			closestDiff = currentDiff
		}

		if inRange {
			return true, closestDiff
		}
	}

	return false, closestDiff
}

func getVideoTrackNumberFromEBML(ebml *matroska.EbmlMetadata) uint64 {
	for _, track := range ebml.Tracks {
		if track.Type == "video" {
			return uint64(track.Properties.Number)
		}
	}

	return 0
}

func checkChaptersKeyframeAlignment(filePath string, ebml *matroska.EbmlMetadata, chapters *matroska.Chapters) *CheckResult {
	if filePath == "" {
		return nil
	}

	videoTrackNum := getVideoTrackNumberFromEBML(ebml)
	if videoTrackNum == 0 {
		return nil
	}

	keyframes, err := matroska.ReadKeyframeTimestamps(filePath, videoTrackNum, ebml.Container.Properties.TimestampScale)
	if err != nil {
		return &CheckResult{
			Identifier: "matroska_chapters_keyframe_alignment",
			Warning:    "Failed to read video cues index (SeekHead/Cues may be missing or invalid)",
			Passed:     false,
			Severity:   "warning",
			Actual:     err.Error(),
		}
	}

	if len(keyframes) == 0 {
		return &CheckResult{
			Identifier: "matroska_chapters_keyframe_alignment",
			Warning:    "No video cues/index entries found (seeking might be slow or broken)",
			Passed:     false,
			Severity:   "warning",
			Actual:     "zero cues indexed for video track",
		}
	}

	if chapters == nil || len(chapters.Atoms) == 0 {
		return nil
	}

	var nonAligned []string

	for i, ch := range chapters.Atoms {
		if aligned, diff := isAligned(ch.TimeStart, keyframes); !aligned {
			nonAligned = append(nonAligned, fmt.Sprintf(
				"chapter %d at %s (nearest keyframe is off by %.3fs)",
				i+1,
				formatNsToTime(ch.TimeStart),
				float64(diff)/1e9,
			))
		}
	}

	if len(nonAligned) > 0 {
		return &CheckResult{
			Identifier: "matroska_chapters_keyframe_alignment",
			Warning:    "Chapters are not aligned with video keyframes",
			Passed:     false,
			Severity:   "warning",
			Actual:     strings.Join(nonAligned, "; "),
		}
	}

	return nil
}

func checkCommentaryChannels(tracks []matroska.EbmlTrack) *CheckResult {
	res := &CheckResult{
		Identifier: "matroska_commentary_channels",
		Warning:    "Commentary audio track has more than 2 channels",
		Passed:     true,
	}

	for i := range tracks {
		track := &tracks[i]

		if track.Type == "audio" && track.Properties.Commentary {
			if track.Properties.AudioChannels > 2 {
				res.Passed = false
				res.Severity = "warning"
				res.Tracks = append(res.Tracks, ebmlTrackToResult(track, false, fmt.Sprintf("Commentary track has %d channels (expected <= 2)", track.Properties.AudioChannels)))
			}
		}
	}

	if !res.Passed {
		return res
	}

	return nil
}

func isLosslessCodec(miTrack *mediainfo.Track) bool {
	codec := metadata.AudioCodecName(miTrack.Format, miTrack.FormatProfile, miTrack.FormatAdditionalFeatures)
	c := strings.ToUpper(codec)

	return strings.Contains(c, "TRUEHD") || strings.Contains(c, "DTS-HD MA") || strings.Contains(c, "FLAC") || strings.Contains(c, "PCM") || strings.Contains(c, "ALAC")
}

func checkCommentaryBitrate(filePath string, tracks []matroska.EbmlTrack) *CheckResult {
	isRemux := strings.Contains(strings.ToUpper(filepath.Base(filePath)), "REMUX")
	if isRemux {
		return nil
	}

	mi, err := getMediaInfo(filePath)
	if err != nil {
		return nil
	}

	miAudioTracks := make(map[string]*mediainfo.Track)

	for i := range mi.Media.Tracks {
		t := &mi.Media.Tracks[i]

		if t.Type == "Audio" {
			miAudioTracks[t.ID] = t
		}
	}

	res := &CheckResult{
		Identifier: "matroska_commentary_bitrate",
		Warning:    "Commentary audio track bitrate exceeds 128 kbps",
		Passed:     true,
	}

	for i := range tracks {
		if tr := verifyCommentaryTrackBitrate(&tracks[i], miAudioTracks); tr != nil {
			res.Passed = false
			res.Severity = "warning"
			res.Tracks = append(res.Tracks, *tr)
		}
	}

	if !res.Passed {
		return res
	}

	return nil
}

func verifyCommentaryTrackBitrate(track *matroska.EbmlTrack, miAudioTracks map[string]*mediainfo.Track) *TrackCheckResult {
	if track.Type != "audio" || !track.Properties.Commentary {
		return nil
	}

	idStr := strconv.Itoa(track.Properties.Number)

	miTrack, ok := miAudioTracks[idStr]
	if !ok {
		return nil
	}

	if isLosslessCodec(miTrack) {
		return nil
	}

	if miTrack.BitRate > 0 && miTrack.BitRate > 128000 {
		bitrateKbps := float64(miTrack.BitRate) / 1000.0
		tr := ebmlTrackToResult(track, false, fmt.Sprintf("Commentary track bitrate is %.1f kbps (expected <= 128 kbps)", bitrateKbps))

		return &tr
	}

	return nil
}

var commentaryPrefixRegex = regexp.MustCompile(`^(?:.*/\s*)?(?:Commentary by|Isolated score with commentary by)\b`)

func checkCommentaryPrefix(tracks []matroska.EbmlTrack) *CheckResult {
	res := &CheckResult{
		Identifier: "matroska_commentary_prefix",
		Warning:    "Commentary track name does not start with a standard prefix",
		Passed:     true,
	}

	for i := range tracks {
		track := &tracks[i]

		if track.Properties.Commentary {
			name := track.Properties.Name
			if name == "" {
				res.Passed = false
				res.Severity = "warning"
				res.Tracks = append(res.Tracks, ebmlTrackToResult(track, false, "Commentary track has no name"))

				continue
			}

			if !commentaryPrefixRegex.MatchString(name) {
				res.Passed = false
				res.Severity = "warning"
				res.Tracks = append(res.Tracks, ebmlTrackToResult(track, false, fmt.Sprintf("Track name %q does not start with standard prefix (e.g., \"Commentary by ...\")", name)))
			}
		}
	}

	if !res.Passed {
		return res
	}

	return nil
}

func extractCoreCommentaryName(name string) string {
	lowerName := strings.ToLower(name)
	if idx := strings.Index(lowerName, "commentary by"); idx != -1 {
		name = name[idx:]
	} else if idx := strings.Index(lowerName, "isolated score"); idx != -1 {
		name = name[idx:]
	} else {
		if idx := strings.Index(name, "/"); idx != -1 {
			name = name[idx+1:]
		}
	}

	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "(SDH)", "")
	name = strings.ReplaceAll(name, "[SDH]", "")
	name = strings.ReplaceAll(name, "SDH", "")

	return strings.ToLower(strings.TrimSpace(name))
}

func checkCommentaryPairing(tracks []matroska.EbmlTrack) *CheckResult {
	res := &CheckResult{
		Identifier: "matroska_commentary_pairing",
		Warning:    "Commentary subtitle track name does not match any audio commentary track name",
		Passed:     true,
	}

	var audioCommentaries []string

	for i := range tracks {
		track := &tracks[i]

		if track.Type == "audio" && track.Properties.Commentary {
			core := extractCoreCommentaryName(track.Properties.Name)
			audioCommentaries = append(audioCommentaries, core)
		}
	}

	for i := range tracks {
		track := &tracks[i]

		if track.Type == "subtitles" && track.Properties.Commentary {
			core := extractCoreCommentaryName(track.Properties.Name)

			if !slices.Contains(audioCommentaries, core) {
				res.Passed = false
				res.Severity = "warning"
				res.Tracks = append(res.Tracks, ebmlTrackToResult(track, false, fmt.Sprintf("Commentary subtitle %q does not match any audio commentary track", track.Properties.Name)))
			}
		}
	}

	if !res.Passed {
		return res
	}

	return nil
}
