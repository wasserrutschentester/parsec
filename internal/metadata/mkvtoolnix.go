package metadata

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"codeberg.org/n0ne/parsec/internal/mdb"
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

type mkvTags struct {
	XMLName xml.Name `xml:"Tags"`
	Tags    []mkvTag `xml:"Tag"`
}

type mkvTag struct {
	Targets target   `xml:"Targets"`
	Simple  []simple `xml:"Simple"`
}

type target struct {
	TargetTypeValue int `xml:"TargetTypeValue,omitempty"`
}

type simple struct {
	Name   string `xml:"Name"`
	String string `xml:"String"`
}

func isMatroska(filePath string) (bool, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	header := make([]byte, 4)
	n, err := f.Read(header)
	if err != nil {
		return false, err
	}
	if n < 4 {
		return false, nil
	}

	ebmlHeader := []byte{0x1A, 0x45, 0xDF, 0xA3}
	return bytes.Equal(header, ebmlHeader), nil
}

func checkForMatroska(filePath string) error {
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	isMKV, err := isMatroska(filePath)
	if err != nil {
		return fmt.Errorf("failed to check if file is Matroska: %w", err)
	}
	if !isMKV {
		return fmt.Errorf("file is not a Matroska file: %s", filePath)
	}
	return nil
}

func GetEbmlMetadata(filePath string) (*EbmlMetadata, error) {
	err := checkForMatroska(filePath)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("mkvmerge", "-J", filePath)
	output, err := cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("mkvmerge is not installed or not available in PATH: %w", err)
		}
		return nil, fmt.Errorf("failed to get ebml metadata: %w", err)
	}
	var metadata EbmlMetadata
	if err := json.Unmarshal(output, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ebml metadata: %w", err)
	}
	return &metadata, nil
}

func SetGlobalTags(filePath string, tags mdb.MatroskaTags) error {
	err := checkForMatroska(filePath)
	if err != nil {
		return err
	}

	tagsXML, err := createTagsXML(filePath, tags)
	if err != nil {
		return err
	}
	defer os.Remove(tagsXML)

	cmd := exec.Command("mkvpropedit", filePath, "--tags", "global:"+tagsXML)
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("mkvpropedit is not installed or not available in PATH: %w", err)
		}
		return fmt.Errorf("failed to set global tags: %w", err)
	}

	return nil
}

func createTagsXML(filePath string, tags mdb.MatroskaTags) (string, error) {
	mkvTags := mkvTags{
		Tags: []mkvTag{
			{
				Targets: target{TargetTypeValue: 50},
				Simple:  []simple{},
			},
		},
	}

	if tags.Title != "" {
		mkvTags.Tags[0].Simple = append(mkvTags.Tags[0].Simple, simple{Name: "TITLE", String: tags.Title})
	}
	if tags.Imdb != "" {
		mkvTags.Tags[0].Simple = append(mkvTags.Tags[0].Simple, simple{Name: "IMDB", String: tags.Imdb})
	}
	if tags.Tmdb != "" {
		mkvTags.Tags[0].Simple = append(mkvTags.Tags[0].Simple, simple{Name: "TMDB", String: tags.Tmdb})
	}
	if tags.Tvdb != 0 {
		mkvTags.Tags[0].Simple = append(mkvTags.Tags[0].Simple, simple{Name: "TVDB", String: strconv.Itoa(tags.Tvdb)})
	}
	if tags.Tvdb2 != "" {
		mkvTags.Tags[0].Simple = append(mkvTags.Tags[0].Simple, simple{Name: "TVDB2", String: tags.Tvdb2})
	}

	output, err := xml.MarshalIndent(mkvTags, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal tags to XML: %w", err)
	}

	xmlContent := []byte(xml.Header + string(output))

	tmpFile, err := os.CreateTemp("", "parsec-tags-*.xml")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file for tags: %w", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.Write(xmlContent); err != nil {
		return "", fmt.Errorf("failed to write tags to temp file: %w", err)
	}

	return tmpFile.Name(), nil
}
