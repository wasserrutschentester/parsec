package metadata

import (
	"fmt"
	"strings"
	"text/template"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/templateutil"
)

// DetermineCodecStyle evaluates the appropriate codec style name for the given metadata.
func DetermineCodecStyle(meta *Metadata) string {
	if meta.CodecStyle != "" {
		return meta.CodecStyle
	}

	if meta.IsRemux {
		return "remux"
	}

	src := strings.ToUpper(meta.Source)
	isRip := strings.Contains(src, "RIP")

	if !isRip && (strings.Contains(src, "WEB") || meta.Service != "") {
		return "web_dl"
	}

	if meta.IsEncode || isRip {
		return "encode"
	}

	return "default"
}

// FormatVideoCodec formats a video codec name according to the specified style table.
func FormatVideoCodec(rawCodec, style string) string {
	canonical := canonicalCodec(rawCodec)

	if (style == "" || style == "default") && canonical != "" {
		if override := legacyCodecOverride(canonical); override != "" {
			return override
		}
	}

	styles := config.GetVideoCodecStyle()

	if val := matchCodecTable(styles[style], rawCodec, canonical); val != "" {
		return val
	}

	if val := matchCodecTable(styles["default"], rawCodec, canonical); val != "" {
		return val
	}

	return rawCodec
}

func matchCodecTable(table map[string]string, rawCodec, canonical string) string {
	for k, v := range table {
		if strings.EqualFold(k, rawCodec) || strings.EqualFold(k, canonical) {
			return v
		}
	}

	return ""
}

//nolint:staticcheck // fallback for deprecated video_codec_avc and video_codec_hevc
func legacyCodecOverride(canonical string) string {
	switch canonical {
	case "AVC":
		return config.GetVideoCodecAVC()
	case "HEVC":
		return config.GetVideoCodecHEVC()
	default:
		return ""
	}
}

func canonicalCodec(codec string) string {
	u := strings.ToUpper(codec)

	switch {
	case strings.Contains(u, "264") || u == "AVC":
		return "AVC"
	case strings.Contains(u, "265") || u == "HEVC":
		return "HEVC"
	default:
		return codec
	}
}

// ReleaseTemplateFuncMap constructs the complete FuncMap for release name templates.
func ReleaseTemplateFuncMap() template.FuncMap {
	return template.FuncMap{
		// join: skips nil and empty strings, unpacks slices in place
		"join": templateutil.Join,

		// cat joins parts with NO separator
		"cat": templateutil.Cat,

		// when evaluates flat pairs: when cond1 val1 cond2 val2 ... fallback
		"when": templateutil.When,

		// pad zero-pads an integer: pad 2 .Season -> "01"
		"pad": templateutil.Pad,

		// eprange formats an episode range with optional prefix and padding:
		"eprange": templateutil.Eprange,

		// vcodec resolves raw codec to style table: vcodec "remux" .VideoCodec -> "AVC"
		"vcodec": Vcodec,

		// aka formats an AKA foreign title string: aka beforeTitle afterTitle [year]
		"aka": Aka,

		// String manipulation & casing
		"toUpper":      strings.ToUpper,
		"toLower":      strings.ToLower,
		"titleCase":    cases.Title(language.Und).String,
		"replace":      templateutil.Replace,
		"regexReplace": templateutil.RegexReplace,
		"contains":     templateutil.Contains,
		"trimPrefix":   templateutil.TrimPrefix,
		"trimSuffix":   templateutil.TrimSuffix,
		"hasPrefix":    templateutil.HasPrefix,
		"hasSuffix":    templateutil.HasSuffix,

		// Query utilities
		"default":      templateutil.Default,
		"where":        templateutil.Where,
		"pluck":        templateutil.Pluck,
		"first":        templateutil.First,
		"last":         templateutil.Last,
		"uniq":         templateutil.Uniq,
		"indexOrEmpty": templateutil.IndexOrEmpty,
		"languageName": LanguageName,
		"formatDate":   templateutil.FormatDate,
		"list":         templateutil.List,
	}
}

// Vcodec resolves raw codec to style table: vcodec "remux" .VideoCodec -> "AVC".
func Vcodec(style, codec string) string {
	return FormatVideoCodec(codec, style)
}

// Aka formats an AKA foreign title string: aka beforeTitle afterTitle [year]
// e.g. aka .OriginalTitle .Title .YearTag -> "Orig.2024.AKA.Title".
//
//nolint:cyclop // foreign title formatting handles multiple fallback branches
func Aka(beforeTitle, afterTitle string, year ...any) string {
	sep := config.GetWordSeparator()
	if sep == "" {
		sep = "."
	}

	var yearStr string

	if len(year) > 0 && year[0] != nil {
		y := strings.TrimSpace(fmt.Sprint(year[0]))
		if y != "" && y != "<nil>" && y != "0" {
			yearStr = y
		}
	}

	if beforeTitle == "" || beforeTitle == "<nil>" || beforeTitle == afterTitle {
		if yearStr != "" && afterTitle != "" {
			return afterTitle + sep + yearStr
		}

		return afterTitle
	}

	if afterTitle == "" || afterTitle == "<nil>" {
		if yearStr != "" {
			return beforeTitle + sep + yearStr
		}

		return beforeTitle
	}

	if yearStr != "" {
		return beforeTitle + sep + yearStr + sep + "AKA" + sep + afterTitle
	}

	return beforeTitle + sep + "AKA" + sep + afterTitle
}
