package templateutil

import (
	"fmt"
	"strings"
	"text/template"
)

// KnownTemplateFuncs returns a stub FuncMap containing all supported template function names.
// This allows parsing and validating template syntax without external package dependencies.
func KnownTemplateFuncs() template.FuncMap {
	stub := func(_ ...any) any { return "" }
	stubBool := func(_ ...any) bool { return false }
	stubSlice := func(_ ...any) []any { return nil }

	return template.FuncMap{
		"join":         stub,
		"cat":          stub,
		"cond":         stub,
		"when":         stub,
		"pad":          stub,
		"eprange":      stub,
		"vcodec":       stub,
		"aka":          stub,
		"upper":        stub,
		"lower":        stub,
		"title":        stub,
		"replace":      stub,
		"regexReplace": stub,
		"contains":     stubBool,
		"trimPrefix":   stub,
		"trimSuffix":   stub,
		"hasPrefix":    stubBool,
		"hasSuffix":    stubBool,
		"default":      stub,
		"where":        stub,
		"pluck":        stubSlice,
		"first":        stub,
		"last":         stub,
		"uniq":         stub,
		"indexOrEmpty": stub,
		"languageName": stub,
		"parseDate":    stub,
	}
}

// ValidateTemplate checks if a template string contains valid Go template syntax.
// If the template does not contain "{{", it is transpiled from legacy syntax before validation.
func ValidateTemplate(tmplStr string) error {
	if !strings.Contains(tmplStr, "{{") {
		tmplStr = TranspileLegacyTemplate(tmplStr)
	}

	_, err := template.New("validate").Funcs(KnownTemplateFuncs()).Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("invalid template syntax: %w", err)
	}

	return nil
}
