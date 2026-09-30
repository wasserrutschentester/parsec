package metadata

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"
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
	if strings.Contains(src, "WEB-DL") || strings.Contains(src, "WEBDL") || (meta.Service != "" && !strings.Contains(src, "RIP")) {
		return "web_dl"
	}

	if meta.IsEncode {
		return "encode"
	}

	if strings.Contains(src, "WEB") || meta.Service != "" {
		return "web_dl"
	}

	return "default"
}

// FormatVideoCodec formats a video codec name according to the specified style table.
func FormatVideoCodec(rawCodec, style string) string {
	canonical := canonicalCodec(rawCodec)
	styles := config.GetVideoCodecStyle()

	table, ok := styles[style]
	if !ok {
		return rawCodec
	}

	for k, v := range table {
		if strings.EqualFold(k, rawCodec) || strings.EqualFold(k, canonical) {
			return v
		}
	}

	return rawCodec
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
//
//nolint:cyclop,funlen,gocognit // FuncMap definition with inline closures has high structural complexity
func ReleaseTemplateFuncMap() template.FuncMap {
	return template.FuncMap{
		// Variadic join: skips empty strings, nil, and int 0
		"join": func(sep string, parts ...any) string {
			var items []string

			for _, p := range parts {
				if p == nil {
					continue
				}

				if i, ok := p.(int); ok && i == 0 {
					continue
				}

				v := reflect.ValueOf(p)
				if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
					for i := 0; i < v.Len(); i++ {
						val := v.Index(i).Interface()
						if num, isInt := val.(int); isInt && num == 0 {
							continue
						}

						if s := fmt.Sprint(val); s != "" && s != "<nil>" {
							items = append(items, s)
						}
					}

					continue
				}

				if s := fmt.Sprint(p); s != "" && s != "<nil>" {
					items = append(items, s)
				}
			}

			return strings.Join(items, sep)
		},

		// cat joins parts with NO separator
		"cat": func(parts ...any) string {
			var sb strings.Builder

			for _, p := range parts {
				if s := fmt.Sprint(p); s != "" && s != "<nil>" {
					sb.WriteString(s)
				}
			}

			return sb.String()
		},

		// cond is a ternary helper: cond condition trueVal falseVal
		// when evaluates flat pairs: when cond1 val1 cond2 val2 ... fallback
		"when": func(args ...any) any {
			for i := 0; i < len(args)-1; i += 2 {
				if isTruthy(args[i]) {
					return args[i+1]
				}
			}

			if len(args)%2 != 0 {
				return args[len(args)-1]
			}

			return ""
		},

		// pad zero-pads an integer or slice of integers: pad 2 .Season -> "01"
		"pad": func(digits int, val any) string {
			return fmt.Sprintf("%0*d", digits, val)
		},

		// eprange formats an episode range with optional prefix and padding:
		// eprange "E" 2 .Episodes -> "E01" or "E01-E05"
		// eprange "" 2 .Episodes  -> "01" or "01-05"
		"eprange": func(prefix string, digits int, episodes []int) string {
			if len(episodes) == 0 {
				return ""
			}

			first, last := slices.Min(episodes), slices.Max(episodes)
			if first == last {
				return fmt.Sprintf("%s%0*d", prefix, digits, first)
			}

			return fmt.Sprintf("%s%0*d-%s%0*d", prefix, digits, first, prefix, digits, last)
		},

		// vcodec resolves raw codec to style table: vcodec "remux" .VideoCodec -> "AVC"
		"vcodec": func(style, codec string) string {
			return FormatVideoCodec(codec, style)
		},

		// aka formats an AKA foreign title string: aka beforeTitle afterTitle [year]
		// e.g. aka .OriginalTitle .Title .YearTag -> "Orig.2024.AKA.Title"
		"aka": func(beforeTitle, afterTitle string, year ...any) string {
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
		},

		// String manipulation
		"upper":   strings.ToUpper,
		"lower":   strings.ToLower,
		"title":   cases.Title(language.Und).String,
		"replace": strings.ReplaceAll,
		"regexReplace": func(pattern, repl, s string) string {
			re, err := regexp.Compile(pattern)
			if err != nil {
				return s
			}

			return re.ReplaceAllString(s, repl)
		},
		"contains": func(substr, s any) bool {
			return strings.Contains(fmt.Sprint(s), fmt.Sprint(substr))
		},
		"trimPrefix": strings.TrimPrefix,
		"trimSuffix": strings.TrimSuffix,
		"hasPrefix":  strings.HasPrefix,
		"hasSuffix":  strings.HasSuffix,

		// Query utilities
		"default":      templateutil.Default,
		"where":        templateutil.Where,
		"pluck":        templateutil.Pluck,
		"first":        templateutil.First,
		"last":         templateutil.Last,
		"uniq":         templateutil.Uniq,
		"indexOrEmpty": templateutil.IndexOrEmpty,
		"languageName": LanguageName,
		"parseDate":    templateutil.ParseDate,
	}
}

func isTruthy(val any) bool {
	return !templateutil.IsEmpty(val)
}
