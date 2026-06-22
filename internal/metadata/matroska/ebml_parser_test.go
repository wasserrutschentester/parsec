package matroska

import (
	"os"
	"testing"
)

func createTestEBMLFile(tb testing.TB) string {
	tb.Helper()

	seekIDData := encodeElement(idSeekID, []byte{0x1C, 0x53, 0xBB, 0x6B})
	seekPosData := encodeElement(idSeekPosition, []byte{100})
	seekData := encodeElement(idSeek, append(seekIDData, seekPosData...))
	seekHeadData := encodeElement(idSeekHead, seekData)

	cueTrack := encodeElement(idCueTrack, []byte{1})
	cueTrackPos := encodeElement(idCueTrackPositions, cueTrack)

	cueTime1 := encodeElement(idCueTime, []byte{0x03, 0xE8}) // 1000
	cuePoint1 := encodeElement(idCuePoint, append(cueTime1, cueTrackPos...))

	cueTime2 := encodeElement(idCueTime, []byte{0x13, 0x88}) // 5000
	cuePoint2 := encodeElement(idCuePoint, append(cueTime2, cueTrackPos...))

	cuesData := encodeElement(idCues, append(cuePoint1, cuePoint2...))

	padSize := 100 - len(seekHeadData)
	if padSize < 0 {
		tb.Fatalf("SeekHead too large for offset 100")
	}

	var padding []byte

	if padSize > 0 {
		voidPayloadSize := max(0, padSize-2)
		padding = encodeElement(0xEC, make([]byte, voidPayloadSize))

		if len(seekHeadData)+len(padding) < 100 {
			padding = append(padding, make([]byte, 100-(len(seekHeadData)+len(padding)))...)
		}
	}

	segmentPayload := make([]byte, 0, len(seekHeadData)+len(padding)+len(cuesData))
	segmentPayload = append(segmentPayload, seekHeadData...)
	segmentPayload = append(segmentPayload, padding...)
	segmentPayload = append(segmentPayload, cuesData...)

	segmentData := encodeElement(idSegment, segmentPayload)
	ebmlHeader := encodeElement(idEBMLHeader, nil)

	fileData := make([]byte, 0, len(ebmlHeader)+len(segmentData))
	fileData = append(fileData, ebmlHeader...)
	fileData = append(fileData, segmentData...)

	tmpFile, err := os.CreateTemp("", "test-ebml-*.mkv")
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

func TestEBMLParser(t *testing.T) {
	t.Parallel()

	filePath := createTestEBMLFile(t)

	defer func() {
		_ = os.Remove(filePath)
	}()

	timestamps, err := ReadKeyframeTimestamps(filePath, 1, 1000000)
	if err != nil {
		t.Fatalf("ReadKeyframeTimestamps failed: %v", err)
	}

	expected := []int64{1000000000, 5000000000} // in nanoseconds

	if len(timestamps) != len(expected) {
		t.Fatalf("expected %d timestamps, got %d", len(expected), len(timestamps))
	}

	for i, v := range timestamps {
		if v != expected[i] {
			t.Errorf("timestamp %d: expected %d, got %d", i, expected[i], v)
		}
	}
}

func encodeVINT(val uint64) []byte {
	if val < 0x80-1 {
		return []byte{byte(val | 0x80)}
	}

	if val < 0x4000-1 {
		return []byte{byte((val >> 8) | 0x40), byte(val)}
	}

	if val < 0x200000-1 {
		return []byte{byte((val >> 16) | 0x20), byte(val >> 8), byte(val)}
	}

	panic("value too large for test VINT encoder")
}

func encodeElement(id uint64, data []byte) []byte {
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

	sizeBytes := encodeVINT(uint64(len(data)))

	res := make([]byte, 0, len(idBytes)+len(sizeBytes)+len(data))
	res = append(res, idBytes...)
	res = append(res, sizeBytes...)
	res = append(res, data...)

	return res
}
