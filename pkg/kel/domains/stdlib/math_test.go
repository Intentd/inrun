// pkg/note/math_test.go
package note

import (
	"testing"
)

func TestNoteAdd(t *testing.T) {
	tests := []struct {
		name    string
		a, b    interface{}
		want    interface{}
		wantErr bool
	}{
		{"int + int", 1, 2, int64(3), false},
		{"int64 + int64", int64(5), int64(7), int64(12), false},
		{"float + float", 1.5, 2.5, int64(4), false},
		{"int + float", 3, 2.5, 5.5, false},
		{"string ints", "10", "20", int64(30), false},
		{"string floats", "1.2", "3.4", 4.6, false},
		{"non-numeric a", "abc", 5, nil, true},
		{"non-numeric b", 5, "def", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := noteAdd(tt.a, tt.b)
			if (err != nil) != tt.wantErr {
				t.Errorf("noteAdd() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("noteAdd() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteSub(t *testing.T) {
	tests := []struct {
		name    string
		a, b    interface{}
		want    interface{}
		wantErr bool
	}{
		{"int - int", 10, 3, int64(7), false},
		{"float - float", 5.5, 2.2, 3.3, false},
		{"neg result", 2, 5, int64(-3), false},
		{"string", "100", "30", int64(70), false},
		{"invalid a", "x", 10, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := noteSub(tt.a, tt.b)
			if (err != nil) != tt.wantErr {
				t.Errorf("noteSub() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("noteSub() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteMul(t *testing.T) {
	tests := []struct {
		name    string
		a, b    interface{}
		want    interface{}
		wantErr bool
	}{
		{"int * int", 4, 5, int64(20), false},
		{"float * float", 2.5, 4.0, int64(10), false},
		{"mixed", 3, 1.5, 4.5, false},
		{"string", "2", "3", int64(6), false},
		{"invalid", "a", 2, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := noteMul(tt.a, tt.b)
			if (err != nil) != tt.wantErr {
				t.Errorf("noteMul() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("noteMul() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteDiv(t *testing.T) {
	tests := []struct {
		name    string
		a, b    interface{}
		want    interface{}
		wantErr bool
	}{
		{"int division", 10, 3, 3.3333333333333335, false}, // floats
		{"exact integer", 9, 3, int64(3), false},
		{"float", 7.5, 2.5, int64(3), false},
		{"division by zero", 1, 0, nil, true},
		{"string", "10", "2", int64(5), false},
		{"invalid", 5, "x", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := noteDiv(tt.a, tt.b)
			if (err != nil) != tt.wantErr {
				t.Errorf("noteDiv() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("noteDiv() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteMod(t *testing.T) {
	tests := []struct {
		name    string
		a, b    interface{}
		want    interface{}
		wantErr bool
	}{
		{"int mod", 10, 3, int64(1), false},
		{"negative", -10, 3, int64(-1), false},
		{"division by zero", 5, 0, nil, true},
		{"float converted", 7.5, 2.5, int64(1), false}, // floats truncated
		{"string", "15", "4", int64(3), false},
		{"invalid", "x", 2, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := noteMod(tt.a, tt.b)
			if (err != nil) != tt.wantErr {
				t.Errorf("noteMod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("noteMod() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteMin(t *testing.T) {
	tests := []struct {
		name    string
		a, b    interface{}
		want    interface{}
		wantErr bool
	}{
		{"int smaller first", 5, 10, int64(5), false},
		{"int smaller second", 20, 15, int64(15), false},
		{"float smaller first", 3.5, 4.2, 3.5, false},
		{"int vs float", 4, 3.9, 3.9, false},
		{"equal", 7, 7, int64(7), false},
		{"string", "100", "50", int64(50), false},
		{"invalid", "a", 10, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := noteMin(tt.a, tt.b)
			if (err != nil) != tt.wantErr {
				t.Errorf("noteMin() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("noteMin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteMax(t *testing.T) {
	tests := []struct {
		name    string
		a, b    interface{}
		want    interface{}
		wantErr bool
	}{
		{"int larger first", 15, 10, int64(15), false},
		{"float larger second", 3.2, 4.5, 4.5, false},
		{"equal", 7, 7, int64(7), false},
		{"string", "200", "300", int64(300), false},
		{"invalid", "x", 5, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := noteMax(tt.a, tt.b)
			if (err != nil) != tt.wantErr {
				t.Errorf("noteMax() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("noteMax() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteClamp(t *testing.T) {
	tests := []struct {
		name        string
		val, lo, hi interface{}
		want        interface{}
		wantErr     bool
	}{
		{"within", 5, 1, 10, int64(5), false},
		{"below", 0, 1, 10, int64(1), false},
		{"above", 20, 1, 10, int64(10), false},
		{"float", 5.5, 1.0, 10.0, 5.5, false},
		{"string", "8", "1", "10", int64(8), false},
		{"invalid val", "x", 1, 10, nil, true},
		{"invalid lo", 5, "x", 10, nil, true},
		{"invalid hi", 5, 1, "y", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := noteClamp(tt.val, tt.lo, tt.hi)
			if (err != nil) != tt.wantErr {
				t.Errorf("noteClamp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("noteClamp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteAbs(t *testing.T) {
	tests := []struct {
		name    string
		a       interface{}
		want    interface{}
		wantErr bool
	}{
		{"positive", 5, int64(5), false},
		{"negative", -5, int64(5), false},
		{"float positive", 3.2, 3.2, false},
		{"float negative", -4.8, 4.8, false},
		{"zero", 0, int64(0), false},
		{"string", "-123", int64(123), false},
		{"invalid", "abc", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := noteAbs(tt.a)
			if (err != nil) != tt.wantErr {
				t.Errorf("noteAbs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("noteAbs() = %v, want %v", got, tt.want)
			}
		})
	}
}
