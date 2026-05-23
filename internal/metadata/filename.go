package metadata

import (
	"regexp"
)

func ParseFilename(filename string) *Metadata {
	meta := &Metadata{}

	// match Title, Year, SeasonID, EpisodeID, and EpisodeTitle if available

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

	return meta
}

func matchStreamingService(filename string) string {

	return ""
	}
