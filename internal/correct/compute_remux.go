package correct

import (
	"cmp"
	"slices"
	"strings"

	"golang.org/x/text/language"

	"codeberg.org/upPollo/parsec/internal/checks"
	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

// RemovalKind identifies why a track is proposed for removal.
type RemovalKind string

const (
	// RemovalUnwantedAudioLang identifies non-preferred, non-original audio removal.
	RemovalUnwantedAudioLang RemovalKind = "unwanted_audio_language"
	// RemovalEmptyTrack identifies an audio track carrying no channels.
	RemovalEmptyTrack RemovalKind = "empty_track"
)

// RemovalCandidate describes a track proposed for removal.
type RemovalCandidate struct {
	TrackID int                `json:"track_id"`
	Kind    RemovalKind        `json:"kind"`
	Reason  string             `json:"reason"`
	Track   matroska.EbmlTrack `json:"-"`
}

// MatroskaRemuxPlan describes the lossless remux operations needed to satisfy
// the remux-only Matroska checks. Track IDs are mkvmerge track IDs.
type MatroskaRemuxPlan struct {
	// TrackOrder is the desired output order of track IDs.
	// It is nil when the tracks are already correctly ordered.
	TrackOrder []int
	// StripCompressionIDs lists track IDs whose container compression should be
	// removed.
	StripCompressionIDs []int
	// RemovalCandidates lists tracks proposed for removal (requires confirmation).
	RemovalCandidates []RemovalCandidate
}

// IsEmpty reports whether the plan contains no work.
func (p MatroskaRemuxPlan) IsEmpty() bool {
	return len(p.TrackOrder) == 0 && len(p.StripCompressionIDs) == 0 && len(p.RemovalCandidates) == 0
}

// ComputeMatroskaRemux returns the remux operations needed for track ordering,
// compression removal, and pruning empty/unwanted audio tracks.
func ComputeMatroskaRemux(tracks []matroska.EbmlTrack, originalLang string) MatroskaRemuxPlan {
	removals := computeRemovalCandidates(tracks, originalLang)

	var survivingTracks []matroska.EbmlTrack

	for _, t := range tracks {
		removed := false

		for _, r := range removals {
			if r.TrackID == t.ID {
				removed = true

				break
			}
		}

		if !removed {
			survivingTracks = append(survivingTracks, t)
		}
	}

	return MatroskaRemuxPlan{
		TrackOrder:          computeTrackOrder(survivingTracks, originalLang),
		StripCompressionIDs: computeCompressionStrips(tracks),
		RemovalCandidates:   removals,
	}
}

// computeTrackOrder returns the desired output order (by track ID).
// It returns nil when the current order is already correct.
func computeTrackOrder(tracks []matroska.EbmlTrack, originalLang string) []int {
	if !config.IsCheckEnabled(config.CheckMatroskaTrackOrder) {
		return nil
	}

	var video, audio, subs, other []matroska.EbmlTrack

	for _, track := range tracks {
		switch track.Type {
		case "audio":
			audio = append(audio, track)
		case "subtitles":
			subs = append(subs, track)
		case "video":
			video = append(video, track)
		default:
			other = append(other, track)
		}
	}

	byPriority := func(a, b matroska.EbmlTrack) int {
		return cmp.Compare(checks.GetTrackPriority(a, originalLang), checks.GetTrackPriority(b, originalLang))
	}
	slices.SortStableFunc(audio, byPriority)
	slices.SortStableFunc(subs, byPriority)

	ordered := slices.Concat(video, audio, subs, other)

	desired := make([]int, len(ordered))
	for i, track := range ordered {
		desired[i] = track.ID
	}

	current := make([]int, len(tracks))
	for i, track := range tracks {
		current[i] = track.ID
	}

	if slices.Equal(desired, current) {
		return nil
	}

	return desired
}

func computeCompressionStrips(tracks []matroska.EbmlTrack) []int {
	if !config.IsCheckEnabled(config.CheckMatroskaZlibCompression) {
		return nil
	}

	var ids []int

	for _, track := range tracks {
		for algo := range strings.SplitSeq(track.Properties.ContentEncodingAlgorithms, ",") {
			if algo == "0" { // 0 = zlib
				ids = append(ids, track.ID)

				break
			}
		}
	}

	return ids
}

// removalCollector accumulates removal candidates, keeping the first reason
// recorded for a track so the more specific signal (e.g. exact duplicate) wins.
type removalCollector struct {
	seen       map[int]bool
	candidates []RemovalCandidate
}

func (c *removalCollector) add(track matroska.EbmlTrack, kind RemovalKind, reason string) {
	if c.seen[track.ID] {
		return
	}

	c.seen[track.ID] = true
	c.candidates = append(c.candidates, RemovalCandidate{TrackID: track.ID, Kind: kind, Reason: reason, Track: track})
}

func computeRemovalCandidates(tracks []matroska.EbmlTrack, originalLang string) []RemovalCandidate {
	collector := &removalCollector{seen: make(map[int]bool)}

	collectUnwantedLanguageAudio(collector, tracks, originalLang)
	collectEmptyAudioTracks(collector, tracks)

	return collector.candidates
}

// collectEmptyAudioTracks proposes removal of audio tracks reporting zero channels.
func collectEmptyAudioTracks(collector *removalCollector, tracks []matroska.EbmlTrack) {
	if !config.IsCheckEnabled(config.CheckMediainfoEmptyTracks) {
		return
	}

	for _, track := range tracks {
		if track.Type == "audio" && track.Properties.AudioChannels <= 0 {
			collector.add(track, RemovalEmptyTrack, "audio track has zero channels")
		}
	}
}

func collectUnwantedLanguageAudio(collector *removalCollector, tracks []matroska.EbmlTrack, originalLang string) {
	// Without the original language we cannot tell which non-preferred track is
	// the legitimate original, so we skip pruning entirely to stay safe.
	if originalLang == "" || !config.IsCheckEnabled(config.CheckMdbUnwantedAudioLang) {
		return
	}

	prefTag := language.Make(config.GetPreferredLanguage())
	origTag := language.Make(originalLang)

	for _, track := range tracks {
		if track.Type != "audio" {
			continue
		}

		langTag := language.Make(track.Properties.Language)
		if !checks.IsWantedAudioLang(langTag, prefTag, origTag) {
			collector.add(track, RemovalUnwantedAudioLang, "unwanted audio language '"+track.Properties.Language+"'")
		}
	}
}
