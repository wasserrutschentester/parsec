// Package filename provides utilities for parsing and normalizing media filenames.
package filename

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/ui"
)

// GetBaseName returns the filename without extension.
func GetBaseName(filePath string) string {
	name := filepath.Base(filePath)
	if ext := filepath.Ext(name); ext != "" {
		name = name[:len(name)-len(ext)]
	}

	return name
}

// ApplyReplacements applies a slice of regex replacement rules to the input string.
func ApplyReplacements(input string, rules []config.Replacement) string {
	result := input

	for _, rule := range rules {
		if rule.Pattern == "" {
			continue
		}

		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			ui.PrintDebug(fmt.Sprintf("Invalid regex pattern in replacements: %s", err))

			continue
		}

		result = re.ReplaceAllString(result, rule.Replacement)
	}

	return result
}

// ApplyTitleReplacements applies the title cleaning regex replacements from the configuration.
func ApplyTitleReplacements(title string) string {
	rules := config.GetTitleReplacements()

	// For backwards compatibility, if GetTitleCleaningRegex() is set, use it as a rule
	oldRegex := config.GetTitleCleaningRegex()
	if oldRegex != "" {
		rules = append([]config.Replacement{{Pattern: oldRegex, Replacement: ""}}, rules...)
	}

	return strings.TrimSpace(ApplyReplacements(title, rules))
}

// Parse parses a filename to extract metadata.
func Parse(filename string) *metadata.Metadata {
	// 0. Apply input replacements
	filename = ApplyReplacements(filename, config.GetInputReplacements())

	meta := &metadata.Metadata{}

	// match Title, Year, SeasonID, EpisodeID, and EpisodeTitle if available
	title, year := matchTitleYear(filename)
	meta.Title = title
	meta.Year = year

	// Season/Episode ID
	meta.Season, meta.Episodes = matchSeasonEpisode(filename)

	// Date (YYYY-MM-DD)
	matchDate(filename, meta)

	// TV Show
	if meta.Season > 0 || len(meta.Episodes) > 0 || meta.Date != "" {
		meta.IsTV = true
	}

	// Language
	matchLanguage(filename, meta)

	// Dual Audio
	matchDualAudio(filename, meta)

	// CRC32
	matchCRC32(filename, meta)

	// match REPACK
	matchRepack(filename, meta)

	// Basic regex for resolution
	matchResolution(filename, meta)

	// Basic regex for Audio (AAC|DDP|DD|DTS|TrueHD|Atmos|Opus|FLAC) and channels
	matchAudio(filename, meta)

	// Basic regex for Video Codec
	matchVideo(filename, meta)

	// Service
	meta.Service = matchStreamingService(filename)

	// Source
	matchSource(filename, meta)

	// Edition
	matchEdition(filename, meta)

	// Group after last - in the filename
	matchGroup(filename, meta)

	if meta.Title == "" {
		meta.Title = extractTitleFallback(filename, meta)
	}

	// Clean title if it contains the leading group or has trailing junk
	if meta.Group != "" {
		meta.Title = strings.TrimPrefix(meta.Title, "["+meta.Group+"]")
	}

	meta.Title = strings.Trim(meta.Title, ". -")

	// Episode title
	meta.EpisodeTitle = matchEpisodeTitle(filename, meta)

	ui.PrintDebug(fmt.Sprintf("Filename meta: %+v", meta))

	return meta
}

func extractTitleFallback(filename string, meta *metadata.Metadata) string {
	end := len(filename)

	tags := []string{
		meta.Language,
		meta.Resolution,
		meta.Service,
		meta.Source,
		meta.CutEdition,
	}
	if meta.Repack {
		tags = append(tags, "REPACK")
	}

	for _, tag := range tags {
		if tag == "" {
			continue
		}

		re := regexp.MustCompile("(?i)[ .]" + regexp.QuoteMeta(tag))
		if loc := re.FindStringIndex(filename); loc != nil {
			if loc[0] < end {
				end = loc[0]
			}
		}
	}

	title := filename[:end]

	return strings.Trim(title, ". ")
}

func matchStreamingService(filename string) string {
	serviceRegex := regexp.MustCompile(`(?i)[ .](hmax|hbom|hbo[ ._-]?max|hbo|amzn|amazon(hd)?|atvp|aptv|apple[ ._-]?tv\+?|atv|cnlp|canp|canal\+|dsnp|dsny|disney(\+)?|hulu|itunes|nf|netflix(u?hd)?|pcok|peacock([ ._-]?tv)?|pmtp|paramount(\+)?|sho|showtime|stan|syfy|wowtv|cr|crunchyroll|adn|joyn|rtlp|rtl\+|ardp|ard\+|ard|br|hr|mdr|ndr|rbb|sr|swr|wdr|ardmediathek|3sat|kika|arte)([ .]|$)`) //nolint:misspell // "adn" is Animation Digital Network, not a misspelling of "and"
	if match := serviceRegex.FindStringSubmatch(filename); len(match) > 1 {
		return match[1]
	}

	// only match the filename after the resolution
	resolutionRegex := regexp.MustCompile(`[ .]\d{3,4}p([ .]|$)`)
	if loc := resolutionRegex.FindStringIndex(filename); loc != nil {
		filename = filename[loc[1]-1:]
	}

	// only use everything before the WEB-DL or WEBRip source tag
	webRegex := regexp.MustCompile(`[ .](\w+)[ .](WEB(?:-?DL|-?Rip)?)([ .]|$)`)
	if match := webRegex.FindStringSubmatch(filename); len(match) > 1 {
		return match[1]
	}

	return ""
}

func matchEpisodeTitle(filename string, meta *metadata.Metadata) string {
	if !meta.IsTV {
		return ""
	}

	start := findEpisodeTitleStart(filename, meta)
	if start == 0 || start >= len(filename) {
		return ""
	}

	sub := filename[start:]
	end := findEpisodeTitleEnd(sub, meta)

	if end <= 0 {
		return ""
	}

	return strings.Trim(sub[:end], ". ")
}

func skipMultiEpisodeSpecification(filename string, startPos int, episodes []int) int {
	if len(episodes) <= 1 {
		return startPos
	}

	rest := filename[startPos:]
	multiEpRegex := regexp.MustCompile(`(?i)^([ .&-]*)E?(\d{1,3})`)

	pos := startPos

	for {
		match := multiEpRegex.FindStringSubmatchIndex(rest)
		if match == nil {
			break
		}

		separator := rest[match[2]:match[3]]
		matchedText := rest[match[0]:match[1]]
		hasE := strings.Contains(strings.ToLower(matchedText), "e")

		isRange := strings.Contains(separator, "-")
		isList := strings.Contains(separator, "&") || hasE

		if !isRange && !isList {
			break
		}

		pos += match[1]
		rest = rest[match[1]:]
	}

	return pos
}

func findEpisodeTitleStart(filename string, meta *metadata.Metadata) int {
	start := 0

	if meta.Season != 0 || len(meta.Episodes) != 0 {
		ep := 0
		if len(meta.Episodes) > 0 {
			ep = meta.Episodes[0]
		}

		tag := fmt.Sprintf("S%02dE%02d", meta.Season, ep)
		if loc := strings.Index(strings.ToUpper(filename), tag); loc != -1 {
			start = loc + len(tag)
			start = skipMultiEpisodeSpecification(filename, start, meta.Episodes)
		}
	}

	if meta.Date != "" {
		if loc := strings.Index(filename, meta.Date); loc != -1 {
			if end := loc + len(meta.Date); end > start {
				start = end
			}
		}
	}

	return start
}

func findEpisodeTitleEnd(sub string, meta *metadata.Metadata) int {
	end := len(sub)

	if meta.Language != "" {
		re := regexp.MustCompile("(?i)[ .\\(\\[]" + regexp.QuoteMeta(meta.Language))
		if loc := re.FindStringIndex(sub); loc != nil {
			end = loc[0]
		}
	} else if meta.Resolution != "" {
		re := regexp.MustCompile("(?i)[ .\\(\\[]" + regexp.QuoteMeta(meta.Resolution))
		if loc := re.FindStringIndex(sub); loc != nil {
			end = loc[0]
		}
	}

	return end
}

func matchTitleYear(filename string) (string, int) {
	re := regexp.MustCompile(`(?i)^(.*?)(?:[ .](\d{4})|[ .]S\d{1,4}(?:E\d{1,3}(?:(?:[ .\&-]E?\d{1,3})*)?)?|(?:[ .]\d{4}-\d{2}-\d{2}))([ .]|$)`)

	match := re.FindStringSubmatchIndex(filename)
	if match != nil {
		title := filename[match[2]:match[3]]
		year := 0

		if match[4] != -1 && match[5] != -1 {
			yearStr := filename[match[4]:match[5]]
			year, _ = strconv.Atoi(yearStr)
		}

		return title, year
	}

	return "", 0
}

func matchSeasonEpisode(filenameStr string) (int, []int) {
	// First, find S\d{1,4}
	sRegex := regexp.MustCompile(`(?i)S(\d{1,4})`)

	sMatch := sRegex.FindStringSubmatchIndex(filenameStr)
	if sMatch == nil {
		return 0, nil
	}

	season, _ := strconv.Atoi(filenameStr[sMatch[2]:sMatch[3]])

	// Now look at the rest of the string
	rest := filenameStr[sMatch[3]:]

	var episodes []int

	// We want to match an initial episode, e.g. E01, E01, -E01, etc.
	firstEpRegex := regexp.MustCompile(`(?i)^[ .&-]*E(\d{1,3})`)
	epMatch := firstEpRegex.FindStringSubmatchIndex(rest)

	if epMatch == nil {
		return season, nil
	}

	epStr := rest[epMatch[2]:epMatch[3]]
	ep, _ := strconv.Atoi(epStr)
	episodes = append(episodes, ep)

	rest = rest[epMatch[1]:]

	// Now iteratively look for subsequent episodes
	nextEpRegex := regexp.MustCompile(`(?i)^([ .&-]*)E?(\d{1,3})`)

	for {
		nextMatch := nextEpRegex.FindStringSubmatchIndex(rest)
		if nextMatch == nil {
			break
		}

		sep := rest[nextMatch[2]:nextMatch[3]]
		nextEpStr := rest[nextMatch[4]:nextMatch[5]]

		matchedText := rest[nextMatch[0]:nextMatch[1]]
		hasE := strings.Contains(strings.ToLower(matchedText), "e")

		isRange := strings.Contains(sep, "-")
		isList := strings.Contains(sep, "&") || hasE

		if !isRange && !isList {
			break // It's just a number like 1080p or year
		}

		nextEp, _ := strconv.Atoi(nextEpStr)

		if isRange {
			for j := episodes[len(episodes)-1] + 1; j < nextEp; j++ {
				episodes = append(episodes, j)
			}
		}

		episodes = append(episodes, nextEp)

		rest = rest[nextMatch[1]:]
	}

	return season, episodes
}

func matchLanguage(filename string, meta *metadata.Metadata) {
	re := regexp.MustCompile(`(?i)[ .\[\(](GERMAN|ENGLISH|FRENCH|SPANISH|ITALIAN|PORTUGUESE|DUTCH|SWEDISH|NORWEGIAN|FINNISH|GREEK|HEBREW|ARABIC|CHINESE|JAPANESE|KOREAN|THAI|VIETNAMESE|HUNGARIAN|ROMANIAN|POLISH|CZECH|SLOVAK|SLOVENIAN|MULTI|ZXX|SiLENT)(?:[ .](DL|ML|SUBBED))?([ .\]\)]|$)`)

	if match := re.FindStringSubmatch(filename); len(match) > 0 {
		meta.Language = match[1]
		if len(match) > 2 && match[2] != "" {
			meta.LanguageExt = match[2]
			if match[2] == "SUBBED" {
				meta.Subbed = true
			}
		}
	}

	audioDescriptionRegex := regexp.MustCompile(`(?i)[ .\[\(](WiTH\.AD|with\.Audio\.Description)([ .\]\)]|$)`)
	if match := audioDescriptionRegex.FindStringSubmatch(filename); len(match) > 0 {
		meta.Accessibility = match[1]
		meta.HasAudioDesc = true
	}
}

func matchEdition(filename string, meta *metadata.Metadata) {
	// Regex for Editions
	editionRegex := regexp.MustCompile(`(?i)[ .\[\(](Open[ .]Matte|((4K|8K)[ .]?)?REMASTERED|IMAX(?:[ .]Enhanced)?|DIRECTOR'?S[ .]CUT|DC|EXTENDED|THEATRICAL|CRITERION|UNCENSORED|SPECIAL[ .]EDITION)([ .\]\)]|$)`)
	if match := editionRegex.FindStringSubmatch(filename); len(match) > 1 {
		meta.CutEdition = match[1]
	}

	// Regex for 3D
	threeDRegex := regexp.MustCompile(`(?i)[ .\[\(](3D(?:[ .](?:HSBS|SBS|HOU))?)([ .\]\)]|$)`)
	if match := threeDRegex.FindStringSubmatch(filename); len(match) > 1 {
		tag := match[1]
		if meta.CutEdition != "" {
			meta.CutEdition += "." + tag
		} else {
			meta.CutEdition = tag
		}
	}
}

// DeobfuscateTitle removes common obfuscation from titles (e.g. dots, umlaut replacements).
func DeobfuscateTitle(title string) string {
	result := title
	result = strings.ReplaceAll(result, ".", " ")

	// replace umlaut replacements (ae, oe, ue) with their corresponding characters
	// We want to avoid replacing if preceded by a vowel.
	// We also want to avoid replacing "oe" at the end of a word (like Monroe, Poe, Toe, Aloe)
	re := regexp.MustCompile(`(?i)(^|[^aeiou])(ae|oe|ue)($|[^a-z]|.)`)
	result = re.ReplaceAllStringFunc(result, func(m string) string {
		match := re.FindStringSubmatch(m)
		if len(match) < 4 {
			return m
		}

		prefix := match[1]
		umlautMatch := match[2]
		suffix := match[3]

		r := getUmlautReplacement(umlautMatch, suffix)

		return prefix + r + suffix
	})

	return result
}

func getUmlautReplacement(umlautMatch, suffix string) string {
	lowerUmlaut := strings.ToLower(umlautMatch)

	if isEnglishUmlautException(lowerUmlaut, suffix) {
		return umlautMatch
	}

	isUpper := umlautMatch[0] >= 'A' && umlautMatch[0] <= 'Z'

	switch lowerUmlaut {
	case "ae":
		if isUpper {
			return "Ä"
		}

		return "ä"
	case "oe":
		if isUpper {
			return "Ö"
		}

		return "ö"
	case "ue":
		if isUpper {
			return "Ü"
		}

		return "ü"
	default:
		return umlautMatch
	}
}

func isEnglishUmlautException(lowerUmlaut, suffix string) bool {
	// Exception: English words ending in "oe" (e.g. Monroe, Poe, Toe, Aloe)
	isEndOfWord := suffix == "" || !regexp.MustCompile(`(?i)[a-z]`).MatchString(suffix)

	return lowerUmlaut == "oe" && isEndOfWord
}

// NormalizeTitle normalizes a title for use in filenames.
func NormalizeTitle(title string) string {
	// 0. Apply custom cleaning regex from config
	title = ApplyTitleReplacements(title)

	// replace umlauts and similar characters if enabled
	if config.GetNormalizeDiacritics() {
		title = RemoveDiacritics(title)
	}

	// remove unnecessary characters: [(),?!"_|\:] and '
	reUnwanted := regexp.MustCompile(`[(),?!"_|'\:]`)
	title = reUnwanted.ReplaceAllString(title, "")

	// remove .-. or .. like stuff (including spaces)
	reSequences := regexp.MustCompile(`[\. ](-|–|·)?[\. ]+`)
	title = reSequences.ReplaceAllString(title, " ")

	// remove all remaining unwanted characters (preserving spaces)
	reRemaining := regexp.MustCompile(`[^a-zA-Z0-9\-\. ]`)
	title = reRemaining.ReplaceAllString(title, "")

	// collapse multiple spaces
	reSpaces := regexp.MustCompile(`\s+`)
	title = reSpaces.ReplaceAllString(title, " ")

	return strings.Trim(title, ". ")
}

// RemoveDiacritics replaces diacritics and special characters with their ASCII equivalents (e.g., ä -> ae, ß -> ss).
func RemoveDiacritics(title string) string {
	replacements := map[string]string{
		"ä": "ae", "ö": "oe", "ü": "ue",
		"Ä": "Ae", "Ö": "Oe", "Ü": "Ue",
		"ß": "ss",
		"æ": "ae", "Æ": "Ae",
		"ø": "oe", "Ø": "Oe",
		"å": "aa", "Å": "Aa",
	}
	for old, new := range replacements {
		title = strings.ReplaceAll(title, old, new)
	}

	diacritics := map[rune]rune{
		'à': 'a', 'á': 'a', 'â': 'a', 'ã': 'a',
		'è': 'e', 'é': 'e', 'ê': 'e', 'ë': 'e',
		'ì': 'i', 'í': 'i', 'î': 'i', 'ï': 'i',
		'ò': 'o', 'ó': 'o', 'ô': 'o', 'õ': 'o',
		'ù': 'u', 'ú': 'u', 'û': 'u',
		'ý': 'y', 'ÿ': 'y',
	}
	title = strings.Map(func(r rune) rune {
		if val, ok := diacritics[r]; ok {
			return val
		}
		// Also remove combining diacritical marks if any
		if r >= 0x0300 && r <= 0x036F {
			return -1
		}

		return r
	}, title)

	return title
}

// NormalizeService normalizes a streaming service name to a standard code.
func NormalizeService(service string) string {
	s := strings.ToLower(service)

	type serviceMap struct {
		pattern string
		code    string
	}

	services := []serviceMap{
		{`^(hmax|hbom|hbo[ ._-]?max)$`, "HMAX"},
		{`^(amzn|amazon(hd)?)$`, "AMZN"},
		{`^(atvp|aptv|apple[ ._-]?tv\+?)$`, "ATVP"},
		{`^(cnlp|canp|canal\+)$`, "CNLP"},
		{`^(dsnp|dsny|disney(\+)?)$`, "DSNP"},
		{`^(it|itunes)$`, "iT"},
		{`^(nf|netflix(u?hd)?)$`, "NF"},
		{`^(pcok|peacock([ ._-]?tv)?)$`, "PCOK"},
		{`^(pmtp|paramount(\+)?)$`, "PMTP"},
		{`^(sho|showtime)$`, "SHO"},
		{`^(cr|crunchyroll)$`, "CR"},
		{`^(rtlp|rtl\+)$`, "RTLP"},
		{`^(ardp|ard\+)$`, "ARDP"},
		{`^(ard(mediathek)?|br|hr|mdr|ndr|rbb|sr|swr|wdr|rbtv)$`, "ARD"},
		{`^kika$`, "KiKA"},
	}

	for _, sm := range services {
		if regexp.MustCompile(sm.pattern).MatchString(s) {
			return sm.code
		}
	}

	if strings.HasPrefix(s, "zdf") {
		return "ZDF"
	}

	return strings.ToUpper(service)
}

func matchDate(filename string, meta *metadata.Metadata) {
	dataRegex := regexp.MustCompile(`[ .\[\(](\d{4}-\d{2}-\d{2})([ .\]\)]|$)`)
	if match := dataRegex.FindStringSubmatch(filename); len(match) > 1 {
		meta.Date = match[1]
	}
}

func matchRepack(filename string, meta *metadata.Metadata) {
	repackRegex := regexp.MustCompile(`[ .\[\(]REPACK([ .\]\)-]|$|\d)`)
	if repackRegex.MatchString(filename) {
		meta.Repack = true
	}
}

func matchCRC32(filename string, meta *metadata.Metadata) {
	crcRegex := regexp.MustCompile(`[\[\(]([A-Fa-f0-9]{8})[\]\)]`)
	if match := crcRegex.FindStringSubmatch(filename); len(match) > 1 {
		meta.CRC32 = match[1]
	}
}

func matchDualAudio(filename string, meta *metadata.Metadata) {
	dualAudioRegex := regexp.MustCompile(`(?i)[ .\[\(]Dual[ -]Audio[ .\]\)]`)
	if dualAudioRegex.MatchString(filename) {
		meta.DualAudio = true
	}
}

func matchResolution(filename string, meta *metadata.Metadata) {
	resRegex := regexp.MustCompile(`[ .\[\(](\d{3,4}[p|i])([ .\]\)-]|$| )`)
	if match := resRegex.FindStringSubmatch(filename); len(match) > 1 {
		meta.Resolution = match[1]
	}
}

func matchAudio(filename string, meta *metadata.Metadata) {
	audioRegex := regexp.MustCompile(`[ .\[\(](AAC|DDP|DD|DTS(?:-HD|:X)?|TrueHD|Atmos|Opus|FLAC)([ .](?:MA|HRA?))?([ .]?([0-9]\.[0-9]))?([ .]Atmos)?([ .\]\)-]|$| )`)
	if match := audioRegex.FindStringSubmatch(filename); len(match) > 1 {
		meta.AudioCodec = match[1]
		if len(match) > 2 && match[2] != "" {
			meta.AudioCodec += match[2]
		}

		if len(match) > 4 && match[4] != "" {
			meta.AudioChannels = match[4]
		}

		if len(match) > 5 && match[5] != "" {
			meta.AudioMeta = "Atmos"
		}

		if meta.AudioCodec == "Atmos" {
			meta.AudioCodec = ""
			meta.AudioMeta = "Atmos"
		}
	}
}

func matchVideo(filename string, meta *metadata.Metadata) {
	videoRegex := regexp.MustCompile(`[ .\[\(]((H\.|H|h|x)26[456]|AVC|HEVC|AV1)([ .\]\)-]|$| )`)
	if match := videoRegex.FindStringSubmatch(filename); len(match) > 1 {
		meta.VideoCodec = match[1]
	}
}

func matchSource(filename string, meta *metadata.Metadata) {
	sourceRegex := regexp.MustCompile(`(?i)[ .\[\(](BD|WEB(?:-?DL|-?Rip)?|UHD[ .]Blu-?Ray|Blu-?Ray|BRRip|BDRip|(?:PAL|NTSC)[ .]DVD[59]?|DVD[59]?|HDTV|DVDRip|HDDVD)([ .\]\)-]|$| )`)
	if match := sourceRegex.FindStringSubmatch(filename); len(match) > 1 {
		meta.Source = match[1]
	}
}

func matchGroup(filename string, meta *metadata.Metadata) {
	// 1. match leading [Group]
	leadingGroupRegex := regexp.MustCompile(`^\[([^\]]+)\]`)
	if match := leadingGroupRegex.FindStringSubmatch(filename); len(match) > 1 {
		meta.Group = match[1]

		return
	}

	// 2. match trailing -Group
	groupRegex := regexp.MustCompile(`\-([^-]+)$`)
	if match := groupRegex.FindStringSubmatch(filename); len(match) > 1 {
		group := match[1]
		// Don't match WEB-DL as group if it's the source
		isWebDL := (group == "DL" || strings.HasPrefix(group, "DL.")) && strings.HasSuffix(filename[:strings.LastIndex(filename, "-")], "WEB")
		if !isWebDL {
			meta.Group = group
		}
	}
}
