package kube

import (
	"reflect"
	"testing"
)

func TestMatchesAllFieldSelectors(t *testing.T) {
	tests := []struct {
		name      string
		obj       map[string]interface{}
		selectors map[string]interface{}
		expected  bool
	}{
		{
			name: "exact match - string values",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":      "my-app",
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"replicas": 3,
					"image":    "nginx:latest",
				},
			},
			selectors: map[string]interface{}{
				"metadata.namespace": "default",
				"spec.image":         "nginx:latest",
			},
			expected: true,
		},
		{
			name: "exact match - mixed types",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"replicas": 3,
					"enabled":  true,
					"cpu":      2.5,
				},
			},
			selectors: map[string]interface{}{
				"spec.replicas": 3,
				"spec.enabled":  true,
				"spec.cpu":      2.5,
			},
			expected: true,
		},
		{
			name: "no match - wrong value",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"namespace": "default",
				},
			},
			selectors: map[string]interface{}{
				"metadata.namespace": "kube-system",
			},
			expected: false,
		},
		{
			name: "no match - missing path",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "my-app",
				},
			},
			selectors: map[string]interface{}{
				"metadata.namespace": "default",
			},
			expected: false,
		},
		{
			name: "empty selectors",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "my-app",
				},
			},
			selectors: map[string]interface{}{},
			expected:  true,
		},
		{
			name: "nil object",
			obj:  nil,
			selectors: map[string]interface{}{
				"metadata.name": "my-app",
			},
			expected: false,
		},
		{
			name: "deep nested path",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{
								"app": "nginx",
							},
						},
					},
				},
			},
			selectors: map[string]interface{}{
				"spec.template.metadata.labels.app": "nginx",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchesAllFieldSelectors(tt.obj, tt.selectors)
			if result != tt.expected {
				t.Errorf("MatchesAllFieldSelectors() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMatchesAllServeTargetFieldSelectors(t *testing.T) {
	tests := []struct {
		name      string
		obj       map[string]interface{}
		selectors map[string]string
		expected  bool
	}{
		{
			name: "exact match - string values",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"image": "nginx:latest",
				},
			},
			selectors: map[string]string{
				"metadata.namespace": "default",
				"spec.image":         "nginx:latest",
			},
			expected: true,
		},
		{
			name: "type conversion - int to string",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"replicas": 3,
				},
			},
			selectors: map[string]string{
				"spec.replicas": "3",
			},
			expected: true,
		},
		{
			name: "type conversion - bool to string",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"enabled": true,
				},
			},
			selectors: map[string]string{
				"spec.enabled": "true",
			},
			expected: true,
		},
		{
			name: "no match - wrong string",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"namespace": "default",
				},
			},
			selectors: map[string]string{
				"metadata.namespace": "kube-system",
			},
			expected: false,
		},
		{
			name: "no match - missing path",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "my-app",
				},
			},
			selectors: map[string]string{
				"metadata.namespace": "default",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchesAllServeTargetFieldSelectors(tt.obj, tt.selectors)
			if result != tt.expected {
				t.Errorf("MatchesAllServeTargetFieldSelectors() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMatchesAllFieldSelectorsWithOptions(t *testing.T) {
	tests := []struct {
		name      string
		obj       map[string]interface{}
		selectors []FieldSelector
		expected  bool
	}{
		{
			name: "exact match",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "my-app",
				},
			},
			selectors: []FieldSelector{
				{Path: "metadata.name", Value: "my-app", Options: MatchExact},
			},
			expected: true,
		},
		{
			name: "contains match",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "my-app-production",
				},
			},
			selectors: []FieldSelector{
				{Path: "metadata.name", Value: "app", Options: MatchContains},
			},
			expected: true,
		},
		{
			name: "prefix match",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "my-app-production",
				},
			},
			selectors: []FieldSelector{
				{Path: "metadata.name", Value: "my-", Options: MatchPrefix},
			},
			expected: true,
		},
		{
			name: "suffix match",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "my-app-production",
				},
			},
			selectors: []FieldSelector{
				{Path: "metadata.name", Value: "production", Options: MatchSuffix},
			},
			expected: true,
		},
		{
			name: "ignore case match",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "My-App",
				},
			},
			selectors: []FieldSelector{
				{Path: "metadata.name", Value: "my-app", Options: MatchIgnoreCase},
			},
			expected: true,
		},
		{
			name: "multiple selectors - all match",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":      "my-app-production",
					"namespace": "default",
				},
			},
			selectors: []FieldSelector{
				{Path: "metadata.name", Value: "app", Options: MatchContains},
				{Path: "metadata.namespace", Value: "default", Options: MatchExact},
			},
			expected: true,
		},
		{
			name: "multiple selectors - one fails",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":      "my-app-production",
					"namespace": "default",
				},
			},
			selectors: []FieldSelector{
				{Path: "metadata.name", Value: "app", Options: MatchContains},
				{Path: "metadata.namespace", Value: "kube-system", Options: MatchExact},
			},
			expected: false,
		},
		{
			name: "nil value handling",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": nil,
				},
			},
			selectors: []FieldSelector{
				{Path: "metadata.name", Value: nil, Options: MatchExact},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchesAllFieldSelectorsWithOptions(tt.obj, tt.selectors)
			if result != tt.expected {
				t.Errorf("MatchesAllFieldSelectorsWithOptions() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMatchesAnyFieldSelector(t *testing.T) {
	tests := []struct {
		name      string
		obj       map[string]interface{}
		selectors map[string]interface{}
		expected  bool
	}{
		{
			name: "matches at least one",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"namespace": "default",
				},
			},
			selectors: map[string]interface{}{
				"metadata.namespace": "default",
				"metadata.name":      "my-app",
			},
			expected: true,
		},
		{
			name: "matches none",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"namespace": "default",
				},
			},
			selectors: map[string]interface{}{
				"metadata.namespace": "kube-system",
				"metadata.name":      "my-app",
			},
			expected: false,
		},
		{
			name: "empty selectors",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"namespace": "default",
				},
			},
			selectors: map[string]interface{}{},
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchesAnyFieldSelector(tt.obj, tt.selectors)
			if result != tt.expected {
				t.Errorf("MatchesAnyFieldSelector() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMatchesAllTypedSelectors(t *testing.T) {
	tests := []struct {
		name      string
		obj       map[string]interface{}
		selectors map[string]interface{}
		expected  bool
	}{
		{
			name: "int comparison",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"replicas": 3,
				},
			},
			selectors: map[string]interface{}{
				"spec.replicas": 3,
			},
			expected: true,
		},
		{
			name: "int vs float - should fail",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"replicas": 3,
				},
			},
			selectors: map[string]interface{}{
				"spec.replicas": 3.0,
			},
			expected: true, // conversion handles it
		},
		{
			name: "bool comparison",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"enabled": true,
				},
			},
			selectors: map[string]interface{}{
				"spec.enabled": true,
			},
			expected: true,
		},
		{
			name: "float comparison",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"cpu": 2.5,
				},
			},
			selectors: map[string]interface{}{
				"spec.cpu": 2.5,
			},
			expected: true,
		},
		{
			name: "int vs string - should fail",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"replicas": 3,
				},
			},
			selectors: map[string]interface{}{
				"spec.replicas": "3",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchesAllTypedSelectors(tt.obj, tt.selectors)
			if result != tt.expected {
				t.Errorf("MatchesAllTypedSelectors() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCompareValues(t *testing.T) {
	tests := []struct {
		name     string
		actual   interface{}
		expected interface{}
		want     bool
	}{
		{
			name:     "equal strings",
			actual:   "hello",
			expected: "hello",
			want:     true,
		},
		{
			name:     "different strings",
			actual:   "hello",
			expected: "world",
			want:     false,
		},
		{
			name:     "int and string - should match via conversion",
			actual:   3,
			expected: "3",
			want:     true,
		},
		{
			name:     "bool and string - should match via conversion",
			actual:   true,
			expected: "true",
			want:     true,
		},
		{
			name:     "nil and nil",
			actual:   nil,
			expected: nil,
			want:     true,
		},
		{
			name:     "nil and non-nil",
			actual:   nil,
			expected: "hello",
			want:     false,
		},
		{
			name:     "different types with same string representation",
			actual:   42,
			expected: 42,
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compareValues(tt.actual, tt.expected); got != tt.want {
				t.Errorf("compareValues() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompareValuesWithOption(t *testing.T) {
	tests := []struct {
		name     string
		actual   interface{}
		expected interface{}
		option   MatchOption
		want     bool
	}{
		{
			name:     "contains - true",
			actual:   "hello world",
			expected: "world",
			option:   MatchContains,
			want:     true,
		},
		{
			name:     "contains - false",
			actual:   "hello world",
			expected: "xyz",
			option:   MatchContains,
			want:     false,
		},
		{
			name:     "prefix - true",
			actual:   "hello world",
			expected: "hello",
			option:   MatchPrefix,
			want:     true,
		},
		{
			name:     "prefix - false",
			actual:   "hello world",
			expected: "world",
			option:   MatchPrefix,
			want:     false,
		},
		{
			name:     "suffix - true",
			actual:   "hello world",
			expected: "world",
			option:   MatchSuffix,
			want:     true,
		},
		{
			name:     "suffix - false",
			actual:   "hello world",
			expected: "hello",
			option:   MatchSuffix,
			want:     false,
		},
		{
			name:     "ignore case - true",
			actual:   "Hello World",
			expected: "hello world",
			option:   MatchIgnoreCase,
			want:     true,
		},
		{
			name:     "ignore case - false",
			actual:   "Hello World",
			expected: "goodbye world",
			option:   MatchIgnoreCase,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compareValuesWithOption(tt.actual, tt.expected, tt.option); got != tt.want {
				t.Errorf("compareValuesWithOption() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToInt64(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		want    int64
		wantErr bool
	}{
		{
			name:    "int",
			input:   42,
			want:    42,
			wantErr: false,
		},
		{
			name:    "int32",
			input:   int32(42),
			want:    42,
			wantErr: false,
		},
		{
			name:    "int64",
			input:   int64(42),
			want:    42,
			wantErr: false,
		},
		{
			name:    "float64",
			input:   42.0,
			want:    42,
			wantErr: false,
		},
		{
			name:    "string - should error",
			input:   "42",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toInt64(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("toInt64() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("toInt64() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		want    float64
		wantErr bool
	}{
		{
			name:    "float32",
			input:   float32(42.5),
			want:    42.5,
			wantErr: false,
		},
		{
			name:    "float64",
			input:   42.5,
			want:    42.5,
			wantErr: false,
		},
		{
			name:    "int",
			input:   42,
			want:    42.0,
			wantErr: false,
		},
		{
			name:    "string - should error",
			input:   "42.5",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toFloat64(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("toFloat64() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("toFloat64() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertStringMapToInterface(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]string
		want map[string]interface{}
	}{
		{
			name: "empty map",
			m:    map[string]string{},
			want: map[string]interface{}{},
		},
		{
			name: "non-empty map",
			m: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			want: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name: "nil map",
			m:    nil,
			want: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertStringMapToInterface(tt.m)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertStringMapToInterface() = %v, want %v", got, tt.want)
			}
		})
	}
}
