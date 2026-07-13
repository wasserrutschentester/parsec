package nfo

import (
	"bytes"
	"testing"
	"text/template"
)

func renderTestHelper(t *testing.T, tmplStr string, data any) string {
	t.Helper()

	funcs := templateFuncs()

	tmpl, err := template.New("test").Funcs(funcs).Parse(tmplStr)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("Failed to execute template: %v", err)
	}

	return buf.String()
}

func TestFuncDict(t *testing.T) {
	t.Parallel()

	res := renderTestHelper(t, `{{ $d := dict "a" 1 "b" "two" }}{{ index $d "a" }}-{{ index $d "b" }}`, nil)
	if res != "1-two" {
		t.Errorf("Expected 1-two, got %q", res)
	}
}

func TestFuncDefault(t *testing.T) {
	t.Parallel()

	res := renderTestHelper(t, `{{ default "fallback" .Val }}`, map[string]any{"Val": ""})
	if res != "fallback" {
		t.Errorf("Expected fallback, got %q", res)
	}

	res2 := renderTestHelper(t, `{{ default "fallback" .Val }}`, map[string]any{"Val": "value"})
	if res2 != "value" {
		t.Errorf("Expected value, got %q", res2)
	}
}

func TestFuncWhere(t *testing.T) {
	t.Parallel()

	items := []map[string]any{
		{"Name": "one", "Forced": true},
		{"Name": "two", "Forced": false},
		{"Name": "three", "Forced": true},
	}

	res := renderTestHelper(t, `{{ range where "Forced" true . }}{{ .Name }} {{ end }}`, items)
	if res != "one three " {
		t.Errorf("Expected 'one three ', got %q", res)
	}
}

func TestFuncFirstLast(t *testing.T) {
	t.Parallel()

	items := []string{"a", "b", "c", "d"}

	resFirst := renderTestHelper(t, `{{ range first 2 . }}{{ . }}{{ end }}`, items)
	if resFirst != "ab" {
		t.Errorf("Expected ab, got %q", resFirst)
	}

	resLast := renderTestHelper(t, `{{ range last 2 . }}{{ . }}{{ end }}`, items)
	if resLast != "cd" {
		t.Errorf("Expected cd, got %q", resLast)
	}
}

func TestFuncToLower(t *testing.T) {
	t.Parallel()

	res := renderTestHelper(t, `{{ "HELLO" | toLower }}`, nil)
	if res != "hello" {
		t.Errorf("Expected hello, got %q", res)
	}
}

func TestFuncTrim(t *testing.T) {
	t.Parallel()

	resPrefix := renderTestHelper(t, `{{ "V_AVC" | trimPrefix "V_" }}`, nil)
	if resPrefix != "AVC" {
		t.Errorf("Expected AVC, got %q", resPrefix)
	}

	resSuffix := renderTestHelper(t, `{{ "DTS MA" | trimSuffix " MA" }}`, nil)
	if resSuffix != "DTS" {
		t.Errorf("Expected DTS, got %q", resSuffix)
	}
}

func TestFuncHasPrefixSuffix(t *testing.T) {
	t.Parallel()

	resPrefix := renderTestHelper(t, `{{ if hasPrefix "V_" "V_AVC" }}yes{{ else }}no{{ end }}`, nil)
	if resPrefix != "yes" {
		t.Errorf("Expected yes, got %q", resPrefix)
	}

	resSuffix := renderTestHelper(t, `{{ if hasSuffix " MA" "DTS MA" }}yes{{ else }}no{{ end }}`, nil)
	if resSuffix != "yes" {
		t.Errorf("Expected yes, got %q", resSuffix)
	}
}

func TestFuncHumanize(t *testing.T) {
	t.Parallel()

	res := renderTestHelper(t, `{{ "HearingImpaired" | humanize }}`, nil)
	if res != "Hearing impaired" {
		t.Errorf("Expected 'Hearing impaired', got %q", res)
	}
}

func TestFuncTruncate(t *testing.T) {
	t.Parallel()

	res := renderTestHelper(t, `{{ "This is a very long plot description that needs truncation" | truncate 20 }}`, nil)
	if res != "This is a very..." {
		t.Errorf("Expected 'This is a very...', got %q", res)
	}
}

func TestFuncChomp(t *testing.T) {
	t.Parallel()

	res := renderTestHelper(t, `{{ "hello\n\n" | chomp }}`, nil)
	if res != "hello" {
		t.Errorf("Expected hello, got %q", res)
	}
}

func TestFuncMath(t *testing.T) {
	t.Parallel()

	resMin := renderTestHelper(t, `{{ min 5 2 8 }}`, nil)
	if resMin != "2" {
		t.Errorf("Expected 2, got %q", resMin)
	}

	resMax := renderTestHelper(t, `{{ max 5 2 8 }}`, nil)
	if resMax != "8" {
		t.Errorf("Expected 8, got %q", resMax)
	}

	resRound := renderTestHelper(t, `{{ round 5.6 }}`, nil)
	if resRound != "6" {
		t.Errorf("Expected 6, got %q", resRound)
	}
}

func TestFuncMediaFormatting(t *testing.T) {
	t.Parallel()

	resDur1 := renderTestHelper(t, `{{ 6330 | formatDuration }}`, nil)
	if resDur1 != "01:45:30" {
		t.Errorf("Expected 01:45:30, got %q", resDur1)
	}

	resDur2 := renderTestHelper(t, `{{ 6330 | formatDuration "h'h' m'min' s's'" }}`, nil)
	if resDur2 != "1h 45min 30s" {
		t.Errorf("Expected 1h 45min 30s, got %q", resDur2)
	}

	resSize1 := renderTestHelper(t, `{{ 1500000000 | formatSizeDynamic 2 }}`, nil)
	if resSize1 != "1.40 GiB" {
		t.Errorf("Expected 1.40 GiB, got %q", resSize1)
	}

	resSize2 := renderTestHelper(t, `{{ 2748779069440 | formatSizeDynamic 2 }}`, nil)
	if resSize2 != "2.50 TiB" {
		t.Errorf("Expected 2.50 TiB, got %q", resSize2)
	}

	resBit1 := renderTestHelper(t, `{{ 5120000 | formatBitrate }}`, nil)
	if resBit1 != "5 120 kb/s" {
		t.Errorf("Expected 5 120 kb/s, got %q", resBit1)
	}

	resBit2 := renderTestHelper(t, `{{ 5120000 | formatBitrate "mbps" }}`, nil)
	if resBit2 != "5.12 Mbps" {
		t.Errorf("Expected 5.12 Mbps, got %q", resBit2)
	}

	resBit3 := renderTestHelper(t, `{{ "5000 kb/s" | formatBitrate "mbps" }}`, nil)
	if resBit3 != "5.00 Mbps" {
		t.Errorf("Expected 5.00 Mbps, got %q", resBit3)
	}
}
