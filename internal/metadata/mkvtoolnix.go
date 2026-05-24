package metadata

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

type EbmlMetadata struct {
	Attachments []string    `json:"attachments,omitempty"`
	Errors      []string    `json:"errors,omitempty"`
	FileName    string      `json:"file_name,omitempty"`
	Tracks      []EbmlTrack `json:"tracks,omitempty"`
}

type EbmlTrack struct {
	ID         int                 `json:"id,omitempty"`
	Codec      string              `json:"codec,omitempty"`
	Type       string              `json:"type,omitempty"`
	Properties EbmlTrackProperties `json:"properties,omitempty"`
}

type EbmlTrackProperties struct {
	Language         string `json:"language,omitempty"`
	LanguageIetf     string `json:"language_ietf,omitempty"`
	Name             string `json:"track_name,omitempty"`
	Source           string `json:"tag_source,omitempty"`
	Enabled          bool   `json:"flag_enabled,omitempty"`
	Default          bool   `json:"flag_default,omitempty"`
	Forced           bool   `json:"flag_forced,omitempty"`
	HearingImpaired  bool   `json:"flag_hearing_impaired,omitempty"`
	VisualImpaired   bool   `json:"flag_visual_impaired,omitempty"`
	Commentary       bool   `json:"flag_commentary,omitempty"`
	OriginalLanguage bool   `json:"flag_original,omitempty"`
	TextDescriptions bool   `json:"flag_text_descriptions,omitempty"`
}

func GetEbmlMetadata(filePath string) (*EbmlMetadata, error) {
	cmd := exec.Command("mkvmerge", "-J", filePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get ebml metadata: %w", err)
	}
	var metadata EbmlMetadata
	if err := json.Unmarshal(output, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ebml metadata: %w", err)
	}
	return &metadata, nil
}
