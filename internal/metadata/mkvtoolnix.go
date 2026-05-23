package metadata

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

type EbmlMetadata struct {
	Attachments []string    `json:"attachments,omitempty"`
	Chapters    []string    `json:"chapters,omitempty"`
	Errors      []string    `json:"errors,omitempty"`
	FileName    string      `json:"file_name,omitempty"`
	GlobalTags  []string    `json:"global_tags,omitempty"`
	TrackTags   []string    `json:"track_tags,omitempty"`
	Tracks      []EbmlTrack `json:"tracks,omitempty"`
}

type EbmlTrack struct {
	ID         int                    `json:"id,omitempty"`
	Codec      string                 `json:"codec,omitempty"`
	Type       string                 `json:"type,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
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
