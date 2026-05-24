package metadata

import (
	"fmt"

	"codeberg.org/n0ne/parsec/internal/config"
)

type Metadata struct {
	Title         string
	Year          int
	Season        int
	Episode       int
	Date          string
	EpisodeTitle  string
	Language      string
	Repack        bool
	Resolution    string
	Service       string
	Source        string
	AudioCodec    string
	AudioChannels string
	VideoCodec    string
	Group         string
}

func languageName(lang string) string {
	switch lang {
	case "de":
		return "GERMAN"
	case "en":
		return "ENGLISH"
	case "fr":
		return "FRENCH"
	default:
		return lang
	}
}

func chanToNotation(channels int) string {
	switch channels {
	case 1:
		return "1.0"
	case 2:
		return "2.0"
	case 5:
		return "5.0"
	case 6:
		return "5.1"
	case 7:
		return "6.1"
	case 8:
		return "7.1"
	default:
		return fmt.Sprintf("%d", channels)
	}
}

func audioCodecName(codec string) string {
	switch codec {
	case "AAC":
		return "AAC"
	case "AC-3":
		return "DD"
	case "E-AC-3":
		return "DDP"
	default:
		return codec
	}
}

func videoCodecName(codec string) string {
	switch codec {
	case "AVC":
		return "H.264"
	case "HEVC":
		return "H.265"
	default:
		return codec
	}
}

func heightToResolution(height int) string {
	if height <= 0 {
		return ""
	}
	return fmt.Sprintf("%dp", height)
}

func (meta *Metadata) SetDefaults() {
	if meta.Title == "" {
		meta.Title = "Missing.Title"
	}
	if meta.Source == "" {
		meta.Source = config.GetSource()
	}
	if meta.Group == "" {
		meta.Group = config.GetGroup()
	}
}

func (meta *Metadata) String() string {
	name := meta.Title
	if meta.Year > 0 {
		name += fmt.Sprintf(".%d", meta.Year)
	}
	if meta.Season > 0 || meta.Episode > 0 {
		name += fmt.Sprintf(".S%02dE%02d", meta.Season, meta.Episode)
	}
	if meta.Date != "" {
		name += fmt.Sprintf(".%s", meta.Date)
	}
	if meta.EpisodeTitle != "" {
		name += fmt.Sprintf(".%s", meta.EpisodeTitle)
	}
	if meta.Language != "" {
		name += fmt.Sprintf(".%s", languageName(meta.Language))
	}
	if meta.Repack {
		name += fmt.Sprintf(".REPACK")
	}
	if meta.Resolution != "" {
		name += fmt.Sprintf(".%s", meta.Resolution)
	}
	if meta.Service != "" {
		name += fmt.Sprintf(".%s", meta.Service)
	}
	if meta.Source != "" {
		name += fmt.Sprintf(".%s", meta.Source)
	}
	if meta.AudioCodec != "" {
		name += fmt.Sprintf(".%s", meta.AudioCodec)
	}
	if meta.AudioChannels != "" {
		name += fmt.Sprintf("%s", meta.AudioChannels)
	}
	if meta.VideoCodec != "" {
		name += fmt.Sprintf(".%s", meta.VideoCodec)
	}
	if meta.Group != "" {
		name += fmt.Sprintf("-%s", meta.Group)
	}
	return name
}

func (meta *Metadata) Override(newMeta *Metadata) bool {
	updated := false
	if newMeta.Title != "" && meta.Title != newMeta.Title {
		fmt.Printf("Update Title: %s -> %s\n", meta.Title, newMeta.Title)
		meta.Title = newMeta.Title
		updated = true
	}
	if newMeta.Year > 0 && meta.Year != newMeta.Year {
		fmt.Printf("Update Year: %d -> %d\n", meta.Year, newMeta.Year)
		meta.Year = newMeta.Year
		updated = true
	}
	if newMeta.Season > 0 && meta.Season != newMeta.Season {
		fmt.Printf("Update Season: %d -> %d\n", meta.Season, newMeta.Season)
		meta.Season = newMeta.Season
		updated = true
	}
	if newMeta.Episode > 0 && meta.Episode != newMeta.Episode {
		fmt.Printf("Update Episode: %d -> %d\n", meta.Episode, newMeta.Episode)
		meta.Episode = newMeta.Episode
		updated = true
	}
	if newMeta.Date != "" && meta.Date != newMeta.Date {
		fmt.Printf("Update Date: %s -> %s\n", meta.Date, newMeta.Date)
		meta.Date = newMeta.Date
		updated = true
	}
	if newMeta.EpisodeTitle != "" && meta.EpisodeTitle != newMeta.EpisodeTitle {
		fmt.Printf("Update EpisodeTitle: %s -> %s\n", meta.EpisodeTitle, newMeta.EpisodeTitle)
		meta.EpisodeTitle = newMeta.EpisodeTitle
		updated = true
	}
	if newMeta.Language != "" && meta.Language != newMeta.Language {
		fmt.Printf("Update Language: %s -> %s\n", meta.Language, newMeta.Language)
		meta.Language = newMeta.Language
		updated = true
	}
	if newMeta.Repack != meta.Repack {
		fmt.Printf("Update Repack: %t -> %t\n", meta.Repack, newMeta.Repack)
		meta.Repack = newMeta.Repack
		updated = true
	}
	if newMeta.Resolution != "" && meta.Resolution != newMeta.Resolution {
		fmt.Printf("Update Resolution: %s -> %s\n", meta.Resolution, newMeta.Resolution)
		meta.Resolution = newMeta.Resolution
		updated = true
	}
	if newMeta.Service != "" && meta.Service != newMeta.Service {
		fmt.Printf("Update Service: %s -> %s\n", meta.Service, newMeta.Service)
		meta.Service = newMeta.Service
		updated = true
	}
	if newMeta.Source != "" && meta.Source != newMeta.Source {
		fmt.Printf("Update Source: %s -> %s\n", meta.Source, newMeta.Source)
		meta.Source = newMeta.Source
		updated = true
	}
	if newMeta.AudioCodec != "" && meta.AudioCodec != newMeta.AudioCodec {
		fmt.Printf("Update AudioCodec: %s -> %s\n", meta.AudioCodec, newMeta.AudioCodec)
		meta.AudioCodec = newMeta.AudioCodec
		updated = true
	}
	if newMeta.AudioChannels != "" && meta.AudioChannels != newMeta.AudioChannels {
		fmt.Printf("Update AudioChannels: %s -> %s\n", meta.AudioChannels, newMeta.AudioChannels)
		meta.AudioChannels = newMeta.AudioChannels
		updated = true
	}
	if newMeta.VideoCodec != "" && meta.VideoCodec != newMeta.VideoCodec {
		fmt.Printf("Update VideoCodec: %s -> %s\n", meta.VideoCodec, newMeta.VideoCodec)
		meta.VideoCodec = newMeta.VideoCodec
		updated = true
	}
	if newMeta.Group != "" && meta.Group != newMeta.Group {
		fmt.Printf("Update Group: %s -> %s\n", meta.Group, newMeta.Group)
		meta.Group = newMeta.Group
		updated = true
	}
	return updated
}
