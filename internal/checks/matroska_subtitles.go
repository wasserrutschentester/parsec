package checks

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/types"
	"codeberg.org/upPollo/parsec/internal/ui"
)

var (
	assTimeRegex          = regexp.MustCompile(`^\d:\d\d:\d\d\.\d\d$`)
	fontSeparatorReplacer = strings.NewReplacer(" ", "", "-", "", "_", "")
)

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
		res := newFailedTrackResult("matroska_ass_script_info", "ASS Script Info missing recommended headers", "info", &track, "")
		res.Tracks[0].List = errors

		return res
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
	rows := validateStyles(lines)

	if len(rows) > 0 {
		res := newFailedTrackResult("matroska_ass_styles", "ASS Style validation failed", "warning", &track, "See table below")
		res.Tracks[0].Table = &types.TableData{
			Headers: []string{"Line #", "Style Name", "Validation Issue"},
			Rows:    rows,
		}

		return res
	}

	return nil
}

func getStyleName(rest string, formatFields []string) string {
	values := strings.Split(rest, ",")
	for i, field := range formatFields {
		if strings.ToLower(strings.TrimSpace(field)) == "name" && i < len(values) {
			return strings.TrimSpace(values[i])
		}
	}

	return "-"
}

func validateStyles(lines []string) [][]string {
	inStyles := false
	formatFields := []string{}

	var rows [][]string

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
			styleName := getStyleName(rest, formatFields)
			for _, err := range validateStyleLine(rest, formatFields) {
				rows = append(rows, []string{
					strconv.Itoa(i + 1),
					styleName,
					err,
				})
			}
		}
	}

	return rows
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
		res := newFailedTrackResult("matroska_ass_events", "ASS Event validation failed", "warning", &track, "")
		res.Tracks[0].List = errors

		return res
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

func isSRTSubtitles(track matroska.EbmlTrack) bool {
	return track.Type == "subtitles" && strings.Contains(track.Codec, "SRT")
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

func getUnusedFontsTableRows(unused []matroska.EbmlAttachment, attachmentFonts []matroska.AttachmentFontInfo) [][]string {
	rows := make([][]string, 0, len(unused))

	for _, u := range unused {
		fontName := "-"

		for _, fInfo := range attachmentFonts {
			if fInfo.AttachmentID == u.ID {
				if len(fInfo.FullNames) > 0 {
					fontName = strings.Join(fInfo.FullNames, ", ")
				} else if fInfo.FamilyName != "" {
					fontName = fInfo.FamilyName
				}

				break
			}
		}

		sizeStr := ""

		const unit = 1024

		switch {
		case u.Size < unit:
			sizeStr = fmt.Sprintf("%d B", u.Size)
		case u.Size < unit*unit:
			sizeStr = fmt.Sprintf("%.1f KB", float64(u.Size)/float64(unit))
		default:
			sizeStr = fmt.Sprintf("%.1f MB", float64(u.Size)/float64(unit*unit))
		}

		rows = append(rows, []string{u.FileName, fontName, sizeStr})
	}

	return rows
}

func checkUnusedFonts(attachments []matroska.EbmlAttachment, attachmentFonts []matroska.AttachmentFontInfo, allUsedFonts map[fontStyle]bool) *CheckResult {
	if len(attachments) == 0 {
		return nil
	}

	var unused []matroska.EbmlAttachment

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
			unused = append(unused, att)
		}
	}

	if len(unused) > 0 {
		warning := "Font attachments not used by any subtitle track"
		if !config.IsCheckEnabled("matroska_subtitle_inline_fonts") {
			warning += " (some might be used by inline styles since matroska_subtitle_inline_fonts is disabled)"
		}

		return &CheckResult{
			Identifier: "matroska_unused_fonts",
			Warning:    warning,
			Passed:     false,
			Severity:   "warning",
			Table: &types.TableData{
				Headers: []string{"Attachment Name", "Full Name", "Size"},
				Rows:    getUnusedFontsTableRows(unused, attachmentFonts),
			},
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

func cleanFallbackFontName(fullName string, familyName string) string {
	if familyName != "" && fullName != familyName && strings.HasPrefix(strings.ToLower(fullName), strings.ToLower(familyName)) {
		// e.g. fullName: "Times New Roman Bold", familyName: "Times New Roman"
		style := fullName[len(familyName):]
		style = strings.TrimSpace(style)

		familyClean := strings.ReplaceAll(familyName, " ", "")
		styleClean := strings.ReplaceAll(style, " ", "")

		if styleClean != "" {
			return familyClean + "-" + styleClean
		}
	}

	return strings.ReplaceAll(fullName, " ", "")
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
				return cleanFallbackFontName(fInfo.FullNames[0], fInfo.FamilyName) + ext
			}

			if fInfo.FamilyName != "" {
				return cleanFallbackFontName(fInfo.FamilyName, "") + ext
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

type fontComplianceRow struct {
	attID    int
	current  string
	proposed string
}

func findNonCompliantFonts(attachments []matroska.EbmlAttachment, attachmentFonts []matroska.AttachmentFontInfo) []fontComplianceRow {
	var nonCompliant []fontComplianceRow

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

			nonCompliant = append(nonCompliant, fontComplianceRow{
				attID:    att.ID,
				current:  att.FileName,
				proposed: proposed,
			})
		}
	}

	return nonCompliant
}

func buildFontComplianceResult(nonCompliant []fontComplianceRow, attachmentFonts []matroska.AttachmentFontInfo) *CheckResult {
	headers := []string{"Attachment Name", "Full Name", "PostScript Name", "Proposed Name"}

	var rows [][]string

	for _, row := range nonCompliant {
		var fInfo *matroska.AttachmentFontInfo

		for _, f := range attachmentFonts {
			if f.AttachmentID == row.attID {
				fCopy := f
				fInfo = &fCopy

				break
			}
		}

		if fInfo != nil {
			psName := fInfo.PostScriptName
			if psName == "" {
				psName = "-"
			}

			fullName := strings.Join(fInfo.FullNames, ", ")
			if fullName == "" {
				fullName = "-"
			}

			rows = append(rows, []string{
				row.current,
				fullName,
				psName,
				row.proposed,
			})
		}
	}

	return &CheckResult{
		Identifier: "matroska_font_filename_compliance",
		Warning:    "Font attachment filenames do not match internal font names",
		Passed:     false,
		Severity:   "info",
		Table: &types.TableData{
			Headers: headers,
			Rows:    rows,
		},
	}
}

func checkFontFilenameCompliance(attachments []matroska.EbmlAttachment, attachmentFonts []matroska.AttachmentFontInfo) *CheckResult {
	nonCompliant := findNonCompliantFonts(attachments, attachmentFonts)
	if len(nonCompliant) == 0 {
		return nil
	}

	return buildFontComplianceResult(nonCompliant, attachmentFonts)
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

func checkSRTValidation(track matroska.EbmlTrack, content []byte) *CheckResult {
	if len(content) == 0 {
		return nil
	}

	contentStr := string(content)
	contentStr = strings.TrimPrefix(contentStr, "\ufeff")
	contentStr = strings.ReplaceAll(contentStr, "\r\n", "\n")
	lines := strings.Split(contentStr, "\n")

	parser := &srtParser{
		timestampRegex: regexp.MustCompile(`^\s*\d{2}:\d{2}:\d{2}[,.]\d{3}\s*-->\s*\d{2}:\d{2}:\d{2}[,.]\d{3}`),
	}

	for i := range lines {
		line := strings.TrimSpace(lines[i])
		parser.feedLine(line, i+1)
	}

	if parser.state == 2 {
		parser.validateCurrentBlock()
	}

	return buildSRTCheckResult(&track, parser)
}

func appendFormattedErrors(msgs []string, prefix string, errs []string) []string {
	for _, err := range limitErrorList(errs) {
		msgs = append(msgs, prefix+err)
	}

	return msgs
}

func buildSRTCheckResult(track *matroska.EbmlTrack, parser *srtParser) *CheckResult {
	if len(parser.syntaxErrors) > 0 || len(parser.tagErrors) > 0 {
		var msgs []string

		if len(parser.syntaxErrors) > 0 {
			msgs = appendFormattedErrors(msgs, "Syntax Error: ", parser.syntaxErrors)
		}

		if len(parser.tagErrors) > 0 {
			msgs = appendFormattedErrors(msgs, "Invalid HTML tag: ", parser.tagErrors)
		}

		if len(parser.posWarnings) > 0 {
			msgs = appendFormattedErrors(msgs, "Alignment/positioning detected: ", parser.posWarnings)
		}

		res := newFailedTrackResult("matroska_srt_validation", "SRT subtitle validation failed", "warning", track, "")
		res.Tracks[0].List = msgs

		return res
	}

	if len(parser.posWarnings) > 0 {
		msgs := appendFormattedErrors(nil, "Alignment/positioning detected: ", parser.posWarnings)

		res := newFailedTrackResult("matroska_srt_validation", "SRT subtitle contains alignment or positioning info (ASS should probably be used instead)", "info", track, "")
		res.Tracks[0].List = msgs

		return res
	}

	return nil
}

type srtParser struct {
	state             int
	blockStartLineNum int
	posWarnings       []string
	syntaxErrors      []string
	tagErrors         []string
	currentBlockLines []string
	timestampRegex    *regexp.Regexp
}

func (p *srtParser) feedLine(line string, lineNum int) {
	if line == "" {
		if p.state == 2 {
			p.validateCurrentBlock()

			p.state = 0
		}

		return
	}

	switch p.state {
	case 0:
		if _, err := strconv.Atoi(line); err == nil {
			p.state = 1
			p.blockStartLineNum = lineNum
		} else if p.timestampRegex.MatchString(line) {
			p.state = 2
			p.blockStartLineNum = lineNum

			p.checkTimestamp(line, lineNum)
		} else {
			p.syntaxErrors = append(p.syntaxErrors, fmt.Sprintf("Line %d: expected sequence number, got %q", lineNum, line))
		}
	case 1:
		if p.timestampRegex.MatchString(line) {
			p.state = 2

			p.checkTimestamp(line, lineNum)
		} else {
			p.syntaxErrors = append(p.syntaxErrors, fmt.Sprintf("Line %d: expected timestamp line, got %q", lineNum, line))

			p.state = 0
		}
	case 2:
		p.currentBlockLines = append(p.currentBlockLines, line)
	}
}

func (p *srtParser) validateCurrentBlock() {
	tagErr, hasPos, posWarn := validateSRTBlock(p.currentBlockLines, p.blockStartLineNum)

	if tagErr != "" {
		p.tagErrors = append(p.tagErrors, tagErr)
	}

	if hasPos {
		p.posWarnings = append(p.posWarnings, posWarn)
	}

	p.currentBlockLines = nil
}

func (p *srtParser) checkTimestamp(line string, lineNum int) {
	parts := strings.Split(line, "-->")
	if len(parts) < 2 {
		return
	}

	endPart := strings.TrimSpace(parts[1])
	endTimestampRegex := regexp.MustCompile(`^\d{2}:\d{2}:\d{2}[,.]\d{3}`)

	loc := endTimestampRegex.FindStringIndex(endPart)
	if loc == nil {
		return
	}

	remaining := strings.TrimSpace(endPart[loc[1]:])
	if remaining != "" {
		p.posWarnings = append(p.posWarnings, fmt.Sprintf("Line %d: contains display coordinates/metadata %q", lineNum, remaining))
	}
}

func validateSRTBlock(currentBlockLines []string, blockStartLineNum int) (tagErr string, hasPos bool, posWarn string) {
	if len(currentBlockLines) == 0 {
		return "", false, ""
	}

	blockText := strings.Join(currentBlockLines, "\n")

	if ok, errMsg := validateSRTText(blockText); !ok {
		tagErr = fmt.Sprintf("Block starting at line %d: %s", blockStartLineNum, errMsg)
	}

	if strings.Contains(blockText, "{\\") {
		hasPos = true
		posWarn = fmt.Sprintf("Block starting at line %d: contains inline styling/positioning tag \"%s\"", blockStartLineNum, extractASSTags(blockText))
	}

	return tagErr, hasPos, posWarn
}

func limitErrorList(errs []string) []string {
	if len(errs) <= 5 {
		return errs
	}

	truncated := make([]string, 0, 6)
	truncated = append(truncated, errs[:5]...)
	truncated = append(truncated, fmt.Sprintf("... and %d more", len(errs)-5))

	return truncated
}

func extractASSTags(line string) string {
	start := strings.Index(line, "{")
	if start == -1 {
		return ""
	}

	end := strings.Index(line[start:], "}")
	if end == -1 {
		return line[start:]
	}

	return line[start : start+end+1]
}

func parseHTMLTagName(text string, startIdx int) (string, int) {
	i := startIdx
	n := len(text)

	for i < n && ((text[i] >= 'a' && text[i] <= 'z') || (text[i] >= 'A' && text[i] <= 'Z') || (text[i] >= '0' && text[i] <= '9')) {
		i++
	}

	return strings.ToLower(text[startIdx:i]), i
}

func parseHTMLTagAttributes(text string, startIdx int) (bool, int) {
	i := startIdx
	n := len(text)
	isSelfClosing := false

	for i < n && text[i] != '>' {
		if text[i] == '/' {
			isSelfClosing = true
		}

		i++
	}

	return isSelfClosing, i
}

func parseHTMLTag(text string, startIdx int) (string, bool, bool, int, string) {
	i := startIdx + 1 // skip '<'
	n := len(text)
	isClose := false

	if i < n && text[i] == '/' {
		isClose = true
		i++
	}

	tagName, nextIdx := parseHTMLTagName(text, i)
	i = nextIdx

	isSelfClosing, nextIdx := parseHTMLTagAttributes(text, i)
	i = nextIdx

	if i >= n {
		return "", false, false, i, "malformed or unclosed HTML tag starting with '<'"
	}

	i++ // consume '>'

	if tagName == "" {
		return "", false, false, i, "empty HTML-like tag '<>'"
	}

	return tagName, isClose, isSelfClosing, i, ""
}

func processSRTTag(tagName string, isClose bool, stack []string) ([]string, string) {
	if isClose {
		if len(stack) == 0 {
			return nil, fmt.Sprintf("closing tag '</%s>' without opening tag", tagName)
		}

		top := stack[len(stack)-1]
		if top != tagName {
			return nil, fmt.Sprintf("mismatched closing tag '</%s>' (expected '</%s>')", tagName, top)
		}

		return stack[:len(stack)-1], ""
	}

	return append(stack, tagName), ""
}

func isAllowedSRTTag(tag string) bool {
	return tag == "b" || tag == "i" || tag == "u" || tag == "font" || tag == "br"
}

func handleHTMLTag(text string, idx int, stack []string) ([]string, int, bool, string) {
	tagName, isClose, isSelfClosing, nextIdx, errMsg := parseHTMLTag(text, idx)
	if errMsg != "" {
		return nil, nextIdx, false, errMsg
	}

	if !isAllowedSRTTag(tagName) {
		return nil, nextIdx, false, fmt.Sprintf("disallowed HTML-like tag '<%s>' (broken conversion from WebVTT or other format)", tagName)
	}

	if tagName == "br" || isSelfClosing {
		return stack, nextIdx, true, ""
	}

	newStack, err := processSRTTag(tagName, isClose, stack)
	if err != "" {
		return nil, nextIdx, false, err
	}

	return newStack, nextIdx, true, ""
}

func validateSRTText(text string) (bool, string) {
	var stack []string

	i := 0
	n := len(text)

	for i < n {
		if text[i] == '<' {
			newStack, nextIdx, ok, errMsg := handleHTMLTag(text, i, stack)
			if !ok {
				return false, errMsg
			}

			stack = newStack
			i = nextIdx
		} else {
			i++
		}
	}

	if len(stack) > 0 {
		return false, fmt.Sprintf("unclosed HTML tag '%s'", stack[len(stack)-1])
	}

	return true, ""
}
