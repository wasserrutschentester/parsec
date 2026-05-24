package metadata

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func ParseFilename(filename string) *Metadata {
	meta := &Metadata{}

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

	// Language
	meta.Language = matchLanguage(filename)

	// match REPACK
	repackRegex := regexp.MustCompile(`\.REPACK\.(?:\w+)\.`)
	if repackRegex.MatchString(filename) {
		meta.Repack = true
	}

	// Basic regex for resolution
	resRegex := regexp.MustCompile(`\.(\d+p)\.`)
	if match := resRegex.FindStringSubmatch(filename); len(match) > 1 {
		meta.Resolution = match[1]
	}

	// Basic regex for Audio (AAC2.0, DDP5.1, DD2.0, etc.)
	audioRegex := regexp.MustCompile(`\.(AAC|DDP|DD)([0-9]\.[0-9])\.`)
	if match := audioRegex.FindStringSubmatch(filename); len(match) > 2 {
		meta.AudioCodec = match[1]
		meta.AudioChannels = match[2]
	}

	// Basic regex for Video Codec H.264/H264/h264/x264 and H.265/H265/h265/x265
	videoRegex := regexp.MustCompile(`\.((H\.|H|h|x)26[456]|AVC|HEVC|AV1)(-|\.|$)`)
	if match := videoRegex.FindStringSubmatch(filename); len(match) > 1 {
		meta.VideoCodec = match[1]
	}

	// Service (only with WEB-DL and WEBRip, normally between the resolution and source. e.g. 1080p.ARD.WEB-DL)
	meta.Service = matchStreamingService(filename)

	// Source
	sourceRegex := regexp.MustCompile(`\.(WEB?-(\w+)|BluRay|DVD)\.`)
	if match := sourceRegex.FindStringSubmatch(filename); len(match) > 1 {
		meta.Source = match[1]
	}

	// Group after last - in the filename
	groupRegex := regexp.MustCompile(`\-([^-]+)$`)
	if match := groupRegex.FindStringSubmatch(filename); len(match) > 1 {
		meta.Group = match[1]
	}

	// Episode title
	meta.EpisodeTitle = meta.matchEpisodeTitle(filename)

	return meta
}

func matchStreamingService(filename string) string {
	// only match the filename after the resolution
	resolutionRegex := regexp.MustCompile(`\.\d{3,4}p\.`)
	if match := resolutionRegex.FindStringSubmatch(filename); len(match) > 0 {
		filename = filename[len(match[0]):]
	}

	// only use everything before the WEB-DL or WEBRip source tag
	webRegex := regexp.MustCompile(`\.(\w+)\.(WEB?-(\w+))\.`)
	if match := webRegex.FindStringSubmatch(filename); len(match) > 1 {
		return match[1]
	}

	return ""
}

func (meta *Metadata) matchEpisodeTitle(filename string) string {
	// Episode title should be between the season/episode and language tags
	if meta.Season != 0 || meta.Episode != 0 || meta.Date != "" {
		seasonEpisodeID := fmt.Sprintf("S%02dE%02d.", meta.Season, meta.Episode)
		filename = strings.Split(filename, seasonEpisodeID)[1]
		if meta.Date != "" {
			filename = strings.Split(filename, meta.Date+".")[1]
		}
		if meta.Language != "" {
			filename = strings.Split(filename, "."+meta.Language)[0]
		} else if meta.Resolution != "" {
			filename = strings.Split(filename, "."+meta.Resolution)[0]
		} else {
			return ""
		}
		return filename
	}
	return ""
}

func matchTitleYear(filename string) (string, int) {
	re := regexp.MustCompile(`^(.*?)(?:[ .](\d{4})|[ .]S\d{2,4}(?:E\d{2})?|(?:[ .]\d{4}-\d{2}-\d{2}))[ .]`)
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

func matchLanguage(filename string) string {
	re := regexp.MustCompile(`\.(GERMAN|ENGLISH|FRENCH|SPANISH|ITALIAN|PORTUGUESE|DUTCH|SWEDISH|NORWEGIAN|FINNISH|GREEK|HEBREW|ARABIC|CHINESE|JAPANESE|KOREAN|THAI|VIETNAMESE|HUNGARIAN|ROMANIAN|POLISH|CZECH|SLOVAK|SLOVENIAN|MULTI|ZXX|SiLENT)(?:\.(DL|ML|SUBBED))?\.`)
	languageTag := ""
	if match := re.FindStringSubmatch(filename); len(match) > 0 {
		languageTag = match[1]
		if len(match) > 2 && match[2] != "" {
			languageTag += "." + match[2]
		}
	}

	audioDescriptionRegex := regexp.MustCompile(`\.(WiTH\.AD|with\.Audio\.Description)\.`)
	if match := audioDescriptionRegex.FindStringSubmatch(filename); len(match) > 0 {
		languageTag += "." + match[1]
	}

	return languageTag
}

func CheckAllowedCharacters(filename string) []string {
	re := regexp.MustCompile(`[^a-zA-Z0-9\-\.]`)
	match := re.FindStringSubmatch(filename)
	if match != nil {
		fmt.Printf("disallowed character found: %s\n", strings.Join(match, " "))
		return match
	}
	return nil
}

func CheckCharacterSequences(filename string) []string {
	re := regexp.MustCompile(`\.-*\.+`)
	match := re.FindStringSubmatch(filename)
	if match != nil {
		fmt.Printf("disallowed character sequence found: %s\n", strings.Join(match, " "))
		return match
	}
	return nil
}
