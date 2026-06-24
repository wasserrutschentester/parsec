// Package config handles parsec configuration management.
package config

import (
	_ "embed"
	"fmt"
	"slices"

	"github.com/spf13/viper"
)

//go:embed default_config.toml
var defaultConfig string

// InitDefaults initializes the default configuration values in viper.
func InitDefaults() {
	viper.SetDefault("template", "{title}.{year}.{season_id}{episode_id}.{cut_edition}.{episode_title}.{language}.{language_ext}.{accessibility}.{repack}.{resolution}.{service}.{source}.{audio_codec}{audio_channels}.{audio_meta}.{hdr}.{video_codec}-{group}")
	viper.SetDefault("preferred_language", "de")
	viper.SetDefault("original_language", "")
	viper.SetDefault("subbed_tagging", true)
	viper.SetDefault("audio_description", false)
	viper.SetDefault("source", "WEB-DL")
	viper.SetDefault("group", "PAARSEX")
	viper.SetDefault("video_codec_avc", "H.264")
	viper.SetDefault("video_codec_hevc", "H.265")
	viper.SetDefault("disable_update_check", false)
	viper.SetDefault("title_cleaning_regex", "")
	viper.SetDefault("word_separator", ".")
	viper.SetDefault("normalize_diacritics", true)
	viper.SetDefault("output_path", "")
	viper.SetDefault("replacements.title", []map[string]any{
		{
			"pattern":     `(?i)(\s*\|.*|\s*\((Teil|Part)\s*\d+\))`,
			"replacement": "",
		},
		{
			"pattern":     `&`,
			"replacement": "und",
		},
	})
	viper.SetDefault("prowlarr.movie_categories", []int{2000})
	viper.SetDefault("prowlarr.tv_categories", []int{5000})
	viper.SetDefault("disabled_checks", []string{"matroska_subtitle_inline_fonts", "matroska_ass_events"})
}

var (
	activePreset string
	// NoCache bypasses the API cache if set to true.
	NoCache bool
)

// SetPreset sets the active configuration preset.
func SetPreset(name string) {
	activePreset = name
}

func getPresetKey(key string) string {
	if activePreset != "" {
		getPresetKey := fmt.Sprintf("preset.%s.%s", activePreset, key)
		if viper.IsSet(getPresetKey) {
			return getPresetKey
		}
	}

	return key
}

// PresetExists returns true if the given preset name exists in the configuration.
func PresetExists(name string) bool {
	return viper.IsSet("preset." + name)
}

func getString(key string) string {
	return viper.GetString(getPresetKey(key))
}

func getInt(key string) int {
	return viper.GetInt(getPresetKey(key))
}

func getBool(key string) bool {
	return viper.GetBool(getPresetKey(key))
}

func getStringSlice(key string) []string {
	return viper.GetStringSlice(getPresetKey(key))
}

// GetTemplate returns the naming template from the configuration.
func GetTemplate() string {
	return getString("template")
}

// GetPreferredLanguage returns the preferred language code from the configuration.
func GetPreferredLanguage() string {
	return getString("preferred_language")
}

// GetOriginalLanguage returns the original language override from the configuration.
func GetOriginalLanguage() string {
	return getString("original_language")
}

// GetOutputPath returns the output path from the configuration.
func GetOutputPath() string {
	return getString("output_path")
}

// GetSubbedTagging returns true if subbed tagging is enabled.
func GetSubbedTagging() bool {
	return getBool("subbed_tagging")
}

// GetTitle returns the title override from the configuration.
func GetTitle() string {
	return getString("title")
}

// GetYear returns the year override from the configuration.
func GetYear() int {
	return getInt("year")
}

// GetSeason returns the season override from the configuration.
func GetSeason() int {
	return getInt("season")
}

// GetEpisode returns the episode override from the configuration.
func GetEpisode() int {
	return getInt("episode")
}

// GetDate returns the date override from the configuration.
func GetDate() string {
	return getString("date")
}

// GetEpisodeTitle returns the episode title override from the configuration.
func GetEpisodeTitle() string {
	return getString("episode_title")
}

// GetCutEdition returns the cut/edition override from the configuration.
func GetCutEdition() string {
	return getString("cut_edition")
}

// GetHDR returns the HDR override from the configuration.
func GetHDR() string {
	return getString("hdr")
}

// GetService returns the service override from the configuration.
func GetService() string {
	return getString("service")
}

// GetSource returns the source override from the configuration.
func GetSource() string {
	return getString("source")
}

// GetRepack returns true if repack override is enabled.
func GetRepack() bool {
	return getBool("repack")
}

// GetAudioDescription returns true if audio description is enabled.
func GetAudioDescription() bool {
	return getBool("audio_description")
}

// GetGroup returns the group override from the configuration.
func GetGroup() string {
	return getString("group")
}

// GetIsTV returns true if the media is a TV show.
func GetIsTV() bool {
	return getBool("is_tv")
}

// GetIsMovie returns true if the media is a movie.
func GetIsMovie() bool {
	return getBool("is_movie")
}

// GetImdbID returns the IMDB ID override from the configuration.
func GetImdbID() string {
	return getString("imdb_id")
}

// GetTmdbID returns the TMDB ID override from the configuration.
func GetTmdbID() int {
	return getInt("tmdb_id")
}

// GetTvdbID returns the TVDB ID override from the configuration.
func GetTvdbID() int {
	return getInt("tvdb_id")
}

// GetAllowSpecials returns true if special episodes are allowed.
func GetAllowSpecials() bool {
	return getBool("allow_special_matches")
}

// GetTitleCleaningRegex returns the regex used for title cleaning.
func GetTitleCleaningRegex() string {
	return getString("title_cleaning_regex")
}

// GetWordSeparator returns the character used to replace spaces in titles and codecs.
func GetWordSeparator() string {
	return getString("word_separator")
}

// GetVideoCodecAVC returns the AVC video codec name.
func GetVideoCodecAVC() string {
	return getString("video_codec_avc")
}

// GetVideoCodecHEVC returns the HEVC video codec name.
func GetVideoCodecHEVC() string {
	return getString("video_codec_hevc")
}

// Replacement represents a regex pattern and its replacement string.
type Replacement struct {
	Pattern     string `mapstructure:"pattern" toml:"pattern"`
	Replacement string `mapstructure:"replacement" toml:"replacement"`
}

// GetInputReplacements returns the input filename regex replacements.
func GetInputReplacements() []Replacement {
	var replacements []Replacement

	_ = viper.UnmarshalKey(getPresetKey("replacements.input"), &replacements)

	return replacements
}

// GetOutputReplacements returns the output filename regex replacements.
func GetOutputReplacements() []Replacement {
	var replacements []Replacement

	_ = viper.UnmarshalKey(getPresetKey("replacements.output"), &replacements)

	return replacements
}

// GetTitleReplacements returns the title cleaning regex replacements.
func GetTitleReplacements() []Replacement {
	var replacements []Replacement

	_ = viper.UnmarshalKey(getPresetKey("replacements.title"), &replacements)

	return replacements
}

// GetDisableUpdateCheck returns true if background update checks are disabled.
func GetDisableUpdateCheck() bool {
	return getBool("disable_update_check")
}

// GetNormalizeDiacritics returns true if diacritics should be normalized.
func GetNormalizeDiacritics() bool {
	return getBool("normalize_diacritics")
}

// IsCheckEnabled returns true if the given check is enabled in the configuration.
func IsCheckEnabled(checkName string) bool {
	enabledChecks := getStringSlice("enabled_checks")
	if len(enabledChecks) > 0 {
		if slices.Contains(enabledChecks, "all") {
			return true
		}

		return slices.Contains(enabledChecks, checkName)
	}

	disabledChecks := getStringSlice("disabled_checks")

	return !slices.Contains(disabledChecks, checkName)
}

// GetTmdbAPIKey returns the TMDB API key.
func GetTmdbAPIKey() string {
	return viper.GetString("api_keys.tmdb")
}

// GetTvdbAPIKey returns the TVDB API key.
func GetTvdbAPIKey() string {
	return viper.GetString("api_keys.tvdb")
}

// GetProwlarrURL returns the Prowlarr URL.
func GetProwlarrURL() string {
	return viper.GetString("prowlarr.url")
}

// GetProwlarrAPIKey returns the Prowlarr API key.
func GetProwlarrAPIKey() string {
	return viper.GetString("prowlarr.api_key")
}

// GetProwlarrIndexers returns the list of Prowlarr indexer IDs to use.
func GetProwlarrIndexers() []int {
	return viper.GetIntSlice("prowlarr.indexers")
}

// GetProwlarrMovieCategories returns the list of Prowlarr movie categories to search.
func GetProwlarrMovieCategories() []int {
	return viper.GetIntSlice("prowlarr.movie_categories")
}

// GetProwlarrTvCategories returns the list of Prowlarr TV categories to search.
func GetProwlarrTvCategories() []int {
	return viper.GetIntSlice("prowlarr.tv_categories")
}

// GetConfigFileUsed returns the path to the configuration file being used.
func GetConfigFileUsed() string {
	return viper.ConfigFileUsed()
}

// GetDefaultConfig returns the default configuration as a TOML string.
func GetDefaultConfig() string {
	return defaultConfig
}
