// Package nfo provides logic for generating and rendering NFO templates.
package nfo

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/viper"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/templateutil"
)

// Render evaluates the given context against the specified template.
func Render(ctx *Context, tmplName string) (string, error) {
	tmpl, err := LoadTemplate(tmplName)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		// Run static validator to provide better syntax/structural context
		if valErr := ValidateTemplate(tmpl); valErr != nil {
			return "", FormatTemplateError(valErr, getConfigDirs())
		}

		return "", FormatTemplateError(fmt.Errorf("failed to execute template: %w", err), getConfigDirs())
	}

	return buf.String(), nil
}

//go:embed templates/*.tmpl templates/partial/*.tmpl
var builtinTemplates embed.FS

var (
	errUnknownTemplate  = errors.New("unknown template; local file not found")
	errDictOddArgs      = errors.New("invalid dict call: must have an even number of arguments")
	errDictKeyNotString = errors.New("dict keys must be strings")
	errTruncateNoInput  = errors.New("truncate requires an input string")
	errNonNumeric       = errors.New("non-numeric argument")
	errDurationNoInput  = errors.New("formatDuration requires a duration in seconds")
	errDurationInvalid  = errors.New("formatDuration: non-integer argument")
	errBitrateNoInput   = errors.New("formatBitrate requires a bitrate input")
)

// LoadTemplate loads a template by name either from builtins or the config directory,
// and automatically loads any partials from nfo/partial/*.tmpl to allow overriding sections.
func LoadTemplate(tmplName string) (*template.Template, error) {
	var tmpl *template.Template

	funcs := templateFuncs()
	funcs["include"] = func(name string, data any) (string, error) {
		var buf bytes.Buffer

		err := tmpl.ExecuteTemplate(&buf, name, data)

		return strings.Trim(buf.String(), "\n\r"), err
	}

	tmpl = template.New("nfo").Funcs(funcs)

	configDirs := getConfigDirs()

	if err := loadBuiltinPartials(tmpl, configDirs); err != nil {
		return nil, err
	}

	tmplLoaded := false

	var (
		loadErr    error
		parsedTmpl *template.Template
	)

	parsedTmpl, tmplLoaded, loadErr = loadTemplateFromFile(tmpl, tmplName, configDirs)
	if loadErr != nil {
		return nil, loadErr
	}

	if parsedTmpl != nil {
		tmpl = parsedTmpl
	}

	if !tmplLoaded {
		if b, err := builtinTemplates.ReadFile("templates/" + tmplName + ".tmpl"); err == nil {
			tmpl, err = tmpl.New(tmplName + ".tmpl").Parse(string(b))
			if err != nil {
				return nil, FormatTemplateError(fmt.Errorf("failed to parse builtin template %s: %w", tmplName, err), configDirs)
			}

			tmplLoaded = true
		}
	}

	if !tmplLoaded {
		return nil, fmt.Errorf("%w: %s", errUnknownTemplate, tmplName)
	}

	if err := loadPartials(tmpl, configDirs); err != nil {
		return nil, err
	}

	return tmpl, nil
}

func getConfigDirs() []string {
	var configDirs []string

	mainConfigPath := viper.ConfigFileUsed()
	if mainConfigPath != "" {
		configDirs = append(configDirs, filepath.Dir(mainConfigPath))
	}

	userConfigDir, _ := os.UserConfigDir()
	configDirs = append(configDirs, filepath.Join(userConfigDir, "parsec"))

	return configDirs
}

func loadTemplateFromFile(tmpl *template.Template, tmplName string, configDirs []string) (*template.Template, bool, error) {
	for _, dir := range configDirs {
		tmplPath := filepath.Join(dir, "nfo", tmplName+".tmpl")
		if _, statErr := os.Stat(tmplPath); statErr == nil {
			b, readErr := os.ReadFile(tmplPath)
			if readErr == nil {
				parsedTmpl, err := tmpl.New(tmplName + ".tmpl").Parse(string(b))
				if err != nil {
					return nil, true, FormatTemplateError(err, configDirs)
				}

				return parsedTmpl, true, nil
			}
		}
	}

	return nil, false, nil
}

func loadPartials(tmpl *template.Template, configDirs []string) error {
	for _, dir := range configDirs {
		partialGlob := filepath.Join(dir, "nfo", "partial", "*.tmpl")
		if matches, err := filepath.Glob(partialGlob); err == nil {
			for _, match := range matches {
				if b, err := os.ReadFile(match); err == nil {
					if _, err := tmpl.New(filepath.Base(match)).Parse(string(b)); err != nil {
						return FormatTemplateError(err, configDirs)
					}
				}
			}
		}
	}

	return nil
}

func loadBuiltinPartials(tmpl *template.Template, configDirs []string) error {
	entries, err := builtinTemplates.ReadDir("templates/partial")
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".tmpl") {
			if b, err := builtinTemplates.ReadFile("templates/partial/" + entry.Name()); err == nil {
				if _, err := tmpl.New(entry.Name()).Parse(string(b)); err != nil {
					return FormatTemplateError(err, configDirs)
				}
			}
		}
	}

	return nil
}

func appendWordToWrap(result []string, currentLine, word string, limit int) ([]string, string) {
	wordRunes := []rune(word)
	currentLineRunes := []rune(currentLine)

	if len(currentLineRunes) > 0 && len(currentLineRunes)+1+len(wordRunes) <= limit {
		return result, currentLine + " " + word
	}

	if len(currentLineRunes) > 0 {
		result = append(result, currentLine)
	}

	for len(wordRunes) > limit {
		result = append(result, string(wordRunes[:limit]))
		wordRunes = wordRunes[limit:]
	}

	return result, string(wordRunes)
}

func wordWrapSlice(s string, limit int) []string {
	var result []string

	if strings.Contains(s, "\n") {
		lines := strings.SplitSeq(s, "\n")
		for line := range lines {
			if strings.TrimSpace(line) == "" {
				result = append(result, "")
			} else {
				result = append(result, wordWrapSlice(line, limit)...)
			}
		}

		return result
	}

	words := strings.Fields(s)

	var currentLine string
	for _, word := range words {
		result, currentLine = appendWordToWrap(result, currentLine, word, limit)
	}

	if len(currentLine) > 0 {
		result = append(result, currentLine)
	}

	return result
}

func wordWrapFunc(limit int, s string) string {
	return strings.Join(wordWrapSlice(s, limit), "\n")
}

func findBestBreakPoint(current []rune, limit int) int {
	target := len(current) / 2
	if target > limit {
		target = limit - 10 // Try to break slightly before the limit to keep it somewhat balanced
	}

	bestDotIdx := -1
	bestDist := len(current)

	for i, r := range current {
		if r == '.' || r == '-' || r == '_' {
			dist := i - target
			if dist < 0 {
				dist = -dist
			}

			if dist < bestDist && i <= limit {
				bestDist = dist
				bestDotIdx = i
			}
		}
	}

	return bestDotIdx
}

func breakReleaseNameFunc(limit int, s string) []string {
	var result []string

	current := []rune(s)

	for len(current) > limit {
		bestDotIdx := findBestBreakPoint(current, limit)

		if bestDotIdx != -1 && bestDotIdx > 0 {
			result = append(result, string(current[:bestDotIdx+1]))
			current = current[bestDotIdx+1:]
		} else {
			result = append(result, string(current[:limit]))
			current = current[limit:]
		}
	}

	if len(current) > 0 {
		result = append(result, string(current))
	}

	return result
}

func formatNumberFunc(val any) string {
	s := fmt.Sprint(val)

	parts := strings.Split(s, " ")
	if len(parts) > 0 {
		numStr := parts[0]

		var resultSb452 strings.Builder

		for i, r := range numStr {
			if i > 0 && (len(numStr)-i)%3 == 0 {
				resultSb452.WriteString(" ")
			}

			resultSb452.WriteString(string(r))
		}

		parts[0] = resultSb452.String()
	}

	return strings.Join(parts, " ")
}

func parsePadArgs(args []any) (string, string, bool) {
	if len(args) == 0 {
		return "", " ", false
	}

	val := fmt.Sprint(args[len(args)-1])
	padChar := " "
	allowOverflow := false

	if len(args) > 1 {
		padChar = fmt.Sprint(args[0])
	}

	if len(args) > 2 {
		if b, ok := args[1].(bool); ok {
			allowOverflow = b
		}
	}

	return val, padChar, allowOverflow
}

func padRightFunc(length int, args ...any) string {
	s, padChar, allowOverflow := parsePadArgs(args)
	if strings.Contains(s, "\n") {
		lines := strings.Split(s, "\n")

		result := make([]string, 0, len(lines))
		for _, line := range lines {
			result = append(result, padRightFunc(length, append(args[:len(args)-1], line)...))
		}

		return strings.Join(result, "\n")
	}

	runes := []rune(s)
	if len(runes) > length {
		if allowOverflow {
			return s
		}

		return string(runes[:length])
	}

	return s + strings.Repeat(padChar, length-len(runes))
}

func padLeftFunc(length int, args ...any) string {
	s, padChar, allowOverflow := parsePadArgs(args)
	if strings.Contains(s, "\n") {
		lines := strings.Split(s, "\n")

		result := make([]string, 0, len(lines))
		for _, line := range lines {
			result = append(result, padLeftFunc(length, append(args[:len(args)-1], line)...))
		}

		return strings.Join(result, "\n")
	}

	runes := []rune(s)
	if len(runes) > length {
		if allowOverflow {
			return s
		}

		return string(runes[:length])
	}

	return strings.Repeat(padChar, length-len(runes)) + s
}

func centerFunc(length int, val any) string {
	s := fmt.Sprint(val)
	if strings.Contains(s, "\n") {
		lines := strings.Split(s, "\n")

		result := make([]string, 0, len(lines))
		for _, line := range lines {
			result = append(result, centerFunc(length, line))
		}

		return strings.Join(result, "\n")
	}

	runes := []rune(s)
	if len(runes) >= length {
		return string(runes[:length])
	}

	padding := length - len(runes)
	leftPad := padding / 2
	rightPad := padding - leftPad

	return strings.Repeat(" ", leftPad) + s + strings.Repeat(" ", rightPad)
}

func indexOrEmptyFunc(index int, lines []string) string {
	if index >= 0 && index < len(lines) {
		return lines[index]
	}

	return ""
}

func applyBorderFunc(leftBorder, rightBorder, s string) string {
	s = strings.Trim(s, "\n\r")
	if s == "" {
		return ""
	}

	lines := strings.Split(s, "\n")

	result := make([]string, 0, len(lines))
	for _, line := range lines {
		result = append(result, fmt.Sprintf("%s%s%s", leftBorder, line, rightBorder))
	}

	return strings.Join(result, "\n")
}

func toInt(val any) (int, bool) {
	v := reflect.ValueOf(val)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return int(v.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int(v.Uint()), true
	}

	return 0, false
}

func formatDurationFunc(args ...any) (string, error) {
	if len(args) == 0 {
		return "", errDurationNoInput
	}

	secVal, ok := toInt(args[len(args)-1])
	if !ok {
		return "", fmt.Errorf("formatDuration: %w of type %T", errDurationInvalid, args[len(args)-1])
	}

	layout := "hh:mm:ss"

	if len(args) > 1 {
		if lStr, ok := args[0].(string); ok {
			layout = lStr
		}
	}

	hasH := strings.Contains(layout, "h")
	hasM := strings.Contains(layout, "m")

	var h, m, s int

	switch {
	case hasH && hasM:
		h = secVal / 3600
		m = (secVal % 3600) / 60
		s = secVal % 60
	case hasH:
		h = secVal / 3600
		s = secVal % 3600
	case hasM:
		m = secVal / 60
		s = secVal % 60
	default:
		s = secVal
	}

	return replaceDurationPlaceholders(layout, h, m, s), nil
}

func replaceDurationPlaceholders(layout string, h, m, s int) string {
	var sb strings.Builder

	runes := []rune(layout)
	i := 0

	for i < len(runes) {
		if runes[i] == '\'' {
			i++

			for i < len(runes) && runes[i] != '\'' {
				sb.WriteRune(runes[i])
				i++
			}

			if i < len(runes) {
				i++
			}

			continue
		}

		i += matchAndWritePlaceholder(runes, i, h, m, s, &sb)
	}

	return sb.String()
}

func matchAndWritePlaceholder(runes []rune, i, h, m, s int, sb *strings.Builder) int {
	r := runes[i]

	switch r {
	case 'h':
		return writeValue(runes, i, 'h', h, sb)
	case 'm':
		return writeValue(runes, i, 'm', m, sb)
	case 's':
		return writeValue(runes, i, 's', s, sb)
	default:
		sb.WriteRune(r)

		return 1
	}
}

func writeValue(runes []rune, i int, char rune, val int, sb *strings.Builder) int {
	if i+1 < len(runes) && runes[i+1] == char {
		fmt.Fprintf(sb, "%02d", val)

		return 2
	}

	sb.WriteString(strconv.Itoa(val))

	return 1
}

func toFloat64(val any) (float64, bool) {
	return templateutil.ToFloat64(val)
}

func toInt64(val any) int64 {
	return templateutil.ToInt64(val)
}

func formatSizeFunc(precision int, bytes any) string {
	bVal := toInt64(bytes)
	gib := float64(bVal) / (1024 * 1024 * 1024)
	format := fmt.Sprintf("%%.%df GiB", precision)

	return fmt.Sprintf(format, gib)
}

func formatSizeDynamicFunc(precision int, bytes any) string {
	const (
		kib = 1024
		mib = 1024 * kib
		gib = 1024 * mib
		tib = 1024 * gib
		pib = 1024 * tib
	)

	bVal := toInt64(bytes)
	val := float64(bVal)

	var unit string

	switch {
	case bVal < kib:
		unit = "B"
	case bVal < mib:
		val /= kib
		unit = "KiB"
	case bVal < gib:
		val /= mib
		unit = "MiB"
	case bVal < tib:
		val /= gib
		unit = "GiB"
	case bVal < pib:
		val /= tib
		unit = "TiB"
	default:
		val /= pib
		unit = "PiB"
	}

	format := fmt.Sprintf("%%.%df %%s", precision)

	return fmt.Sprintf(format, val, unit)
}

func parseBitrateInput(input string) (float64, bool) {
	cleanInput := strings.ReplaceAll(input, " ", "")
	cleanInput = strings.ReplaceAll(cleanInput, "\u00a0", "")

	switch {
	case strings.Contains(cleanInput, "kb/s") || strings.Contains(cleanInput, "kbps"):
		numStr := strings.TrimSuffix(strings.TrimSuffix(cleanInput, "kb/s"), "kbps")

		val, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0, false
		}

		return val * 1000, true

	case strings.Contains(cleanInput, "mb/s") || strings.Contains(cleanInput, "mbps"):
		numStr := strings.TrimSuffix(strings.TrimSuffix(cleanInput, "mb/s"), "mbps")

		val, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0, false
		}

		return val * 1000000, true

	default:
		val, err := strconv.ParseFloat(cleanInput, 64)
		if err != nil {
			return 0, false
		}

		if val < 100000 {
			return val * 1000, true
		}

		return val, true
	}
}

func formatBitrateFunc(args ...any) (string, error) {
	if len(args) == 0 {
		return "", errBitrateNoInput
	}

	input := fmt.Sprint(args[len(args)-1])
	unit := "kb/s"

	if len(args) > 1 {
		if uStr, ok := args[0].(string); ok {
			unit = strings.ToLower(uStr)
		}
	}

	rawBps, ok := parseBitrateInput(input)
	if !ok {
		return input, nil
	}

	switch unit {
	case "mbps", "mb/s":
		mbps := rawBps / 1000000

		return fmt.Sprintf("%.2f Mbps", mbps), nil
	case "kbps", "kb/s":
		kbps := int(math.Round(rawBps / 1000))

		return formatNumberFunc(kbps) + " kb/s", nil
	default:
		return input, nil
	}
}

func parseDateFunc(dateStr string) time.Time {
	if dateStr == "" {
		return time.Time{}
	}

	var layout string

	switch len(dateStr) {
	case 4:
		layout = "2006"
	case 7:
		layout = "2006-01"
	case 10:
		layout = "2006-01-02"
	default:
		layout = "2006-01-02"
	}

	t, err := time.Parse(layout, dateStr)
	if err != nil {
		if t, err = time.Parse(time.RFC3339, dateStr); err != nil {
			return time.Time{}
		}
	}

	return t
}

func replaceFunc(oldStr, newStr, s string) string {
	return strings.ReplaceAll(s, oldStr, newStr)
}

func repeatFunc(count int, s string) string {
	return strings.Repeat(s, count)
}

func splitFunc(sep, s string) []string {
	return strings.Split(s, sep)
}

func addFunc(a, b int) int { return a + b }
func subFunc(a, b int) int { return a - b }
func mulFunc(a, b int) int { return a * b }
func divFunc(a, b int) int {
	if b == 0 {
		return 0
	}

	return a / b
}

func joinFunc(sep string, items any) string {
	if items == nil {
		return ""
	}

	v := reflect.ValueOf(items)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return fmt.Sprint(items)
	}

	var strParts []string
	for i := 0; i < v.Len(); i++ {
		strParts = append(strParts, fmt.Sprint(v.Index(i).Interface()))
	}

	return strings.Join(strParts, sep)
}

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"padRight":          padRightFunc,
		"padLeft":           padLeftFunc,
		"center":            centerFunc,
		"toUpper":           strings.ToUpper,
		"languageName":      metadata.LanguageName,
		"replace":           replaceFunc,
		"repeat":            repeatFunc,
		"split":             splitFunc,
		"pluck":             templateutil.Pluck,
		"uniq":              templateutil.Uniq,
		"contains":          templateutil.Contains,
		"join":              joinFunc,
		"wordWrap":          wordWrapFunc,
		"breakReleaseName":  breakReleaseNameFunc,
		"indexOrEmpty":      indexOrEmptyFunc,
		"applyBorder":       applyBorderFunc,
		"formatNumber":      formatNumberFunc,
		"formatDuration":    formatDurationFunc,
		"formatSize":        formatSizeFunc,
		"formatSizeDynamic": formatSizeDynamicFunc,
		"parseDate":         parseDateFunc,
		"formatBitrate":     formatBitrateFunc,
		"titleCase":         cases.Title(language.Und).String,
		"sum":               sumFunc,
		"mean":              meanFunc,
		"median":            medianFunc,
		"mode":              modeFunc,
		"add":               addFunc,
		"sub":               subFunc,
		"mul":               mulFunc,
		"div":               divFunc,
		"dict":              dictFunc,
		"default":           templateutil.Default,
		"where":             templateutil.Where,
		"first":             templateutil.First,
		"last":              templateutil.Last,
		"toLower":           strings.ToLower,
		"trimPrefix":        trimPrefixFunc,
		"trimSuffix":        trimSuffixFunc,
		"hasPrefix":         hasPrefixFunc,
		"hasSuffix":         hasSuffixFunc,
		"humanize":          humanizeFunc,
		"truncate":          truncateFunc,
		"chomp":             chompFunc,
		"max":               maxFunc,
		"min":               minFunc,
		"round":             roundFunc,
	}
}

func dictFunc(values ...any) (map[string]any, error) {
	if len(values)%2 != 0 {
		return nil, errDictOddArgs
	}

	dict := make(map[string]any, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, errDictKeyNotString
		}

		dict[key] = values[i+1]
	}

	return dict, nil
}

func trimPrefixFunc(prefix, s string) string {
	return strings.TrimPrefix(s, prefix)
}

func trimSuffixFunc(suffix, s string) string {
	return strings.TrimSuffix(s, suffix)
}

func hasPrefixFunc(prefix, s string) bool {
	return strings.HasPrefix(s, prefix)
}

func hasSuffixFunc(suffix, s string) bool {
	return strings.HasSuffix(s, suffix)
}

func isCapOrDigit(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func isWordBoundary(prev, next rune) bool {
	return (prev >= 'a' && prev <= 'z') || (prev >= 'A' && prev <= 'Z' && next >= 'a' && next <= 'z')
}

func shouldInsertSpace(r, prev, next rune) bool {
	if isCapOrDigit(r) && isWordBoundary(prev, next) {
		return prev != ' '
	}

	return false
}

func insertCamelCaseSpaces(runes []rune) string {
	var sb strings.Builder

	for i, r := range runes {
		if i > 0 && i < len(runes)-1 {
			prev := runes[i-1]
			next := runes[i+1]

			if shouldInsertSpace(r, prev, next) {
				sb.WriteRune(' ')
			}
		}

		sb.WriteRune(r)
	}

	return sb.String()
}

func humanizeFunc(s string) string {
	if s == "" {
		return ""
	}

	s = strings.ReplaceAll(s, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")

	s = insertCamelCaseSpaces([]rune(s))
	s = strings.ToLower(s)

	s = strings.Join(strings.Fields(s), " ")
	if len(s) == 0 {
		return ""
	}

	runes := []rune(s)
	runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]

	return string(runes)
}

func trimAtWordBoundary(truncated []rune) []rune {
	lastSpace := -1

	for i, v := range slices.Backward(truncated) {
		if v == ' ' || v == '\t' || v == '\n' || v == '\r' {
			lastSpace = i

			break
		}
	}

	if lastSpace != -1 && len(truncated)-lastSpace < 20 {
		return truncated[:lastSpace]
	}

	return truncated
}

func truncateFunc(size int, args ...any) (string, error) {
	if len(args) == 0 {
		return "", errTruncateNoInput
	}

	input := fmt.Sprint(args[len(args)-1])

	ellipsis := "..."
	if len(args) > 1 {
		ellipsis = fmt.Sprint(args[0])
	}

	runes := []rune(input)
	if len(runes) <= size {
		return input, nil
	}

	limit := size - len([]rune(ellipsis))
	if limit <= 0 {
		return ellipsis, nil
	}

	truncated := runes[:limit]
	truncated = trimAtWordBoundary(truncated)

	res := strings.TrimRight(string(truncated), " \t\n\r.,!?;:-")

	return res + ellipsis, nil
}

func chompFunc(val any) string {
	return strings.TrimRight(fmt.Sprint(val), "\r\n")
}

func isIntegerType(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	default:
		return false
	}
}

func maxFunc(a any, b ...any) (any, error) {
	vals := append([]any{a}, b...)

	maxFloat, isNum := toFloat64(vals[0])
	if !isNum {
		return nil, fmt.Errorf("max: %w of type %T", errNonNumeric, vals[0])
	}

	allInt := isIntegerType(reflect.ValueOf(vals[0]).Kind())

	var maxInt int64
	if allInt {
		maxInt = toInt64(vals[0])
	}

	for _, val := range vals[1:] {
		vFloat, ok := toFloat64(val)
		if !ok {
			return nil, fmt.Errorf("max: %w of type %T", errNonNumeric, val)
		}

		isThisInt := isIntegerType(reflect.ValueOf(val).Kind())
		if !isThisInt {
			allInt = false
		}

		if vFloat > maxFloat {
			maxFloat = vFloat

			if isThisInt {
				maxInt = toInt64(val)
			}
		}
	}

	if allInt {
		return maxInt, nil
	}

	return maxFloat, nil
}

func minFunc(a any, b ...any) (any, error) {
	vals := append([]any{a}, b...)

	minFloat, isNum := toFloat64(vals[0])
	if !isNum {
		return nil, fmt.Errorf("min: %w of type %T", errNonNumeric, vals[0])
	}

	allInt := isIntegerType(reflect.ValueOf(vals[0]).Kind())

	var minInt int64
	if allInt {
		minInt = toInt64(vals[0])
	}

	for _, val := range vals[1:] {
		vFloat, ok := toFloat64(val)
		if !ok {
			return nil, fmt.Errorf("min: %w of type %T", errNonNumeric, val)
		}

		isThisInt := isIntegerType(reflect.ValueOf(val).Kind())
		if !isThisInt {
			allInt = false
		}

		if vFloat < minFloat {
			minFloat = vFloat

			if isThisInt {
				minInt = toInt64(val)
			}
		}
	}

	if allInt {
		return minInt, nil
	}

	return minFloat, nil
}

func roundFunc(val any) (any, error) {
	vFloat, ok := toFloat64(val)
	if !ok {
		return nil, fmt.Errorf("round: %w of type %T", errNonNumeric, val)
	}

	return int(math.Round(vFloat)), nil
}

// extractTemplateDescription attempts to extract a description from a template's first line.
func extractTemplateDescription(content []byte) string {
	lines := strings.SplitN(string(content), "\n", 2)
	if len(lines) == 0 {
		return ""
	}

	line := strings.TrimSpace(lines[0])
	if strings.HasPrefix(line, "{{/* Description:") && strings.HasSuffix(line, "*/}}") {
		desc := strings.TrimPrefix(line, "{{/* Description:")
		desc = strings.TrimSuffix(desc, "*/}}")

		return strings.TrimSpace(desc)
	}

	return ""
}

func loadBuiltinTemplateDescriptions(templates map[string]string) {
	entries, err := builtinTemplates.ReadDir("templates")
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tmpl") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".tmpl")
		desc := "Built-in template"

		if b, err := builtinTemplates.ReadFile("templates/" + entry.Name()); err == nil {
			if parsedDesc := extractTemplateDescription(b); parsedDesc != "" {
				desc = parsedDesc
			}
		}

		templates[name] = desc
	}
}

func loadCustomTemplateDescriptions(templates map[string]string) {
	for _, dir := range getConfigDirs() {
		tmplDir := filepath.Join(dir, "nfo")

		entries, err := os.ReadDir(tmplDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tmpl") {
				continue
			}

			name := strings.TrimSuffix(entry.Name(), ".tmpl")
			if _, exists := templates[name]; exists {
				continue
			}

			desc := "User template in " + tmplDir
			if b, err := os.ReadFile(filepath.Join(tmplDir, entry.Name())); err == nil {
				if parsedDesc := extractTemplateDescription(b); parsedDesc != "" {
					desc = parsedDesc
				}
			}

			templates[name] = desc
		}
	}
}

// GetAvailableTemplates returns a map of template names to their descriptions.
// It scans both built-in templates and custom templates in the configuration directories.
func GetAvailableTemplates() map[string]string {
	templates := make(map[string]string)

	loadBuiltinTemplateDescriptions(templates)
	loadCustomTemplateDescriptions(templates)

	return templates
}
