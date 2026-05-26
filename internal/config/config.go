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

func GetTemplate() string {
	return getString("template")
}

func GetPreferredLanguage() string {
	return getString("preferred_language")
}

func GetSource() string {
	return getString("source")
}

func GetGroup() string {
	return getString("group")
}

func GetVideoCodecAVC() string {
	return getString("video_codec_avc")
}

func GetVideoCodecHEVC() string {
	return getString("video_codec_hevc")
}

func GetTmdbApiKey() string {
	return viper.GetString("tmdb_api_key")
}

func GetTvdbApiKey() string {
	return viper.GetString("tvdb_api_key")
}
