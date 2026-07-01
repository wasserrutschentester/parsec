package checks

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

func createMockCuesFile(tb testing.TB, cueTimes []uint64) string {
	tb.Helper()

	seekIDData := encodeTestElement(0x53AB, []byte{0x1C, 0x53, 0xBB, 0x6B})
	seekPosData := encodeTestElement(0x53AC, []byte{40})
	seekData := encodeTestElement(0x4DBB, append(seekIDData, seekPosData...))
	seekHeadData := encodeTestElement(0x114D9B74, seekData)

	padding := append([]byte{0xEC, 0x93}, make([]byte, 19)...)

	cueTrack := encodeTestElement(0xF7, []byte{1})
	cueTrackPos := encodeTestElement(0xB7, cueTrack)

	var cuesInner []byte

	for _, ct := range cueTimes {
		var ctBytes []byte
		if ct > 0xFF {
			ctBytes = []byte{byte(ct >> 8), byte(ct)}
		} else {
			ctBytes = []byte{byte(ct)}
		}

		cueTime := encodeTestElement(0xB3, ctBytes)
		cuePoint := encodeTestElement(0xBB, append(cueTime, cueTrackPos...))
		cuesInner = append(cuesInner, cuePoint...)
	}

	cuesData := encodeTestElement(0x1C53BB6B, cuesInner)

	segmentPayload := make([]byte, 0, len(seekHeadData)+len(padding)+len(cuesData))
	segmentPayload = append(segmentPayload, seekHeadData...)
	segmentPayload = append(segmentPayload, padding...)
	segmentPayload = append(segmentPayload, cuesData...)

	segmentData := encodeTestElement(0x18538067, segmentPayload)
	ebmlHeader := encodeTestElement(0x1A45DFA3, nil)

	fileData := make([]byte, 0, len(ebmlHeader)+len(segmentData))
	fileData = append(fileData, ebmlHeader...)
	fileData = append(fileData, segmentData...)

	tmpFile, err := os.CreateTemp("", "test-check-ebml-*.mkv")
	if err != nil {
		tb.Fatalf("failed to create temp file: %v", err)
	}

	defer func() {
		_ = tmpFile.Close()
	}()

	if _, err := tmpFile.Write(fileData); err != nil {
		tb.Fatalf("failed to write temp file: %v", err)
	}

	return tmpFile.Name()
}

func encodeTestVINT(val uint64) []byte {
	if val < 0x80-1 {
		return []byte{byte(val | 0x80)}
	}

	if val < 0x4000-1 {
		return []byte{byte((val >> 8) | 0x40), byte(val)}
	}

	panic("too large for test VINT")
}

func encodeTestElement(id uint64, data []byte) []byte {
	idBytes := make([]byte, 0, 4)

	switch {
	case id > 0xFFFFFF:
		idBytes = append(idBytes, byte(id>>24), byte(id>>16), byte(id>>8), byte(id))
	case id > 0xFFFF:
		idBytes = append(idBytes, byte(id>>16), byte(id>>8), byte(id))
	case id > 0xFF:
		idBytes = append(idBytes, byte(id>>8), byte(id))
	default:
		idBytes = append(idBytes, byte(id))
	}

	sizeBytes := encodeTestVINT(uint64(len(data)))

	res := make([]byte, 0, len(idBytes)+len(sizeBytes)+len(data))
	res = append(res, idBytes...)
	res = append(res, sizeBytes...)
	res = append(res, data...)

	return res
}

//nolint:paralleltest // Test mutates global viper config and cannot run in parallel
func TestCheckChaptersKeyframeAlignmentAligned(t *testing.T) {
	config.InitDefaults()
	viper.Set("enabled_checks", []string{"all"})

	filePath := createMockCuesFile(t, []uint64{0, 10000, 20000})

	defer func() {
		_ = os.Remove(filePath)
	}()

	ebml := &matroska.EbmlMetadata{
		Tracks: []matroska.EbmlTrack{
			{ID: 1, Type: "video", Properties: matroska.EbmlTrackProperties{Number: 1}},
		},
		Chapters: []matroska.EbmlChapters{
			{
				Editions: []matroska.EbmlEdition{
					{
						Chapters: []matroska.EbmlChapterAtom{
							{TimeStart: 0},
							{TimeStart: 10000000000},
						},
					},
				},
			},
		},
	}

	var xmlChs *matroska.Chapters
	if len(ebml.Chapters) > 0 && len(ebml.Chapters[0].Editions) > 0 {
		xmlChs = &matroska.Chapters{Atoms: ebml.Chapters[0].Editions[0].Chapters}
	}

	res := runTrackChecks(filePath, ebml, xmlChs, nil, nil)

	for _, r := range res {
		if r.Identifier == "matroska_chapters_keyframe_alignment" {
			if !r.Passed {
				t.Errorf("Expected alignment check to pass, got warning: %s", r.Warning)
			}
		}
	}
}

//nolint:paralleltest // Test mutates global viper config and cannot run in parallel
func TestCheckChaptersKeyframeAlignmentNonAligned(t *testing.T) {
	config.InitDefaults()
	viper.Set("enabled_checks", []string{"all"})

	filePath := createMockCuesFile(t, []uint64{0, 10000, 20000})

	defer func() {
		_ = os.Remove(filePath)
	}()

	ebml := &matroska.EbmlMetadata{
		Tracks: []matroska.EbmlTrack{
			{ID: 1, Type: "video", Properties: matroska.EbmlTrackProperties{Number: 1}},
		},
		Chapters: []matroska.EbmlChapters{
			{
				Editions: []matroska.EbmlEdition{
					{
						Chapters: []matroska.EbmlChapterAtom{
							{TimeStart: 0},
							{TimeStart: 15000000000},
						},
					},
				},
			},
		},
	}

	var xmlChs *matroska.Chapters
	if len(ebml.Chapters) > 0 && len(ebml.Chapters[0].Editions) > 0 {
		xmlChs = &matroska.Chapters{Atoms: ebml.Chapters[0].Editions[0].Chapters}
	}

	res := runTrackChecks(filePath, ebml, xmlChs, nil, nil)
	found := false

	for _, r := range res {
		if r.Identifier == "matroska_chapters_keyframe_alignment" {
			found = true

			if r.Passed {
				t.Error("Expected alignment check to fail for non-aligned chapters")
			}

			if r.Table == nil || len(r.Table.Rows) == 0 {
				t.Fatalf("Expected table rows, got: %v", r.Table)
			}

			if !strings.Contains(r.Table.Rows[0][3], "5.000s") {
				t.Errorf("Expected offset to be 5.000s, got: %q", r.Table.Rows[0][3])
			}
		}
	}

	if !found {
		t.Error("Did not find keyframe alignment check result")
	}
}

//nolint:paralleltest // Test mutates global viper config and cannot run in parallel
func TestCheckChaptersKeyframeAlignmentAsymmetric(t *testing.T) {
	config.InitDefaults()
	viper.Set("enabled_checks", []string{"all"})

	tests := []struct {
		name       string
		timeStarts []int64
		wantPassed bool
	}{
		{
			name:       "aligned slightly after (5ms)",
			timeStarts: []int64{0, 10005000000},
			wantPassed: true,
		},
		{
			name:       "aligned slightly before within rounding tolerance (0.5ms)",
			timeStarts: []int64{0, 9999500000},
			wantPassed: true,
		},
		{
			name:       "non-aligned before rounding tolerance (2ms)",
			timeStarts: []int64{0, 9998000000},
			wantPassed: false,
		},
		{
			name:       "non-aligned after tolerance (15ms)",
			timeStarts: []int64{0, 10015000000},
			wantPassed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := createMockCuesFile(t, []uint64{0, 10000, 20000})

			defer func() {
				_ = os.Remove(filePath)
			}()

			res := runAsymmetricCheck(filePath, tt.timeStarts)
			foundResult := findAlignmentResult(res)

			if tt.wantPassed {
				if foundResult != nil {
					t.Errorf("Expected check to pass (not be present in issues), but got failure: %+v", foundResult)
				}
			} else {
				if foundResult == nil {
					t.Error("Expected check to fail, but found no issue result")
				} else if foundResult.Passed {
					t.Error("Expected check result to have Passed=false, but got Passed=true")
				}
			}
		})
	}
}

func runAsymmetricCheck(filePath string, timeStarts []int64) []CheckResult {
	ebml := &matroska.EbmlMetadata{
		Tracks: []matroska.EbmlTrack{
			{ID: 1, Type: "video", Properties: matroska.EbmlTrackProperties{Number: 1}},
		},
		Chapters: []matroska.EbmlChapters{
			{
				Editions: []matroska.EbmlEdition{
					{
						Chapters: []matroska.EbmlChapterAtom{
							{TimeStart: timeStarts[0]},
							{TimeStart: timeStarts[1]},
						},
					},
				},
			},
		},
	}

	var xmlChs *matroska.Chapters
	if len(ebml.Chapters) > 0 && len(ebml.Chapters[0].Editions) > 0 {
		xmlChs = &matroska.Chapters{Atoms: ebml.Chapters[0].Editions[0].Chapters}
	}

	return runTrackChecks(filePath, ebml, xmlChs, nil, nil)
}

func findAlignmentResult(res []CheckResult) *CheckResult {
	for i := range res {
		if res[i].Identifier == "matroska_chapters_keyframe_alignment" {
			return &res[i]
		}
	}

	return nil
}
