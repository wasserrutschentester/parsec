package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pelletier/go-toml/v2"

	"codeberg.org/upPollo/parsec/internal/ui"
)

//nolint:funlen,paralleltest // test cases cover many validation scenarios; depends on shared global state (ui.IsSilent)
func TestValidate(t *testing.T) {
	// Silence UI output during tests
	ui.IsSilent = true
	defer func() { ui.IsSilent = false }()

	t.Run("DefaultConfig", func(t *testing.T) {
		tmpDir := t.TempDir()
		confPath := filepath.Join(tmpDir, "config.toml")

		err := os.WriteFile(confPath, []byte(GetDefaultConfig()), 0o644)
		if err != nil {
			t.Fatalf("Failed to write default config: %v", err)
		}

		data, _ := os.ReadFile(confPath)

		var configMap map[string]any

		_ = toml.Unmarshal(data, &configMap)

		errors := validateMapTypes(configMap, "", expectedTypes)
		if len(errors) > 0 {
			t.Errorf("Default config produced validation errors: %v", errors)
		}
	})

	t.Run("BadConfig", func(t *testing.T) {
		badConfig := `
group = 123
source = "WEB-DL"
repack = "true"
preferred_language = "not-a-lang"
template = "{title}.{invalid_key}"
unknown_key = "value"

[api_keys]
tmdb = 456
unknown_api = true

[preset.bad]
is_tv = "yes"
tmdb_id = "abc"
template = "{year}{bad_token}"
enabled_checks = ["mediainfo_bitrate", "unknown_identifier"]

[[replacements.input]]
pattern = "[invalid_regex("
replacement = ""
`

		var configMap map[string]any

		_ = toml.Unmarshal([]byte(badConfig), &configMap)

		errors := validateMapTypes(configMap, "", expectedTypes)

		expectedErrors := map[string]bool{
			"Invalid type for 'group': expected string, got int64":     true,
			"Invalid type for 'repack': expected bool, got string":     true,
			"Invalid language tag in 'preferred_language': not-a-lang": true,
			"Invalid template key in 'template': {invalid_key}":        true,

			"Unknown configuration key: 'unknown_key'": true,

			"Invalid type for 'api_keys.tmdb': expected string, got int64":                                                 true,
			"Unknown configuration key: 'api_keys.unknown_api'":                                                            true,
			"Invalid type for 'preset.bad.is_tv': expected bool, got string":                                               true,
			"Invalid type for 'preset.bad.tmdb_id': expected int64, got string":                                            true,
			"Invalid template key in 'preset.bad.template': {bad_token}":                                                   true,
			"Invalid check identifier in 'preset.bad.enabled_checks': unknown_identifier":                                  true,
			"Invalid regex pattern in 'replacements.input[0]': error parsing regexp: missing closing ]: `[invalid_regex(`": true,
		}

		for _, err := range errors {
			if !expectedErrors[err] {
				t.Errorf("Unexpected error found: %s", err)
			}

			delete(expectedErrors, err)
		}

		for err := range expectedErrors {
			t.Errorf("Expected error not found: %s", err)
		}
	})
}

func TestValidateLanguage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		lang    string
		wantErr bool
	}{
		{lang: "de", wantErr: false},
		{lang: "de-DE", wantErr: false},
		{lang: "chi", wantErr: false}, // 3-letter ISO 639-2/3 code
		{lang: "zh-Hant", wantErr: false},
		// Regression: language.Parse accepts this without error (base
		// language "not", a real obscure ISO 639-3 code, plus a private
		// "a-lang" BCP 47 extension it silently tolerates). Must still be
		// rejected since it's not a plain media language tag.
		{lang: "not-a-lang", wantErr: true},
		{lang: "de-1901", wantErr: true}, // variant subtag, not supported
		{lang: "xx", wantErr: true},
		{lang: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			t.Parallel()

			got := validateLanguage(tt.lang, "preferred_language")
			if (len(got) > 0) != tt.wantErr {
				t.Errorf("validateLanguage(%q) = %v, wantErr %v", tt.lang, got, tt.wantErr)
			}
		})
	}
}
