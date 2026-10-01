// Package templateutil provides query and manipulation utilities for template rendering.
package templateutil

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"
)

var (
	// ErrWhereInvalidArgs is returned when Where receives fewer than 3 or more than 4 arguments.
	ErrWhereInvalidArgs = errors.New("where requires 3 or 4 arguments: where key [operator] matchValue items")
	// ErrWhereKeyNotString is returned when Where's key argument is not a string.
	ErrWhereKeyNotString = errors.New("where key must be a string")
	// ErrWhereOpNotString is returned when Where's operator argument is not a string.
	ErrWhereOpNotString = errors.New("where operator must be a string")
	// ErrWhereNotSlice is returned when Where's items argument is not a slice or array.
	ErrWhereNotSlice = errors.New("where items must be a slice or array")
	// ErrFirstNotSlice is returned when First's items argument is not a slice or array.
	ErrFirstNotSlice = errors.New("first: items must be a slice or array")
	// ErrFirstNegative is returned when First's limit argument is negative.
	ErrFirstNegative = errors.New("first: limit must be non-negative")
	// ErrLastNotSlice is returned when Last's items argument is not a slice or array.
	ErrLastNotSlice = errors.New("last: items must be a slice or array")
	// ErrLastNegative is returned when Last's limit argument is negative.
	ErrLastNegative = errors.New("last: limit must be non-negative")
)

// Pluck extracts values of a named field across a slice or array of structs or maps into a []any.
func Pluck(key string, items any) []any {
	if items == nil {
		return nil
	}

	v := reflect.ValueOf(items)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return nil
	}

	var result []any

	for i := 0; i < v.Len(); i++ {
		if val, ok := pluckValue(v.Index(i), key); ok {
			result = append(result, val)
		}
	}

	return result
}

func pluckValue(elem reflect.Value, key string) (any, bool) {
	if elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}

	if elem.Kind() == reflect.Struct {
		field := elem.FieldByName(key)
		if field.IsValid() {
			return field.Interface(), true
		}
	} else if elem.Kind() == reflect.Map {
		keyVal := reflect.ValueOf(key)

		val := elem.MapIndex(keyVal)
		if val.IsValid() {
			return val.Interface(), true
		}
	}

	return nil, false
}

// Uniq deduplicates elements in a slice or array while preserving order.
func Uniq(items any) []any {
	if items == nil {
		return nil
	}

	v := reflect.ValueOf(items)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return []any{items}
	}

	var result []any

	seen := make(map[string]bool)

	for i := 0; i < v.Len(); i++ {
		val := v.Index(i).Interface()

		strKey := fmt.Sprint(val)
		if !seen[strKey] {
			seen[strKey] = true

			result = append(result, val)
		}
	}

	return result
}

// Contains checks if search exists in items (slice, array, or substring of string).
func Contains(search, items any) bool {
	if items == nil {
		return false
	}

	if s, ok := items.(string); ok {
		if searchStr, ok := search.(string); ok {
			return strings.Contains(s, searchStr)
		}
	}

	v := reflect.ValueOf(items)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return items == search
	}

	for i := 0; i < v.Len(); i++ {
		if v.Index(i).Interface() == search {
			return true
		}
	}

	return false
}

// First returns the first limit items of a slice or array.
func First(limit int, items any) (any, error) {
	if items == nil {
		return []any{}, nil
	}

	v := reflect.ValueOf(items)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return nil, ErrFirstNotSlice
	}

	n := v.Len()

	if limit < 0 {
		return nil, ErrFirstNegative
	}

	if limit > n {
		limit = n
	}

	result := make([]any, limit)
	for i := 0; i < limit; i++ {
		result[i] = v.Index(i).Interface()
	}

	return result, nil
}

// Last returns the last limit items of a slice or array.
func Last(limit int, items any) (any, error) {
	if items == nil {
		return []any{}, nil
	}

	v := reflect.ValueOf(items)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return nil, ErrLastNotSlice
	}

	n := v.Len()

	if limit < 0 {
		return nil, ErrLastNegative
	}

	if limit > n {
		limit = n
	}

	result := make([]any, limit)

	start := n - limit
	for i := 0; i < limit; i++ {
		result[i] = v.Index(start + i).Interface()
	}

	return result, nil
}

// IndexOrEmpty safely extracts an item by index from a slice or array, returning "" if out of bounds.
func IndexOrEmpty(index int, items any) any {
	if items == nil {
		return ""
	}

	v := reflect.ValueOf(items)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	if (v.Kind() == reflect.Slice || v.Kind() == reflect.Array) && index >= 0 && index < v.Len() {
		return v.Index(index).Interface()
	}

	return ""
}

// IsEmpty checks if a value is nil, zero, empty string, or empty slice/map.
func IsEmpty(val any) bool {
	if val == nil {
		return true
	}

	v := reflect.ValueOf(val)
	switch v.Kind() {
	case reflect.String:
		return v.Len() == 0
	case reflect.Slice, reflect.Array, reflect.Map, reflect.Chan:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Pointer:
		if v.IsNil() {
			return true
		}

		return IsEmpty(v.Elem().Interface())
	}

	return false
}

// Default returns defaultVal if input is empty, otherwise returns input.
func Default(defaultVal, input any) any {
	if IsEmpty(input) {
		return defaultVal
	}

	return input
}

// Join joins parts using sep, skipping nil and empty-string values.
// Accepts any mix of scalar values and slices/arrays; slices are unpacked in place.
func Join(sep string, parts ...any) string {
	var items []string

	for _, p := range parts {
		if p == nil {
			continue
		}

		v := reflect.ValueOf(p)
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				continue
			}

			v = v.Elem()
		}

		if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
			for i := 0; i < v.Len(); i++ {
				if s := fmt.Sprint(v.Index(i).Interface()); s != "" {
					items = append(items, s)
				}
			}

			continue
		}

		if s := fmt.Sprint(p); s != "" {
			items = append(items, s)
		}
	}

	return strings.Join(items, sep)
}

// ParseDate parses dateStr (YYYY, YYYY-MM, YYYY-MM-DD, RFC3339) and returns time.Time.
// Use formatDate to parse and reformat in one step.
func ParseDate(dateStr string) time.Time {
	return parseDateInternal(dateStr)
}

func parseDateInternal(dateStr string) time.Time {
	if dateStr == "" {
		return time.Time{}
	}

	var layout string

	switch len(dateStr) {
	case 4:
		layout = "2006"
	case 7:
		layout = "2006-01"
	case 10:
		layout = "2006-01-02"
	default:
		layout = "2006-01-02"
	}

	t, err := time.Parse(layout, dateStr)
	if err != nil {
		if t, err = time.Parse(time.RFC3339, dateStr); err != nil {
			return time.Time{}
		}
	}

	return t
}

// FormatDate formats a date (string or time.Time) using the specified layout.
// Pipe-friendly: date is the last argument.
// Supports: formatDate format date, or piped: date | formatDate [format].
// Defaults to "2006-01-02" if format is omitted.
func FormatDate(args ...any) string {
	if len(args) == 0 {
		return ""
	}

	var (
		format string
		date   any
	)

	if len(args) == 1 {
		format = "2006-01-02"
		date = args[0]
	} else {
		format = fmt.Sprint(args[0])
		date = args[len(args)-1]
	}

	if date == nil {
		return ""
	}

	switch d := date.(type) {
	case time.Time:
		if d.IsZero() {
			return ""
		}

		return d.Format(format)
	case string:
		if d == "" {
			return ""
		}

		t := parseDateInternal(d)
		if t.IsZero() {
			return d
		}

		return t.Format(format)
	default:
		return fmt.Sprint(date)
	}
}

// List returns the arguments as a slice of any.
func List(args ...any) []any {
	return args
}

// Replace replaces occurrences of oldStr with newStr in s. (Pipe-friendly: s is last).
func Replace(oldStr, newStr, s string) string {
	return strings.ReplaceAll(s, oldStr, newStr)
}

// TrimPrefix removes prefix from the start of s. (Pipe-friendly: s is last).
func TrimPrefix(prefix, s string) string {
	return strings.TrimPrefix(s, prefix)
}

// TrimSuffix removes suffix from the end of s. (Pipe-friendly: s is last).
func TrimSuffix(suffix, s string) string {
	return strings.TrimSuffix(s, suffix)
}

// HasPrefix tests whether s begins with prefix. (Pipe-friendly: s is last).
func HasPrefix(prefix, s string) bool {
	return strings.HasPrefix(s, prefix)
}

// HasSuffix tests whether s ends with suffix. (Pipe-friendly: s is last).
func HasSuffix(suffix, s string) bool {
	return strings.HasSuffix(s, suffix)
}

// RegexReplace replaces regex matches of pattern with repl in s. (Pipe-friendly: s is last).
func RegexReplace(pattern, repl, s string) string {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return s
	}

	return re.ReplaceAllString(s, repl)
}

// Cat joins parts with NO separator.
func Cat(parts ...any) string {
	var sb strings.Builder

	for _, p := range parts {
		if s := fmt.Sprint(p); s != "" {
			sb.WriteString(s)
		}
	}

	return sb.String()
}

// When evaluates flat condition-value pairs: when cond1 val1 cond2 val2 ... fallback.
func When(args ...any) any {
	for i := 0; i < len(args)-1; i += 2 {
		if !IsEmpty(args[i]) {
			return args[i+1]
		}
	}

	if len(args)%2 != 0 {
		return args[len(args)-1]
	}

	return ""
}

// Pad zero-pads an integer: pad 2 .Season -> "01".
func Pad(digits int, val any) string {
	return fmt.Sprintf("%0*d", digits, val)
}

// Eprange formats an episode range with optional prefix and padding:
// eprange "E" 2 .Episodes -> "E01" or "E01-E05".
func Eprange(prefix string, digits int, episodes []int) string {
	if len(episodes) == 0 {
		return ""
	}

	first, last := slices.Min(episodes), slices.Max(episodes)
	if first == last {
		return fmt.Sprintf("%s%0*d", prefix, digits, first)
	}

	return fmt.Sprintf("%s%0*d-%s%0*d", prefix, digits, first, prefix, digits, last)
}

// ResolveKeyPath navigates dotted key paths (e.g. "Properties.Language") across structs and maps.
func ResolveKeyPath(val reflect.Value, keyPath string) (reflect.Value, bool) {
	parts := strings.Split(keyPath, ".")
	curr := val

	for _, part := range parts {
		if curr.Kind() == reflect.Pointer || curr.Kind() == reflect.Interface {
			curr = curr.Elem()
		}

		nextVal, ok := resolveStep(curr, part)
		if !ok {
			return reflect.Value{}, false
		}

		curr = nextVal
	}

	return curr, true
}

func resolveStep(curr reflect.Value, part string) (reflect.Value, bool) {
	switch curr.Kind() {
	case reflect.Struct:
		f := curr.FieldByName(part)
		if f.IsValid() {
			return f, true
		}

		m := curr.MethodByName(part)
		if m.IsValid() && m.Type().NumIn() == 0 && m.Type().NumOut() > 0 {
			return m.Call(nil)[0], true
		}

		return reflect.Value{}, false
	case reflect.Map:
		res := curr.MapIndex(reflect.ValueOf(part))
		if res.IsValid() {
			return res, true
		}

		return reflect.Value{}, false
	default:
		return reflect.Value{}, false
	}
}

// Where filters a slice of structs or maps by key, operator, and expected value.
// Supported calling forms:
//
//	where key matchValue items
//	where key operator matchValue items
func Where(args ...any) (any, error) {
	p, err := parseWhereArgs(args)
	if err != nil {
		return nil, err
	}

	if p.items == nil {
		return []any{}, nil
	}

	v := reflect.ValueOf(p.items)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return nil, ErrWhereNotSlice
	}

	return filterSlice(v, p.key, p.operator, p.matchValue), nil
}

type whereParams struct {
	key        string
	operator   string
	matchValue any
	items      any
}

func parseWhereArgs(args []any) (whereParams, error) {
	if len(args) < 3 || len(args) > 4 {
		return whereParams{}, ErrWhereInvalidArgs
	}

	keyStr, ok := args[0].(string)
	if !ok {
		return whereParams{}, ErrWhereKeyNotString
	}

	var p whereParams

	p.key = keyStr

	if len(args) == 3 {
		p.matchValue = args[1]
		p.items = args[2]
		p.operator = "=="
	} else {
		p.operator, ok = args[1].(string)
		if !ok {
			return whereParams{}, ErrWhereOpNotString
		}

		p.matchValue = args[2]
		p.items = args[3]
	}

	return p, nil
}

func filterSlice(v reflect.Value, key, operator string, matchValue any) []any {
	var result []any

	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)

		resolved, ok := ResolveKeyPath(elem, key)
		if !ok {
			if operator == "!=" || operator == "<>" {
				result = append(result, elem.Interface())
			}

			continue
		}

		if compareValues(resolved, operator, matchValue) {
			result = append(result, elem.Interface())
		}
	}

	return result
}

func compareValues(val reflect.Value, op string, matchVal any) bool {
	if !val.IsValid() {
		return op == "!=" || op == "<>"
	}

	itemVal := val.Interface()

	switch op {
	case "==", "=":
		return itemVal == matchVal
	case "!=", "<>":
		return itemVal != matchVal
	case ">", ">=", "<", "<=":
		return compareNumeric(itemVal, op, matchVal)
	case "in":
		return Contains(itemVal, matchVal)
	case "not in":
		return !Contains(itemVal, matchVal)
	case "contains":
		return Contains(matchVal, itemVal)
	case "not contains":
		return !Contains(matchVal, itemVal)
	}

	return false
}

func compareNumeric(itemVal any, op string, matchVal any) bool {
	vFloat, ok1 := ToFloat64(itemVal)
	mFloat, ok2 := ToFloat64(matchVal)

	if !ok1 || !ok2 {
		return false
	}

	switch op {
	case ">":
		return vFloat > mFloat
	case ">=":
		return vFloat >= mFloat
	case "<":
		return vFloat < mFloat
	case "<=":
		return vFloat <= mFloat
	}

	return false
}

// ToFloat64 converts any integer or float value to float64.
func ToFloat64(val any) (float64, bool) {
	v := reflect.ValueOf(val)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint()), true
	case reflect.Float32, reflect.Float64:
		return v.Float(), true
	}

	return 0, false
}

// ToInt64 converts any integer or unsigned integer value to int64.
func ToInt64(val any) int64 {
	v := reflect.ValueOf(val)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(v.Uint())
	}

	return 0
}
