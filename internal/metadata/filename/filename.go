package filename

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"codeberg.org/n0ne/parsec/internal/config"
	"codeberg.org/n0ne/parsec/internal/metadata"
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
	dataRegex := regexp.MustCompile(`\.(?:\d{4}-\d{2}-\d{2})\.`)
	if match := dataRegex.FindStringSubmatch(filename); len(match) > 0 {
		meta.Date = match[0][1 : len(match[0])-1]
	}

	// TV Show
	if meta.Season > 0 || meta.Episode > 0 || meta.Date != "" {
		meta.IsTV = true
	}

	// Language
	matchLanguage(filename, meta)

	// match REPACK
	repackRegex := regexp.MustCompile(`\.REPACK(\.|-|$|\d)`)
	if repackRegex.MatchString(filename) {
		meta.Repack = true
	}

	// Basic regex for resolution
	resRegex := regexp.MustCompile(`\.(\d+p)\.`)
	if match := resRegex.FindStringSubmatch(filename); len(match) > 1 {
		meta.Resolution = match[1]
	}

	// Basic regex for Audio (AAC|DDP|DD) and channels
	audioRegex := regexp.MustCompile(`\.(AAC|DDP|DD)([0-9]\.[0-9])\.`)
	if match := audioRegex.FindStringSubmatch(filename); len(match) > 2 {
		meta.AudioCodec = match[1]
		meta.AudioChannels = match[2]
	}

	// Basic regex for Video Codec
	videoRegex := regexp.MustCompile(`\.((H\.|H|h|x)26[456]|AVC|HEVC|AV1)(-|\.|$)`)
	if match := videoRegex.FindStringSubmatch(filename); len(match) > 1 {
		meta.VideoCodec = match[1]
	}

	// Service
	meta.Service = matchStreamingService(filename)

	// Source
	sourceRegex := regexp.MustCompile(`\.(WEB-?(\w+)|BluRay|DVD|HDTV|DVDRip|HDDVD)\.`)
	if match := sourceRegex.FindStringSubmatch(filename); len(match) > 1 {
		meta.Source = match[1]
	}

	// Edition
	matchEdition(filename, meta)

	// Group after last - in the filename
	groupRegex := regexp.MustCompile(`\-([^-]+)$`)
	if match := groupRegex.FindStringSubmatch(filename); len(match) > 1 {
		meta.Group = match[1]
	}

	if meta.Title == "" {
		meta.Title = extractTitleFallback(filename, meta)
	}

	// Episode title
	meta.EpisodeTitle = matchEpisodeTitle(filename, meta)

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
		re := regexp.MustCompile("(?i)\\." + regexp.QuoteMeta(tag))
		if loc := re.FindStringIndex(filename); loc != nil {
			if loc[0] < end {
				end = loc[0]
			}
		}
	}

	title := filename[:end]
	return strings.Trim(title, ".")
}

func matchStreamingService(filename string) string {
	serviceRegex := regexp.MustCompile(`(?i)\.(hmax|hbom|hbo[ ._-]?max|hbo|amzn|amazon(hd)?|atvp|aptv|apple[ ._-]?tv\+?|atv|cnlp|canp|canal\+|dsnp|dsny|disney(\+)?|hulu|itunes|nf|netflix(u?hd)?|pcok|peacock([ ._-]?tv)?|pmtp|paramount(\+)?|sho|showtime|stan|syfy|wowtv|cr|crunchyroll|adn|joyn|rtlp|rtl\+|ardp|ard\+|ard|br|hr|mdr|ndr|rbb|sr|swr|wdr|ardmediathek|3sat|kika|arte)\.`)
	if match := serviceRegex.FindStringSubmatch(filename); len(match) > 1 {
		return match[1]
	}

	// only match the filename after the resolution
	resolutionRegex := regexp.MustCompile(`\.\d{3,4}p\.`)
	if loc := resolutionRegex.FindStringIndex(filename); loc != nil {
		filename = filename[loc[1]-1:]
	}

	// only use everything before the WEB-DL or WEBRip source tag
	webRegex := regexp.MustCompile(`\.(\w+)\.(WEB-?(\w+))\.`)
	if match := webRegex.FindStringSubmatch(filename); len(match) > 1 {
		return match[1]
	}

	return ""
}

func matchEpisodeTitle(filename string, meta *metadata.Metadata) string {
	if !meta.IsTV {
		return ""
	}

	// Find the end of the season/episode and date tags
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

	if start == 0 || start >= len(filename) {
		return ""
	}

	sub := filename[start:]

	// Now find the beginning of Language or Resolution
	end := len(sub)
	if meta.Language != "" {
		re := regexp.MustCompile("(?i)\\." + regexp.QuoteMeta(meta.Language))
		if loc := re.FindStringIndex(sub); loc != nil {
			end = loc[0]
		}
	} else if meta.Resolution != "" {
		re := regexp.MustCompile("(?i)\\." + regexp.QuoteMeta(meta.Resolution))
		if loc := re.FindStringIndex(sub); loc != nil {
			end = loc[0]
		}
	}

	if end <= 0 {
		return ""
	}

	return strings.TrimPrefix(sub[:end], ".")
}

func matchTitleYear(filename string) (string, int) {
	re := regexp.MustCompile(`^(.*?)(?:[ .](\d{4})|[ .]S\d{2,4}(?:E\d{2,3})?|(?:[ .]\d{4}-\d{2}-\d{2}))[ .]`)
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
	re := regexp.MustCompile(`S(\d{2,4})(?:E(\d{2,3}))?`)
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
	re := regexp.MustCompile(`(?i)\.(GERMAN|ENGLISH|FRENCH|SPANISH|ITALIAN|PORTUGUESE|DUTCH|SWEDISH|NORWEGIAN|FINNISH|GREEK|HEBREW|ARABIC|CHINESE|JAPANESE|KOREAN|THAI|VIETNAMESE|HUNGARIAN|ROMANIAN|POLISH|CZECH|SLOVAK|SLOVENIAN|MULTI|ZXX|SiLENT)(?:\.(DL|ML|SUBBED))?\.`)

	if match := re.FindStringSubmatch(filename); len(match) > 0 {
		meta.Language = match[1]
		if len(match) > 2 && match[2] != "" {
			meta.LanguageExt = match[2]
			if match[2] == "SUBBED" {
				meta.Subbed = true
			}
		}
	}

	audioDescriptionRegex := regexp.MustCompile(`(?i)\.(WiTH\.AD|with\.Audio\.Description)\.`)
	if match := audioDescriptionRegex.FindStringSubmatch(filename); len(match) > 0 {
		meta.Accessibility = match[1]
		meta.HasAudioDesc = true
	}
}

func matchEdition(filename string, meta *metadata.Metadata) {
	// Regex for Editions
	editionRegex := regexp.MustCompile(`(?i)\.(Open\.Matte|REMASTERED|IMAX(\.Enhanced)?|DIRECTORS\.CUT|DC|EXTENDED|THEATRICAL|CRITERION|SPECIAL\.EDITION)\.`)
	if match := editionRegex.FindStringSubmatch(filename); len(match) > 1 {
		meta.CutEdition = strings.ToUpper(match[1])
	}

	// Regex for 3D
	threeDRegex := regexp.MustCompile(`(?i)\.(3D(\.HSBS|\.SBS|\.HOU)?)\.`)
	if match := threeDRegex.FindStringSubmatch(filename); len(match) > 1 {
		tag := strings.ToUpper(match[1])
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

	// replace umlaut replacements (ae, oe, ue) with their corresponding characters,
	// unless preceded by a vowel
	re := regexp.MustCompile(`(?i)(^|[^aeou])(ae|oe|ue)`)
	result = re.ReplaceAllStringFunc(result, func(m string) string {
		lower := strings.ToLower(m)
		var r string
		switch {
		case strings.HasSuffix(lower, "ae"):
			r = "ä"
		case strings.HasSuffix(lower, "oe"):
			r = "ö"
		case strings.HasSuffix(lower, "ue"):
			r = "ü"
		}
		if len(m) > 2 {
			return m[:1] + r
		}
		return r
	})

	// umlaut at the start of the word (capitalized)
	result = strings.ReplaceAll(result, "Ae", "ä")
	result = strings.ReplaceAll(result, "Oe", "ö")
	result = strings.ReplaceAll(result, "Ue", "ü")

	return result
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

	if regexp.MustCompile(`^(hmax|hbom|hbo[ ._-]?max)$`).MatchString(s) {
		return "HMAX"
	}
	if regexp.MustCompile(`^(amzn|amazon(hd)?)$`).MatchString(s) {
		return "AMZN"
	}
	if regexp.MustCompile(`^(atvp|aptv|apple[ ._-]?tv\+?)$`).MatchString(s) {
		return "ATVP"
	}
	if regexp.MustCompile(`^(cnlp|canp|canal\+)$`).MatchString(s) {
		return "CNLP"
	}
	if regexp.MustCompile(`^(dsnp|dsny|disney(\+)?)$`).MatchString(s) {
		return "DSNP"
	}
	if regexp.MustCompile(`^(it|itunes)$`).MatchString(s) {
		return "iT"
	}
	if regexp.MustCompile(`^(nf|netflix(u?hd)?)$`).MatchString(s) {
		return "NF"
	}
	if regexp.MustCompile(`^(pcok|peacock([ ._-]?tv)?)$`).MatchString(s) {
		return "PCOK"
	}
	if regexp.MustCompile(`^(pmtp|paramount(\+)?)$`).MatchString(s) {
		return "PMTP"
	}
	if regexp.MustCompile(`^(sho|showtime)$`).MatchString(s) {
		return "SHO"
	}
	if regexp.MustCompile(`^(cr|crunchyroll)$`).MatchString(s) {
		return "CR"
	}
	if regexp.MustCompile(`^(rtlp|rtl\+)$`).MatchString(s) {
		return "RTLP"
	}
	if regexp.MustCompile(`^(ardp|ard\+)$`).MatchString(s) {
		return "ARDP"
	}
	if regexp.MustCompile(`^(ard(mediathek)?|br|hr|mdr|ndr|rbb|sr|swr|wdr|rbtv)$`).MatchString(s) {
		return "ARD"
	}
	if strings.HasPrefix(s, "zdf") {
		return "ZDF"
	}
	if regexp.MustCompile(`^kika$`).MatchString(s) {
		return "KiKA"
	}
	return strings.ToUpper(service)
}
