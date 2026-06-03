package mediainfo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"unicode/utf8"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/ui"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/language"
)

// SanitizeUTF8Bytes ensures a byte slice is valid UTF-8, converting invalid sequences from Windows-1252.
func SanitizeUTF8Bytes(b []byte) []byte {
	if utf8.Valid(b) {
		return b
	}

	var res bytes.Buffer
	res.Grow(len(b))
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		if r == utf8.RuneError && size == 1 {
			// Invalid UTF-8, treat as Windows-1252
			r = charmap.Windows1252.DecodeByte(b[0])
		}
		res.WriteRune(r)
		b = b[size:]
	}
	return res.Bytes()
}

// SanitizeUTF8 ensures a string is valid UTF-8, converting invalid sequences from Windows-1252.
func SanitizeUTF8(s string) string {
	return string(SanitizeUTF8Bytes([]byte(s)))
}

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

// Extra represents the extra fields in a MediaInfo track.
type Extra map[string]interface{}

// GetString returns the value of the extra field with the given key as a string.
func (e Extra) GetString(key string) string {
	if e == nil {
		return ""
	}
	v, ok := e[key]
	if !ok {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return strconv.FormatFloat(val, 'f', 0, 64)
	default:
		return ""
	}
}

// MediaBool represents a boolean value that can be unmarshaled  from "Yes"/"No" or standard boolean strings.
type MediaBool bool

func (mb *MediaBool) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		// Try unmarshaling as a literal bool
		var boolean bool
		if err := json.Unmarshal(b, &boolean); err != nil {
			return err
		}
		*mb = MediaBool(boolean)
		return nil
	}

	switch strings.ToLower(s) {
	case "yes", "true", "1":
		*mb = true
	case "no", "false", "0", "":
		*mb = false
	default:
		*mb = false
	}
	return nil
}

type Track struct {
	Type                      string    `json:"@type"`
	TypeOrder                 *int      `json:"@typeorder,string,omitempty"`
	ID                        string    `json:"ID,omitempty"`
	UniqueID                  string    `json:"UniqueID,omitempty"`
	Format                    string    `json:"Format,omitempty"`
	Format_Profile            string    `json:"Format_Profile,omitempty"`
	Format_Version            string    `json:"Format_Version,omitempty"`
	Format_AdditionalFeatures string    `json:"Format_AdditionalFeatures,omitempty"`
	Title                     string    `json:"Title,omitempty"`
	Language                  string    `json:"Language,omitempty"`
	Duration                  float64   `json:"Duration,string,omitempty"`
	Channels                  int       `json:"Channels,string,omitempty"`
	BitRate                   int       `json:"BitRate,string,omitempty"`
	BitRate_Mode              string    `json:"BitRate_Mode,omitempty"`
	HDR_Format_Compatibility  string    `json:"HDR_Format_Compatibility,omitempty"`
	HDR_Format                string    `json:"HDR_Format,omitempty"`
	Transfer_Characteristics  string    `json:"transfer_characteristics,omitempty"`
	Height                    int       `json:"Height,string,omitempty"`
	Width                     int       `json:"Width,string,omitempty"`
	DisplayAspectRatio        string    `json:"DisplayAspectRatio,omitempty"`
	ScanType                  string    `json:"ScanType,omitempty"`
	FrameRate                 float64   `json:"FrameRate,string,omitempty"`
	FrameCount                int       `json:"FrameCount,string,omitempty"`
	BitDepth                  int       `json:"BitDepth,string,omitempty"`
	ChromaSubsampling         string    `json:"ChromaSubsampling,omitempty"`
	SamplingRate              int       `json:"SamplingRate,string,omitempty"`
	CodecID                   string    `json:"CodecID,omitempty"`
	CodecID_Hint              string    `json:"CodecID_Hint,omitempty"`
	Encoded_Library           string    `json:"Encoded_Library,omitempty"`
	StreamSize                int64     `json:"StreamSize,string,omitempty"`
	Default                   MediaBool `json:"Default,omitempty"`
	Forced                    MediaBool `json:"Forced,omitempty"`

	// General track specific
	VideoCount     int    `json:"VideoCount,string,omitempty"`
	AudioCount     int    `json:"AudioCount,string,omitempty"`
	TextCount      int    `json:"TextCount,string,omitempty"`
	FileSize       int64  `json:"FileSize,string,omitempty"`
	FileExtension  string `json:"FileExtension,omitempty"`
	OverallBitRate int    `json:"OverallBitRate,string,omitempty"`

	// Add more fields as needed, matching the JSON keys
	Extra Extra `json:"extra,omitempty"`
}

func Get(filePath string) (*MediaInfo, error) {
	if _, err := os.Stat(filePath); err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	ui.PrintDebug(fmt.Sprintf("Executing: mediainfo --Output=JSON --ParseSpeed=0 %s", filePath))
	cmd := exec.Command("mediainfo", "--Output=JSON", "--ParseSpeed=0", filePath)
	out, err := cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("mediainfo is not installed or not available in PATH: %w", err)
		}
		return nil, fmt.Errorf("failed to run mediainfo: %w", err)
	}

	out = SanitizeUTF8Bytes(out)

	var mi MediaInfo
	if err := json.Unmarshal(out, &mi); err != nil {
		return nil, fmt.Errorf("failed to parse mediainfo output: %w", err)
	}

	if !mi.isVideo() {
		return nil, fmt.Errorf("no video track found")
	}

	if !mi.hasAudio() {
		return nil, fmt.Errorf("no audio track found")
	}

	return &mi, nil
}

func (mi *MediaInfo) GetMdbIDs() (imdb string, tmdb, tvdb int, isTV bool) {
	extra := mi.getGeneralExtra()
	if extra == nil {
		return imdb, tmdb, tvdb, isTV
	}

	imdb = extra.GetString("IMDB")

	// TMDB
	tmdbVal := extra.GetString("TMDB")
	tmdb, tmdbType := parseID(tmdbVal)
	if tmdbType == "tv" {
		isTV = true
	}

	// TVDB
	tvdbTag := extra.GetString("TVDB")
	tvdb, tvdbType := parseID(tvdbTag)
	if tvdbType == "series" || tvdbType == "tv" {
		isTV = true
	}

	// TVDB2
	tvdb2 := extra.GetString("TVDB2")
	tvdb2ID, tvdb2Type := parseID(tvdb2)
	switch tvdb2Type {
	case "series":
		if tvdb == 0 {
			tvdb = tvdb2ID
		}
		isTV = true
	case "episodes":
		isTV = true
	}

	return imdb, tmdb, tvdb, isTV
}

func (mi *MediaInfo) getGeneralExtra() Extra {
	for i := range mi.Media.Tracks {
		if mi.Media.Tracks[i].Type == "General" {
			return mi.Media.Tracks[i].Extra
		}
	}
	return nil
}

func parseID(val string) (int, string) {
	if val == "" {
		return 0, ""
	}
	parts := strings.Split(val, "/")
	if len(parts) > 1 {
		id, _ := strconv.Atoi(parts[1])
		return id, parts[0]
	}
	id, _ := strconv.Atoi(val)
	return id, ""
}

func (mi *MediaInfo) isVideo() bool {
	for _, track := range mi.Media.Tracks {
		if track.Type == "Video" {
			return true
		}
	}
	return false
}

func (mi *MediaInfo) hasAudio() bool {
	for _, track := range mi.Media.Tracks {
		if track.Type == "Audio" {
			return true
		}
	}
	return false
}

func (mi *MediaInfo) GetMetadata() *metadata.Metadata {
	meta := &metadata.Metadata{}
	for _, track := range mi.Media.Tracks {
		if track.Type == "Video" && meta.Resolution == "" {
			meta.Resolution = metadata.HeightToResolution(track.Height, track.ScanType, track.FrameRate)
			meta.VideoCodec = metadata.VideoCodecName(track.Format, track.Format_Version, track.CodecID_Hint)
			if track.BitDepth != 8 {
				// ignore bit depth if it's 8 (default)
				meta.BitDepth = track.BitDepth
			}
			meta.HDR = track.detectHDR()

		} else if track.Type == "Audio" && meta.AudioCodec == "" {
			meta.AudioCodec = metadata.AudioCodecName(track.Format, track.Format_Profile, track.Format_AdditionalFeatures)
			meta.AudioChannels = metadata.ChanToNotation(track.Channels)
			meta.AudioMeta = metadata.AudioMetaName(track.Title, track.Format_AdditionalFeatures)
		}
	}
	mi.SetLanguageTag(meta)
	ui.PrintDebug(fmt.Sprintf("Mediainfo meta: %+v", meta))
	return meta
}

func (track *Track) detectHDR() string {
	hdrFormat := strings.ToUpper(track.HDR_Format)
	hdrCompat := strings.ToUpper(track.HDR_Format_Compatibility)
	hdr := fmt.Sprintf("%s %s", hdrFormat, hdrCompat)

	transfer := strings.ToUpper(track.Transfer_Characteristics)
	var result string
	if strings.Contains(hdr, "DOLBY VISION") {
		result = "DV."
	}
	if strings.Contains(hdr, "HDR10+") {
		result += "HDR10Plus"
	} else if strings.Contains(hdr, "HDR10") {
		result += "HDR"
	} else if strings.Contains(transfer, "HLG") {
		result += "HLG"
	} else if strings.Contains(hdr, "PQ10") || strings.Contains(transfer, "PQ") {
		result += "PQ10"
	}

	return strings.Trim(result, ".")
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

func (mi *MediaInfo) SetLanguageTag(meta *metadata.Metadata) {
	languages := mi.GetAudioLanguages()
	preferredLanguage := config.GetPreferredLanguage()
	if len(languages) == 0 {
		fmt.Println("no audio languages found")
		meta.Language = ""
		meta.LanguageExt = ""
		return
	}

	prefTag := language.Make(preferredLanguage)
	firstAudioTag := language.Make(languages[0])

	// check for preferred language subs if it's not the first audio language
	if config.GetSubbedTagging() && prefTag != firstAudioTag {
		subtitleLanguages := mi.GetSubtitleLanguages()
		if len(subtitleLanguages) > 0 {
			for _, lang := range subtitleLanguages {
				if language.Make(lang) == prefTag {
					meta.Language = metadata.LanguageName(preferredLanguage)
					meta.LanguageExt = "SUBBED"
					meta.Subbed = true
					return
				}
			}
		}
	}

	meta.Language = metadata.LanguageName(languages[0])
	switch len(languages) {
	case 0:
	case 1:
	case 2:
		meta.LanguageExt = "DL"
	default:
		meta.LanguageExt = "ML"
	}
}

func (mi *MediaInfo) Print() {
	b, err := json.MarshalIndent(mi, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling to JSON: %v\n", err)
		return
	}
	fmt.Println(string(b))
}
