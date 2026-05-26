package config

import (
	"github.com/spf13/viper"
)

func InitDefaults() {
	viper.SetDefault("template", "{title}.{year}.{season_id}{episode_id}.{date}.{episode_title}.{language}.{repack}.{resolution}.{service}.{source}.{audio_codec}{audio_channels}.{video_codec}-{group}")
	viper.SetDefault("preferred_language", "de")
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

func GetService() string {
	return getString("service")
}

func GetSource() string {
	return getString("source")
}

func GetRepack() bool {
	return getBool("repack")
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

func GetTmdbApiKey() string {
	return viper.GetString("api_keys.tmdb")
}

func GetTvdbApiKey() string {
	return viper.GetString("api_keys.tvdb")
}
