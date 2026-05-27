package config

import (
	"github.com/spf13/viper"
)

func InitDefaults() {
	viper.SetDefault("template", "{title}.{year}.{season_id}{episode_id}.{date}.{cut_edition}.{episode_title}.{language}.{language_ext}.{accessibility}.{repack}.{resolution}.{service}.{source}.{audio_codec}{audio_channels}.{audio_meta}.{hdr}.{video_codec}-{group}")
	viper.SetDefault("preferred_language", "de")
	viper.SetDefault("subbed_tagging", true)
	viper.SetDefault("audio_description", false)
	viper.SetDefault("source", "WEB-DL")
	viper.SetDefault("group", "4Rocket")
	viper.SetDefault("video_codec_avc", "H.264")
	viper.SetDefault("video_codec_hevc", "H.265")
}

var activePreset string

func SetPreset(name string) {
	activePreset = name
}

func getString(key string) string {
	if activePreset != "" {
		presetKey := "preset." + activePreset + "." + key
		if viper.IsSet(presetKey) {
			return viper.GetString(presetKey)
		}
	}
	return viper.GetString(key)
}

func getInt(key string) int {
	if activePreset != "" {
		presetKey := "preset." + activePreset + "." + key
		if viper.IsSet(presetKey) {
			return viper.GetInt(presetKey)
		}
	}
	return viper.GetInt(key)
}

func getBool(key string) bool {
	if activePreset != "" {
		presetKey := "preset." + activePreset + "." + key
		if viper.IsSet(presetKey) {
			return viper.GetBool(presetKey)
		}
	}
	return viper.GetBool(key)
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

func GetVideoCodecAVC() string {
	return getString("video_codec_avc")
}

func GetVideoCodecHEVC() string {
	return getString("video_codec_hevc")
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

func getStringSlice(key string) []string {
	if activePreset != "" {
		presetKey := "preset." + activePreset + "." + key
		if viper.IsSet(presetKey) {
			return viper.GetStringSlice(presetKey)
		}
	}
	return viper.GetStringSlice(key)
}

func GetTmdbApiKey() string {
	return viper.GetString("api_keys.tmdb")
}

func GetTvdbApiKey() string {
	return viper.GetString("api_keys.tvdb")
}
