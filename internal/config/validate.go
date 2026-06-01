package config

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"

	"codeberg.org/n0ne/parsec/internal/ui"
	"github.com/pelletier/go-toml/v2"
	"golang.org/x/text/language"
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
		ui.PrintInfo(fmt.Sprintf("Using configuration file: %s", configFile))
	}
}

var apiKeyRegex = regexp.MustCompile(`^[0-9a-fA-F-]+$`)

func isValidAPIKey(key string) bool {
	return len(key) > 25 && apiKeyRegex.MatchString(key)
}

func checkKey(key, name string) {
	if key != "" {
		if isValidAPIKey(key) {
			ui.PrintSuccess(fmt.Sprintf("%s API key found and appears valid.", name))
		} else {
			ui.PrintWarning(fmt.Sprintf("%s API key found, but is probably invalid.", name))
		}
	} else {
		ui.PrintWarning(fmt.Sprintf("%s API key missing.", name))
	}
}

func checkAPIKeys() {
	tmdbKey := GetTmdbApiKey()
	tvdbKey := GetTvdbApiKey()

	if tmdbKey == "" && tvdbKey == "" {
		ui.PrintWarning("No API keys found. Metadata fetching might be limited.")
	} else {
		checkKey(tmdbKey, "TMDB")
		checkKey(tvdbKey, "TVDB")
	}
}

var expectedTypes = map[string]string{
	"group":                 "string",
	"source":                "string",
	"preferred_language":    "string",
	"subbed_tagging":        "bool",
	"audio_description":     "bool",
	"template":              "string",
	"video_codec_avc":       "string",
	"video_codec_hevc":      "string",
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
	"tmdb": "string",
	"tvdb": "string",
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
	"repack":         true,
}

var validCheckIdentifiers = map[string]bool{
	"filename_generation_mismatch": true,
	"filename_characters":          true,
	"filename_sequences":           true,
	"filename_year_missing":        true,
	"filename_year_redundant":      true,
	"filename_streaming":           true,
	"filename_tv_special":          true,
	"mdb_title":                    true,
	"mdb_movie_year":               true,
	"mdb_series_year":              true,
	"mdb_track_languages":          true,
	"mdb_unknown_original_lang":    true,
	"mdb_unwanted_audio_lang":      true,
	"mdb_episode_existence":        true,
	"mdb_episode_title":            true,
	"mdb_episode_date":             true,
	"mediainfo_interlaced_web":     true,
	"mediainfo_framerate":          true,
	"mediainfo_bitrate":            true,
	"mediainfo_durations":          true,
	"mediainfo_redundant_audio":    true,
	"mediainfo_resolution":         true,
	"matroska_language_tag":        true,
	"matroska_multi_lang":          true,
	"matroska_name_quality":        true,
	"matroska_name_codecs":         true,
	"matroska_name_redundant_lang": true,
	"matroska_original_language":   true,
	"matroska_duplicate_tracks":    true,
	"matroska_name_keywords":       true,
	"matroska_default_flags":       true,
	"matroska_subtitle_format":     true,
	"matroska_track_order":         true,
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

	var configMap map[string]interface{}
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

func validateMapTypes(m map[string]interface{}, prefix string, schema map[string]string) []string {
	var errors []string

	for k, v := range m {
		fullKey := k
		if prefix != "" {
			fullKey = prefix + "." + k
		}

		// Handle structural sections FIRST
		if k == "api_keys" && prefix == "" {
			if subMap, ok := v.(map[string]interface{}); ok {
				errors = append(errors, validateMapTypes(subMap, fullKey, apiKeysExpectedTypes)...)
				continue
			}
		}
		if k == "preset" && prefix == "" {
			if subMap, ok := v.(map[string]interface{}); ok {
				for presetName, presetContent := range subMap {
					if pcMap, ok := presetContent.(map[string]interface{}); ok {
						errors = append(errors, validateMapTypes(pcMap, "preset."+presetName, expectedTypes)...)
					}
				}
				continue
			}
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

		// Template key validation
		if k == "template" {
			if templateStr, ok := v.(string); ok {
				errors = append(errors, validateTemplateKeys(templateStr, fullKey)...)
			}
		}

		// Language validation
		if k == "preferred_language" {
			if langStr, ok := v.(string); ok {
				errors = append(errors, validateLanguage(langStr, fullKey)...)
			}
		}

		// Check identifier validation
		if k == "enabled_checks" || k == "disabled_checks" {
			if checkList, ok := v.([]interface{}); ok {
				errors = append(errors, validateCheckIdentifiers(checkList, fullKey)...)
			}
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

func validateCheckIdentifiers(identifiers []interface{}, keyPath string) []string {
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

	// Collapse to base language and check if it has a 2-letter ISO 639-1 code.
	// Most common languages used in media (de, en, fr, etc.) have a 2-letter code.
	base, _ := tag.Base()
	if len(base.String()) != 2 {
		errors = append(errors, fmt.Sprintf("Invalid language tag in '%s': %s (must be a language that has a 2-letter ISO code)", keyPath, lang))
	}

	return errors
}
