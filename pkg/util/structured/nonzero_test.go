package structured

import (
	"reflect"
	"testing"
)

func TestNonZeroSetsEveryFieldButSkipped(t *testing.T) {
	type inner struct {
		Name   string
		Preset string
	}
	type outer struct {
		S      string
		N      int32
		B      bool
		P      *inner
		L      []inner
		M      map[string]string
		Preset string
	}
	got := NonZero[outer]("Preset")

	v := reflect.ValueOf(got)
	for i := 0; i < v.NumField(); i++ {
		name := v.Type().Field(i).Name
		if zero := v.Field(i).IsZero(); zero != (name == "Preset") {
			t.Errorf("field %s zero = %v", name, zero)
		}
	}
	if got.P.Name == "" || got.P.Preset != "" || got.L[0].Preset != "" {
		t.Errorf("nested fields = %+v, %+v; want Name set and Preset skipped at every depth", *got.P, got.L[0])
	}
}
