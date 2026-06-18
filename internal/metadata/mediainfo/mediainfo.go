// Package mediainfo provides tools to extract and parse metadata using MediaInfo.
package mediainfo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/language"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata"
	"codeberg.org/upPollo/parsec/internal/ui"
)

var (
	errNoVideoTrack = errors.New("no video track found")
	errNoAudioTrack = errors.New("no audio track found")
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

// MediaInfo represents the complete JSON output from mediainfo.
type MediaInfo struct {
	CreatingLibrary CreatingLibrary `json:"creatingLibrary"`
	Media           Media           `json:"media"`
}

// CreatingLibrary contains information about the library that created the mediainfo output.
type CreatingLibrary struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	URL     string `json:"url"`
}

// Media contains the tracks of the media file.
type Media struct {
	Ref          string  `json:"@ref"`
	GeneralTrack *Track  `json:"-"`
	Tracks       []Track `json:"track"`
}

// Extra represents the extra fields in a MediaInfo track.
type Extra map[string]any

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

// UnmarshalJSON custom unmarshaler for MediaBool to handle "Yes"/"No" strings.
func (mb *MediaBool) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		// Try unmarshaling as a literal bool
		var boolean bool
		if err := json.Unmarshal(b, &boolean); err != nil {
			return fmt.Errorf("failed to unmarshal MediaBool: %w", err)
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

// Track represents a single track in the mediainfo output.
type Track struct {
	Type                     string    `json:"@type"`
	TypeOrder                *int      `json:"@typeorder,string,omitempty"`
	ID                       string    `json:"ID,omitempty"`
	UniqueID                 string    `json:"UniqueID,omitempty"`
	Format                   string    `json:"Format,omitempty"`
	FormatProfile            string    `json:"Format_Profile,omitempty"`
	FormatVersion            string    `json:"Format_Version,omitempty"`
	FormatAdditionalFeatures string    `json:"Format_AdditionalFeatures,omitempty"`
	Title                    string    `json:"Title,omitempty"`
	Language                 string    `json:"Language,omitempty"`
	Duration                 float64   `json:"Duration,string,omitempty"`
	Channels                 int       `json:"Channels,string,omitempty"`
	BitRate                  int       `json:"BitRate,string,omitempty"`
	BitRateMode              string    `json:"BitRate_Mode,omitempty"`
	HDRFormatCompatibility   string    `json:"HDR_Format_Compatibility,omitempty"`
	HDRFormat                string    `json:"HDR_Format,omitempty"`
	TransferCharacteristics  string    `json:"transfer_characteristics,omitempty"`
	Height                   int       `json:"Height,string,omitempty"`
	Width                    int       `json:"Width,string,omitempty"`
	DisplayAspectRatio       string    `json:"DisplayAspectRatio,omitempty"`
	ScanType                 string    `json:"ScanType,omitempty"`
	FrameRate                float64   `json:"FrameRate,string,omitempty"`
	FrameCount               int       `json:"FrameCount,string,omitempty"`
	BitDepth                 int       `json:"BitDepth,string,omitempty"`
	ChromaSubsampling        string    `json:"ChromaSubsampling,omitempty"`
	SamplingRate             int       `json:"SamplingRate,string,omitempty"`
	DialogNormalization      string    `json:"Dialog_Normalization,omitempty"`
	CodecID                  string    `json:"CodecID,omitempty"`
	CodecIDHint              string    `json:"CodecID_Hint,omitempty"`
	EncodedLibrary           string    `json:"Encoded_Library,omitempty"`
	StreamSize               int64     `json:"StreamSize,string,omitempty"`
	Default                  MediaBool `json:"Default,omitempty"`
	Forced                   MediaBool `json:"Forced,omitempty"`

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

// GetDialNorm returns the dialog normalization value from the track properties.
func (t *Track) GetDialNorm() string {
	val := t.DialogNormalization
	if val == "" {
		val = t.Extra.GetString("Dialog_Normalization")
	}

	if val == "" {
		val = t.Extra.GetString("dialnorm")
	}

	if val == "" {
		val = t.Extra.GetString("dialnorm_Average")
	}

	// Strip " dB" suffix if present
	return strings.TrimSuffix(val, " dB")
}

// Get runs mediainfo on the given file path and returns a MediaInfo struct.
func Get(filePath string) (*MediaInfo, error) {
	if _, err := os.Stat(filePath); err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	ui.PrintDebug("Executing: mediainfo --Output=JSON --ParseSpeed=0 " + ui.AnonymizePath(filePath))
	cmd := exec.CommandContext(context.Background(), "mediainfo", "--Output=JSON", "--ParseSpeed=0", filePath)

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
		return nil, errNoVideoTrack
	}

	if !mi.hasAudio() {
		return nil, errNoAudioTrack
	}

	return &mi, nil
}

// GetMdbIDs extracts IMDB, TMDB, and TVDB IDs from the General track's extra metadata.
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

// GetMetadata converts MediaInfo data into a normalized Metadata struct.
func (mi *MediaInfo) GetMetadata() *metadata.Metadata {
	meta := &metadata.Metadata{}
	for _, track := range mi.Media.Tracks {
		if track.Type == "Video" && meta.Resolution == "" {
			meta.Resolution = metadata.HeightToResolution(track.Height, track.ScanType, track.FrameRate)

			meta.VideoCodec = metadata.VideoCodecName(track.Format, track.FormatVersion, track.CodecIDHint)
			if track.BitDepth != 8 {
				// ignore bit depth if it's 8 (default)
				meta.BitDepth = track.BitDepth
			}

			meta.HDR = track.detectHDR()
		} else if track.Type == "Audio" && meta.AudioCodec == "" {
			meta.AudioCodec = metadata.AudioCodecName(track.Format, track.FormatProfile, track.FormatAdditionalFeatures)
			meta.AudioChannels = metadata.ChanToNotation(track.Channels)
			meta.AudioMeta = metadata.AudioMetaName(track.Title, track.FormatAdditionalFeatures)
		}
	}

	mi.SetLanguageTag(meta)

	if strings.Contains(config.GetTemplate(), "{crc32}") {
		meta.CRC32 = calculateCRC32(mi.Media.Ref)
	}

	ui.PrintDebug(fmt.Sprintf("Mediainfo meta: %+v", meta))

	return meta
}

func calculateCRC32(filePath string) string {
	f, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer func() {
		_ = f.Close()
	}()

	ui.PrintInfo("Calculating CRC32 for " + ui.AnonymizePath(filePath) + "...")

	h := crc32.NewIEEE()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}

	return fmt.Sprintf("%08X", h.Sum32())
}

func (t *Track) detectHDR() string {
	hdrFormat := strings.ToUpper(t.HDRFormat)
	hdrCompat := strings.ToUpper(t.HDRFormatCompatibility)
	hdr := fmt.Sprintf("%s %s", hdrFormat, hdrCompat)

	transfer := strings.ToUpper(t.TransferCharacteristics)

	var result string
	if strings.Contains(hdr, "DOLBY VISION") {
		result = "DV."
	}

	switch {
	case strings.Contains(hdr, "HDR10+"):
		result += "HDR10Plus"
	case strings.Contains(hdr, "HDR10"):
		result += "HDR"
	case strings.Contains(transfer, "HLG"):
		result += "HLG"
	case strings.Contains(hdr, "PQ10") || strings.Contains(transfer, "PQ"):
		result += "PQ10"
	}

	return strings.Trim(result, ".")
}

// GetAudioLanguages returns a list of unique audio language codes.
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

// GetSubtitleLanguages returns a list of unique subtitle language codes.
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

// SetLanguageTag determines the primary language and tagging for the metadata.
func (mi *MediaInfo) SetLanguageTag(meta *metadata.Metadata) {
	languages := mi.GetAudioLanguages()
	preferredLanguage := config.GetPreferredLanguage()

	if len(languages) == 0 {
		fmt.Println("no audio languages found")

		meta.Language = ""
		meta.LanguageExt = ""

		return
	}

	selectedLanguage, hasPreferredAudio := selectAudioLanguage(languages, preferredLanguage)

	// Only tag SUBBED when preferred-language subtitles exist but preferred
	// audio does not. A preferred audio track later in the file still makes the
	// release dual-/multi-language, even if another audio track is first.
	if config.GetSubbedTagging() && !hasPreferredAudio {
		if mi.checkIsSubbed(meta, preferredLanguage) {
			return
		}
	}

	meta.Language = metadata.LanguageName(selectedLanguage)

	switch len(languages) {
	case 0:
	case 1:
	case 2:
		meta.LanguageExt = "DL"
		meta.DualAudio = true
	default:
		meta.LanguageExt = "ML"
		meta.DualAudio = true
	}
}

// selectAudioLanguage returns the preferred audio language when one of the
// tracks matches it, otherwise the first available language. The boolean
// reports whether a preferred audio track was found.
func selectAudioLanguage(languages []string, preferredLanguage string) (string, bool) {
	for _, lang := range languages {
		if sameLanguage(lang, preferredLanguage) {
			return preferredLanguage, true
		}
	}

	return languages[0], false
}

func (mi *MediaInfo) checkIsSubbed(meta *metadata.Metadata, preferredLanguage string) bool {
	subtitleLanguages := mi.GetSubtitleLanguages()
	if len(subtitleLanguages) > 0 {
		for _, lang := range subtitleLanguages {
			if sameLanguage(lang, preferredLanguage) {
				meta.Language = metadata.LanguageName(preferredLanguage)
				meta.LanguageExt = "SUBBED"
				meta.Subbed = true

				return true
			}
		}
	}

	return false
}

func sameLanguage(a, b string) bool {
	return languageKey(a) == languageKey(b)
}

func languageKey(lang string) string {
	if lang == "" {
		return ""
	}

	tag := language.Make(lang)
	if tag == language.Und {
		return strings.ToLower(lang)
	}

	base, _ := tag.Base()

	return base.String()
}
