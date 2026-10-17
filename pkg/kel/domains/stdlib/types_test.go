// pkg/note/types_test.go
package note

import (
	"testing"
)

func TestTypeOf(t *testing.T) {
	tests := []struct {
		name string
		v    interface{}
		want string
	}{
		{"string", "hello", "string"},
		{"int", 42, "number"},
		{"int64", int64(42), "number"},
		{"float64", 3.14, "number"},
		{"bool true", true, "bool"},
		{"bool false", false, "bool"},
		{"map", map[string]interface{}{"a": 1}, "map"},
		{"slice", []interface{}{1, 2}, "slice"},
		{"nil", nil, "null"},
		{"struct", struct{}{}, "unknown"}, // fallback
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TypeOf(tt.v); got != tt.want {
				t.Errorf("TypeOf(%v) = %q, want %q", tt.v, got, tt.want)
			}
		})
	}
}

func TestInrunLen(t *testing.T) {
	tests := []struct {
		name string
		v    interface{}
		want int
	}{
		{"string", "hello", 5},
		{"empty string", "", 0},
		{"slice", []interface{}{1, 2, 3}, 3},
		{"empty slice", []interface{}{}, 0},
		{"map", map[string]interface{}{"a": 1, "b": 2}, 2},
		{"empty map", map[string]interface{}{}, 0},
		{"nil", nil, 0},
		{"other type", 42, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InrunLen(tt.v); got != tt.want {
				t.Errorf("InrunLen(%v) = %d, want %d", tt.v, got, tt.want)
			}
		})
	}
}

func TestIsEmpty(t *testing.T) {
	tests := []struct {
		name string
		v    interface{}
		want bool
	}{
		{"nil", nil, true},
		{"empty string", "", true},
		{"non-empty string", "x", false},
		{"empty slice", []interface{}{}, true},
		{"non-empty slice", []interface{}{1}, false},
		{"empty map", map[string]interface{}{}, true},
		{"non-empty map", map[string]interface{}{"k": "v"}, false},
		{"int zero", 0, false}, // not empty according to definition
		{"int non-zero", 42, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEmpty(tt.v); got != tt.want {
				t.Errorf("isEmpty(%v) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

func TestIsScalar(t *testing.T) {
	tests := []struct {
		name string
		v    interface{}
		want bool
	}{
		{"string", "hello", true},
		{"number", 42, true},
		{"bool", true, true},
		{"map", map[string]interface{}{}, false},
		{"slice", []interface{}{}, false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isScalar(tt.v); got != tt.want {
				t.Errorf("isScalar(%v) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

func TestIsComposite(t *testing.T) {
	tests := []struct {
		name string
		v    interface{}
		want bool
	}{
		{"map", map[string]interface{}{}, true},
		{"slice", []interface{}{}, true},
		{"string", "hello", false},
		{"number", 42, false},
		{"bool", true, false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isComposite(tt.v); got != tt.want {
				t.Errorf("isComposite(%v) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

func TestTypeMapListStringNumberBoolNull(t *testing.T) {
	tests := []struct {
		name                                                          string
		v                                                             interface{}
		wantMap, wantList, wantString, wantNumber, wantBool, wantNull bool
	}{
		{"map", map[string]interface{}{}, true, false, false, false, false, false},
		{"slice", []interface{}{}, false, true, false, false, false, false},
		{"string", "hi", false, false, true, false, false, false},
		{"number", 42, false, false, false, true, false, false},
		{"bool", true, false, false, false, false, true, false},
		{"nil", nil, false, false, false, false, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := typeMap(tt.v); got != tt.wantMap {
				t.Errorf("typeMap(%v) = %v, want %v", tt.v, got, tt.wantMap)
			}
			if got := typeList(tt.v); got != tt.wantList {
				t.Errorf("typeList(%v) = %v, want %v", tt.v, got, tt.wantList)
			}
			if got := typeString(tt.v); got != tt.wantString {
				t.Errorf("typeString(%v) = %v, want %v", tt.v, got, tt.wantString)
			}
			if got := typeNumber(tt.v); got != tt.wantNumber {
				t.Errorf("typeNumber(%v) = %v, want %v", tt.v, got, tt.wantNumber)
			}
			if got := typeBool(tt.v); got != tt.wantBool {
				t.Errorf("typeBool(%v) = %v, want %v", tt.v, got, tt.wantBool)
			}
			if got := typeNull(tt.v); got != tt.wantNull {
				t.Errorf("typeNull(%v) = %v, want %v", tt.v, got, tt.wantNull)
			}
		})
	}
}

func TestNoteToInt(t *testing.T) {
	tests := []struct {
		name    string
		v       interface{}
		want    int64
		wantErr bool
	}{
		{"int", 42, 42, false},
		{"int64", int64(99), 99, false},
		{"float64", 3.7, 3, false},
		{"bool true", true, 1, false},
		{"bool false", false, 0, false},
		{"string integer", "123", 123, false},
		{"string float", "45.6", 45, false},
		{"invalid string", "abc", 0, true},
		{"nil", nil, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := noteToInt(tt.v)
			if (err != nil) != tt.wantErr {
				t.Errorf("noteToInt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("noteToInt() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNoteToFloat(t *testing.T) {
	tests := []struct {
		name    string
		v       interface{}
		want    float64
		wantErr bool
	}{
		{"int", 42, 42.0, false},
		{"float", 3.14, 3.14, false},
		{"string", "2.5", 2.5, false},
		{"bool true", true, 1.0, false},
		{"bool false", false, 0.0, false},
		{"invalid", "abc", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := noteToFloat(tt.v)
			if (err != nil) != tt.wantErr {
				t.Errorf("noteToFloat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("noteToFloat() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestNoteToBool(t *testing.T) {
	tests := []struct {
		name    string
		v       interface{}
		want    bool
		wantErr bool
	}{
		{"bool true", true, true, false},
		{"bool false", false, false, false},
		{"int 1", 1, true, false},
		{"int 0", 0, false, false},
		{"float 1.0", 1.0, true, false},
		{"float 0.0", 0.0, false, false},
		{"string true", "true", true, false},
		{"string yes", "yes", true, false},
		{"string on", "on", true, false},
		{"string 1", "1", true, false},
		{"string false", "false", false, false},
		{"string no", "no", false, false},
		{"string off", "off", false, false},
		{"string 0", "0", false, false},
		{"string empty", "", false, false},
		{"invalid string", "maybe", false, true},
		{"nil", nil, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := noteToBool(tt.v)
			if (err != nil) != tt.wantErr {
				t.Errorf("noteToBool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("noteToBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteToString(t *testing.T) {
	tests := []struct {
		name string
		v    interface{}
		want string
	}{
		{"string", "hello", "hello"},
		{"int", 42, "42"},
		{"float", 3.14, "3.14"},
		{"bool", true, "true"},
		{"map", map[string]int{"a": 1}, "map[a:1]"},
		{"nil", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteToString(tt.v); got != tt.want {
				t.Errorf("noteToString(%v) = %q, want %q", tt.v, got, tt.want)
			}
		})
	}
}
