// Package matroska provides tools for interacting with Matroska (MKV) files using mkvtoolnix.
package matroska

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/image/font/sfnt"

	"codeberg.org/upPollo/parsec/internal/mdb"
	"codeberg.org/upPollo/parsec/internal/ui"
)

var errNotMatroska = errors.New("file is not a Matroska file")

// EbmlMetadata represents the JSON output from mkvmerge -J.
type EbmlMetadata struct {
	Attachments []EbmlAttachment `json:"attachments,omitempty"`
	Container   EbmlContainer    `json:"container,omitzero"`
	Errors      []string         `json:"errors,omitempty"`
	FileName    string           `json:"file_name,omitempty"`
	Tracks      []EbmlTrack      `json:"tracks,omitempty"`
}

// EbmlContainer represents the global container properties.
type EbmlContainer struct {
	Properties EbmlContainerProperties `json:"properties,omitzero"`
}

// EbmlContainerProperties contains global properties of a Matroska container.
type EbmlContainerProperties struct {
	Title              string `json:"title,omitempty"`
	WritingApplication string `json:"writing_application,omitempty"`
}

// EbmlTrack represents a single track in a Matroska container.
type EbmlTrack struct {
	ID         int                 `json:"id,omitempty"`
	Codec      string              `json:"codec,omitempty"`
	Type       string              `json:"type,omitempty"`
	Properties EbmlTrackProperties `json:"properties,omitzero"`
	TypeOrder  int                 `json:"-"`
}

// EbmlTrackProperties contains detailed properties of a Matroska track.
type EbmlTrackProperties struct {
	Language                  string `json:"language,omitempty"`
	LanguageIetf              string `json:"language_ietf,omitempty"`
	Name                      string `json:"track_name,omitempty"`
	Source                    string `json:"tag_source,omitempty"`
	Number                    int    `json:"number,omitempty"`
	IndexEntries              int    `json:"num_index_entries,omitempty"`
	Enabled                   bool   `json:"enabled_track,omitempty"`
	Default                   bool   `json:"default_track,omitempty"`
	Forced                    bool   `json:"forced_track,omitempty"`
	HearingImpaired           bool   `json:"flag_hearing_impaired,omitempty"`
	VisualImpaired            bool   `json:"flag_visual_impaired,omitempty"`
	Commentary                bool   `json:"flag_commentary,omitempty"`
	OriginalLanguage          bool   `json:"flag_original,omitempty"`
	TextDescriptions          bool   `json:"flag_text_descriptions,omitempty"`
	TextSubtitles             bool   `json:"text_subtitles,omitempty"`
	ContentEncodingAlgorithms string `json:"content_encoding_algorithms,omitempty"`
	CodecPrivate              string `json:"codec_private_data,omitempty"`
	PixelWidth                int    `json:"pixel_width,omitempty"`
	PixelHeight               int    `json:"pixel_height,omitempty"`
	DisplayWidth              int    `json:"display_width,omitempty"`
	DisplayHeight             int    `json:"display_height,omitempty"`
	PixelCroppingLeft         int    `json:"pixel_cropping_left,omitempty"`
	PixelCroppingTop          int    `json:"pixel_cropping_top,omitempty"`
	PixelCroppingRight        int    `json:"pixel_cropping_right,omitempty"`
	PixelCroppingBottom       int    `json:"pixel_cropping_bottom,omitempty"`
	Delay                     int64  `json:"packet_delay,omitempty"`
}

// DecodeCodecPrivate decodes the base16/hex encoded CodecPrivate string.
func (p EbmlTrackProperties) DecodeCodecPrivate() ([]byte, error) {
	if p.CodecPrivate == "" {
		return nil, nil
	}

	data, err := hex.DecodeString(p.CodecPrivate)
	if err != nil {
		return nil, fmt.Errorf("failed to decode codec private data: %w", err)
	}

	return data, nil
}

// EbmlAttachment represents an attachment in a Matroska container.
type EbmlAttachment struct {
	ID          int    `json:"id,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	FileName    string `json:"file_name,omitempty"`
	Size        int    `json:"size,omitempty"`
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
		return false, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	header := make([]byte, 4)

	n, err := f.Read(header)
	if err != nil {
		return false, fmt.Errorf("failed to read file header: %w", err)
	}

	if n < 4 {
		return false, nil
	}

	ebmlHeader := []byte{0x1A, 0x45, 0xDF, 0xA3}

	return bytes.Equal(header, ebmlHeader), nil
}

// CheckForMatroska verifies if the given file exists and is a valid Matroska container.
func CheckForMatroska(filePath string) error {
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	isMKV, err := isMatroska(filePath)
	if err != nil {
		return fmt.Errorf("failed to check if file is Matroska: %w", err)
	}

	if !isMKV {
		return fmt.Errorf("%w: %s", errNotMatroska, filePath)
	}

	return nil
}

// GetEbmlMetadata runs mkvmerge -J on the file to extract detailed EBML metadata.
func GetEbmlMetadata(filePath string) (*EbmlMetadata, error) {
	err := CheckForMatroska(filePath)
	if err != nil {
		return nil, err
	}

	ui.PrintDebug("Executing: mkvmerge -J " + ui.AnonymizePath(filePath))
	cmd := exec.CommandContext(context.Background(), "mkvmerge", "-J", filePath)

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

	metadata.countTypes()

	return &metadata, nil
}

// ExtractTrack uses mkvextract to extract a specific track from a Matroska file.
func ExtractTrack(filePath string, trackID int) ([]byte, error) {
	err := CheckForMatroska(filePath)
	if err != nil {
		return nil, err
	}

	tmpFile, err := os.CreateTemp("", "parsec-extract-*.ass")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	tmpFilePath := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	defer func() {
		_ = os.Remove(tmpFilePath)
	}()

	ui.PrintDebug(fmt.Sprintf("Executing: mkvextract %s tracks %d:%s", ui.AnonymizePath(filePath), trackID, tmpFilePath))
	cmd := exec.CommandContext(context.Background(), "mkvextract", filePath, "tracks", fmt.Sprintf("%d:%s", trackID, tmpFilePath))

	_, err = cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("mkvextract is not installed or not available in PATH: %w", err)
		}

		return nil, fmt.Errorf("failed to extract track %d: %w", trackID, err)
	}

	content, err := os.ReadFile(tmpFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read extracted track: %w", err)
	}

	return content, nil
}

// ExtractAttachments uses mkvextract to extract multiple attachments from a Matroska file in one command.
// It returns a map where the key is the attachment ID and the value is the extracted content.
func ExtractAttachments(filePath string, ids []int) (map[int][]byte, error) {
	if len(ids) == 0 {
		return make(map[int][]byte), nil
	}

	err := CheckForMatroska(filePath)
	if err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "parsec-attachments-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	args := []string{filePath, "attachments"}
	idToPath := make(map[int]string)

	for _, id := range ids {
		tmpPath := filepath.Join(tmpDir, fmt.Sprintf("attachment-%d", id))
		args = append(args, fmt.Sprintf("%d:%s", id, tmpPath))
		idToPath[id] = tmpPath
	}

	ui.PrintDebug("Executing: mkvextract " + ui.AnonymizePath(filePath) + " attachments " + strings.Join(args[2:], " "))
	cmd := exec.CommandContext(context.Background(), "mkvextract", args...)

	_, err = cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("mkvextract is not installed or not available in PATH: %w", err)
		}

		return nil, fmt.Errorf("failed to extract attachments: %w", err)
	}

	results := make(map[int][]byte)

	for id, path := range idToPath {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read extracted attachment %d: %w", id, err)
		}

		results[id] = content
	}

	return results, nil
}

// GetFontNames extracts the internal Family and Full names from font data.
func GetFontNames(data []byte) ([]string, error) {
	f, err := sfnt.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse font: %w", err)
	}

	var names []string

	var b sfnt.Buffer

	// NameIDFamily (0) and NameIDFull (4) are the most common ways fonts are identified.
	// NameIDTypographicFamily (15) is also important for some modern fonts.
	for _, id := range []sfnt.NameID{sfnt.NameIDFamily, sfnt.NameIDFull, sfnt.NameIDTypographicFamily} {
		name, err := f.Name(&b, id)
		if err == nil && name != "" {
			names = append(names, name)
		}
	}

	return names, nil
}

func (metadata *EbmlMetadata) countTypes() {
	numVideo, numAudio, numSubtitles := 0, 0, 0

	for i := range metadata.Tracks {
		switch metadata.Tracks[i].Type {
		case "video":
			numVideo++
			metadata.Tracks[i].TypeOrder = numVideo
		case "audio":
			numAudio++
			metadata.Tracks[i].TypeOrder = numAudio
		case "subtitles", "subtitle":
			numSubtitles++
			metadata.Tracks[i].TypeOrder = numSubtitles
		}
	}
}

// HasVisualImpairedAudio returns true if the metadata contains an audio track with the visual impaired flag set.
func (metadata *EbmlMetadata) HasVisualImpairedAudio() bool {
	for _, track := range metadata.Tracks {
		if track.Type == "audio" && track.Properties.VisualImpaired {
			ui.PrintDebug("found Visual Impaired audio Track")

			return true
		}
	}

	return false
}

// SetGlobalTags uses mkvpropedit to set global tags (TITLE, IMDB, TMDB, TVDB) on a Matroska file.
func SetGlobalTags(filePath string, tags mdb.MatroskaTags) error {
	err := CheckForMatroska(filePath)
	if err != nil {
		return err
	}

	tagsXML, err := createTagsXML(tags)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tagsXML) }()

	ui.PrintDebug(fmt.Sprintf("Executing: mkvpropedit %s --tags global:%s", ui.AnonymizePath(filePath), tagsXML))

	cmd := exec.CommandContext(context.Background(), "mkvpropedit", filePath, "--tags", "global:"+tagsXML)
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("mkvpropedit is not installed or not available in PATH: %w", err)
		}

		return fmt.Errorf("failed to set global tags: %w", err)
	}

	return nil
}

func createTagsXML(tags mdb.MatroskaTags) (string, error) {
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
	defer func() { _ = tmpFile.Close() }()

	if _, err := tmpFile.Write(xmlContent); err != nil {
		return "", fmt.Errorf("failed to write tags to temp file: %w", err)
	}

	return tmpFile.Name(), nil
}
