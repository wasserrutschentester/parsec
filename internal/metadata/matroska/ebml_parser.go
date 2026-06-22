package matroska

import (
	"errors"
	"fmt"
	"io"
	"math/bits"
	"os"
)

// EBML ID Constants
const (
	idEBMLHeader        = 0x1A45DFA3
	idSegment           = 0x18538067
	idSeekHead          = 0x114D9B74
	idSeek              = 0x4DBB
	idSeekID            = 0x53AB
	idSeekPosition      = 0x53AC
	idCues              = 0x1C53BB6B
	idCuePoint          = 0xBB
	idCueTime           = 0xB3
	idCueTrackPositions = 0xB7
	idCueTrack          = 0xF7
)

var (
	errInvalidVINT       = errors.New("invalid VINT leading byte")
	errInvalidEBMLID     = errors.New("invalid EBML ID leading byte")
	errUintTooLarge      = errors.New("uint size too large")
	errCuesNotFound      = errors.New("cues offset not found in SeekHead")
	errInvalidEBMLHeader = errors.New("invalid EBML header ID")
	errExpectedSegment   = errors.New("expected Segment ID")
	errSeekHeadNotFound  = errors.New("SeekHead element not found in first few Segment children")
	errExpectedCues      = errors.New("expected Cues ID")
)

// readVINT reads a VINT (variable size integer) and returns its value and length.
// The value returned does NOT include the length descriptor bits.
func readVINT(r io.Reader) (uint64, int, error) {
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, 0, fmt.Errorf("failed to read VINT first byte: %w", err)
	}

	first := b[0]
	leadingZeros := bits.LeadingZeros8(first)

	if leadingZeros >= 8 {
		return 0, 0, fmt.Errorf("invalid VINT prefix byte 0x%02X: %w", first, errInvalidVINT)
	}

	length := leadingZeros + 1
	val := uint64(first & (0xFF >> uint(length)))

	for i := 1; i < length; i++ {
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return 0, 0, fmt.Errorf("failed to read VINT continuation byte: %w", err)
		}

		val = (val << 8) | uint64(b[0])
	}

	return val, length, nil
}

// readID reads an EBML ID. It returns the raw VINT bytes parsed as a uint64.
func readID(r io.Reader) (uint64, error) {
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, fmt.Errorf("failed to read EBML ID first byte: %w", err)
	}

	first := b[0]
	leadingZeros := bits.LeadingZeros8(first)

	if leadingZeros >= 4 {
		return 0, fmt.Errorf("invalid EBML ID prefix byte 0x%02X: %w", first, errInvalidEBMLID)
	}

	length := leadingZeros + 1
	val := uint64(first)

	for i := 1; i < length; i++ {
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return 0, fmt.Errorf("failed to read EBML ID continuation byte: %w", err)
		}

		val = (val << 8) | uint64(b[0])
	}

	return val, nil
}

// readSize reads an EBML element size.
func readSize(r io.Reader) (uint64, error) {
	val, _, err := readVINT(r)
	if err != nil {
		return 0, fmt.Errorf("failed to read VINT for size: %w", err)
	}

	return val, nil
}

// readUint reads an unsigned integer of variable size (1-8 bytes).
func readUint(r io.Reader, size uint64) (uint64, error) {
	if size == 0 {
		return 0, nil
	}

	if size > 8 {
		return 0, fmt.Errorf("%w: size %d", errUintTooLarge, size)
	}

	var b [8]byte
	if _, err := io.ReadFull(r, b[:size]); err != nil {
		return 0, fmt.Errorf("failed to read uint bytes: %w", err)
	}

	var val uint64

	for i := range size {
		val = (val << 8) | uint64(b[i])
	}

	return val, nil
}

func parseSeekChild(r io.Reader, id uint64, size uint64) (uint64, uint64, error) {
	switch id {
	case idSeekID:
		val, err := readUint(r, size)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to read seek ID value: %w", err)
		}

		return val, 0, nil
	case idSeekPosition:
		val, err := readUint(r, size)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to read seek position value: %w", err)
		}

		return 0, val, nil
	default:
		if _, err := io.CopyN(io.Discard, r, int64(size)); err != nil {
			return 0, 0, fmt.Errorf("failed to skip seek child payload: %w", err)
		}

		return 0, 0, nil
	}
}

func parseSeekEntry(r io.Reader, size uint64) (uint64, uint64, error) {
	seekLimit := io.LimitReader(r, int64(size))

	var (
		currentID  uint64
		currentPos uint64
	)

	for {
		seekID, err := readID(seekLimit)
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return 0, 0, fmt.Errorf("failed to read seek child ID: %w", err)
		}

		seekSize, err := readSize(seekLimit)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to read seek child size: %w", err)
		}

		idVal, posVal, err := parseSeekChild(seekLimit, seekID, seekSize)
		if err != nil {
			return 0, 0, err
		}

		if idVal != 0 {
			currentID = idVal
		}

		if posVal != 0 {
			currentPos = posVal
		}
	}

	return currentID, currentPos, nil
}

func parseSeekHead(r io.Reader, size uint64) (uint64, error) {
	limitReader := io.LimitReader(r, int64(size))

	var (
		cuesPos   uint64
		foundCues bool
	)

	for {
		id, err := readID(limitReader)
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return 0, fmt.Errorf("failed to read seekhead child ID: %w", err)
		}

		elSize, err := readSize(limitReader)
		if err != nil {
			return 0, fmt.Errorf("failed to read seekhead child size: %w", err)
		}

		if id == idSeek {
			currentID, currentPos, err := parseSeekEntry(limitReader, elSize)
			if err != nil {
				return 0, err
			}

			if currentID == idCues {
				cuesPos = currentPos
				foundCues = true
			}
		} else {
			if _, err := io.CopyN(io.Discard, limitReader, int64(elSize)); err != nil {
				return 0, fmt.Errorf("failed to skip seekhead child payload: %w", err)
			}
		}
	}

	if !foundCues {
		return 0, fmt.Errorf("%w", errCuesNotFound)
	}

	return cuesPos, nil
}

func parseCueTrackPositions(r io.Reader, size uint64) (uint64, error) {
	limit := io.LimitReader(r, int64(size))

	track := uint64(0)

	for {
		id, err := readID(limit)
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return 0, fmt.Errorf("failed to read cuetrackpositions child ID: %w", err)
		}

		elSize, err := readSize(limit)
		if err != nil {
			return 0, fmt.Errorf("failed to read cuetrackpositions child size: %w", err)
		}

		if id == idCueTrack {
			val, err := readUint(limit, elSize)
			if err != nil {
				return 0, fmt.Errorf("failed to read cuetrack value: %w", err)
			}

			track = val
		} else {
			if _, err := io.CopyN(io.Discard, limit, int64(elSize)); err != nil {
				return 0, fmt.Errorf("failed to skip cuetrackpositions child payload: %w", err)
			}
		}
	}

	return track, nil
}

func parseCuePointChild(r io.Reader, id uint64, size uint64, targetTrack uint64) (uint64, bool, bool, error) {
	switch id {
	case idCueTime:
		val, err := readUint(r, size)
		if err != nil {
			return 0, false, false, fmt.Errorf("failed to read cuepoint time value: %w", err)
		}

		return val, true, false, nil
	case idCueTrackPositions:
		track, err := parseCueTrackPositions(r, size)
		if err != nil {
			return 0, false, false, err
		}

		if targetTrack == 0 || track == targetTrack {
			return 0, false, true, nil
		}

		return 0, false, false, nil
	default:
		if _, err := io.CopyN(io.Discard, r, int64(size)); err != nil {
			return 0, false, false, fmt.Errorf("failed to skip cuepoint child payload: %w", err)
		}

		return 0, false, false, nil
	}
}

func parseCuePoint(r io.Reader, size uint64, targetTrack uint64) (uint64, bool, error) {
	cuePointLimit := io.LimitReader(r, int64(size))

	var (
		cueTime uint64
		hasTime bool
		matches bool
	)

	for {
		cpID, err := readID(cuePointLimit)
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return 0, false, fmt.Errorf("failed to read cuepoint child ID: %w", err)
		}

		cpSize, err := readSize(cuePointLimit)
		if err != nil {
			return 0, false, fmt.Errorf("failed to read cuepoint child size: %w", err)
		}

		timeVal, gotTime, ok, err := parseCuePointChild(cuePointLimit, cpID, cpSize, targetTrack)
		if err != nil {
			return 0, false, err
		}

		if gotTime {
			cueTime = timeVal
			hasTime = true
		}

		if ok {
			matches = true
		}
	}

	return cueTime, hasTime && matches, nil
}

func parseCues(r io.Reader, size uint64, targetTrack uint64) ([]int64, error) {
	limitReader := io.LimitReader(r, int64(size))

	var timestamps []int64

	for {
		id, err := readID(limitReader)
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("failed to read cues child ID: %w", err)
		}

		elSize, err := readSize(limitReader)
		if err != nil {
			return nil, fmt.Errorf("failed to read cues child size: %w", err)
		}

		if id == idCuePoint {
			cueTime, matches, err := parseCuePoint(limitReader, elSize, targetTrack)
			if err != nil {
				return nil, err
			}

			if matches {
				timestamps = append(timestamps, int64(cueTime))
			}
		} else {
			if _, err := io.CopyN(io.Discard, limitReader, int64(elSize)); err != nil {
				return nil, fmt.Errorf("failed to skip cues child payload: %w", err)
			}
		}
	}

	return timestamps, nil
}

func verifyHeaderAndGetSegmentOffset(file *os.File) (int64, error) {
	id, err := readID(file)
	if err != nil {
		return 0, fmt.Errorf("failed to read EBML ID: %w", err)
	}

	if id != idEBMLHeader {
		return 0, fmt.Errorf("%w: 0x%02X", errInvalidEBMLHeader, id)
	}

	headerSize, err := readSize(file)
	if err != nil {
		return 0, fmt.Errorf("failed to read EBML header size: %w", err)
	}

	if _, err := file.Seek(int64(headerSize), io.SeekCurrent); err != nil {
		return 0, fmt.Errorf("failed to seek past EBML header: %w", err)
	}

	segID, err := readID(file)
	if err != nil {
		return 0, fmt.Errorf("failed to read Segment ID: %w", err)
	}

	if segID != idSegment {
		return 0, fmt.Errorf("%w 0x%02X, got 0x%02X", errExpectedSegment, idSegment, segID)
	}

	_, err = readSize(file)
	if err != nil {
		return 0, fmt.Errorf("failed to read Segment size: %w", err)
	}

	offset, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, fmt.Errorf("failed to get Segment payload start offset: %w", err)
	}

	return offset, nil
}

func findCuesOffset(file *os.File) (uint64, error) {
	for i := range 20 {
		id, err := readID(file)
		if err != nil {
			return 0, fmt.Errorf("failed to read element ID (index %d): %w", i, err)
		}

		size, err := readSize(file)
		if err != nil {
			return 0, fmt.Errorf("failed to read element size (index %d): %w", i, err)
		}

		if id == idSeekHead {
			pos, err := parseSeekHead(file, size)
			if err != nil {
				return 0, fmt.Errorf("failed to parse SeekHead: %w", err)
			}

			return pos, nil
		}

		if _, err := file.Seek(int64(size), io.SeekCurrent); err != nil {
			return 0, fmt.Errorf("failed to seek past element: %w", err)
		}
	}

	return 0, fmt.Errorf("%w", errSeekHeadNotFound)
}

func readAndParseCues(file *os.File, cuesAbsoluteOffset int64, targetTrack uint64) ([]int64, error) {
	if _, err := file.Seek(cuesAbsoluteOffset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to seek to Cues offset: %w", err)
	}

	cuesID, err := readID(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read Cues ID: %w", err)
	}

	if cuesID != idCues {
		return nil, fmt.Errorf("%w 0x%02X, got 0x%02X", errExpectedCues, idCues, cuesID)
	}

	cuesSize, err := readSize(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read Cues size: %w", err)
	}

	timestamps, err := parseCues(file, cuesSize, targetTrack)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Cues: %w", err)
	}

	return timestamps, nil
}

// ReadKeyframeTimestamps reads all keyframe (cue) timestamps from a Matroska file for a specific track number.
// Timestamps are returned in nanoseconds, using the provided timestampScale (which defaults to 1,000,000 ns / 1 ms if 0).
func ReadKeyframeTimestamps(filePath string, videoTrackNumber uint64, timestampScale int64) ([]int64, error) {
	scale := timestampScale
	if scale <= 0 {
		scale = 1_000_000
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	defer func() {
		_ = file.Close()
	}()

	segmentPayloadStart, err := verifyHeaderAndGetSegmentOffset(file)
	if err != nil {
		return nil, err
	}

	cuesPos, err := findCuesOffset(file)
	if err != nil {
		return nil, err
	}

	rawTimestamps, err := readAndParseCues(file, segmentPayloadStart+int64(cuesPos), videoTrackNumber)
	if err != nil {
		return nil, err
	}

	scaledTimestamps := make([]int64, len(rawTimestamps))
	for i, t := range rawTimestamps {
		scaledTimestamps[i] = t * scale
	}

	return scaledTimestamps, nil
}
