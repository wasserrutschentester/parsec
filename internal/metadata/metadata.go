// Package metadata defines structures and utilities for media metadata processing.
package metadata

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/ui"
)

// Metadata represents the metadata for a media file.
type Metadata struct {
	Title    string
	Year     int
	Season   int
	Episodes []int

	Date          string
	EpisodeTitles []string
	Language      string
	LanguageExt   string
	Subbed        bool
	CutEdition    string
	Accessibility string
	HasAudioDesc  bool
	Repack        bool
	Resolution    string
	Service       string
	Source        string
	HDR           string
	BitDepth      int
	AudioCodec    string
	AudioChannels string
	AudioMeta     string
	VideoCodec    string
	DualAudio     bool
	CRC32         string
	Group         string
	ImdbID        string
	TmdbID        int
	TvdbID        int
	IsTV          bool
}

// MatchLanguage determines if two language tags match based on their base language.
func MatchLanguage(tag1, tag2 language.Tag) bool {
	// Treat undefined languages as matching only themselves.
	if tag1 == language.Und || tag2 == language.Und {
		return tag1 == tag2
	}

	b1, _ := tag1.Base()
	b2, _ := tag2.Base()

	return b1 == b2
}

// LanguageName returns the full name of a language given its code.
func LanguageName(lang string) string {
	if lang == "" {
		return ""
	}

	tag := language.Make(lang)
	if tag == language.Und {
		return strings.ToUpper(lang)
	}

	if tag == language.Make("mul") {
		return "MULTI"
	}

	if tag == language.Make("zxx") {
		return "SILENT"
	}

	name := display.English.Languages().Name(tag)
	if name != "" {
		return strings.ToUpper(name)
	}

	return strings.ToUpper(lang)
}

// ChanToNotation converts a number of channels to a string notation (e.g. 6 -> "5.1").
func ChanToNotation(channels int) string {
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
		return strconv.Itoa(channels)
	}
}

// AudioMetaName returns the audio metadata name (e.g. "Atmos") from title and features.
func AudioMetaName(title, additionalFeatures string) string {
	uTitle := strings.ToUpper(title)
	uFeatures := strings.ToUpper(additionalFeatures)

	if strings.Contains(uFeatures, "ATMOS") || strings.Contains(uFeatures, "JOC") || strings.Contains(uTitle, "ATMOS") {
		return "Atmos"
	}

	if strings.Contains(uFeatures, "AURO3D") || strings.Contains(uTitle, "AURO3D") {
		return "Auro3D"
	}

	return ""
}

// AudioCodecName returns a standard audio codec name.
func AudioCodecName(format, profile, additionalFeatures string) string {
	uFormat := strings.ToUpper(format)
	uProfile := strings.ToUpper(profile)
	uFeatures := strings.ToUpper(additionalFeatures)

	switch uFormat {
	case "AAC":
		return "AAC"
	case "AC-3":
		if strings.Contains(uFeatures, "DEP") || strings.Contains(uFeatures, "DEPENDENT") {
			return "DDP"
		}

		return "DD"
	case "E-AC-3":
		return "DDP"
	case "MLP FBA":
		return "TrueHD"
	case "DTS":
		return detectDTS(uProfile, uFeatures)
	default:
		return format
	}
}

func detectDTS(uProfile, uFeatures string) string {
	combined := uProfile + " " + uFeatures
	isXLL := strings.Contains(combined, "XLL")

	// Check for "X" as a standalone word/token to avoid matching inside "XLL"
	reX := regexp.MustCompile(`\bX\b`)
	isX := reX.MatchString(combined)

	if isXLL && isX {
		return "DTS-X"
	}

	if isXLL || strings.Contains(combined, "MA") {
		return "DTS-HD MA"
	}

	if strings.Contains(combined, "XBR") || strings.Contains(combined, "XXCH") || strings.Contains(combined, "HRA") {
		return "DTS-HD HRA"
	}

	if strings.Contains(combined, "ES") {
		return "DTS-ES"
	}

	if strings.Contains(combined, "96/24") {
		return "DTS"
	}

	return "DTS"
}

// VideoCodecName returns a standard video codec name.
func VideoCodecName(format, formatVersion, codecIDHint string) string {
	switch format {
	case "AVC":
		return config.GetVideoCodecAVC()
	case "HEVC":
		return config.GetVideoCodecHEVC()
	case "MPEG Video":
		if strings.Contains(formatVersion, "2") {
			return "MPEG2"
		}

		return "MPEG"
	case "MPEG-4 Visual":
		if strings.Contains(strings.ToUpper(codecIDHint), "XVID") {
			return "XviD"
		}

		if strings.Contains(strings.ToUpper(codecIDHint), "DIVX") {
			return "DivX"
		}

		return "MPEG4"
	default:
		return format
	}
}

// HeightToResolution converts a video height to a resolution string (e.g. 1080 -> "1080p").
func HeightToResolution(height int, scanType string, frameRate float64) string {
	if height <= 0 {
		return ""
	}

	suffix := "p"
	if strings.Contains(strings.ToLower(scanType), "interlaced") || strings.Contains(strings.ToLower(scanType), "mbaff") {
		suffix = "i"
	}

	return matchResolution(height, suffix, frameRate)
}

func matchResolution(height int, suffix string, frameRate float64) string {
	switch {
	case height >= 3200:
		return "4320p"
	case height >= 1600:
		return "2160p"
	case height >= 1164:
		return "1440p"
	case height >= 800:
		return "1080" + suffix
	case height >= 640:
		return "720p"
	case height >= 576:
		return "576" + suffix
	case height >= 480:
		return handleSDResolution(suffix, frameRate)
	default:
		return handleLowResolution(height, suffix, frameRate)
	}
}

func handleSDResolution(suffix string, frameRate float64) string {
	if frameRate < 24.9 {
		// If it's at least 480 but frame rate is NTSC-like, it's 480
		return "480" + suffix
	}
	// If it's at least 480 and frame rate is PAL-like, it's 576 (likely cropped 576)
	return "576" + suffix
}

func handleLowResolution(height int, suffix string, frameRate float64) string {
	if frameRate > 0 {
		if isPALFrameRate(frameRate) {
			return "576" + suffix
		}

		if isNTSCFrameRate(frameRate) {
			return "480" + suffix
		}
	}

	return fmt.Sprintf("%d%s", height, suffix)
}

func isPALFrameRate(frameRate float64) bool {
	return (frameRate >= 24.9 && frameRate <= 25.1) || (frameRate >= 49.9 && frameRate <= 50.1)
}

func isNTSCFrameRate(frameRate float64) bool {
	return (frameRate >= 23.9 && frameRate <= 24.1) || (frameRate >= 29.9 && frameRate <= 30.1) || (frameRate >= 59.9 && frameRate <= 60.1)
}

// SetDefaults enriches metadata with default values from the configuration.
func (meta *Metadata) SetDefaults() {
	meta.setBasicDefaults()
	meta.setTechnicalDefaults()
	meta.setIDDefaults()
	meta.setTypeDefaults()

	ui.PrintDebug(fmt.Sprintf("set config overrides: %+v", meta))
}

func (meta *Metadata) setBasicDefaults() {
	if meta.Title == "" {
		meta.Title = config.GetTitle()
	}

	if meta.Year == 0 {
		meta.Year = config.GetYear()
	}

	if meta.Season == 0 {
		meta.Season = config.GetSeason()
	}

	if len(meta.Episodes) == 0 {
		meta.Episodes = config.GetEpisodes()
	}

	if meta.Date == "" {
		meta.Date = config.GetDate()
	}

	if len(meta.EpisodeTitles) == 0 {
		if t := config.GetEpisodeTitle(); t != "" {
			meta.EpisodeTitles = []string{t}
		}
	}
}

func (meta *Metadata) setTechnicalDefaults() {
	if meta.CutEdition == "" {
		meta.CutEdition = config.GetCutEdition()
	}

	if meta.HDR == "" {
		meta.HDR = config.GetHDR()
	}

	if meta.Service == "" {
		meta.Service = config.GetService()
	}

	if meta.Source == "" {
		meta.Source = config.GetSource()
	}

	if !meta.Repack {
		meta.Repack = config.GetRepack()
	}

	if !meta.HasAudioDesc {
		meta.HasAudioDesc = config.GetAudioDescription()
	}

	if meta.Group == "" {
		meta.Group = config.GetGroup()
	}
}

func (meta *Metadata) setIDDefaults() {
	if meta.ImdbID == "" {
		meta.ImdbID = config.GetImdbID()
	}

	if meta.TmdbID == 0 {
		meta.TmdbID = config.GetTmdbID()
	}

	if meta.TvdbID == 0 {
		meta.TvdbID = config.GetTvdbID()
	}
}

func (meta *Metadata) setTypeDefaults() {
	if !meta.IsTV && config.GetIsTV() {
		meta.IsTV = true
	} else if meta.IsTV && config.GetIsMovie() {
		meta.IsTV = false
	}
}

// GetReleaseName returns the full release name generated from metadata.
func (meta *Metadata) GetReleaseName() string {
	template := config.GetTemplate()
	ui.PrintDebug("using template: " + template)

	return meta.render(template)
}

// GetSeasonPackName returns a folder name for a season pack, omitting episode-specific details.
func (meta *Metadata) GetSeasonPackName() string {
	// Operate on a copy to avoid mutating the original metadata
	metaCopy := *meta
	metaCopy.Episodes = nil
	metaCopy.EpisodeTitles = nil
	metaCopy.Date = ""

	return metaCopy.GetReleaseName()
}

func (meta *Metadata) render(template string) string {
	replacements := meta.getReplacements()

	result := template
	for tag, val := range replacements {
		result = strings.ReplaceAll(result, tag, val)
	}

	finalName := cleanName(result)
	sep := config.GetWordSeparator()

	if sep != " " {
		finalName = strings.ReplaceAll(finalName, " ", sep)
	}

	// Filename length safeguard
	finalName = meta.truncateIfTooLong(finalName, template)

	return finalName
}

func (meta *Metadata) truncateIfTooLong(finalName, template string) string {
	if len(finalName) <= 245 {
		return finalName
	}

	ui.PrintWarning(fmt.Sprintf("Generated filename exceeds 245 bytes (%d bytes). Attempting to truncate.", len(finalName)))

	if len(meta.EpisodeTitles) == 0 {
		ui.PrintError("Cannot truncate: no episode title to remove. This might cause filesystem errors.")

		return finalName
	}

	metaCopy := *meta
	metaCopy.EpisodeTitles = nil

	// Recursively render without episode title
	truncatedName := metaCopy.render(template)

	if len(truncatedName) <= 245 {
		ui.PrintInfo("Successfully truncated by removing the episode title.")
	} else {
		ui.PrintError(fmt.Sprintf("Even without the episode title, the filename is still too long (%d bytes). This might cause filesystem errors.", len(truncatedName)))
	}

	return truncatedName
}

func (meta *Metadata) getReplacements() map[string]string {
	replacements := map[string]string{
		"{title}":          meta.Title,
		"{date}":           meta.Date,
		"{episode_title}":  strings.Join(meta.EpisodeTitles, " / "),
		"{language}":       LanguageName(meta.Language),
		"{language_ext}":   meta.LanguageExt,
		"{cut_edition}":    meta.CutEdition,
		"{accessibility}":  meta.Accessibility,
		"{resolution}":     meta.Resolution,
		"{service}":        meta.Service,
		"{source}":         meta.Source,
		"{hdr}":            meta.HDR,
		"{audio_codec}":    meta.AudioCodec,
		"{audio_channels}": meta.AudioChannels,
		"{audio_meta}":     meta.AudioMeta,
		"{video_codec}":    meta.VideoCodec,
		"{group}":          meta.Group,
	}

	if meta.DualAudio {
		replacements["{dual_audio}"] = "Dual-Audio"
	}

	if meta.CRC32 != "" {
		replacements["{crc32}"] = strings.ToUpper(strings.Trim(meta.CRC32, "[]"))
	}

	if meta.BitDepth > 8 {
		replacements["{bit_depth}"] = fmt.Sprintf("%dbit", meta.BitDepth)
	}

	if meta.Year > 0 {
		replacements["{year}"] = strconv.Itoa(meta.Year)
	}

	if meta.Season > 0 || meta.IsTV {
		replacements["{season_raw}"] = strconv.Itoa(meta.Season)
		replacements["{season_02}"] = fmt.Sprintf("%02d", meta.Season)
		replacements["{season_id}"] = fmt.Sprintf("S%02d", meta.Season)
	}

	meta.setEpisodeReplacements(replacements)

	if meta.Repack {
		replacements["{repack}"] = "REPACK"
	}

	if meta.HasAudioDesc && meta.Accessibility == "" {
		replacements["{accessibility}"] = "with.Audio.Description"
	}

	return replacements
}

func (meta *Metadata) setEpisodeReplacements(replacements map[string]string) {
	if len(meta.Episodes) == 0 {
		return
	}

	eps := make([]int, len(meta.Episodes))
	copy(eps, meta.Episodes)

	first := eps[0]

	last := eps[0]
	for _, e := range eps {
		if e < first {
			first = e
		}

		if e > last {
			last = e
		}
	}

	if len(eps) == 1 || first == last {
		replacements["{episode_raw}"] = strconv.Itoa(first)
		replacements["{episode_02}"] = fmt.Sprintf("%02d", first)
		replacements["{episode_03}"] = fmt.Sprintf("%03d", first)
		replacements["{episode_id}"] = fmt.Sprintf("E%02d", first)
	} else {
		replacements["{episode_raw}"] = fmt.Sprintf("%d-%d", first, last)
		replacements["{episode_02}"] = fmt.Sprintf("%02d-%02d", first, last)
		replacements["{episode_03}"] = fmt.Sprintf("%03d-%03d", first, last)
		replacements["{episode_id}"] = fmt.Sprintf("E%02d-E%02d", first, last)
	}
}

func cleanName(name string) string {
	// 0. remove remaining keys
	reKeys := []*regexp.Regexp{
		regexp.MustCompile(`\{[^}]*\}`),
	}
	for _, re := range reKeys {
		name = re.ReplaceAllString(name, "")
	}

	// 1. Remove empty enclosures (parentheses, brackets, braces) that might contain only separators
	reEmptyEnclosures := []*regexp.Regexp{
		regexp.MustCompile(`\(\s*[\.\-]*\s*\)`),
		regexp.MustCompile(`\[\s*[\.\-]*\s*\]`),
		regexp.MustCompile(`\{\s*[\.\-]*\s*\}`),
	}
	for _, re := range reEmptyEnclosures {
		name = re.ReplaceAllString(name, "")
	}

	// 2. Collapse multiple separators
	name = regexp.MustCompile(`\.+`).ReplaceAllString(name, ".")
	name = regexp.MustCompile(`-+`).ReplaceAllString(name, "-")
	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")

	// 3. Clean up separator combinations
	name = strings.ReplaceAll(name, ".-", "-")
	name = strings.ReplaceAll(name, "-.", "-")
	name = strings.ReplaceAll(name, ". ", ".")
	name = strings.ReplaceAll(name, " .", ".")

	// 4. Trim leading/trailing separators and spaces
	name = strings.Trim(name, ". -")

	return name
}

// Normalize normalizes a string by lowercasing and removing non-alphanumeric characters.
func Normalize(s string) string {
	s = strings.ToLower(s)

	s = strings.ReplaceAll(s, ".", " ")
	s = strings.ReplaceAll(s, "-", " ")

	re := regexp.MustCompile(`[^a-z0-9 ]`)
	s = re.ReplaceAllString(s, "")

	re = regexp.MustCompile(`\s+`)
	s = re.ReplaceAllString(s, " ")

	return strings.TrimSpace(s)
}

// Override overrides metadata fields with values from another Metadata struct.
func (meta *Metadata) Override(newMeta *Metadata) bool {
	updated := false
	mVal := reflect.ValueOf(meta).Elem()
	nVal := reflect.ValueOf(newMeta).Elem()
	typ := mVal.Type()

	for i := 0; i < mVal.NumField(); i++ {
		mField := mVal.Field(i)
		nField := nVal.Field(i)

		f := typ.Field(i)
		if applyOverride(f, nField, mField) {
			updated = true
		}
	}

	return updated
}

func applyOverride(f reflect.StructField, nField, mField reflect.Value) bool {
	switch f.Type.Kind() {
	case reflect.Bool:
		if mField.Bool() != nField.Bool() && nField.Bool() {
			ui.PrintDebug(fmt.Sprintf("%s: %t %s %t", f.Name, mField.Bool(), ui.Muted.Render("->"), nField.Bool()))
			mField.SetBool(nField.Bool())

			return true
		}
	case reflect.Slice:
		if !nField.IsZero() && !reflect.DeepEqual(mField.Interface(), nField.Interface()) {
			ui.PrintDebug(fmt.Sprintf("%s: %v %s %v", f.Name, mField.Interface(), ui.Muted.Render("->"), nField.Interface()))
			mField.Set(nField)

			return true
		}
	default:
		if !nField.IsZero() && mField.Interface() != nField.Interface() {
			ui.PrintDebug(fmt.Sprintf("%s: %v %s %v", f.Name, mField.Interface(), ui.Muted.Render("->"), nField.Interface()))
			mField.Set(nField)

			return true
		}
	}

	return false
}

// RemoveDuplicates removes duplicate elements from a slice.
func RemoveDuplicates[T comparable](slice []T) []T {
	seen := make(map[T]struct{})

	result := make([]T, 0, len(slice))
	for _, v := range slice {
		if _, exists := seen[v]; !exists {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}

	return result
}
