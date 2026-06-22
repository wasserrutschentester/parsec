package fix

import (
	"os"
	"testing"
)

// This file builds a minimal synthetic Matroska (EBML) file with a Cues
// index, for testing ComputeChapterKeyframeSnaps without a real video file.
// Duplicated from internal/checks/matroska_test.go (Go test helpers can't be
// shared across package boundaries) rather than exported, since it's a
// small, self-contained fixture builder used only by these two tests.

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

	tmpFile, err := os.CreateTemp("", "test-fix-ebml-*.mkv")
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
