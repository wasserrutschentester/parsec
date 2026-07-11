package matroska

import (
	"errors"
	"fmt"
	"io"
	"math/bits"
)

var (
	// ErrInvalidEBMLIDLength is returned when an EBML ID exceeds 4 bytes.
	ErrInvalidEBMLIDLength = errors.New("invalid or unsupported EBML ID length")
	// ErrInvalidEBMLSizeLength is returned when an EBML size exceeds 8 bytes.
	ErrInvalidEBMLSizeLength = errors.New("invalid EBML size length")
)

// ElementID rich type for Matroska Element IDs
type ElementID uint32

// Standard Matroska Element IDs
const (
	// Level 0
	ElementEBML    ElementID = 0x1A45DFA3
	ElementSegment ElementID = 0x18538067
	// Level 1
	ElementSeekHead    ElementID = 0x114D9B74
	ElementInfo        ElementID = 0x1549A966
	ElementTracks      ElementID = 0x1654AE6B
	ElementChapters    ElementID = 0x1043A770
	ElementTags        ElementID = 0x1254C367
	ElementAttachments ElementID = 0x1941A469
	ElementCues        ElementID = 0x1C53BB6B
	ElementCluster     ElementID = 0x1F43B675
	// Global
	ElementVoid  ElementID = 0xEC
	ElementCRC32 ElementID = 0xBF
)

//nolint:cyclop // there's a lot of Element types
func (id ElementID) String() string {
	switch id {
	case ElementEBML:
		return "Header"
	case ElementSegment:
		return "Segment"
	case ElementSeekHead:
		return "SeekHead"
	case ElementInfo:
		return "Info"
	case ElementTracks:
		return "Tracks"
	case ElementChapters:
		return "Chapters"
	case ElementTags:
		return "Tags"
	case ElementAttachments:
		return "Attachments"
	case ElementCues:
		return "Cues"
	case ElementCluster:
		return "Cluster"
	case ElementVoid:
		return "Void"
	case ElementCRC32:
		return "CRC-32"
	default:
		return fmt.Sprintf("Unknown (0x%X)", uint32(id))
	}
}

// ElementPosition defines the physical location of an EBML element in a file.
type ElementPosition struct {
	ID     ElementID
	Offset int64
	Size   int64
}

// ExtractOptions configures the behavior of the segment extractor.
type ExtractOptions struct {
	// HaltOnFirstCluster stops extraction immediately upon encountering the first
	// Cluster element. This drastically improves performance for layout validation.
	HaltOnFirstCluster bool
}

// ExtractSegmentPositions scans a Matroska file and returns the byte offsets
// and sizes of all Level 1 elements (children of the main Segment).
func ExtractSegmentPositions(r io.ReadSeeker, opts ExtractOptions) ([]ElementPosition, error) {
	if err := skipToSegment(r); err != nil {
		return nil, err
	}

	return extractLevel1Elements(r, opts)
}

// skipToSegment scans the top-level elements until it encounters the Segment ID.
func skipToSegment(r io.ReadSeeker) error {
	for {
		id, err := readEBMLID(r)
		if err != nil {
			return fmt.Errorf("failed to read top-level ID: %w", err)
		}

		size, err := readEBMLSize(r)
		if err != nil {
			return fmt.Errorf("failed to read top-level size: %w", err)
		}

		if id == ElementSegment { // Segment ID
			return nil
		}

		if size > 0 {
			if _, err := r.Seek(size, io.SeekCurrent); err != nil {
				return fmt.Errorf("failed to skip top-level element: %w", err)
			}
		}
	}
}

func parseNextElementHeader(r io.ReadSeeker) (ElementPosition, error) {
	offset, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return ElementPosition{}, fmt.Errorf("failed to get current offset: %w", err)
	}

	id, err := readEBMLID(r)
	if err != nil {
		return ElementPosition{}, err // Pass EOF through naturally
	}

	size, err := readEBMLSize(r)
	if err != nil {
		return ElementPosition{}, err // Pass EOF through naturally
	}

	return ElementPosition{ID: id, Offset: offset, Size: size}, nil
}

func skipPayload(r io.ReadSeeker, size int64) error {
	if size > 0 {
		if _, err := r.Seek(size, io.SeekCurrent); err != nil {
			return fmt.Errorf("failed to skip payload of size %d: %w", size, err)
		}
	}

	return nil
}

func extractLevel1Elements(r io.ReadSeeker, opts ExtractOptions) ([]ElementPosition, error) {
	var positions []ElementPosition

	for {
		pos, err := parseNextElementHeader(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, fmt.Errorf("failed to read Level 1 header: %w", err)
		}

		positions = append(positions, pos)

		if opts.HaltOnFirstCluster && pos.ID == ElementCluster {
			break
		}

		if pos.Size == -1 {
			break
		}

		if err := skipPayload(r, pos.Size); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, fmt.Errorf("failed to skip Level 1 payload: %w", err)
		}
	}

	return positions, nil
}

// readEBMLID reads an EBML Element ID (which retains its length descriptor bits).
func readEBMLID(r io.Reader) (ElementID, error) {
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, fmt.Errorf("failed to read ID length descriptor: %w", err)
	}

	length := bits.LeadingZeros8(b[0]) + 1
	if length > 4 {
		return 0, ErrInvalidEBMLIDLength
	}

	id := uint32(b[0])
	for i := 1; i < length; i++ {
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return 0, fmt.Errorf("failed to read ID payload: %w", err)
		}

		id = (id << 8) | uint32(b[0])
	}

	return ElementID(id), nil
}

// readEBMLSize reads an EBML Data Size (stripping its length descriptor bits).
func readEBMLSize(r io.Reader) (int64, error) {
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, fmt.Errorf("failed to read size length descriptor: %w", err)
	}

	length := bits.LeadingZeros8(b[0]) + 1
	if length > 8 {
		return 0, ErrInvalidEBMLSizeLength
	}

	size := int64(b[0] & (0xFF >> length))
	isUnknown := b[0] == 0xFF

	for i := 1; i < length; i++ {
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return 0, fmt.Errorf("failed to read size payload: %w", err)
		}

		size = (size << 8) | int64(b[0])
		if b[0] != 0xFF {
			isUnknown = false
		}
	}

	if isUnknown {
		return -1, nil
	}

	return size, nil
}
