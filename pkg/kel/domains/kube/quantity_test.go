// pkg/note/kube_quantity_test.go
package note

import (
	"testing"
)

func TestNoteParseQuantity(t *testing.T) {
	tests := []struct {
		name    string
		q       string
		want    float64
		wantErr bool
	}{
		{"CPU milli", "100m", 0.1, false},
		{"CPU whole", "2", 2.0, false},
		{"CPU zero", "0", 0.0, false},
		{"Memory binary", "1Gi", 1073741824.0, false},
		{"Memory mixed", "512Mi", 536870912.0, false},
		{"Memory decimal", "1.5G", 1.5e9, false},
		{"Invalid", "invalid", 0, true},
		{"Empty", "", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := noteParseQuantity(tt.q)
			if (err != nil) != tt.wantErr {
				t.Errorf("noteParseQuantity() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("noteParseQuantity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteFormatQuantity(t *testing.T) {
	tests := []struct {
		name string
		f    float64
		want string
	}{
		{"CPU milli exact", 0.1, "100m"},
		{"CPU half", 0.5, "500m"},
		{"CPU whole", 1.0, "1"},
		{"CPU large", 2.5, "2500m"},
		{"Memory bytes", 1073741824, "1Gi"},
		{"Memory half gi", 536870912, "512Mi"},
		{"Zero", 0, "0"},
		{"Negative", -0.1, "-100m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := noteFormatQuantity(tt.f)
			if err != nil {
				t.Errorf("noteFormatQuantity() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("noteFormatQuantity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteSumQuantity(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want string
		err  bool
	}{
		{"CPU milli sum", "100m", "200m", "300m", false},
		{"CPU whole + milli", "500m", "500m", "1", false},
		{"Memory Gi + Mi", "1Gi", "512Mi", "1536Mi", false},
		{"Memory Ti + Gi", "1Ti", "1Gi", "1025Gi", false}, // 1Ti = 1024Gi
		{"Negative sum", "100m", "-50m", "50m", false},
		{"Invalid a", "invalid", "100m", "", true},
		{"Invalid b", "100m", "invalid", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := noteSumQuantity(tt.a, tt.b)
			if (err != nil) != tt.err {
				t.Errorf("noteSumQuantity() error = %v, wantErr %v", err, tt.err)
				return
			}
			if !tt.err && got != tt.want {
				t.Errorf("noteSumQuantity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteSubtractQuantity(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want string
		err  bool
	}{
		{"CPU milli subtract", "300m", "100m", "200m", false},
		{"CPU whole minus milli", "1", "500m", "500m", false},
		{"Memory Gi minus Mi", "1536Mi", "512Mi", "1Gi", false},
		{"Negative result", "100m", "200m", "-100m", false},
		{"Memory Ti minus Gi", "1Ti", "1Gi", "1023Gi", false},
		{"Invalid a", "invalid", "100m", "", true},
		{"Invalid b", "100m", "invalid", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := noteSubtractQuantity(tt.a, tt.b)
			if (err != nil) != tt.err {
				t.Errorf("noteSubtractQuantity() error = %v, wantErr %v", err, tt.err)
				return
			}
			if !tt.err && got != tt.want {
				t.Errorf("noteSubtractQuantity() = %v, want %v", got, tt.want)
			}
		})
	}
}
