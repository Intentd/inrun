package structured

import "reflect"

// NonZero returns a T with every exported field below it set to a non-zero
// value: strings "1", numbers 1, bools true, and one element in every slice
// and map. Fields named in skip keep their zero value at any depth. Tests use
// it to prove that every field of a type is carried somewhere, since a field
// that arrives zero was dropped on the way.
func NonZero[T any](skip ...string) T {
	var v T
	names := make(map[string]bool, len(skip))
	for _, s := range skip {
		names[s] = true
	}
	fillNonZero(reflect.ValueOf(&v).Elem(), names, 0)
	return v
}

// fillNonZero stops at a fixed depth, so a type that refers to itself ends.
func fillNonZero(v reflect.Value, skip map[string]bool, depth int) {
	if depth > 6 || !v.CanSet() {
		return
	}
	switch v.Kind() {
	case reflect.String:
		v.SetString("1")
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(1)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1)
	case reflect.Pointer:
		p := reflect.New(v.Type().Elem())
		fillNonZero(p.Elem(), skip, depth+1)
		v.Set(p)
	case reflect.Slice:
		s := reflect.MakeSlice(v.Type(), 1, 1)
		fillNonZero(s.Index(0), skip, depth+1)
		v.Set(s)
	case reflect.Map:
		m := reflect.MakeMap(v.Type())
		key := reflect.New(v.Type().Key()).Elem()
		val := reflect.New(v.Type().Elem()).Elem()
		fillNonZero(key, skip, depth+1)
		fillNonZero(val, skip, depth+1)
		m.SetMapIndex(key, val)
		v.Set(m)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if skip[v.Type().Field(i).Name] {
				continue
			}
			fillNonZero(v.Field(i), skip, depth+1)
		}
	}
}
