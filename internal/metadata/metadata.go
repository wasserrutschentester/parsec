package metadata

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"codeberg.org/n0ne/parsec/internal/config"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
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
	CRC           string
	IsTV          bool
}

func LanguageName(lang string) string {
	if lang == "" {
		return ""
	}
	parts := strings.Split(lang, ".")
	tag := language.Make(parts[0])
	if tag != language.Und {
		if tag == language.Make("mul") {
			parts[0] = "MULTi"
		} else if tag == language.Make("zxx") {
			parts[0] = "SiLENT"
		} else {
			name := display.English.Languages().Name(tag)
			if name != "" {
				parts[0] = strings.ToUpper(name)
			} else {
				parts[0] = strings.ToUpper(parts[0])
			}
		}
	} else {
		parts[0] = strings.ToUpper(parts[0])
	}
	return strings.Join(parts, ".")
}

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
		return fmt.Sprintf("%d", channels)
	}
}

func AudioCodecName(codec string) string {
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

func VideoCodecName(codec string) string {
	switch codec {
	case "AVC":
		return "H.264"
	case "HEVC":
		return "H.265"
	default:
		return codec
	}
}

func HeightToResolution(height int) string {
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
	return meta.Render(config.GetTemplate())
}

func (meta *Metadata) Render(template string) string {
	replacements := map[string]string{
		"{title}":          meta.Title,
		"{date}":           meta.Date,
		"{episode_title}":  meta.EpisodeTitle,
		"{language}":       LanguageName(meta.Language),
		"{resolution}":     meta.Resolution,
		"{service}":        meta.Service,
		"{source}":         meta.Source,
		"{audio_codec}":    meta.AudioCodec,
		"{audio_channels}": meta.AudioChannels,
		"{video_codec}":    meta.VideoCodec,
		"{group}":          meta.Group,
		"{crc}":            meta.CRC,
	}

	if meta.Year > 0 {
		replacements["{year}"] = fmt.Sprintf("%d", meta.Year)
	}
	if meta.Season > 0 || meta.IsTV {
		replacements["{season_raw}"] = fmt.Sprintf("%d", meta.Season)
		replacements["{season_02}"] = fmt.Sprintf("%02d", meta.Season)
		replacements["{season_id}"] = fmt.Sprintf("S%02d", meta.Season)
	}
	if meta.Episode > 0 {
		replacements["{episode_raw}"] = fmt.Sprintf("%d", meta.Episode)
		replacements["{episode_02}"] = fmt.Sprintf("%02d", meta.Episode)
		replacements["{episode_03}"] = fmt.Sprintf("%03d", meta.Episode)
		replacements["{episode_id}"] = fmt.Sprintf("E%02d", meta.Episode)
	}
	if meta.Repack {
		replacements["{repack}"] = "REPACK"
	}

	result := template
	for tag, val := range replacements {
		result = strings.ReplaceAll(result, tag, val)
	}

	return CleanName(result)
}

func CleanName(name string) string {
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

func (meta *Metadata) Override(newMeta *Metadata, quiet bool) bool {
	updated := false
	mVal := reflect.ValueOf(meta).Elem()
	nVal := reflect.ValueOf(newMeta).Elem()
	typ := mVal.Type()

	for i := 0; i < mVal.NumField(); i++ {
		mField := mVal.Field(i)
		nField := nVal.Field(i)
		f := typ.Field(i)

		if f.Type.Kind() == reflect.Bool {
			if mField.Bool() != nField.Bool() && nField.Bool() {
				if !quiet {
					fmt.Printf("Update %s: %t -> %t\n", f.Name, mField.Bool(), nField.Bool())
				}
				mField.SetBool(nField.Bool())
				updated = true
			}
		} else {
			if !nField.IsZero() && mField.Interface() != nField.Interface() {
				if !quiet {
					fmt.Printf("Update %s: %v -> %v\n", f.Name, mField.Interface(), nField.Interface())
				}
				mField.Set(nField)
				updated = true
			}
		}
	}
	return updated
}
