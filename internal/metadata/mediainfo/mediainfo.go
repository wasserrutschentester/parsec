package mediainfo

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"codeberg.org/n0ne/parsec/internal/config"
	"codeberg.org/n0ne/parsec/internal/metadata"
)

type MediaInfo struct {
	CreatingLibrary CreatingLibrary `json:"creatingLibrary"`
	Media           Media           `json:"media"`
}

type CreatingLibrary struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	URL     string `json:"url"`
}

type Media struct {
	Ref          string  `json:"@ref"`
	GeneralTrack *Track  `json:"-"`
	Tracks       []Track `json:"track"`
}

type Track struct {
	Type                      string `json:"@type"`
	TypeOrder                 *int   `json:"@typeorder,string,omitempty"`
	ID                        string `json:"ID,omitempty"`
	UniqueID                  string `json:"UniqueID,omitempty"`
	Format                    string `json:"Format,omitempty"`
	Title                     string `json:"Title,omitempty"`
	Language                  string `json:"Language,omitempty"`
	Duration                  string `json:"Duration,omitempty"`
	Channels                  string `json:"Channels,omitempty"`
	BitRate                   string `json:"BitRate,omitempty"`
	HDR_Format_Compatibility  string `json:"HDR_Format_Compatibility,omitempty"`
	HDR_Format                string `json:"HDR_Format,omitempty"`
	Transfer_Characteristics  string `json:"transfer_characteristics,omitempty"`
	Format_Version            string `json:"Format_Version,omitempty"`
	Format_AdditionalFeatures string `json:"Format_AdditionalFeatures,omitempty"`
	Height                    string `json:"Height,omitempty"`
	Width                     string `json:"Width,omitempty"`
	ScanType                  string `json:"ScanType,omitempty"`
	FrameRate                 string `json:"FrameRate,omitempty"`
	CodecID                   string `json:"CodecID,omitempty"`
	CodecID_Hint              string `json:"CodecID_Hint,omitempty"`
	Default                   string `json:"Default,omitempty"`
	Forced                    string `json:"Forced,omitempty"`

	// Add more fields as needed, matching the JSON keys
	Extra map[string]interface{} `json:"extra,omitempty"`
	// For fields you don't want to explicitly type, use map[string]interface{}
}

func Get(filePath string) (*MediaInfo, error) {
	if _, err := os.Stat(filePath); err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	cmd := exec.Command("mediainfo", "--Output=JSON", filePath)
	out, err := cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("mediainfo is not installed or not available in PATH: %w", err)
		}
		return nil, fmt.Errorf("failed to run mediainfo: %w", err)
	}

	var mi MediaInfo
	if err := json.Unmarshal(out, &mi); err != nil {
		return nil, fmt.Errorf("failed to parse mediainfo output: %w", err)
	}

	if !mi.isVideo() {
		return nil, fmt.Errorf("no video track found")
	}

	return &mi, nil
}

func (mi *MediaInfo) isVideo() bool {
	for _, track := range mi.Media.Tracks {
		if track.Type == "Video" {
			return true
		}
	}
	return false
}

func (mi *MediaInfo) GetMetadata() *metadata.Metadata {
	meta := &metadata.Metadata{}
	for _, track := range mi.Media.Tracks {
		if track.Type == "Video" {
			height, _ := strconv.Atoi(track.Height)
			meta.Resolution = metadata.HeightToResolution(height)
			meta.VideoCodec = metadata.VideoCodecName(track.Format)
		} else if track.Type == "Audio" {
			meta.AudioCodec = metadata.AudioCodecName(track.Format)
			channels, _ := strconv.Atoi(track.Channels)
			meta.AudioChannels = metadata.ChanToNotation(channels)
		}
	}
	meta.Language = mi.GetLanguageTag()
	return meta
}

func (mi *MediaInfo) GetAudioLanguages() []string {
	var languages []string
	seen := make(map[string]bool)
	for _, track := range mi.Media.Tracks {
		if track.Type == "Audio" {
			if !seen[track.Language] {
				languages = append(languages, track.Language)
				seen[track.Language] = true
			}
		}
	}

	return languages
}

func (mi *MediaInfo) GetSubtitleLanguages() []string {
	var languages []string
	seen := make(map[string]bool)
	for _, track := range mi.Media.Tracks {
		if track.Type == "Text" {
			if !seen[track.Language] {
				languages = append(languages, track.Language)
				seen[track.Language] = true
			}
		}
	}
	return languages
}

func (mi *MediaInfo) GetLanguageTag() string {
	languages := mi.GetAudioLanguages()
	preferredLanguage := config.GetPreferredLanguage()
	if len(languages) == 0 {
		fmt.Println("no audio languages found")
		return ""
	}
	firstAudioLanguage := languages[0]

	// check for preferred language subs if it's not the first audio language
	if preferredLanguage != firstAudioLanguage {
		subtitleLanguages := mi.GetSubtitleLanguages()
		if len(subtitleLanguages) > 0 {
			for _, lang := range subtitleLanguages {
				if lang == preferredLanguage {
					return fmt.Sprintf("%s.SUBBED", metadata.LanguageName(preferredLanguage))
				}
			}
		}
	}

	languageTag := metadata.LanguageName(firstAudioLanguage)
	if len(languages) > 2 {
		languageTag += ".ML"
	} else if len(languages) == 2 {
		languageTag += ".DL"
	}
	return languageTag
}
