package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

//nolint:paralleltest // depends on shared global state (viper)
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
	viper.Set("api_keys.tmdb", "GlobalKey")
	viper.Set("preset.my_preset.api_keys.tmdb", "PresetKey")

	SetPreset("my_preset")

	if GetTmdbAPIKey() != "GlobalKey" {
		t.Errorf("GetTmdbAPIKey() = %v, want GlobalKey", GetTmdbAPIKey())
	}
}

//nolint:paralleltest // depends on shared global state (viper)
func TestIsCheckEnabled(t *testing.T) {
	viper.Reset()
	InitDefaults()

	// Default state: all enabled
	if !IsCheckEnabled("some_check") {
		t.Errorf("IsCheckEnabled(some_check) = false, want true (default)")
	}

	// Test blacklist (disabled_checks)
	viper.Set("disabled_checks", []string{"bad_check"})

	if IsCheckEnabled("bad_check") {
		t.Errorf("IsCheckEnabled(bad_check) = true, want false (blacklisted)")
	}

	if !IsCheckEnabled("good_check") {
		t.Errorf("IsCheckEnabled(good_check) = false, want true")
	}
}

//nolint:paralleltest // depends on shared global state (viper)
func TestGetOutputPath(t *testing.T) {
	viper.Reset()
	InitDefaults()

	// Default state: empty
	if GetOutputPath() != "" {
		t.Errorf("GetOutputPath() = %v, want empty string (default)", GetOutputPath())
	}

	// Set global value
	viper.Set("output_path", "/tmp/parsec")

	if GetOutputPath() != "/tmp/parsec" {
		t.Errorf("GetOutputPath() = %v, want /tmp/parsec", GetOutputPath())
	}

	// Test preset override
	viper.Set("preset.my_preset.output_path", "/tmp/parsec_preset")
	SetPreset("my_preset")

	if GetOutputPath() != "/tmp/parsec_preset" {
		t.Errorf("GetOutputPath() = %v, want /tmp/parsec_preset", GetOutputPath())
	}
}

//nolint:paralleltest // depends on shared global state (viper)
func TestIsCheckEnabledWhitelist(t *testing.T) {
	// Test whitelist (enabled_checks)
	viper.Reset()
	InitDefaults()
	viper.Set("enabled_checks", []string{"only_this"})

	if !IsCheckEnabled("only_this") {
		t.Errorf("IsCheckEnabled(only_this) = false, want true (whitelisted)")
	}

	if IsCheckEnabled("other_check") {
		t.Errorf("IsCheckEnabled(other_check) = true, want false (not in whitelist)")
	}

	// Test preset specific blacklist
	viper.Reset()
	InitDefaults()
	viper.Set("disabled_checks", []string{"global_disabled"})
	viper.Set("preset.my_preset.disabled_checks", []string{"preset_disabled"})

	SetPreset("")

	if IsCheckEnabled("global_disabled") {
		t.Errorf("IsCheckEnabled(global_disabled) = true, want false")
	}

	if !IsCheckEnabled("preset_disabled") {
		t.Errorf("IsCheckEnabled(preset_disabled) = false, want true")
	}

	SetPreset("my_preset")

	if !IsCheckEnabled("global_disabled") {
		t.Errorf("IsCheckEnabled(global_disabled) = false, want true (overridden by preset)")
	}

	if IsCheckEnabled("preset_disabled") {
		t.Errorf("IsCheckEnabled(preset_disabled) = true, want false")
	}
}

//nolint:paralleltest // depends on shared global state (viper)
func TestIsCheckEnabledSpecialValues(t *testing.T) {
	viper.Reset()
	InitDefaults()

	// Test "all" in enabled_checks
	viper.Set("enabled_checks", []string{"all"})

	if !IsCheckEnabled("some_check") {
		t.Errorf("IsCheckEnabled(some_check) with 'all' = false, want true")
	}

	if !IsCheckEnabled(CheckMatroskaSubtitleInlineFonts) {
		t.Errorf("IsCheckEnabled(matroska_subtitle_inline_fonts) with 'all' = false, want true")
	}

	// Test empty disabled_checks (should also enable everything)
	viper.Reset()
	InitDefaults()
	viper.Set("disabled_checks", []string{})

	if !IsCheckEnabled(CheckMatroskaSubtitleInlineFonts) {
		t.Errorf("IsCheckEnabled(matroska_subtitle_inline_fonts) with [] = false, want true")
	}
}

//nolint:paralleltest // depends on shared global state (viper)
func TestGetOriginalLanguage(t *testing.T) {
	viper.Reset()
	InitDefaults()

	// Default state: empty
	if GetOriginalLanguage() != "" {
		t.Errorf("GetOriginalLanguage() = %v, want empty string (default)", GetOriginalLanguage())
	}

	// Set global value
	viper.Set("original_language", "en")

	if GetOriginalLanguage() != "en" {
		t.Errorf("GetOriginalLanguage() = %v, want en", GetOriginalLanguage())
	}

	// Test preset override
	viper.Set("preset.my_preset.original_language", "fr")
	SetPreset("my_preset")

	if GetOriginalLanguage() != "fr" {
		t.Errorf("GetOriginalLanguage() = %v, want fr", GetOriginalLanguage())
	}
}

//nolint:paralleltest // depends on shared global state (viper)
func TestListPresets(t *testing.T) {
	t.Run("returns empty slice when no presets are defined", func(t *testing.T) {
		viper.Reset()
		InitDefaults()

		got := ListPresets()
		if len(got) != 0 {
			t.Errorf("ListPresets() = %v, want []", got)
		}
	})

	t.Run("returns sorted preset names without descriptions", func(t *testing.T) {
		viper.Reset()
		InitDefaults()
		viper.Set("preset.zebra.group", "Z")
		viper.Set("preset.alpha.source", "WEB-DL")

		got := ListPresets()
		want := []string{"alpha", "zebra"}

		if len(got) != len(want) {
			t.Fatalf("ListPresets() len = %d, want %d; got %v", len(got), len(want), got)
		}

		for i := range want {
			if got[i] != want[i] {
				t.Errorf("ListPresets()[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("appends tab-separated description when present", func(t *testing.T) {
		viper.Reset()
		InitDefaults()
		viper.Set("preset.foo.group", "Foo")
		viper.Set("preset.foo.description", "A foo preset")
		viper.Set("preset.bar.source", "BluRay")
		// bar has no description

		got := ListPresets()
		want := []string{"bar", "foo\tA foo preset"}

		if len(got) != len(want) {
			t.Fatalf("ListPresets() len = %d, want %d; got %v", len(got), len(want), got)
		}

		for i := range want {
			if got[i] != want[i] {
				t.Errorf("ListPresets()[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})
}

//nolint:paralleltest // depends on shared global state (viper)
func TestGetTemplate(t *testing.T) {
	viper.Reset()
	InitDefaults()

	defaultTmpl := GetTemplate()
	if defaultTmpl == "" || !strings.Contains(defaultTmpl, "{{join") {
		t.Errorf("GetTemplate() default = %v, want template containing {{join", defaultTmpl)
	}

	presets := map[string]string{
		// Updated existing presets converted to Go templates
		"movie-remux": `{{join "." .Title .YearTag .Resolution .Source "REMUX" .VideoCodec .AudioSpec}}-{{.Group}}`,
		"special":     `{{join "." .Title .YearTag .SeasonEpisode .Date .EpisodeTitle .Resolution .Service .Source .VideoCodec}}-{{.Group}}`,
		"anime":       `[{{.Group}}] {{.Title}} - {{.SeasonEpisode}} - ({{join " " .Source .Resolution .VideoCodec .AudioCodec}}){{when .IsDualAudio " Dual-Audio"}} [{{.CRC32}}]`,
		// Showcase templates demonstrating parsec Go template capabilities
		"foreign-aka":       `{{join "." (aka .OriginalTitle .Title .YearTag) .Edition (vcodec "encode" .VideoCodec) .AudioSpec}}-{{.Group}}`,
		"p2p-parenthetical": `{{.Title}} ({{.YearTag}}) {{.SeasonEpisode}} ({{join " " .Resolution .Source (when .IsRemux "REMUX") .VideoCodec (when .IsMultiAudio "MULTI" .IsDualAudio "DUAL" (titleCase .LanguageName))}} - {{.Group}}){{when .IsSubbed " [SUBBED]"}}`,
		"daily-show":        `{{join "." .Title (parseDate "2006.01.02" .Date) .LanguageName .Resolution .Service .Source .VideoCodec}}-{{.Group}}`,
	}

	for name, tmpl := range presets {
		if errs := validateTemplate(tmpl, "template"); len(errs) > 0 {
			t.Fatalf("preset %q template failed validation: %v", name, errs)
		}

		viper.Set("preset."+name+".template", tmpl)
		SetPreset(name)

		if got := GetTemplate(); got != tmpl {
			t.Errorf("GetTemplate() for %s = %v, want %v", name, got, tmpl)
		}
	}

	// Reset preset returns default
	SetPreset("")

	if got := GetTemplate(); got != defaultTmpl {
		t.Errorf("GetTemplate() after reset = %v, want default %v", got, defaultTmpl)
	}
}
