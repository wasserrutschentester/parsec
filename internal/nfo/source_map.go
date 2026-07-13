package nfo

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	// ErrInvalidSourceMapFormat is returned when the source-map string is malformed.
	ErrInvalidSourceMapFormat = errors.New("invalid source-map format, expected '<tracks>:<source>'")
	// ErrInvalidTrackSelector is returned when a specific track selector is invalid.
	ErrInvalidTrackSelector = errors.New("invalid track selector")
	// ErrInvalidVideoTrackIndex is returned when a video track index other than 1 is specified.
	ErrInvalidVideoTrackIndex = errors.New("invalid video track index (only v1 is supported)")
	// ErrAudioTrackOutOfBounds is returned when the specified audio track does not exist.
	ErrAudioTrackOutOfBounds = errors.New("audio track index is out of bounds")
	// ErrSubtitleTrackOutOfBounds is returned when the specified subtitle track does not exist.
	ErrSubtitleTrackOutOfBounds = errors.New("subtitle track index is out of bounds")
	// ErrUnsupportedTrackPrefix is returned when an unknown prefix is used in a track selector.
	ErrUnsupportedTrackPrefix = errors.New("unsupported track prefix")
)

var (
	rangeRegex  = regexp.MustCompile(`^([a-z])(\d+)-(\d+)$`)
	singleRegex = regexp.MustCompile(`^([a-z])(\d+)$`)
)

// ApplySourceMap parses track selections and assigns sources to matched tracks.
func ApplySourceMap(ctx *Context, sourceMaps []string) error {
	for _, mapping := range sourceMaps {
		if err := applyMapping(ctx, mapping); err != nil {
			return err
		}
	}

	return nil
}

func applyMapping(ctx *Context, mapping string) error {
	parts := strings.SplitN(mapping, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("%w: '%s'", ErrInvalidSourceMapFormat, mapping)
	}

	targetsStr := parts[0]
	sourceVal := strings.TrimSpace(parts[1])

	for target := range strings.SplitSeq(targetsStr, ",") {
		target = strings.ToLower(strings.TrimSpace(target))
		if target == "" {
			continue
		}

		if err := applyTarget(ctx, target, sourceVal); err != nil {
			return fmt.Errorf("%w '%s' in mapping '%s'", err, target, mapping)
		}
	}

	return nil
}

func applyTarget(ctx *Context, target, sourceVal string) error {
	// Handle range e.g. a1-5
	if matches := rangeRegex.FindStringSubmatch(target); matches != nil {
		prefix := matches[1]
		start, _ := strconv.Atoi(matches[2])
		end, _ := strconv.Atoi(matches[3])

		if start > end {
			start, end = end, start
		}

		for i := start; i <= end; i++ {
			if err := assignSource(ctx, prefix, i, sourceVal); err != nil {
				return err
			}
		}

		return nil
	}

	// Handle single e.g. v1, a2
	if matches := singleRegex.FindStringSubmatch(target); matches != nil {
		prefix := matches[1]
		idx, _ := strconv.Atoi(matches[2])

		return assignSource(ctx, prefix, idx, sourceVal)
	}

	return ErrInvalidTrackSelector
}

func assignSource(ctx *Context, prefix string, idx int, source string) error {
	switch prefix {
	case "v":
		if idx != 1 {
			// We only keep track of 1 video in ctx.Video currently
			return fmt.Errorf("%w: %d", ErrInvalidVideoTrackIndex, idx)
		}

		ctx.Video.Source = source
	case "a":
		if idx < 1 || idx > len(ctx.Audio) {
			return fmt.Errorf("%w: %d", ErrAudioTrackOutOfBounds, idx)
		}

		ctx.Audio[idx-1].Source = source
	case "s":
		if idx < 1 || idx > len(ctx.Subtitles) {
			return fmt.Errorf("%w: %d", ErrSubtitleTrackOutOfBounds, idx)
		}

		ctx.Subtitles[idx-1].Source = source
	default:
		return fmt.Errorf("%w: '%s'", ErrUnsupportedTrackPrefix, prefix)
	}

	return nil
}
