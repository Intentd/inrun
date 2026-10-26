package structured

import (
	"reflect"
	"testing"
)

func TestSplitBySeparator(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		separator string
		want      []string
	}{
		{
			name:      "one element",
			input:     "a",
			separator: "!",
			want:      []string{"a"},
		},
		{
			name:      "comma separated",
			input:     "a,b,c",
			separator: ",",
			want:      []string{"a", "b", "c"},
		},
		{
			name:      "pipe separated",
			input:     "a|b|c",
			separator: "|",
			want:      []string{"a", "b", "c"},
		},
		{
			name:      "colon separated",
			input:     "a:b:c",
			separator: ":",
			want:      []string{"a", "b", "c"},
		},
		{
			name:      "with spaces",
			input:     "a, b, c",
			separator: ",",
			want:      []string{"a", "b", "c"},
		},
		{
			name:      "empty string",
			input:     "",
			separator: ",",
			want:      []string{},
		},
		{
			name:      "only spaces",
			input:     "  ",
			separator: ",",
			want:      []string{},
		},
		{
			name:      "empty elements",
			input:     "a,,b",
			separator: ",",
			want:      []string{"a", "b"},
		},
		{
			name:      "empty elements with spaces",
			input:     "a, , b",
			separator: ",",
			want:      []string{"a", "b"},
		},
		{
			name:      "single element",
			input:     "foo",
			separator: ",",
			want:      []string{"foo"},
		},
		{
			name:      "empty separator",
			input:     "a,b,c",
			separator: "",
			want:      []string{},
		},
		{
			name:      "multiple character separator",
			input:     "a==b==c",
			separator: "==",
			want:      []string{"a", "b", "c"},
		},
		{
			name:      "trailing separator",
			input:     "a,b,c,",
			separator: ",",
			want:      []string{"a", "b", "c"},
		},
		{
			name:      "leading separator",
			input:     ",a,b,c",
			separator: ",",
			want:      []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitBySeparator(tt.input, tt.separator)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitBySeparator(%q, %q) = %v, want %v", tt.input, tt.separator, got, tt.want)
			}
		})
	}
}

func TestSplitCommaSeparated(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple comma separated",
			input: "a,b,c",
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "with spaces",
			input: "a, b, c",
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "empty string",
			input: "",
			want:  []string{},
		},
		{
			name:  "empty elements",
			input: "a,,b",
			want:  []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitCommaSeparated(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitCommaSeparated(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitPipeSeparated(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple pipe separated",
			input: "a|b|c",
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "with spaces",
			input: "a | b | c",
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "empty elements",
			input: "a||b",
			want:  []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitPipeSeparated(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitPipeSeparated(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitColonSeparated(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple colon separated",
			input: "a:b:c",
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "with spaces",
			input: "a: b: c",
			want:  []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitColonSeparated(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitColonSeparated(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitSemicolonSeparated(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple semicolon separated",
			input: "a;b;c",
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "with spaces",
			input: "a; b; c",
			want:  []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitSemicolonSeparated(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitSemicolonSeparated(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitField(t *testing.T) {
	cases := map[string][]string{
		"":                     nil,
		"spec.replicas":        {"spec", "replicas"},
		".spec.replicas":       {"spec", "replicas"},
		"metadata.labels.tier": {"metadata", "labels", "tier"},
		`metadata.labels.app\.kubernetes\.io/name`: {"metadata", "labels", "app.kubernetes.io/name"},
		`a\b.c`: {`a\b`, "c"}, // a backslash not before a dot is kept
		`a\`:    {`a\`},
	}
	for in, want := range cases {
		if got := SplitField(in); !reflect.DeepEqual(got, want) {
			t.Errorf("SplitField(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEscapePathKey_RoundTrip(t *testing.T) {
	key := "app.kubernetes.io/name"
	got := SplitField("metadata.labels." + EscapePathKey(key))
	if want := []string{"metadata", "labels", key}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNestedPaths_DottedKey(t *testing.T) {
	path := "metadata.labels." + EscapePathKey("app.kubernetes.io/name")
	obj := map[string]interface{}{}

	if err := SetNestedPath(obj, path, "web"); err != nil {
		t.Fatal(err)
	}
	labels := obj["metadata"].(map[string]interface{})["labels"].(map[string]interface{})
	if labels["app.kubernetes.io/name"] != "web" {
		t.Fatalf("labels = %v, want the key kept whole", labels)
	}
	if v, ok := GetNestedPath(obj, path); !ok || v != "web" {
		t.Errorf("GetNestedPath = %v, %v", v, ok)
	}
	DeleteNestedPath(obj, path)
	if len(labels) != 0 {
		t.Errorf("DeleteNestedPath left %v", labels)
	}

	SetFieldPath(obj, path, "api")
	if labels["app.kubernetes.io/name"] != "api" {
		t.Errorf("SetFieldPath: labels = %v", labels)
	}
}

func BenchmarkSplitBySeparator(b *testing.B) {
	input := "a,b,c,d,e,f,g,h,i,j"
	for b.Loop() {
		SplitBySeparator(input, ",")
	}
}

func BenchmarkSplitCommaSeparated(b *testing.B) {
	input := "a,b,c,d,e,f,g,h,i,j"
	for b.Loop() {
		SplitCommaSeparated(input)
	}
}
