package nfo

import (
	"reflect"
	"sort"
	"strconv"
)

func asFloat64(val any) (float64, bool) {
	if val == nil {
		return 0, false
	}

	v := reflect.ValueOf(val)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint()), true
	case reflect.Float32, reflect.Float64:
		return v.Float(), true
	case reflect.String:
		if f, err := strconv.ParseFloat(v.String(), 64); err == nil {
			return f, true
		}
	}

	return 0, false
}

func extractFloats(items any) []float64 {
	var nums []float64
	if items == nil {
		return nums
	}

	v := reflect.ValueOf(items)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		if f, ok := asFloat64(items); ok {
			nums = append(nums, f)
		}

		return nums
	}

	for i := 0; i < v.Len(); i++ {
		if f, ok := asFloat64(v.Index(i).Interface()); ok {
			nums = append(nums, f)
		}
	}

	return nums
}

func sumFunc(items any) float64 {
	nums := extractFloats(items)

	var sum float64
	for _, n := range nums {
		sum += n
	}

	return sum
}

func meanFunc(items any) float64 {
	nums := extractFloats(items)
	if len(nums) == 0 {
		return 0
	}

	var sum float64
	for _, n := range nums {
		sum += n
	}

	return sum / float64(len(nums))
}

func medianFunc(items any) float64 {
	nums := extractFloats(items)
	if len(nums) == 0 {
		return 0
	}

	sort.Float64s(nums)

	n := len(nums)
	if n%2 == 1 {
		return nums[n/2]
	}

	return (nums[n/2-1] + nums[n/2]) / 2
}

func modeFunc(items any) any {
	if items == nil {
		return nil
	}

	v := reflect.ValueOf(items)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return items
	}

	if v.Len() == 0 {
		return nil
	}

	counts := make(map[any]int)

	var (
		maxCount int
		modeVal  any
	)

	for i := 0; i < v.Len(); i++ {
		val := v.Index(i).Interface()

		counts[val]++
		if counts[val] > maxCount {
			maxCount = counts[val]
			modeVal = val
		}
	}

	return modeVal
}
