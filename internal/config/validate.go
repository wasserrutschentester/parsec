package config

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"golang.org/x/text/language"

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
	"group":                 "string",
	"source":                "string",
	"preferred_language":    "string",
	"original_language":     "string",
	"subbed_tagging":        "bool",
	"audio_description":     "bool",
	"template":              "string",
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

var validTemplateKeys = map[string]bool{
	"title":          true,
	"date":           true,
	"episode_title":  true,
	"language":       true,
	"language_ext":   true,
	"cut_edition":    true,
	"accessibility":  true,
	"resolution":     true,
	"service":        true,
	"source":         true,
	"hdr":            true,
	"audio_codec":    true,
	"audio_channels": true,
	"audio_meta":     true,
	"video_codec":    true,
	"group":          true,
	"bit_depth":      true,
	"year":           true,
	"season_raw":     true,
	"season_02":      true,
	"season_id":      true,
	"episode_raw":    true,
	"episode_02":     true,
	"episode_03":     true,
	"episode_id":     true,
	"dual_audio":     true,
	"crc32":          true,
	"repack":         true,
}

var validCheckIdentifiers = map[string]bool{
	"filename_generation_mismatch":     true,
	"filename_characters":              true,
	"filename_sequences":               true,
	"filename_year_missing":            true,
	"filename_year_redundant":          true,
	"filename_streaming":               true,
	"filename_tv_special":              true,
	"mdb_title":                        true,
	"mdb_movie_year":                   true,
	"mdb_series_year":                  true,
	"mdb_track_languages":              true,
	"mdb_unknown_original_lang":        true,
	"mdb_unwanted_audio_lang":          true,
	"mdb_episode_existence":            true,
	"mdb_episode_title":                true,
	"mdb_episode_date":                 true,
	"mediainfo_interlaced_web":         true,
	"mediainfo_framerate":              true,
	"mediainfo_bitrate":                true,
	"mediainfo_durations":              true,
	"mediainfo_redundant_audio":        true,
	"mediainfo_resolution":             true,
	"mediainfo_dialogue_normalization": true,
	"mediainfo_stereo_lossless":        true,
	"mediainfo_empty_tracks":           true,
	"matroska_language_tag":            true,

	"matroska_multi_lang":                  true,
	"matroska_name_quality":                true,
	"matroska_name_codecs":                 true,
	"matroska_name_redundant_lang":         true,
	"matroska_original_language":           true,
	"matroska_duplicate_tracks":            true,
	"matroska_name_keywords":               true,
	"matroska_default_flags":               true,
	"matroska_subtitle_format":             true,
	"matroska_subtitle_fonts":              true,
	"matroska_subtitle_inline_fonts":       true,
	"matroska_srt_validation":              true,
	"matroska_ass_script_info":             true,
	"matroska_ass_styles":                  true,
	"matroska_ass_events":                  true,
	"matroska_zlib_compression":            true,
	"matroska_track_order":                 true,
	"matroska_unused_fonts":                true,
	"matroska_font_filename_compliance":    true,
	"matroska_track_delay":                 true,
	"matroska_video_cropping":              true,
	"matroska_title_hygiene":               true,
	"matroska_app_hygiene":                 true,
	"matroska_truehd_compatibility":        true,
	"matroska_commentary_channels":         true,
	"matroska_commentary_bitrate":          true,
	"matroska_commentary_prefix":           true,
	"matroska_commentary_pairing":          true,
	"matroska_chapters_start_non_zero":     true,
	"matroska_chapters_non_monotonic":      true,
	"matroska_chapters_duplicate":          true,
	"matroska_chapters_too_close":          true,
	"matroska_chapters_exceed_duration":    true,
	"matroska_chapters_name_hygiene":       true,
	"matroska_chapters_language_hygiene":   true,
	"matroska_chapters_keyframe_alignment": true,
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
	if prefix != "" {
		return nil, false
	}

	switch k {
	case "api_keys":
		if subMap, ok := v.(map[string]any); ok {
			return validateMapTypes(subMap, fullKey, apiKeysExpectedTypes), true
		}
	case "prowlarr":
		if subMap, ok := v.(map[string]any); ok {
			return validateMapTypes(subMap, fullKey, prowlarrExpectedTypes), true
		}
	case "preset":
		return validatePresetConfig(v), true
	case "replacements":
		return validateReplacementsConfig(v, fullKey), true
	}

	return nil, false
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
			errors = append(errors, validateTemplateKeys(templateStr, fullKey)...)
		}
	case "preferred_language", "original_language":
		if langStr, ok := v.(string); ok {
			errors = append(errors, validateLanguage(langStr, fullKey)...)
		}
	case "enabled_checks", "disabled_checks":
		if checkList, ok := v.([]any); ok {
			errors = append(errors, validateCheckIdentifiers(checkList, fullKey)...)
		}
	}

	return errors
}

func validateTemplateKeys(template, keyPath string) []string {
	var errors []string

	re := regexp.MustCompile(`\{([^}]+)\}`)
	matches := re.FindAllStringSubmatch(template, -1)

	for _, match := range matches {
		token := match[1]
		if !validTemplateKeys[token] {
			errors = append(errors, fmt.Sprintf("Invalid template key in '%s': {%s}", keyPath, token))
		}
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
	_, confidence := tag.Base()
	if confidence == language.No {
		errors = append(errors, fmt.Sprintf("Invalid language tag in '%s': %s", keyPath, lang))
	}

	return errors
}
