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
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/image/font/sfnt"

	"codeberg.org/upPollo/parsec/internal/mdb"
	"codeberg.org/upPollo/parsec/internal/ui"
)

var errNotMatroska = errors.New("file is not a Matroska file")

// EbmlMetadata represents the JSON output from mkvmerge -J.
type EbmlMetadata struct {
	Attachments                 []EbmlAttachment `json:"attachments,omitempty"`
	Container                   EbmlContainer    `json:"container,omitzero"`
	Errors                      []string         `json:"errors,omitempty"`
	FileName                    string           `json:"file_name,omitempty"`
	Tracks                      []EbmlTrack      `json:"tracks,omitempty"`
	Chapters                    []EbmlChapters   `json:"chapters,omitempty"`
	GlobalTags                  []EbmlGlobalTag  `json:"global_tags,omitempty"`
	IdentificationFormatVersion int              `json:"identification_format_version,omitempty"`
	TrackTags                   []EbmlTrackTag   `json:"track_tags,omitempty"`
	Warnings                    []string         `json:"warnings,omitempty"`
}

// EbmlGlobalTag represents global tag metadata from mkvmerge -J.
type EbmlGlobalTag struct {
	NumEntries int `json:"num_entries"`
}

// EbmlTrackTag represents track tag metadata from mkvmerge -J.
type EbmlTrackTag struct {
	NumEntries int `json:"num_entries"`
	TrackID    int `json:"track_id"`
}

// EbmlChapters represents chapter metadata from mkvmerge or mkvextract.
type EbmlChapters struct {
	XMLName    xml.Name      `json:"-" xml:"Chapters"`
	NumEntries int           `json:"num_entries" xml:"-"`
	Editions   []EbmlEdition `json:"editions" xml:"EditionEntry"`
}

// EbmlEdition represents an edition of chapters.
type EbmlEdition struct {
	UID      uint64            `json:"uid" xml:"EditionUID"`
	Chapters []EbmlChapterAtom `json:"chapters" xml:"ChapterAtom"`
}

// EbmlChapterAtom represents an individual chapter marker.
type EbmlChapterAtom struct {
	UID          uint64        `json:"uid" xml:"ChapterUID"`
	TimeStart    int64         `json:"time_start" xml:"-"`
	TimeStartXML string        `json:"-" xml:"ChapterTimeStart"`
	Display      []EbmlDisplay `json:"display" xml:"ChapterDisplay"`
}

// EbmlDisplay represents display info for a chapter atom.
type EbmlDisplay struct {
	Language string `json:"language" xml:"ChapterLanguage"`
	String   string `json:"string" xml:"ChapterString"`
}

// EbmlContainer represents the global container properties.
type EbmlContainer struct {
	Properties EbmlContainerProperties `json:"properties,omitzero"`
	Recognized bool                    `json:"recognized"`
	Supported  bool                    `json:"supported"`
	Type       string                  `json:"type,omitempty"`
}

// EbmlContainerProperties contains global properties of a Matroska container.
type EbmlContainerProperties struct {
	Title                 string        `json:"title,omitempty"`
	WritingApplication    string        `json:"writing_application,omitempty"`
	Duration              int64         `json:"duration,omitempty"`
	ContainerType         int           `json:"container_type,omitempty"`
	DateLocal             string        `json:"date_local,omitempty"`
	DateUtc               string        `json:"date_utc,omitempty"`
	IsProvidingTimestamps bool          `json:"is_providing_timestamps,omitempty"`
	MuxingApplication     string        `json:"muxing_application,omitempty"`
	NextSegmentUID        string        `json:"next_segment_uid,omitempty"`
	OtherFile             []string      `json:"other_file,omitempty"`
	Playlist              bool          `json:"playlist,omitempty"`
	PlaylistChapters      int           `json:"playlist_chapters,omitempty"`
	PlaylistDuration      int64         `json:"playlist_duration,omitempty"`
	PlaylistFile          []string      `json:"playlist_file,omitempty"`
	PlaylistSize          int64         `json:"playlist_size,omitempty"`
	PreviousSegmentUID    string        `json:"previous_segment_uid,omitempty"`
	Programs              []EbmlProgram `json:"programs,omitempty"`
	SegmentUID            string        `json:"segment_uid,omitempty"`
	TimestampScale        int64         `json:"timestamp_scale,omitempty"`
}

// EbmlProgram represents multiplexed program properties.
type EbmlProgram struct {
	ProgramNumber   int    `json:"program_number"`
	ServiceName     string `json:"service_name,omitempty"`
	ServiceProvider string `json:"service_provider,omitempty"`
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
	Language                     string  `json:"language,omitempty"`
	LanguageIetf                 string  `json:"language_ietf,omitempty"`
	Name                         string  `json:"track_name,omitempty"`
	Source                       string  `json:"tag_source,omitempty"`
	Number                       int     `json:"number,omitempty"`
	IndexEntries                 int     `json:"num_index_entries,omitempty"`
	Enabled                      bool    `json:"enabled_track,omitempty"`
	Default                      bool    `json:"default_track,omitempty"`
	Forced                       bool    `json:"forced_track,omitempty"`
	HearingImpaired              bool    `json:"flag_hearing_impaired,omitempty"`
	VisualImpaired               bool    `json:"flag_visual_impaired,omitempty"`
	Commentary                   bool    `json:"flag_commentary,omitempty"`
	OriginalLanguage             bool    `json:"flag_original,omitempty"`
	TextDescriptions             bool    `json:"flag_text_descriptions,omitempty"`
	TextSubtitles                bool    `json:"text_subtitles,omitempty"`
	ContentEncodingAlgorithms    string  `json:"content_encoding_algorithms,omitempty"`
	CodecPrivate                 string  `json:"codec_private_data,omitempty"`
	CodecDelay                   int64   `json:"codec_delay,omitempty"`
	AacIsSbr                     string  `json:"aac_is_sbr,omitempty"`
	AlphaMode                    int     `json:"alpha_mode,omitempty"`
	AudioBitsPerSample           int     `json:"audio_bits_per_sample,omitempty"`
	AudioChannels                int     `json:"audio_channels,omitempty"`
	AudioEmphasis                int     `json:"audio_emphasis,omitempty"`
	AudioSamplingFrequency       int     `json:"audio_sampling_frequency,omitempty"`
	CbSubsample                  string  `json:"cb_subsample,omitempty"`
	ChromaSiting                 string  `json:"chroma_siting,omitempty"`
	ChromaSubsample              string  `json:"chroma_subsample,omitempty"`
	ChromaticityCoordinates      string  `json:"chromaticity_coordinates,omitempty"`
	CodecID                      string  `json:"codec_id,omitempty"`
	CodecName                    string  `json:"codec_name,omitempty"`
	CodecPrivateLength           int     `json:"codec_private_length,omitempty"`
	ColorBitsPerChannel          int     `json:"color_bits_per_channel,omitempty"`
	ColorMatrixCoefficients      int     `json:"color_matrix_coefficients,omitempty"`
	ColorPrimaries               int     `json:"color_primaries,omitempty"`
	ColorRange                   int     `json:"color_range,omitempty"`
	ColorTransferCharacteristics int     `json:"color_transfer_characteristics,omitempty"`
	DefaultDuration              int64   `json:"default_duration,omitempty"`
	DisplayDimensions            string  `json:"display_dimensions,omitempty"`
	DisplayUnit                  int     `json:"display_unit,omitempty"`
	Encoding                     string  `json:"encoding,omitempty"`
	MaxContentLight              int     `json:"max_content_light,omitempty"`
	MaxFrameLight                int     `json:"max_frame_light,omitempty"`
	MaxLuminance                 float64 `json:"max_luminance,omitempty"`
	MinLuminance                 float64 `json:"min_luminance,omitempty"`
	MinimumTimestamp             int64   `json:"minimum_timestamp,omitempty"`
	MultiplexedTracks            []int   `json:"multiplexed_tracks,omitempty"`
	Packetizer                   string  `json:"packetizer,omitempty"`
	PixelDimensions              string  `json:"pixel_dimensions,omitempty"`
	ProgramNumber                int     `json:"program_number,omitempty"`
	ProjectionPosePitch          float64 `json:"projection_pose_pitch,omitempty"`
	ProjectionPoseRoll           float64 `json:"projection_pose_roll,omitempty"`
	ProjectionPoseYaw            float64 `json:"projection_pose_yaw,omitempty"`
	ProjectionPrivate            string  `json:"projection_private,omitempty"`
	ProjectionType               int     `json:"projection_type,omitempty"`
	StereoMode                   int     `json:"stereo_mode,omitempty"`
	StreamID                     int     `json:"stream_id,omitempty"`
	SubStreamID                  int     `json:"sub_stream_id,omitempty"`
	TeletextPage                 int     `json:"teletext_page,omitempty"`
	UID                          uint64  `json:"uid,omitempty"`
	WhiteColorCoordinates        string  `json:"white_color_coordinates,omitempty"`
}

// ParseDimensions parses a dimensions string in the format "WIDTHxHEIGHT" (e.g., "1920x1080") into width and height.
func ParseDimensions(s string) (int, int) {
	if s == "" {
		return 0, 0
	}

	parts := strings.Split(s, "x")
	if len(parts) != 2 {
		return 0, 0
	}

	w, err1 := strconv.Atoi(parts[0])

	h, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0
	}

	return w, h
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
	ID          int                      `json:"id,omitempty"`
	ContentType string                   `json:"content_type,omitempty"`
	FileName    string                   `json:"file_name,omitempty"`
	Size        int                      `json:"size,omitempty"`
	Description string                   `json:"description,omitempty"`
	Properties  EbmlAttachmentProperties `json:"properties,omitzero"`
	Type        string                   `json:"type,omitempty"`
}

// EbmlAttachmentProperties contains metadata properties of an attachment.
type EbmlAttachmentProperties struct {
	UID uint64 `json:"uid"`
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

// TrackEdit describes a set of property changes for a single track, identified
// by its track number (the "number" property reported by mkvmerge -J).
type TrackEdit struct {
	Number int
	// Props maps an mkvpropedit property name (e.g. "flag-default", "name") to
	// its new value. An empty value deletes the property instead of setting it.
	Props map[string]string
}

// SetTrackProperties applies the given per-track property edits to a Matroska
// file in place using mkvpropedit. It edits the existing container and does not
// remux, so it is fast and lossless.
func SetTrackProperties(filePath string, edits []TrackEdit) error {
	if len(edits) == 0 {
		return nil
	}

	if err := CheckForMatroska(filePath); err != nil {
		return err
	}

	args := buildPropeditArgs(filePath, edits)

	debugArgs := slices.Clone(args)
	debugArgs[0] = ui.AnonymizePath(filePath)
	ui.PrintDebug("Executing: mkvpropedit " + strings.Join(debugArgs, " "))

	cmd := exec.CommandContext(context.Background(), "mkvpropedit", args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("mkvpropedit is not installed or not available in PATH: %w", err)
		}

		return fmt.Errorf("failed to set track properties: %w: %s", err, output)
	}

	return nil
}

// buildPropeditArgs builds the mkvpropedit argument list for the given edits.
// Property keys are sorted so the resulting command is deterministic.
func buildPropeditArgs(filePath string, edits []TrackEdit) []string {
	args := []string{filePath}

	for _, edit := range edits {
		args = append(args, "--edit", "track:@"+strconv.Itoa(edit.Number))

		for _, key := range slices.Sorted(maps.Keys(edit.Props)) {
			if value := edit.Props[key]; value == "" {
				args = append(args, "--delete", key)
			} else {
				args = append(args, "--set", key+"="+value)
			}
		}
	}

	return args
}

// SetContainerProperties applies the given segment-level ("info") property
// edits to a Matroska file in place using mkvpropedit. An empty value deletes
// the property instead of setting it.
func SetContainerProperties(filePath string, props map[string]string) error {
	if len(props) == 0 {
		return nil
	}

	if err := CheckForMatroska(filePath); err != nil {
		return err
	}

	// Some info properties (e.g. writing-application) are mandatory and reject
	// --delete, but --set with an empty value clears them just as well, so
	// container edits always use --set, unlike the per-track edits above.
	args := []string{filePath, "--edit", "info"}

	for _, key := range slices.Sorted(maps.Keys(props)) {
		args = append(args, "--set", key+"="+props[key])
	}

	debugArgs := slices.Clone(args)
	debugArgs[0] = ui.AnonymizePath(filePath)
	ui.PrintDebug("Executing: mkvpropedit " + strings.Join(debugArgs, " "))

	cmd := exec.CommandContext(context.Background(), "mkvpropedit", args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("mkvpropedit is not installed or not available in PATH: %w", err)
		}

		return fmt.Errorf("failed to set container properties: %w: %s", err, output)
	}

	return nil
}

// DeleteAttachments removes the attachments with the given mkvmerge attachment
// IDs from a Matroska file in place using mkvpropedit.
func DeleteAttachments(filePath string, ids []int) error {
	if len(ids) == 0 {
		return nil
	}

	if err := CheckForMatroska(filePath); err != nil {
		return err
	}

	args := []string{filePath}
	for _, id := range ids {
		args = append(args, "--delete-attachment", strconv.Itoa(id))
	}

	debugArgs := slices.Clone(args)
	debugArgs[0] = ui.AnonymizePath(filePath)
	ui.PrintDebug("Executing: mkvpropedit " + strings.Join(debugArgs, " "))

	cmd := exec.CommandContext(context.Background(), "mkvpropedit", args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("mkvpropedit is not installed or not available in PATH: %w", err)
		}

		return fmt.Errorf("failed to delete attachments: %w: %s", err, output)
	}

	return nil
}

// RenameAttachments updates the display name of the attachments with the
// given mkvmerge attachment IDs in a Matroska file in place using mkvpropedit,
// without touching their content.
func RenameAttachments(filePath string, renames map[int]string) error {
	if len(renames) == 0 {
		return nil
	}

	if err := CheckForMatroska(filePath); err != nil {
		return err
	}

	args := []string{filePath}
	for _, id := range slices.Sorted(maps.Keys(renames)) {
		args = append(args, "--update-attachment", strconv.Itoa(id), "--attachment-name", renames[id])
	}

	debugArgs := slices.Clone(args)
	debugArgs[0] = ui.AnonymizePath(filePath)
	ui.PrintDebug("Executing: mkvpropedit " + strings.Join(debugArgs, " "))

	cmd := exec.CommandContext(context.Background(), "mkvpropedit", args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("mkvpropedit is not installed or not available in PATH: %w", err)
		}

		return fmt.Errorf("failed to rename attachments: %w: %s", err, output)
	}

	return nil
}

// RemuxOptions describes a lossless remux of a Matroska file via mkvmerge. All
// track IDs are mkvmerge track IDs (the "id" field reported by mkvmerge -J).
type RemuxOptions struct {
	// TrackOrder is the desired output order of track IDs. Empty keeps the
	// current order. IDs that are also removed are ignored.
	TrackOrder []int
	// RemoveTrackIDs lists track IDs to drop from the output.
	RemoveTrackIDs []int
	// StripCompressionIDs lists track IDs whose compression should be removed.
	StripCompressionIDs []int
	// DisableTrackCompression emits "none" compression for all kept tracks.
	// This prevents mkvmerge from introducing zlib compression while performing
	// another remux operation such as reordering or removing tracks.
	DisableTrackCompression bool
}

// IsEmpty reports whether the options describe no work.
func (o RemuxOptions) IsEmpty() bool {
	return len(o.TrackOrder) == 0 && len(o.RemoveTrackIDs) == 0 && len(o.StripCompressionIDs) == 0
}

var errRemuxFailed = errors.New("mkvmerge remux failed")

// RemuxTracks rewrites a Matroska file with mkvmerge to reorder tracks, strip
// container compression and/or drop tracks. The remux is lossless (streams are
// copied), but unlike SetTrackProperties it rewrites the whole file. The output
// is written to a temporary file in the same directory and atomically swapped
// in on success.
func RemuxTracks(filePath string, opts RemuxOptions) error {
	if opts.IsEmpty() {
		return nil
	}

	if err := CheckForMatroska(filePath); err != nil {
		return err
	}

	ebml, err := GetEbmlMetadata(filePath)
	if err != nil {
		return err
	}

	tmpPath := filepath.Join(filepath.Dir(filePath), "."+filepath.Base(filePath)+".parsec-remux.mkv")
	args := buildRemuxArgs(tmpPath, filePath, opts, ebml.Tracks)

	ui.PrintDebug("Executing: mkvmerge " + strings.Join(args, " "))

	cmd := exec.CommandContext(context.Background(), "mkvmerge", args...)
	if output, runErr := cmd.CombinedOutput(); runErr != nil {
		if err := interpretMkvmergeError(runErr, output); err != nil {
			_ = os.Remove(tmpPath)

			return err
		}
	}

	return replaceFile(filePath, tmpPath)
}

// interpretMkvmergeError maps an mkvmerge exit status to an error. mkvmerge
// returns exit code 1 for warnings (the output is still produced) and 2 for
// fatal errors; only the latter is treated as a failure.
func interpretMkvmergeError(runErr error, output []byte) error {
	if errors.Is(runErr, exec.ErrNotFound) {
		return fmt.Errorf("mkvmerge is not installed or not available in PATH: %w", runErr)
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) && exitErr.ExitCode() == 1 {
		// Warnings only; the output file was written successfully.
		return nil
	}

	return fmt.Errorf("%w: %w: %s", errRemuxFailed, runErr, output)
}

// replaceFile swaps tmpPath in for filePath, preserving the original file mode.
func replaceFile(filePath, tmpPath string) error {
	if info, statErr := os.Stat(filePath); statErr == nil {
		_ = os.Chmod(tmpPath, info.Mode())
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		_ = os.Remove(tmpPath)

		return fmt.Errorf("failed to replace original after remux: %w", err)
	}

	return nil
}

// buildRemuxArgs builds the mkvmerge argument list for the given remux options.
func buildRemuxArgs(outPath, inPath string, opts RemuxOptions, tracks []EbmlTrack) []string {
	args := []string{"-o", outPath}

	removed := make(map[int]bool, len(opts.RemoveTrackIDs))
	for _, id := range opts.RemoveTrackIDs {
		removed[id] = true
	}

	args = append(args, buildRemovalArgs(opts.RemoveTrackIDs, tracks)...)

	args = append(args, buildCompressionArgs(opts, tracks, removed)...)

	if order := buildTrackOrder(opts.TrackOrder, removed); order != "" {
		args = append(args, "--track-order", order)
	}

	return append(args, inPath)
}

func buildCompressionArgs(opts RemuxOptions, tracks []EbmlTrack, removed map[int]bool) []string {
	if opts.DisableTrackCompression {
		args := make([]string, 0, len(tracks)*2)

		for _, track := range tracks {
			if !removed[track.ID] {
				args = append(args, "--compression", strconv.Itoa(track.ID)+":none")
			}
		}

		return args
	}

	args := make([]string, 0, len(opts.StripCompressionIDs)*2)

	for _, id := range opts.StripCompressionIDs {
		if !removed[id] {
			args = append(args, "--compression", strconv.Itoa(id)+":none")
		}
	}

	return args
}

// buildRemovalArgs groups removed track IDs by type and emits the matching
// mkvmerge keep/remove flags (e.g. "--audio-tracks !2,3").
func buildRemovalArgs(removeIDs []int, tracks []EbmlTrack) []string {
	if len(removeIDs) == 0 {
		return nil
	}

	typeByID := make(map[int]string, len(tracks))
	for _, track := range tracks {
		typeByID[track.ID] = track.Type
	}

	byType := make(map[string][]string)
	for _, id := range removeIDs {
		byType[typeByID[id]] = append(byType[typeByID[id]], strconv.Itoa(id))
	}

	flagByType := map[string]string{
		"video":     "--video-tracks",
		"audio":     "--audio-tracks",
		"subtitles": "--subtitle-tracks",
	}

	var args []string
	// Iterate in a fixed order for deterministic output.
	for _, typ := range []string{"video", "audio", "subtitles"} {
		if ids := byType[typ]; len(ids) > 0 {
			args = append(args, flagByType[typ], "!"+strings.Join(ids, ","))
		}
	}

	return args
}

// buildTrackOrder renders the mkvmerge --track-order value for the kept tracks.
func buildTrackOrder(trackOrder []int, removed map[int]bool) string {
	var entries []string

	for _, id := range trackOrder {
		if !removed[id] {
			entries = append(entries, "0:"+strconv.Itoa(id))
		}
	}

	return strings.Join(entries, ",")
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

// HasChapters returns true if mkvmerge detected any chapters in the file.
func (metadata *EbmlMetadata) HasChapters() bool {
	for _, ch := range metadata.Chapters {
		if ch.NumEntries > 0 {
			return true
		}
	}

	return false
}

var (
	errInvalidTimeFormat = errors.New("invalid time format")
	errInvalidTimeValues = errors.New("invalid time values")
	errInvalidSubsecond  = errors.New("invalid subsecond value")
)

// parseTimeToNs parses a time string in format HH:MM:SS.nnnnnnnnn into nanoseconds.
func parseTimeToNs(s string) (int64, error) {
	parts := strings.Split(s, ".")
	timeParts := strings.Split(parts[0], ":")

	if len(timeParts) != 3 {
		return 0, fmt.Errorf("%w", errInvalidTimeFormat)
	}

	hours, err1 := strconv.ParseInt(timeParts[0], 10, 64)
	mins, err2 := strconv.ParseInt(timeParts[1], 10, 64)
	secs, err3 := strconv.ParseInt(timeParts[2], 10, 64)

	if err1 != nil || err2 != nil || err3 != nil {
		return 0, fmt.Errorf("%w", errInvalidTimeValues)
	}

	var ns int64

	if len(parts) > 1 {
		fractionStr := parts[1]

		if len(fractionStr) < 9 {
			fractionStr += strings.Repeat("0", 9-len(fractionStr))
		} else if len(fractionStr) > 9 {
			fractionStr = fractionStr[:9]
		}

		var err error

		ns, err = strconv.ParseInt(fractionStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%w", errInvalidSubsecond)
		}
	}

	totalNs := hours*3600000000000 + mins*60000000000 + secs*1000000000 + ns

	return totalNs, nil
}

// ExtractChapters uses mkvextract to extract and parse chapters in XML format.
func ExtractChapters(filePath string) ([]EbmlChapterAtom, error) {
	xmlContent, err := ExtractChaptersXML(filePath)
	if err != nil {
		return nil, err
	}

	var xmlCh EbmlChapters

	if err := xml.Unmarshal(xmlContent, &xmlCh); err != nil {
		return nil, fmt.Errorf("failed to unmarshal chapters XML: %w", err)
	}

	return parseXMLChapters(xmlCh), nil
}

// ExtractChaptersXML uses mkvextract to extract the raw chapters XML as
// written by mkvtoolnix, without parsing it. Used by fix to surgically rewrite
// a small piece (e.g. timestamps) of the XML while leaving everything else
// (flags, UIDs, nesting) byte-for-byte untouched.
func ExtractChaptersXML(filePath string) ([]byte, error) {
	if err := CheckForMatroska(filePath); err != nil {
		return nil, err
	}

	tmpFilePath, err := runMkvextractChapters(filePath)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = os.Remove(tmpFilePath)
	}()

	xmlContent, err := os.ReadFile(tmpFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read extracted chapters: %w", err)
	}

	return xmlContent, nil
}

// chapterTimeStartRegex matches a <ChapterTimeStart> element as written by
// mkvextract, which always emits it on a single line without attributes.
var chapterTimeStartRegex = regexp.MustCompile(`<ChapterTimeStart>[^<]*</ChapterTimeStart>`)

var errChapterTimestampCountMismatch = errors.New("chapter timestamp count mismatch")

// RewriteChapterTimestamps replaces each <ChapterTimeStart> value found in the
// file's chapters XML, in document order, with newTimes (nanoseconds), then
// applies the result with mkvpropedit. Every other byte of the XML (flags,
// UIDs, display names, nesting) is left untouched. len(newTimes) must equal
// the number of <ChapterTimeStart> elements found, otherwise nothing is
// changed and an error is returned, since a mismatch means the caller's
// chapter model (which only sees the first edition) doesn't match the file.
func RewriteChapterTimestamps(filePath string, newTimes []int64) error {
	if len(newTimes) == 0 {
		return nil
	}

	xmlContent, err := ExtractChaptersXML(filePath)
	if err != nil {
		return err
	}

	updated, err := replaceChapterTimestamps(xmlContent, newTimes)
	if err != nil {
		return err
	}

	return SetChaptersXML(filePath, updated)
}

// replaceChapterTimestamps is the pure XML-rewriting half of
// RewriteChapterTimestamps, split out so it can be tested without mkvextract
// or a real Matroska file.
func replaceChapterTimestamps(xmlContent []byte, newTimes []int64) ([]byte, error) {
	matches := chapterTimeStartRegex.FindAllIndex(xmlContent, -1)
	if len(matches) != len(newTimes) {
		return nil, fmt.Errorf("%w: found %d, expected %d", errChapterTimestampCountMismatch, len(matches), len(newTimes))
	}

	var buf bytes.Buffer

	last := 0

	for i, m := range matches {
		buf.Write(xmlContent[last:m[0]])
		buf.WriteString("<ChapterTimeStart>" + formatChapterTimestamp(newTimes[i]) + "</ChapterTimeStart>")

		last = m[1]
	}

	buf.Write(xmlContent[last:])

	return buf.Bytes(), nil
}

// formatChapterTimestamp formats nanoseconds as the HH:MM:SS.nnnnnnnnn string
// mkvextract/mkvmerge use for ChapterTimeStart, the inverse of parseTimeToNs.
func formatChapterTimestamp(ns int64) string {
	if ns < 0 {
		ns = 0
	}

	hours := ns / 3_600_000_000_000
	ns -= hours * 3_600_000_000_000
	mins := ns / 60_000_000_000
	ns -= mins * 60_000_000_000
	secs := ns / 1_000_000_000
	ns -= secs * 1_000_000_000

	return fmt.Sprintf("%02d:%02d:%02d.%09d", hours, mins, secs, ns)
}

// SetChaptersXML replaces a Matroska file's chapters in place with the given
// XML content, using mkvpropedit.
func SetChaptersXML(filePath string, xmlContent []byte) error {
	if err := CheckForMatroska(filePath); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp("", "parsec-chapters-write-*.xml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	tmpPath := tmpFile.Name()

	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmpFile.Write(xmlContent); err != nil {
		_ = tmpFile.Close()

		return fmt.Errorf("failed to write chapters XML: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to write chapters XML: %w", err)
	}

	debugPath := ui.AnonymizePath(filePath)
	ui.PrintDebug(fmt.Sprintf("Executing: mkvpropedit %s --chapters %s", debugPath, tmpPath))

	cmd := exec.CommandContext(context.Background(), "mkvpropedit", filePath, "--chapters", tmpPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("mkvpropedit is not installed or not available in PATH: %w", err)
		}

		return fmt.Errorf("failed to set chapters: %w: %s", err, output)
	}

	return nil
}

func runMkvextractChapters(filePath string) (string, error) {
	tmpFile, err := os.CreateTemp("", "parsec-chapters-*.xml")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	tmpFilePath := tmpFile.Name()
	_ = tmpFile.Close()

	ui.PrintDebug(fmt.Sprintf("Executing: mkvextract %s chapters %s", ui.AnonymizePath(filePath), tmpFilePath))

	cmd := exec.CommandContext(context.Background(), "mkvextract", filePath, "chapters", tmpFilePath)
	if _, err := cmd.Output(); err != nil {
		_ = os.Remove(tmpFilePath)

		if errors.Is(err, exec.ErrNotFound) {
			return "", fmt.Errorf("mkvextract is not installed or not available in PATH: %w", err)
		}

		return "", fmt.Errorf("failed to extract chapters: %w", err)
	}

	return tmpFilePath, nil
}

func parseXMLChapters(xmlCh EbmlChapters) []EbmlChapterAtom {
	var parsed []EbmlChapterAtom

	for _, edition := range xmlCh.Editions {
		for _, atom := range edition.Chapters {
			ns, err := parseTimeToNs(atom.TimeStartXML)
			if err != nil {
				continue
			}

			atom.TimeStart = ns
			parsed = append(parsed, atom)
		}
	}

	return parsed
}
