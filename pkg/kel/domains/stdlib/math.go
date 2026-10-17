package note

import (
	"fmt"
	"strconv"
	"text/template"
)

func mathNotes() template.FuncMap {
	return template.FuncMap{
		"add":   noteAdd,
		"sub":   noteSub,
		"mul":   noteMul,
		"div":   noteDiv,
		"mod":   noteMod,
		"min":   noteMin,
		"max":   noteMax,
		"clamp": noteClamp,
		"abs":   noteAbs,
	}
}

// noteAdd adds two numbers. Accepts int64, float64, or numeric strings.
// Returns int64 when the result is whole, float64 otherwise.
//
//	{{ add .spec.basePort 1000 }}   →  9080 (if basePort is 8080)
func noteAdd(a, b interface{}) (interface{}, error) {
	af, ae := anyToFloat(a)
	bf, be := anyToFloat(b)
	if ae != nil || be != nil {
		return nil, fmt.Errorf("add: non-numeric arguments %v, %v", a, b)
	}
	return nativeNumber(af + bf), nil
}

func noteSub(a, b interface{}) (interface{}, error) {
	af, ae := anyToFloat(a)
	bf, be := anyToFloat(b)
	if ae != nil || be != nil {
		return nil, fmt.Errorf("sub: non-numeric arguments %v, %v", a, b)
	}
	return nativeNumber(af - bf), nil
}

func noteMul(a, b interface{}) (interface{}, error) {
	af, ae := anyToFloat(a)
	bf, be := anyToFloat(b)
	if ae != nil || be != nil {
		return nil, fmt.Errorf("mul: non-numeric arguments %v, %v", a, b)
	}
	return nativeNumber(af * bf), nil
}

func noteDiv(a, b interface{}) (interface{}, error) {
	af, ae := anyToFloat(a)
	bf, be := anyToFloat(b)
	if ae != nil || be != nil {
		return nil, fmt.Errorf("div: non-numeric arguments %v, %v", a, b)
	}
	if bf == 0 {
		return nil, fmt.Errorf("div: division by zero")
	}
	return nativeNumber(af / bf), nil
}

func noteMod(a, b interface{}) (interface{}, error) {
	af, ae := anyToFloat(a)
	bf, be := anyToFloat(b)
	if ae != nil || be != nil {
		return nil, fmt.Errorf("mod: non-numeric arguments %v, %v", a, b)
	}
	if bf == 0 {
		return nil, fmt.Errorf("mod: division by zero")
	}
	return int64(af) % int64(bf), nil
}

// noteMin returns the smaller of two numbers.
//
//	{{ min .spec.replicas 10 }}   →  caps replicas at 10
func noteMin(a, b interface{}) (interface{}, error) {
	af, ae := anyToFloat(a)
	bf, be := anyToFloat(b)
	if ae != nil || be != nil {
		return nil, fmt.Errorf("min: non-numeric arguments %v, %v", a, b)
	}
	if af < bf {
		return nativeNumber(af), nil
	}
	return nativeNumber(bf), nil
}

// noteMax returns the larger of two numbers.
//
//	{{ max .spec.replicas 2 }}    →  floors replicas at 2
func noteMax(a, b interface{}) (interface{}, error) {
	af, ae := anyToFloat(a)
	bf, be := anyToFloat(b)
	if ae != nil || be != nil {
		return nil, fmt.Errorf("max: non-numeric arguments %v, %v", a, b)
	}
	if af > bf {
		return nativeNumber(af), nil
	}
	return nativeNumber(bf), nil
}

// noteClamp constrains a value to [lo, hi].
//
//	{{ clamp .spec.replicas 1 20 }}   →  value is always between 1 and 20
func noteClamp(val, lo, hi interface{}) (interface{}, error) {
	vf, ve := anyToFloat(val)
	lf, le := anyToFloat(lo)
	hf, he := anyToFloat(hi)
	if ve != nil || le != nil || he != nil {
		return nil, fmt.Errorf("clamp: non-numeric arguments")
	}
	if vf < lf {
		return nativeNumber(lf), nil
	}
	if vf > hf {
		return nativeNumber(hf), nil
	}
	return nativeNumber(vf), nil
}

func noteAbs(a interface{}) (interface{}, error) {
	af, ae := anyToFloat(a)
	if ae != nil {
		return nil, fmt.Errorf("abs: non-numeric argument %v", a)
	}
	if af < 0 {
		af = -af
	}
	return nativeNumber(af), nil
}

// ── internal ──────────────────────────────────────────────────────────────────

// anyToFloat converts any numeric-ish value to float64.
func anyToFloat(v interface{}) (float64, error) {
	switch val := v.(type) {
	case bool:
		if val {
			return 1, nil
		}
		return 0, nil
	case int:
		return float64(val), nil
	case int32:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case float32:
		return float64(val), nil
	case float64:
		return val, nil
	case string:
		return strconv.ParseFloat(val, 64)
	}
	return 0, fmt.Errorf("cannot convert %T to number", v)
}

// nativeNumber returns int64 when f is a whole number, float64 otherwise.
// This keeps integer arithmetic returning integers — important for fields
// like spec.replicas that the API server expects as integers.
func nativeNumber(f float64) interface{} {
	if f == float64(int64(f)) {
		return int64(f)
	}
	return f
}
