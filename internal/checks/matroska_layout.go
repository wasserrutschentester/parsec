package checks

import (
	"fmt"
	"io"
	"os"
	"strings"

	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
)

// LayoutReport contains the diagnostic results of the Matroska layout check.
type LayoutReport struct {
	HasCluster                  bool
	HasSeekHeadBeforeCluster    bool
	HasInfoBeforeCluster        bool
	HasTracksBeforeCluster      bool
	HasCuesBeforeCluster        bool
	HasAttachmentsBeforeCluster bool
	HasChaptersBeforeCluster    bool
}

// IsOptimal returns true if the file follows the streaming-optimized layout.
func (r LayoutReport) IsOptimal() bool {
	return r.HasCluster && r.HasInfoBeforeCluster && r.HasTracksBeforeCluster && r.HasSeekHeadBeforeCluster
}

// DiagnosticMessage generates a human-readable explanation of layout issues.
func (r LayoutReport) DiagnosticMessage() string {
	if r.IsOptimal() {
		return "File layout is optimal. (Info and Tracks precede the media payload)."
	}

	if !r.HasCluster {
		return "File contains no video or audio data (Missing Cluster elements)."
	}

	var issues []string
	if !r.HasInfoBeforeCluster {
		issues = append(issues, "'Info'")
	}

	if !r.HasTracksBeforeCluster {
		issues = append(issues, "'Tracks'")
	}

	if !r.HasSeekHeadBeforeCluster {
		issues = append(issues, "'SeekHead'")
	}

	return fmt.Sprintf("Suboptimal Data layout: %s element is missing or placed after the media data", strings.Join(issues, ", "))
}

// ValidateMatroskaLayout extracts top-level elements up to the first Cluster
// and returns a diagnostic report of the layout structure.
func ValidateMatroskaLayout(r io.ReadSeeker) (*LayoutReport, error) {
	opts := matroska.ExtractOptions{HaltOnFirstCluster: true}

	positions, err := matroska.ExtractSegmentPositions(r, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to extract matroska elements: %w", err)
	}

	report := &LayoutReport{}

	for _, pos := range positions {
		switch pos.ID {
		case matroska.ElementSeekHead:
			report.HasSeekHeadBeforeCluster = true
		case matroska.ElementInfo:
			report.HasInfoBeforeCluster = true
		case matroska.ElementTracks:
			report.HasTracksBeforeCluster = true
		case matroska.ElementCues:
			report.HasCuesBeforeCluster = true
		case matroska.ElementAttachments:
			report.HasAttachmentsBeforeCluster = true
		case matroska.ElementChapters:
			report.HasChaptersBeforeCluster = true
		case matroska.ElementCluster:
			report.HasCluster = true

			return report, nil // Early exit optimization
		}
	}

	return report, nil
}

func checkDataLayout(filePath string) *CheckResult {
	res := &CheckResult{
		Identifier: "matroska_data_layout",
		Passed:     false,
		Severity:   "error",
	}

	f, err := os.Open(filePath)
	if err != nil {
		res.Warning = fmt.Sprintf("Failed to open file for layout check: %v", err)

		return res
	}

	defer func() { _ = f.Close() }()

	report, err := ValidateMatroskaLayout(f)
	if err != nil {
		res.Warning = fmt.Sprintf("Failed to parse Matroska layout: %v", err)

		return res
	}

	if !report.IsOptimal() {
		if report.HasCluster {
			res.Severity = "warning"
		}

		res.Warning = report.DiagnosticMessage()

		return res
	}

	// Returning nil means the check passed successfully (the aggregator ignores nil).
	return nil
}
