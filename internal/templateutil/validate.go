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

	funcs := []string{
		"join", "cat", "when", "pad", "eprange", "vcodec", "aka",
		"upper", "lower", "title", "replace", "regexReplace",
		"contains", "trimPrefix", "trimSuffix", "hasPrefix", "hasSuffix",
		"default", "where", "pluck", "first", "last", "uniq",
		"indexOrEmpty", "languageName", "parseDate",
	}

	fm := make(template.FuncMap, len(funcs))
	for _, fn := range funcs {
		fm[fn] = stub
	}

	return fm
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
