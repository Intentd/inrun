package check

import "testing"

func TestTemplateSample(t *testing.T) {
	for in, want := range map[string]string{
		"{{ .a }}-svc":             "x-svc",
		"pre-{{ .a }}-{{ .b }}.io": "pre-x-x.io",
		"plain":                    "plain",
		"{{ .a }}":                 "x",
		"open {{ .a ":              "open {{ .a ",
	} {
		if got := TemplateSample(in); got != want {
			t.Errorf("TemplateSample(%q) = %q, want %q", in, got, want)
		}
	}
}
