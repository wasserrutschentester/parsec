package checks

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"codeberg.org/upPollo/parsec/internal/config"
	"codeberg.org/upPollo/parsec/internal/metadata/matroska"
	"codeberg.org/upPollo/parsec/internal/types"
)

func getUnusedFontsResult(res []CheckResult) *CheckResult {
	for _, r := range res {
		if r.Identifier == "matroska_unused_fonts" {
			rCopy := r

			return &rCopy
		}
	}

	return nil
}

//nolint:paralleltest // depends on shared global state
func TestRunTrackChecksUnusedFonts(t *testing.T) {
	config.InitDefaults()

	ebml := &matroska.EbmlMetadata{
		Tracks: []matroska.EbmlTrack{
			{
				ID:    1,
				Type:  "subtitles",
				Codec: "S_TEXT/ASS",
				Properties: matroska.EbmlTrackProperties{
					Language:     "ger",
					Number:       1,
					CodecPrivate: "5b56342b205374796c65735d0a466f726d61743a204e616d652c20466f6e746e616d650a5374796c653a2044656661756c742c20417269616c0a",
				},
			},
		},
		Attachments: []matroska.EbmlAttachment{
			{ID: 1, FileName: "Arial.ttf", ContentType: "font/ttf"},
			{ID: 2, FileName: "UnusedFont.ttf", ContentType: "font/ttf"},
		},
	}

	attachmentFonts := []matroska.AttachmentFontInfo{
		{AttachmentID: 1, FileName: "Arial.ttf", FamilyName: "Arial", Weight: 400, Italic: false},
		{AttachmentID: 2, FileName: "UnusedFont.ttf", FamilyName: "UnusedFont", Weight: 400, Italic: false},
	}

	res := runTrackChecks("", ebml, nil, attachmentFonts, nil)
	targetRes := getUnusedFontsResult(res)

	if targetRes == nil {
		t.Fatal("Did not find unused fonts check result")
	}

	if targetRes.Table == nil {
		t.Fatalf("Expected Table to be populated")
	}

	foundUnused := false
	foundArial := false

	for _, row := range targetRes.Table.Rows {
		if len(row) > 0 && row[0] == "UnusedFont.ttf" {
			foundUnused = true
		}

		if len(row) > 0 && row[0] == "Arial.ttf" {
			foundArial = true
		}
	}

	if !foundUnused {
		t.Errorf("Expected table to contain UnusedFont.ttf")
	}

	if foundArial {
		t.Errorf("Table should not contain Arial.ttf")
	}
}

func getFontComplianceResult(res []CheckResult) *CheckResult {
	for _, r := range res {
		if r.Identifier == "matroska_font_filename_compliance" {
			rCopy := r

			return &rCopy
		}
	}

	return nil
}

func assertFontComplianceTable(t *testing.T, table *types.TableData) {
	t.Helper()

	if table == nil {
		t.Fatal("Expected TableData to be populated")
	}

	var contentBuilder strings.Builder

	for _, row := range table.Rows {
		contentBuilder.WriteString(strings.Join(row, " ") + "\n")
	}

	content := contentBuilder.String()

	if !strings.Contains(content, "WrongName.ttf") {
		t.Errorf("Expected table to contain WrongName.ttf")
	}

	if !strings.Contains(content, "CorrectName.ttf") {
		t.Errorf("Expected table to contain proposed CorrectName.ttf")
	}

	if strings.Contains(content, "Arial.ttf") {
		t.Errorf("Table should not contain Arial.ttf")
	}
}

//nolint:paralleltest // mutates global state via viper.Set
func TestRunTrackChecksFontFilenameCompliance(t *testing.T) {
	config.InitDefaults()
	viper.Set("enabled_checks", []string{"all"})

	defer viper.Reset()

	ebml := &matroska.EbmlMetadata{
		Tracks: []matroska.EbmlTrack{},
		Attachments: []matroska.EbmlAttachment{
			{ID: 1, FileName: "Arial.ttf", ContentType: "font/ttf"},
			{ID: 2, FileName: "WrongName.ttf", ContentType: "font/ttf"},
		},
	}

	attachmentFonts := []matroska.AttachmentFontInfo{
		{AttachmentID: 1, FileName: "Arial.ttf", FamilyName: "Arial"},
		{AttachmentID: 2, FileName: "WrongName.ttf", FamilyName: "CorrectName"},
	}

	res := runTrackChecks("", ebml, nil, attachmentFonts, nil)

	targetRes := getFontComplianceResult(res)
	if targetRes == nil {
		t.Fatal("Did not find font filename compliance check result")
	}

	if targetRes.Severity != "info" {
		t.Errorf("Expected severity to be info, got '%s'", targetRes.Severity)
	}

	assertFontComplianceTable(t, targetRes.Table)
}

//nolint:paralleltest // depends on global state via config.InitDefaults()
func TestCheckSubtitleFontsBoldStyle(t *testing.T) {
	config.InitDefaults()

	// Bold Arial style
	codecPrivate := []byte("[V4+ Styles]\nFormat: Name, Fontname, Bold\nStyle: Default, Arial, -1\n")
	track := matroska.EbmlTrack{
		Type:  "subtitles",
		Codec: "S_TEXT/ASS",
		Properties: matroska.EbmlTrackProperties{
			TextSubtitles: true,
			CodecPrivate:  hex.EncodeToString(codecPrivate),
		},
	}

	// 1. Regular Arial attached - should fail
	attachmentFonts := []matroska.AttachmentFontInfo{
		{FamilyName: "Arial", Weight: 400, Italic: false},
	}

	res := checkSubtitleFonts(track, attachmentFonts, make(map[fontStyle]bool))
	if res == nil || res.Passed {
		t.Error("Expected validation failure when bold style uses only regular font")
	}

	// 2. Bold Arial attached - should pass
	attachmentFonts = []matroska.AttachmentFontInfo{
		{FamilyName: "Arial", Weight: 700, Italic: false},
	}

	res = checkSubtitleFonts(track, attachmentFonts, make(map[fontStyle]bool))
	if res != nil {
		t.Errorf("Expected validation pass when bold style has bold font attached: %v", res.Warning)
	}

	// 3. Variable font Arial attached - should pass
	attachmentFonts = []matroska.AttachmentFontInfo{
		{FamilyName: "Arial", Weight: 400, Italic: false, IsVariable: true},
	}

	res = checkSubtitleFonts(track, attachmentFonts, make(map[fontStyle]bool))
	if res != nil {
		t.Errorf("Expected validation pass when bold style has variable font attached: %v", res.Warning)
	}
}

//nolint:paralleltest // depends on global state via config.InitDefaults()
func TestCheckSubtitleInlineFontsWithContentBoldOverrides(t *testing.T) {
	config.InitDefaults()

	codecPrivate := []byte("[V4+ Styles]\nFormat: Name, Fontname, Bold\nStyle: Default, Arial, 0\n")
	track := matroska.EbmlTrack{
		Type:  "subtitles",
		Codec: "S_TEXT/ASS",
		Properties: matroska.EbmlTrackProperties{
			TextSubtitles: true,
			CodecPrivate:  hex.EncodeToString(codecPrivate),
		},
	}
	// Event content using \b1 (bold) inline override
	content := []byte(string(codecPrivate) + "\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:05.00,Default,,0,0,0,,{\\b1}Bold Override Text\n")

	// 1. Regular Arial attached - should fail
	attachmentFonts := []matroska.AttachmentFontInfo{
		{FamilyName: "Arial", Weight: 400, Italic: false},
	}

	res := checkSubtitleInlineFontsWithContent(track, attachmentFonts, content, make(map[fontStyle]bool))
	if res == nil || res.Passed {
		t.Error("Expected validation failure when inline bold tag only has regular font")
	}

	// 2. Bold Arial attached - should pass
	attachmentFonts = []matroska.AttachmentFontInfo{
		{FamilyName: "Arial", Weight: 700, Italic: false},
	}

	res = checkSubtitleInlineFontsWithContent(track, attachmentFonts, content, make(map[fontStyle]bool))
	if res != nil {
		t.Errorf("Expected validation pass when inline bold tag has bold font attached: %v", res.Warning)
	}
}

//nolint:paralleltest // depends on global state via config.InitDefaults()
func TestCheckUnusedFontsStyleAware(t *testing.T) {
	config.InitDefaults()

	attachments := []matroska.EbmlAttachment{
		{ID: 1, FileName: "Arial-Italic.ttf", ContentType: "font/ttf"},
	}
	attachmentFonts := []matroska.AttachmentFontInfo{
		{AttachmentID: 1, FileName: "Arial-Italic.ttf", FamilyName: "Arial", Weight: 400, Italic: true},
	}

	// Only regular Arial is used
	allUsedFonts := map[fontStyle]bool{
		{Family: "Arial", Weight: 400, Italic: false}: true,
	}

	res := checkUnusedFonts(attachments, attachmentFonts, allUsedFonts)
	if res == nil || res.Passed {
		t.Error("Expected Arial-Italic.ttf to be flagged as unused since only Regular is used")
	}
}

type srtTestCase struct {
	name             string
	content          string
	expectedPassed   bool
	expectedSeverity string
	containsWarning  string
}

func getSRTTestCases() []srtTestCase {
	return []srtTestCase{
		{
			name:           "Valid SRT with allowed tags and multiline block",
			content:        "1\n00:00:01,000 --> 00:00:04,500\nWelcome to the <b>SRT\nSpecification Guide</b>.\n\n2\n00:00:05,100 --> 00:00:08,200\nYou can use <i>italics</i> or <font color=\"red\">colored text</font>.\n",
			expectedPassed: true,
		},
		{
			name:             "Invalid SRT with WebVTT tags",
			content:          "1\n00:00:01,000 --> 00:00:04,500\n<v Speaker 1>Welcome to the guide.</v>\n",
			expectedPassed:   false,
			expectedSeverity: "warning",
			containsWarning:  "disallowed HTML-like tag '<v>'",
		},
		{
			name:             "Invalid SRT with unclosed tag",
			content:          "1\n00:00:01,000 --> 00:00:04,500\nWelcome to the <b>SRT Specification Guide.\n",
			expectedPassed:   false,
			expectedSeverity: "warning",
			containsWarning:  "unclosed HTML tag 'b'",
		},
		{
			name:             "SRT with alignment info throws info level warning",
			content:          "1\n00:00:01,000 --> 00:00:04,500\n{\\an8}Welcome to the top center!\n",
			expectedPassed:   false,
			expectedSeverity: "info",
			containsWarning:  "Alignment/positioning detected",
		},
		{
			name:             "SRT with coordinate metadata on timestamp line throws info level warning",
			content:          "1\n00:00:01,000 --> 00:00:04,500 X1:100 Y1:50\nWelcome to the coordinates guide.\n",
			expectedPassed:   false,
			expectedSeverity: "info",
			containsWarning:  "contains display coordinates/metadata",
		},
		{
			name:             "SRT with 6 invalid HTML tags limits to 5 results and adds more message",
			content:          "1\n00:00:01,000 --> 00:00:04,500\n<c>1</c>\n\n2\n00:00:05,000 --> 00:00:08,000\n<d>2</d>\n\n3\n00:00:09,000 --> 00:00:12,000\n<e>3</e>\n\n4\n00:00:13,000 --> 00:00:16,000\n<f>4</f>\n\n5\n00:00:17,000 --> 00:00:20,000\n<g>5</g>\n\n6\n00:00:21,000 --> 00:00:24,000\n<h>6</h>\n",
			expectedPassed:   false,
			expectedSeverity: "warning",
			containsWarning:  "... and 1 more",
		},
		{
			name:           "Valid SRT starting with UTF-8 BOM",
			content:        "\ufeff1\n00:00:01,000 --> 00:00:04,500\nWelcome to the guide.\n",
			expectedPassed: true,
		},
	}
}

func TestCheckSRTValidation(t *testing.T) {
	t.Parallel()

	track := matroska.EbmlTrack{
		ID:    1,
		Type:  "subtitles",
		Codec: "S_TEXT/SRT",
		Properties: matroska.EbmlTrackProperties{
			Number:   1,
			Language: "eng",
		},
	}

	tests := getSRTTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			res := checkSRTValidation(track, []byte(tt.content))
			assertSRTTestCase(t, tt, res)
		})
	}
}

func assertSRTTestCase(t *testing.T, tt srtTestCase, res *CheckResult) {
	t.Helper()

	if tt.expectedPassed {
		if res != nil {
			t.Errorf("Expected nil check result, got: %+v (Warning: %s)", res, res.Warning)
		}

		return
	}

	if res == nil {
		t.Fatalf("Expected non-nil check result")
	}

	if res.Passed {
		t.Errorf("Expected Passed=false, got %v", res.Passed)
	}

	if res.Severity != tt.expectedSeverity {
		t.Errorf("Expected Severity=%q, got %q", tt.expectedSeverity, res.Severity)
	}

	actualWarning := getActualWarning(res)

	if tt.containsWarning != "" && !strings.Contains(actualWarning, tt.containsWarning) {
		t.Errorf("Expected warning to contain %q, got %q", tt.containsWarning, actualWarning)
	}
}

func getActualWarning(res *CheckResult) string {
	actualWarning := ""
	if len(res.Tracks) > 0 {
		actualWarning = res.Tracks[0].Warning
		if len(res.Tracks[0].List) > 0 {
			actualWarning += strings.Join(res.Tracks[0].List, "\n")
		}
	} else {
		actualWarning = res.Warning
		if len(res.List) > 0 {
			actualWarning += strings.Join(res.List, "\n")
		}
	}

	return actualWarning
}
