package config

import (
	"github.com/spf13/viper"
)

func InitDefaults() {
	viper.SetDefault("template", "{title}.{year}.{season_id}{episode_id}.{episode_title}.{language}.{repack}.{resolution}.{service}.{source}.{audio_codec}{audio_channels}.{video_codec}-{group}")
	viper.SetDefault("preferred_language", "de")
	viper.SetDefault("source", "WEB-DL")
	viper.SetDefault("group", "4Rocket")
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

func GetTmdbApiKey() string {
	return viper.GetString("tmdb_api_key")
}
