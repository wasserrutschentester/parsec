package config

import (
	_ "embed"
	"fmt"

	"github.com/spf13/viper"
)

//go:embed default_config.toml
var defaultConfig string

func InitDefaults() {
	viper.SetDefault("template", "{title}.{year}.{season_id}{episode_id}.{date}.{cut_edition}.{episode_title}.{language}.{language_ext}.{accessibility}.{repack}.{resolution}.{service}.{source}.{audio_codec}{audio_channels}.{audio_meta}.{hdr}.{video_codec}-{group}")
	viper.SetDefault("preferred_language", "de")
	viper.SetDefault("subbed_tagging", true)
	viper.SetDefault("audio_description", false)
	viper.SetDefault("source", "WEB-DL")
	viper.SetDefault("group", "PAARSEX")
	viper.SetDefault("video_codec_avc", "H.264")
	viper.SetDefault("video_codec_hevc", "H.265")
	viper.SetDefault("disable_update_check", false)
	viper.SetDefault("title_cleaning_regex", "")
	viper.SetDefault("prowlarr.movie_categories", []int{2000})
	viper.SetDefault("prowlarr.tv_categories", []int{5000})
}

var (
	activePreset string
	NoCache      bool
)

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

func GetTemplate() string {
	return getString("template")
}

func GetPreferredLanguage() string {
	return getString("preferred_language")
}

func GetSubbedTagging() bool {
	return getBool("subbed_tagging")
}

func GetTitle() string {
	return getString("title")
}

func GetYear() int {
	return getInt("year")
}

func GetSeason() int {
	return getInt("season")
}

func GetEpisode() int {
	return getInt("episode")
}

func GetDate() string {
	return getString("date")
}

func GetEpisodeTitle() string {
	return getString("episode_title")
}

func GetCutEdition() string {
	return getString("cut_edition")
}

func GetHDR() string {
	return getString("hdr")
}

func GetService() string {
	return getString("service")
}

func GetSource() string {
	return getString("source")
}

func GetRepack() bool {
	return getBool("repack")
}

func GetAudioDescription() bool {
	return getBool("audio_description")
}

func GetGroup() string {
	return getString("group")
}

func GetIsTV() bool {
	return getBool("is_tv")
}

func GetIsMovie() bool {
	return getBool("is_movie")
}

func GetImdbID() string {
	return getString("imdb_id")
}

func GetTmdbID() int {
	return getInt("tmdb_id")
}

func GetTvdbID() int {
	return getInt("tvdb_id")
}

func GetAllowSpecials() bool {
	return getBool("allow_special_matches")
}

func GetTitleCleaningRegex() string {
	return getString("title_cleaning_regex")
}

func GetVideoCodecAVC() string {
	return getString("video_codec_avc")
}

func GetVideoCodecHEVC() string {
	return getString("video_codec_hevc")
}

func GetDisableUpdateCheck() bool {
	return getBool("disable_update_check")
}

func IsCheckEnabled(checkName string) bool {
	enabledChecks := getStringSlice("enabled_checks")
	if len(enabledChecks) > 0 {
		for _, c := range enabledChecks {
			if c == checkName {
				return true
			}
		}
		return false
	}

	disabledChecks := getStringSlice("disabled_checks")
	for _, c := range disabledChecks {
		if c == checkName {
			return false
		}
	}

	return true
}

func GetTmdbApiKey() string {
	return viper.GetString("api_keys.tmdb")
}

func GetTvdbApiKey() string {
	return viper.GetString("api_keys.tvdb")
}

func GetProwlarrUrl() string {
	return viper.GetString("prowlarr.url")
}

func GetProwlarrApiKey() string {
	return viper.GetString("prowlarr.api_key")
}

func GetProwlarrIndexers() []int {
	return viper.GetIntSlice("prowlarr.indexers")
}

func GetProwlarrMovieCategories() []int {
	return viper.GetIntSlice("prowlarr.movie_categories")
}

func GetProwlarrTvCategories() []int {
	return viper.GetIntSlice("prowlarr.tv_categories")
}

func GetConfigFileUsed() string {
	return viper.ConfigFileUsed()
}

func GetDefaultConfig() string {
	return defaultConfig
}
