package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestPresets(t *testing.T) {
	viper.Reset()
	InitDefaults()

	// Set global value
	viper.Set("group", "GlobalGroup")
	viper.Set("source", "GlobalSource")

	// Set preset values
	viper.Set("preset.my_preset.group", "PresetGroup")

	// Test default (no preset)
	SetPreset("")
	if GetGroup() != "GlobalGroup" {
		t.Errorf("GetGroup() = %v, want GlobalGroup", GetGroup())
	}
	if GetSource() != "GlobalSource" {
		t.Errorf("GetSource() = %v, want GlobalSource", GetSource())
	}

	// Test preset override
	SetPreset("my_preset")
	if GetGroup() != "PresetGroup" {
		t.Errorf("GetGroup() = %v, want PresetGroup", GetGroup())
	}
	// source should still be GlobalSource as it's not overridden in the preset
	if GetSource() != "GlobalSource" {
		t.Errorf("GetSource() = %v, want GlobalSource", GetSource())
	}

	// Test another preset
	SetPreset("other")
	if GetGroup() != "GlobalGroup" {
		t.Errorf("GetGroup() = %v, want GlobalGroup", GetGroup())
	}

	// Test global only setting (API Keys)
	viper.Set("tmdb_api_key", "GlobalKey")
	viper.Set("preset.my_preset.tmdb_api_key", "PresetKey")

	SetPreset("my_preset")
	if GetTmdbApiKey() != "GlobalKey" {
		t.Errorf("GetTmdbApiKey() = %v, want GlobalKey", GetTmdbApiKey())
	}
}
