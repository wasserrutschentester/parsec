package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pelletier/go-toml/v2"

	"codeberg.org/upPollo/parsec/internal/ui"
)

//nolint:funlen
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
`

		var configMap map[string]any

		_ = toml.Unmarshal([]byte(badConfig), &configMap)

		errors := validateMapTypes(configMap, "", expectedTypes)

		expectedErrors := map[string]bool{
			"Invalid type for 'group': expected string, got int64":                                                       true,
			"Invalid type for 'repack': expected bool, got string":                                                       true,
			"Invalid language tag in 'preferred_language': not-a-lang (must be a language that has a 2-letter ISO code)": true,
			"Invalid template key in 'template': {invalid_key}":                                                          true,

			"Unknown configuration key: 'unknown_key'": true,

			"Invalid type for 'api_keys.tmdb': expected string, got int64":                true,
			"Unknown configuration key: 'api_keys.unknown_api'":                           true,
			"Invalid type for 'preset.bad.is_tv': expected bool, got string":              true,
			"Invalid type for 'preset.bad.tmdb_id': expected int64, got string":           true,
			"Invalid template key in 'preset.bad.template': {bad_token}":                  true,
			"Invalid check identifier in 'preset.bad.enabled_checks': unknown_identifier": true,
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
