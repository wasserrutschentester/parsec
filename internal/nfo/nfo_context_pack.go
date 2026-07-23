package nfo

import (
	"fmt"
	"strconv"
	"strings"

	"codeberg.org/upPollo/parsec/internal/mdb"
)

type packAggregator struct {
	earliestDate string

	anySubbed       bool
	anyDualAudio    bool
	anyHasAudioDesc bool
	anyRepack       bool

	resolutions   map[string]int
	audioCodecs   map[string]int
	audioChannels map[string]int
	videoCodecs   map[string]int
	services      map[string]int
	sources       map[string]int
	groups        map[string]int

	hdrFormats   map[string]int
	bitDepths    map[int]int
	scanTypes    map[string]int
	aspectRatios map[string]int

	totalVidBitrate int
	vidBitrateCount int

	maxAudioIdx   int
	maxSubIdx     int
	maxAudioCount int
	maxSubCount   int
}

func newPackAggregator() *packAggregator {
	return &packAggregator{
		resolutions:   make(map[string]int),
		audioCodecs:   make(map[string]int),
		audioChannels: make(map[string]int),
		videoCodecs:   make(map[string]int),
		services:      make(map[string]int),
		sources:       make(map[string]int),
		groups:        make(map[string]int),
		hdrFormats:    make(map[string]int),
		bitDepths:     make(map[int]int),
		scanTypes:     make(map[string]int),
		aspectRatios:  make(map[string]int),
	}
}

func (pa *packAggregator) processFile(i int, fctx *FileContext) {
	if pa.earliestDate == "" || (fctx.Date != "" && fctx.Date < pa.earliestDate) {
		pa.earliestDate = fctx.Date
	}

	pa.processBooleans(fctx)
	pa.processMaps(fctx)
	pa.processTracksAndBitrate(i, fctx)
}

func (pa *packAggregator) processBooleans(fctx *FileContext) {
	if fctx.Subbed {
		pa.anySubbed = true
	}

	if fctx.DualAudio {
		pa.anyDualAudio = true
	}

	if fctx.HasAudioDesc {
		pa.anyHasAudioDesc = true
	}

	if fctx.Repack {
		pa.anyRepack = true
	}
}

func (pa *packAggregator) processMaps(fctx *FileContext) {
	pa.resolutions[fctx.Resolution]++
	pa.audioCodecs[fctx.AudioCodec]++
	pa.audioChannels[fctx.AudioChannels]++
	pa.videoCodecs[fctx.VideoCodec]++
	pa.services[fctx.Service]++
	pa.sources[fctx.Source]++
	pa.groups[fctx.Group]++

	pa.hdrFormats[fctx.Video.HDRFormat]++
	pa.bitDepths[fctx.Video.BitDepth]++
	pa.scanTypes[fctx.Video.ScanType]++
	pa.aspectRatios[fctx.Video.AspectRatio]++
}

func (pa *packAggregator) processTracksAndBitrate(i int, fctx *FileContext) {
	if kbpsStr, ok := strings.CutSuffix(fctx.Video.Bitrate, " kb/s"); ok {
		if kbps, err := strconv.Atoi(kbpsStr); err == nil {
			pa.totalVidBitrate += kbps
			pa.vidBitrateCount++
		}
	}

	if len(fctx.Audio) > pa.maxAudioCount {
		pa.maxAudioCount = len(fctx.Audio)
		pa.maxAudioIdx = i
	}

	if len(fctx.Subtitles) > pa.maxSubCount {
		pa.maxSubCount = len(fctx.Subtitles)
		pa.maxSubIdx = i
	}
}

func (pa *packAggregator) applyToContext(ctx *Context) {
	if ctx.Date == "" {
		ctx.Date = pa.earliestDate
	}

	ctx.Subbed = pa.anySubbed
	ctx.DualAudio = pa.anyDualAudio
	ctx.HasAudioDesc = pa.anyHasAudioDesc
	ctx.Repack = pa.anyRepack

	pa.applyRootFields(ctx)

	if len(ctx.Files) > 0 {
		pa.applyTracksAndVideo(ctx)
	}
}

func (pa *packAggregator) applyRootFields(ctx *Context) {
	if ctx.Resolution == "" {
		ctx.Resolution = getModeStr(pa.resolutions)
	}

	if ctx.AudioCodec == "" {
		ctx.AudioCodec = getModeStr(pa.audioCodecs)
	}

	if ctx.AudioChannels == "" {
		ctx.AudioChannels = getModeStr(pa.audioChannels)
	}

	if ctx.VideoCodec == "" {
		ctx.VideoCodec = getModeStr(pa.videoCodecs)
	}

	if ctx.Service == "" {
		ctx.Service = getModeStr(pa.services)
	}

	if ctx.Source == "" {
		ctx.Source = getModeStr(pa.sources)
	}

	if ctx.Group == "" {
		ctx.Group = getModeStr(pa.groups)
	}
}

func (pa *packAggregator) applyTracksAndVideo(ctx *Context) {
	ctx.Audio = make([]Audio, len(ctx.Files[pa.maxAudioIdx].Audio))
	copy(ctx.Audio, ctx.Files[pa.maxAudioIdx].Audio)

	ctx.Subtitles = make([]Subtitle, len(ctx.Files[pa.maxSubIdx].Subtitles))
	copy(ctx.Subtitles, ctx.Files[pa.maxSubIdx].Subtitles)

	averageAudioTracks(ctx)
	averageSubtitleTracks(ctx)

	baseVid := ctx.Files[0].Video
	baseVid.HDRFormat = getModeStr(pa.hdrFormats)
	baseVid.ScanType = getModeStr(pa.scanTypes)
	baseVid.AspectRatio = getModeStr(pa.aspectRatios)
	baseVid.BitDepth = getModeInt(pa.bitDepths)

	if pa.vidBitrateCount > 0 {
		baseVid.Bitrate = fmt.Sprintf("%d kb/s", pa.totalVidBitrate/pa.vidBitrateCount)
	}

	ctx.Video = baseVid
}

func averageAudioTracks(ctx *Context) {
	for j := range ctx.Audio {
		var (
			totalKbps, countKbps         int
			totalDialNorm, countDialNorm float64
		)

		for _, fctx := range ctx.Files {
			if j < len(fctx.Audio) {
				if kbpsStr, ok := strings.CutSuffix(fctx.Audio[j].Bitrate, " kb/s"); ok {
					if kbps, err := strconv.Atoi(kbpsStr); err == nil {
						totalKbps += kbps
						countKbps++
					}
				}

				dnStr := strings.TrimSpace(strings.TrimSuffix(fctx.Audio[j].DialogNormalization, "dB"))
				if dn, err := strconv.ParseFloat(dnStr, 64); err == nil {
					totalDialNorm += dn
					countDialNorm++
				}
			}
		}

		if countKbps > 0 {
			ctx.Audio[j].Bitrate = fmt.Sprintf("%d kb/s", totalKbps/countKbps)
		}

		if countDialNorm > 0 {
			ctx.Audio[j].DialogNormalization = fmt.Sprintf("%.0f dB", totalDialNorm/countDialNorm)
		}
	}
}

func averageSubtitleTracks(ctx *Context) {
	for j := range ctx.Subtitles {
		var totalElements, countElements int

		for _, fctx := range ctx.Files {
			if j < len(fctx.Subtitles) {
				if count := fctx.Subtitles[j].ElementCount; count > 0 {
					totalElements += count
					countElements++
				}
			}
		}

		if countElements > 0 {
			ctx.Subtitles[j].ElementCount = totalElements / countElements
		}
	}
}

func getModeStr(m map[string]int) string {
	var (
		mode     string
		maxCount int
	)

	for k, v := range m {
		if v > maxCount && k != "" {
			maxCount = v
			mode = k
		}
	}

	return mode
}

func getModeInt(m map[int]int) int {
	var (
		mode     int
		maxCount int
	)

	for k, v := range m {
		if v > maxCount && k != 0 {
			maxCount = v
			mode = k
		}
	}

	return mode
}

func buildPackFileContext(ctx *Context, files []FileInput) {
	ctx.IsPack = true
	ctx.Files = make([]FileContext, len(files))

	var (
		totalBytes      int64
		totalSec        int
		allRawEpResults []mdb.EpisodeResult
		allEpisodes     []int
		allEpTitles     []string
	)

	agg := newPackAggregator()

	for i, fIn := range files {
		fctx := BuildFileContext(fIn)
		ctx.Files[i] = fctx

		totalBytes += fctx.SizeBytes
		totalSec += fctx.DurationSec

		allRawEpResults = append(allRawEpResults, fctx.RawEpisodeResults...)
		allEpisodes = append(allEpisodes, fctx.Episodes...)

		if fctx.EpisodeTitle != "" {
			allEpTitles = append(allEpTitles, fctx.EpisodeTitle)
		}

		agg.processFile(i, &fctx)
	}

	ctx.RawEpisodeResults = allRawEpResults
	ctx.Episodes = allEpisodes
	ctx.EpisodeTitles = allEpTitles
	ctx.EpisodeTitle = strings.Join(allEpTitles, " / ")

	ctx.SizeBytes = totalBytes
	ctx.Size = fmt.Sprintf("%.2f GiB", float64(totalBytes)/(1024*1024*1024))

	ctx.DurationSec = totalSec

	h := totalSec / 3600
	m := (totalSec % 3600) / 60
	s := totalSec % 60

	if h > 0 {
		ctx.Duration = fmt.Sprintf("%d h %d min", h, m)
	} else {
		ctx.Duration = fmt.Sprintf("%d min %d s", m, s)
	}

	agg.applyToContext(ctx)
}
