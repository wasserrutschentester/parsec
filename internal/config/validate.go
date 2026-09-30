package config

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"golang.org/x/text/language"

	"codeberg.org/upPollo/parsec/internal/templateutil"
	"codeberg.org/upPollo/parsec/internal/ui"
)

// Validate performs a series of checks on the current configuration.
func Validate() {
	checkConfigFile()
	checkValueTypes()
	checkAPIKeys()
}

func checkConfigFile() {
	configFile := GetConfigFileUsed()
	if configFile == "" {
		ui.PrintWarning("No configuration file found. Using internal defaults.")
	} else {
		ui.PrintInfo("Using configuration file: " + ui.AnonymizePath(configFile))
	}
}

var apiKeyRegex = regexp.MustCompile(`^[0-9a-fA-F-]+$`)

func isValidAPIKey(key string) bool {
	return len(key) > 25 && apiKeyRegex.MatchString(key)
}

func checkKey(key, name string) {
	if key != "" {
		if isValidAPIKey(key) {
			ui.PrintSuccess(name + " API key found and appears valid.")
		} else {
			ui.PrintWarning(name + " API key found, but is probably invalid.")
		}
	} else {
		ui.PrintWarning(name + " API key missing.")
	}
}

func checkAPIKeys() {
	tmdbKey := GetTmdbAPIKey()
	tvdbKey := GetTvdbAPIKey()
	prowlarrKey := GetProwlarrAPIKey()
	prowlarrURL := GetProwlarrURL()

	if tmdbKey == "" && tvdbKey == "" {
		ui.PrintWarning("No API keys found. Metadata fetching might be limited.")
	} else {
		checkKey(tmdbKey, "TMDB")
		checkKey(tvdbKey, "TVDB")
	}

	if prowlarrURL != "" {
		checkKey(prowlarrKey, "Prowlarr")
	}
}

var expectedTypes = map[string]string{
	"description":           "string",
	"group":                 "string",
	"source":                "string",
	"preferred_language":    "string",
	"original_language":     "string",
	"original_audio_first":  "bool",
	"subbed_tagging":        "bool",
	"audio_description":     "bool",
	"template":              "string",
	"nfogen_template":       "string",
	"codec_style":           "string",
	"video_codec_avc":       "string",
	"video_codec_hevc":      "string",
	"word_separator":        "string",
	"normalize_diacritics":  "bool",
	"title":                 "string",
	"year":                  "int64",
	"season":                "int64",
	"episode":               "int64",
	"date":                  "string",
	"episode_title":         "string",
	"cut_edition":           "string",
	"hdr":                   "string",
	"service":               "string",
	"repack":                "bool",
	"is_tv":                 "bool",
	"is_movie":              "bool",
	"imdb_id":               "string",
	"tmdb_id":               "int64",
	"tvdb_id":               "int64",
	"allow_special_matches": "bool",
	"title_cleaning_regex":  "string",
	"enabled_checks":        "[]interface {}",
	"disabled_checks":       "[]interface {}",
}

// Sub-keys for structural sections
var apiKeysExpectedTypes = map[string]string{
	"google_fonts": "string",
	"tmdb":         "string",
	"tvdb":         "string",
}

var prowlarrExpectedTypes = map[string]string{
	"url":              "string",
	"api_key":          "string",
	"indexers":         "[]interface {}",
	"movie_categories": "[]interface {}",
	"tv_categories":    "[]interface {}",
}

var updateExpectedTypes = map[string]string{
	"check":      "bool",
	"auto":       "bool",
	"prerelease": "bool",
}

var reLegacyToken = regexp.MustCompile(`\{([^}]+)\}`)

var validCheckIdentifiers map[string]bool

func init() {
	validCheckIdentifiers = make(map[string]bool, len(AllChecks))
	for _, check := range AllChecks {
		validCheckIdentifiers[check] = true
	}
}

func checkValueTypes() {
	configFile := GetConfigFileUsed()
	if configFile == "" {
		return
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		ui.PrintError(fmt.Sprintf("Could not read config file for validation: %v", err))

		return
	}

	var configMap map[string]any
	if err := toml.Unmarshal(data, &configMap); err != nil {
		ui.PrintError(fmt.Sprintf("Could not parse config file for validation: %v", err))

		return
	}

	errors := validateMapTypes(configMap, "", expectedTypes)
	if len(errors) == 0 {
		ui.PrintSuccess("All configuration values are correct.")
	} else {
		for _, msg := range errors {
			ui.PrintWarning(msg)
		}
	}
}

func validateMapTypes(m map[string]any, prefix string, schema map[string]string) []string {
	var errors []string

	for k, v := range m {
		fullKey := k
		if prefix != "" {
			fullKey = prefix + "." + k
		}

		// Handle structural sections FIRST
		if structuralErrors, handled := validateStructuralSections(k, v, prefix, fullKey); handled {
			errors = append(errors, structuralErrors...)

			continue
		}

		expected, ok := schema[strings.ToLower(k)]
		if !ok {
			errors = append(errors, fmt.Sprintf("Unknown configuration key: '%s'", fullKey))

			continue
		}

		// Skip type check for sub-structures
		if reflect.TypeOf(v).Kind() == reflect.Map {
			continue
		}

		actualType := reflect.TypeOf(v).String()
		if actualType != expected {
			errors = append(errors, fmt.Sprintf("Invalid type for '%s': expected %s, got %s", fullKey, expected, actualType))

			continue
		}

		errors = append(errors, validateSpecificKeys(k, v, fullKey)...)
	}

	return errors
}

func validateStructuralSections(k string, v any, prefix, fullKey string) ([]string, bool) {
	if k == "video_codec_style" {
		return validateVideoCodecStyle(v, fullKey), true
	}

	if prefix != "" {
		return nil, false
	}

	if errs, handled := validateSubMapSection(k, v, fullKey); handled {
		return errs, true
	}

	switch k {
	case "preset":
		return validatePresetConfig(v), true
	case "replacements":
		return validateReplacementsConfig(v, fullKey), true
	}

	return nil, false
}

func validateSubMapSection(k string, v any, fullKey string) ([]string, bool) {
	subMap, ok := v.(map[string]any)
	if !ok {
		return nil, false
	}

	switch k {
	case "api_keys":
		return validateMapTypes(subMap, fullKey, apiKeysExpectedTypes), true
	case "prowlarr":
		return validateMapTypes(subMap, fullKey, prowlarrExpectedTypes), true
	case "update":
		return validateMapTypes(subMap, fullKey, updateExpectedTypes), true
	}

	return nil, false
}

func validateVideoCodecStyle(v any, fullKey string) []string {
	var errors []string

	subMap, ok := v.(map[string]any)
	if !ok {
		return []string{fmt.Sprintf("Invalid type for '%s': expected table, got %T", fullKey, v)}
	}

	for styleName, styleContent := range subMap {
		if _, ok := styleContent.(string); ok && styleName == "default" {
			continue
		}

		styleMap, ok := styleContent.(map[string]any)
		if !ok {
			errors = append(errors, fmt.Sprintf("Invalid type for '%s.%s': expected table, got %T", fullKey, styleName, styleContent))

			continue
		}

		for codec, val := range styleMap {
			if _, ok := val.(string); !ok {
				errors = append(errors, fmt.Sprintf("Invalid value for '%s.%s.%s': expected string, got %T", fullKey, styleName, codec, val))
			}
		}
	}

	return errors
}

func validatePresetConfig(v any) []string {
	var errors []string

	subMap, ok := v.(map[string]any)
	if !ok {
		return errors
	}

	for presetName, presetContent := range subMap {
		if pcMap, ok := presetContent.(map[string]any); ok {
			errors = append(errors, validateMapTypes(pcMap, "preset."+presetName, expectedTypes)...)
		}
	}

	return errors
}

func validateReplacementsConfig(v any, fullKey string) []string {
	var errors []string

	subMap, ok := v.(map[string]any)
	if !ok {
		return errors
	}

	for category, rulesAny := range subMap {
		rulesArray, ok := rulesAny.([]any)
		if !ok {
			continue
		}

		for i, ruleAny := range rulesArray {
			ruleMap, ok := ruleAny.(map[string]any)
			if !ok {
				continue
			}

			patternAny, ok := ruleMap["pattern"]
			if !ok {
				continue
			}

			pattern, ok := patternAny.(string)
			if !ok {
				continue
			}

			if _, err := regexp.Compile(pattern); err != nil {
				errors = append(errors, fmt.Sprintf("Invalid regex pattern in '%s.%s[%d]': %v", fullKey, category, i, err))
			}
		}
	}

	return errors
}

func validateSpecificKeys(k string, v any, fullKey string) []string {
	var errors []string

	switch k {
	case "template":
		if templateStr, ok := v.(string); ok {
			errors = append(errors, validateTemplate(templateStr, fullKey)...)
		}
	case "preferred_language", "original_language":
		if langStr, ok := v.(string); ok {
			errors = append(errors, validateLanguage(langStr, fullKey)...)
		}
	case "enabled_checks", "disabled_checks":
		if checkList, ok := v.([]any); ok {
			errors = append(errors, validateCheckIdentifiers(checkList, fullKey)...)
		}
	case "video_codec_avc", "video_codec_hevc":
		errors = append(errors, fmt.Sprintf("'%s' is deprecated, please use 'video_codec_style' instead", fullKey))
	}

	return errors
}

func validateTemplate(templateStr, keyPath string) []string {
	var errors []string

	if !strings.Contains(templateStr, "{{") {
		matches := reLegacyToken.FindAllStringSubmatch(templateStr, -1)

		for _, match := range matches {
			token := match[1]
			if _, ok := templateutil.LegacyTokenMap["{"+token+"}"]; !ok {
				errors = append(errors, fmt.Sprintf("Invalid template key in '%s': {%s}", keyPath, token))
			}
		}
	}

	if err := templateutil.ValidateTemplate(templateStr); err != nil {
		errors = append(errors, fmt.Sprintf("Invalid template syntax in '%s': %v", keyPath, err))
	}

	return errors
}

func validateCheckIdentifiers(identifiers []any, keyPath string) []string {
	var errors []string

	for _, id := range identifiers {
		if idStr, ok := id.(string); ok {
			if !validCheckIdentifiers[idStr] {
				errors = append(errors, fmt.Sprintf("Invalid check identifier in '%s': %s", keyPath, idStr))
			}
		}
	}

	return errors
}

func validateLanguage(lang, keyPath string) []string {
	var errors []string

	tag, err := language.Parse(lang)
	if err != nil || tag == language.Und {
		errors = append(errors, fmt.Sprintf("Invalid language tag in '%s': %s", keyPath, lang))

		return errors
	}

	// Reject tags where no base language can be determined with any confidence
	// (e.g. purely private-use or synthetic tags). Accepts both 2-letter
	// (ISO 639-1) and 3-letter (ISO 639-2/3) base codes.
	//
	// Also reject tags carrying BCP 47 extensions or variants: language.Parse
	// is lenient enough to accept garbage like "not-a-lang" without error
	// (parsed as base language "not" plus a private "a-lang" extension,
	// since "not" happens to be a real, obscure ISO 639-3 code), but this
	// config value is only ever matched against a plain base[-script][-region]
	// media language tag, so anything with extra extension/variant subtags
	// is not a value we actually support.
	_, confidence := tag.Base()
	if confidence == language.No || len(tag.Extensions()) > 0 || len(tag.Variants()) > 0 {
		errors = append(errors, fmt.Sprintf("Invalid language tag in '%s': %s", keyPath, lang))
	}

	return errors
}
