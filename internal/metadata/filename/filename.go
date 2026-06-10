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

func GetBaseName(filePath string) string {
	name := filepath.Base(filePath)
	if ext := filepath.Ext(name); ext != "" {
		name = name[:len(name)-len(ext)]
	}

	return name
}

func ApplyTitleCleanRegex(title string) string {
	regexStr := config.GetTitleCleaningRegex()
	if regexStr == "" {
		return title
	}

	re, err := regexp.Compile(regexStr)
	if err != nil {
		return title
	}

	return strings.TrimSpace(re.ReplaceAllString(title, ""))
}

func Parse(filename string) *metadata.Metadata {
	meta := &metadata.Metadata{}

	// match Title, Year, SeasonID, EpisodeID, and EpisodeTitle if available
	title, year := matchTitleYear(filename)
	meta.Title = title
	meta.Year = year

	// Season/Episode ID
	meta.Season, meta.Episode = matchSeasonEpisode(filename)

	// Date (YYYY-MM-DD)
	matchDate(filename, meta)

	// TV Show
	if meta.Season > 0 || meta.Episode > 0 || meta.Date != "" {
		meta.IsTV = true
	}

	// Language
	matchLanguage(filename, meta)

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
	serviceRegex := regexp.MustCompile(`(?i)[ .](hmax|hbom|hbo[ ._-]?max|hbo|amzn|amazon(hd)?|atvp|aptv|apple[ ._-]?tv\+?|atv|cnlp|canp|canal\+|dsnp|dsny|disney(\+)?|hulu|itunes|nf|netflix(u?hd)?|pcok|peacock([ ._-]?tv)?|pmtp|paramount(\+)?|sho|showtime|stan|syfy|wowtv|cr|crunchyroll|adn|joyn|rtlp|rtl\+|ardp|ard\+|ard|br|hr|mdr|ndr|rbb|sr|swr|wdr|ardmediathek|3sat|kika|arte)([ .]|$)`) // nolint:misspell
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

func findEpisodeTitleStart(filename string, meta *metadata.Metadata) int {
	start := 0

	if meta.Season != 0 || meta.Episode != 0 {
		tag := fmt.Sprintf("S%02dE%02d", meta.Season, meta.Episode)
		if loc := strings.Index(filename, tag); loc != -1 {
			start = loc + len(tag)
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
		re := regexp.MustCompile("(?i)[ .]" + regexp.QuoteMeta(meta.Language))
		if loc := re.FindStringIndex(sub); loc != nil {
			end = loc[0]
		}
	} else if meta.Resolution != "" {
		re := regexp.MustCompile("(?i)[ .]" + regexp.QuoteMeta(meta.Resolution))
		if loc := re.FindStringIndex(sub); loc != nil {
			end = loc[0]
		}
	}

	return end
}

func matchTitleYear(filename string) (string, int) {
	re := regexp.MustCompile(`^(.*?)(?:[ .](\d{4})|[ .]S\d{1,4}(?:E\d{1,3})?|(?:[ .]\d{4}-\d{2}-\d{2}))[ .]`)

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

func matchSeasonEpisode(filename string) (int, int) {
	re := regexp.MustCompile(`S(\d{1,4})(?:E(\d{1,3}))?`)

	match := re.FindStringSubmatch(filename)
	if match != nil {
		season, season_err := strconv.Atoi(match[1])

		episode, episode_err := strconv.Atoi(match[2])
		if episode_err == nil && season_err == nil {
			return season, episode
		}
	}

	return 0, 0
}

func matchLanguage(filename string, meta *metadata.Metadata) {
	re := regexp.MustCompile(`(?i)[ .](GERMAN|ENGLISH|FRENCH|SPANISH|ITALIAN|PORTUGUESE|DUTCH|SWEDISH|NORWEGIAN|FINNISH|GREEK|HEBREW|ARABIC|CHINESE|JAPANESE|KOREAN|THAI|VIETNAMESE|HUNGARIAN|ROMANIAN|POLISH|CZECH|SLOVAK|SLOVENIAN|MULTI|ZXX|SiLENT)(?:[ .](DL|ML|SUBBED))?([ .]|$)`)

	if match := re.FindStringSubmatch(filename); len(match) > 0 {
		meta.Language = match[1]
		if len(match) > 2 && match[2] != "" {
			meta.LanguageExt = match[2]
			if match[2] == "SUBBED" {
				meta.Subbed = true
			}
		}
	}

	audioDescriptionRegex := regexp.MustCompile(`(?i)[ .](WiTH\.AD|with\.Audio\.Description)([ .]|$)`)
	if match := audioDescriptionRegex.FindStringSubmatch(filename); len(match) > 0 {
		meta.Accessibility = match[1]
		meta.HasAudioDesc = true
	}
}

func matchEdition(filename string, meta *metadata.Metadata) {
	// Regex for Editions
	editionRegex := regexp.MustCompile(`(?i)[ .](Open[ .]Matte|((4K|8K)[ .]?)?REMASTERED|IMAX(?:[ .]Enhanced)?|DIRECTOR'?S[ .]CUT|DC|EXTENDED|THEATRICAL|CRITERION|UNCENSORED|SPECIAL[ .]EDITION)([ .]|$)`)
	if match := editionRegex.FindStringSubmatch(filename); len(match) > 1 {
		meta.CutEdition = match[1]
	}

	// Regex for 3D
	threeDRegex := regexp.MustCompile(`(?i)[ .](3D(?:[ .](?:HSBS|SBS|HOU))?)([ .]|$)`)
	if match := threeDRegex.FindStringSubmatch(filename); len(match) > 1 {
		tag := match[1]
		if meta.CutEdition != "" {
			meta.CutEdition += "." + tag
		} else {
			meta.CutEdition = tag
		}
	}
}

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

func NormalizeTitle(title string) string {
	// 0. Apply custom cleaning regex from config
	title = ApplyTitleCleanRegex(title)

	// replace umlauts and similar characters
	title = removeDiacritics(title)

	// replace ampersand
	title = strings.ReplaceAll(title, "&", "und")

	// replace spaces with dots
	title = strings.ReplaceAll(title, " ", ".")

	// remove extra (S0x_E0x) info from title
	reExtra := regexp.MustCompile(`\(S[0-9]+[_]E[0-9]+\)`)
	title = reExtra.ReplaceAllString(title, "")

	// remove extra description
	originExp := `(Fernseh|Dokumentar|Spiel|Kurz|Animations|Maerchen|Märchen)film.*(Deutschland|Oesterreich|Österreich|Schweiz|DDR)`
	reOrigin := regexp.MustCompile(originExp)
	title = reOrigin.ReplaceAllString(title, "")

	// remove unnecessary characters: [(),?!"_|\:] and '
	reUnwanted := regexp.MustCompile(`[(),?!"_|'\:]`)
	title = reUnwanted.ReplaceAllString(title, "")

	// remove .-. or .. like stuff
	reSequences := regexp.MustCompile(`\.(-|–|·)?\.+`)
	title = reSequences.ReplaceAllString(title, ".")

	// remove all remaining unwanted characters
	reRemaining := regexp.MustCompile(`[^a-zA-Z0-9\-\.]`)
	title = reRemaining.ReplaceAllString(title, "")

	return strings.Trim(title, ".")
}

func removeDiacritics(title string) string {
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
	dataRegex := regexp.MustCompile(`[ .](\d{4}-\d{2}-\d{2})([ .]|$)`)
	if match := dataRegex.FindStringSubmatch(filename); len(match) > 1 {
		meta.Date = match[1]
	}
}

func matchRepack(filename string, meta *metadata.Metadata) {
	repackRegex := regexp.MustCompile(`[ .]REPACK([ .-]|$|\d)`)
	if repackRegex.MatchString(filename) {
		meta.Repack = true
	}
}

func matchResolution(filename string, meta *metadata.Metadata) {
	resRegex := regexp.MustCompile(`[ .](\d{3,4}[p|i])([ .-]|$| )`)
	if match := resRegex.FindStringSubmatch(filename); len(match) > 1 {
		meta.Resolution = match[1]
	}
}

func matchAudio(filename string, meta *metadata.Metadata) {
	audioRegex := regexp.MustCompile(`[ .](AAC|DDP|DD|DTS(?:-HD|:X)?|TrueHD|Atmos|Opus|FLAC)([ .](?:MA|HRA?))?([ .]?([0-9]\.[0-9]))?([ .]Atmos)?([ .-]|$| )`)
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
	videoRegex := regexp.MustCompile(`[ .]((H\.|H|h|x)26[456]|AVC|HEVC|AV1)([ .-]|$| )`)
	if match := videoRegex.FindStringSubmatch(filename); len(match) > 1 {
		meta.VideoCodec = match[1]
	}
}

func matchSource(filename string, meta *metadata.Metadata) {
	sourceRegex := regexp.MustCompile(`(?i)[ .](WEB(?:-?DL|-?Rip)?|UHD[ .]Blu-?Ray|Blu-?Ray|BRRip|BDRip|(?:PAL|NTSC)[ .]DVD[59]?|DVD[59]?|HDTV|DVDRip|HDDVD)([ .-]|$| )`)
	if match := sourceRegex.FindStringSubmatch(filename); len(match) > 1 {
		meta.Source = match[1]
	}
}

func matchGroup(filename string, meta *metadata.Metadata) {
	groupRegex := regexp.MustCompile(`\-([^-]+)$`)
	if match := groupRegex.FindStringSubmatch(filename); len(match) > 1 {
		group := match[1]
		// Don't match WEB-DL as group if it's the source
		if (group == "DL" || strings.HasPrefix(group, "DL.")) && strings.HasSuffix(filename[:strings.LastIndex(filename, "-")], "WEB") {
			// skip
		} else {
			meta.Group = group
		}
	}
}
