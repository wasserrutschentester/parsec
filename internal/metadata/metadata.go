package metadata

import (
	"fmt"
	"reflect"
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
		name += fmt.Sprintf(".%s", LanguageName(meta.Language))
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
