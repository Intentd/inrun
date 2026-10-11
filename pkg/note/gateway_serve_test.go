package note

import (
	"testing"

	"github.com/inrundev/inrun/pkg/labels"
)

// cr builds a minimal CR map with the given annotations set.
func crWithAnnotations(ann map[string]string) map[string]interface{} {
	a := make(map[string]interface{}, len(ann))
	for k, v := range ann {
		a[k] = v
	}
	return map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": a,
		},
	}
}

func TestNoteGetServeTarget(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"set", crWithAnnotations(map[string]string{labels.AnnotationServeTarget: "smartapp"}), "smartapp"},
		{"empty annotation", crWithAnnotations(map[string]string{labels.AnnotationServeTarget: ""}), ""},
		{"annotation absent", crWithAnnotations(map[string]string{}), ""},
		{"no annotations block", map[string]interface{}{"metadata": map[string]interface{}{}}, ""},
		{"no metadata", map[string]interface{}{}, ""},
		{"not a map", "string", ""},
		{"nil", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteGetServeTarget(tt.obj); got != tt.want {
				t.Errorf("noteGetServeTarget() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNoteGetServeAlias(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want string
	}{
		{"alias set", crWithAnnotations(map[string]string{labels.AnnotationServeAlias: "public"}), "public"},
		{"alias absent", crWithAnnotations(map[string]string{}), ""},
		{"no metadata", map[string]interface{}{}, ""},
		{"nil", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteGetServeAlias(tt.obj); got != tt.want {
				t.Errorf("noteGetServeAlias() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNoteHasServeTarget(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want bool
	}{
		{"present", crWithAnnotations(map[string]string{labels.AnnotationServeTarget: "smartapp"}), true},
		{"empty string", crWithAnnotations(map[string]string{labels.AnnotationServeTarget: ""}), false},
		{"absent", crWithAnnotations(map[string]string{}), false},
		{"no metadata", map[string]interface{}{}, false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteHasServeTarget(tt.obj); got != tt.want {
				t.Errorf("noteHasServeTarget() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteHasServeAlias(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want bool
	}{
		{"alias set", crWithAnnotations(map[string]string{labels.AnnotationServeAlias: "internal"}), true},
		{"alias empty", crWithAnnotations(map[string]string{labels.AnnotationServeAlias: ""}), false},
		{"absent", crWithAnnotations(map[string]string{}), false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteHasServeAlias(tt.obj); got != tt.want {
				t.Errorf("noteHasServeAlias() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoteIsDirectApply(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want bool
	}{
		// no annotations — raw kubectl / CI direct apply
		{"no annotations", crWithAnnotations(map[string]string{}), true},
		{"nil", nil, true},
		{"no metadata", map[string]interface{}{}, true},

		// serve-target present — any gateway apply
		{"target set", crWithAnnotations(map[string]string{labels.AnnotationServeTarget: "app"}), false},
		// serve-alias alone (defensive — shouldn't happen in practice but not a direct apply)
		{"alias only", crWithAnnotations(map[string]string{labels.AnnotationServeAlias: "preview"}), false},
		// full gateway apply — all three present
		{"all three", crWithAnnotations(map[string]string{
			labels.AnnotationServeTarget: "app",
			labels.AnnotationServeAlias:  "preview",
		}), false},
		// primary target apply — alias absent, still not a direct apply
		{"target without alias", crWithAnnotations(map[string]string{
			labels.AnnotationServeTarget: "app",
		}), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noteIsDirectApply(tt.obj); got != tt.want {
				t.Errorf("noteIsDirectApply() = %v, want %v", got, tt.want)
			}
		})
	}
}
