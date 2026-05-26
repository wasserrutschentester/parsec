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

func GetTemplate() string {
	return viper.GetString("template")
}

func GetPreferredLanguage() string {
	return viper.GetString("preferred_language")
}

func GetSource() string {
	return viper.GetString("source")
}

func GetGroup() string {
	return viper.GetString("group")
}

func GetVideoCodecAVC() string {
	return viper.GetString("video_codec_avc")
}

func GetVideoCodecHEVC() string {
	return viper.GetString("video_codec_hevc")
}

func GetTmdbApiKey() string {
	return viper.GetString("tmdb_api_key")
}

func GetTvdbApiKey() string {
	return viper.GetString("tvdb_api_key")
}
