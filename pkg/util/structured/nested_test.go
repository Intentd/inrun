package structured

import (
	"reflect"
	"testing"
)

func TestNestedSlice(t *testing.T) {
	tests := []struct {
		name   string
		obj    map[string]interface{}
		keys   []string
		want   []interface{}
		wantOk bool
	}{
		{
			name: "valid nested slice",
			obj: map[string]interface{}{
				"app": map[string]interface{}{
					"items": []interface{}{"a", "b", "c"},
				},
			},
			keys:   []string{"app", "items"},
			want:   []interface{}{"a", "b", "c"},
			wantOk: true,
		},
		{
			name: "slice at top level",
			obj: map[string]interface{}{
				"items": []interface{}{1, 2, 3},
			},
			keys:   []string{"items"},
			want:   []interface{}{1, 2, 3},
			wantOk: true,
		},
		{
			name: "non-slice value at final key",
			obj: map[string]interface{}{
				"app": map[string]interface{}{
					"name": "myapp",
				},
			},
			keys:   []string{"app", "name"},
			want:   nil,
			wantOk: false,
		},
		{
			name: "missing intermediate key",
			obj: map[string]interface{}{
				"app": map[string]interface{}{
					"items": []interface{}{"a", "b"},
				},
			},
			keys:   []string{"app", "missing", "items"},
			want:   nil,
			wantOk: false,
		},
		{
			name: "missing final key",
			obj: map[string]interface{}{
				"app": map[string]interface{}{
					"items": []interface{}{"a", "b"},
				},
			},
			keys:   []string{"app", "missing"},
			want:   nil,
			wantOk: false,
		},
		{
			name: "empty keys",
			obj: map[string]interface{}{
				"items": []interface{}{"a", "b"},
			},
			keys:   []string{},
			want:   nil,
			wantOk: false,
		},
		{
			name:   "nil object",
			obj:    nil,
			keys:   []string{"app", "items"},
			want:   nil,
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := NestedSlice(tt.obj, tt.keys...)
			if ok != tt.wantOk {
				t.Errorf("NestedSlice() ok = %v, want %v", ok, tt.wantOk)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NestedSlice() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNestedMap(t *testing.T) {
	tests := []struct {
		name   string
		obj    map[string]interface{}
		keys   []string
		want   map[string]interface{}
		wantOk bool
	}{
		{
			name: "valid nested map",
			obj: map[string]interface{}{
				"app": map[string]interface{}{
					"config": map[string]interface{}{
						"key": "value",
					},
				},
			},
			keys:   []string{"app", "config"},
			want:   map[string]interface{}{"key": "value"},
			wantOk: true,
		},
		{
			name: "top level map",
			obj: map[string]interface{}{
				"config": map[string]interface{}{
					"key": "value",
				},
			},
			keys:   []string{"config"},
			want:   map[string]interface{}{"key": "value"},
			wantOk: true,
		},
		{
			name: "non-map value at final key",
			obj: map[string]interface{}{
				"app": map[string]interface{}{
					"name": "myapp",
				},
			},
			keys:   []string{"app", "name"},
			want:   nil,
			wantOk: false,
		},
		{
			name: "missing intermediate key",
			obj: map[string]interface{}{
				"app": map[string]interface{}{
					"config": map[string]interface{}{"key": "value"},
				},
			},
			keys:   []string{"app", "missing", "config"},
			want:   nil,
			wantOk: false,
		},
		{
			name: "empty keys",
			obj: map[string]interface{}{
				"config": map[string]interface{}{"key": "value"},
			},
			keys:   []string{},
			want:   nil,
			wantOk: false,
		},
		{
			name:   "nil object",
			obj:    nil,
			keys:   []string{"app", "config"},
			want:   nil,
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := NestedMap(tt.obj, tt.keys...)
			if ok != tt.wantOk {
				t.Errorf("NestedMap() ok = %v, want %v", ok, tt.wantOk)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NestedMap() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeepCopyMap(t *testing.T) {
	tests := []struct {
		name string
		src  map[string]interface{}
		want map[string]interface{}
	}{
		{
			name: "nil input",
			src:  nil,
			want: nil,
		},
		{
			name: "empty map",
			src:  map[string]interface{}{},
			want: map[string]interface{}{},
		},
		{
			name: "flat map with scalar values",
			src: map[string]interface{}{
				"name":  "myapp",
				"port":  8080,
				"debug": true,
			},
			want: map[string]interface{}{
				"name":  "myapp",
				"port":  8080,
				"debug": true,
			},
		},
		{
			name: "nested maps",
			src: map[string]interface{}{
				"app": map[string]interface{}{
					"name": "myapp",
					"config": map[string]interface{}{
						"key": "value",
					},
				},
				"items": []interface{}{"a", "b"},
			},
			want: map[string]interface{}{
				"app": map[string]interface{}{
					"name": "myapp",
					"config": map[string]interface{}{
						"key": "value",
					},
				},
				"items": []interface{}{"a", "b"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeepCopyMap(tt.src)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DeepCopyMap() got = %v, want %v", got, tt.want)
			}
			// Verify that nested maps are actually copies (not same pointer)
			if len(tt.src) > 0 {
				if reflect.ValueOf(got).Pointer() == reflect.ValueOf(tt.src).Pointer() {
					t.Errorf("DeepCopyMap() returned same map pointer, should be a copy")
				}
				// Check nested maps separately
				for k, v := range tt.src {
					if nested, ok := v.(map[string]interface{}); ok {
						if gotNested, ok := got[k].(map[string]interface{}); ok {
							if reflect.ValueOf(gotNested).Pointer() == reflect.ValueOf(nested).Pointer() {
								t.Errorf("DeepCopyMap() nested map for key %q is same pointer, should be copied", k)
							}
							if !reflect.DeepEqual(gotNested, nested) {
								t.Errorf("DeepCopyMap() nested map for key %q content mismatch", k)
							}
						}
					}
				}
			}
		})
	}
}

func TestSetNestedPath(t *testing.T) {
	tests := []struct {
		name    string
		initial map[string]interface{}
		path    string
		value   interface{}
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name:    "set simple key",
			initial: map[string]interface{}{},
			path:    "name",
			value:   "myapp",
			want:    map[string]interface{}{"name": "myapp"},
			wantErr: false,
		},
		{
			name: "set nested path",
			initial: map[string]interface{}{
				"app": map[string]interface{}{},
			},
			path:  "app.repository",
			value: "myorg/payments-api",
			want: map[string]interface{}{
				"app": map[string]interface{}{
					"repository": "myorg/payments-api",
				},
			},
			wantErr: false,
		},
		{
			name:    "create intermediate maps",
			initial: map[string]interface{}{},
			path:    "app.config.timeout",
			value:   30,
			want: map[string]interface{}{
				"app": map[string]interface{}{
					"config": map[string]interface{}{
						"timeout": 30,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "path segment not a map - overwrite existing value",
			initial: map[string]interface{}{
				"app": "string-value",
			},
			path:    "app.repository",
			value:   "myorg/payments-api",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty path",
			initial: map[string]interface{}{},
			path:    "",
			value:   "test",
			want:    map[string]interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SetNestedPath(tt.initial, tt.path, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetNestedPath() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(tt.initial, tt.want) {
				t.Errorf("SetNestedPath() got = %v, want %v", tt.initial, tt.want)
			}
		})
	}
}

func TestGetNestedPath(t *testing.T) {
	tests := []struct {
		name   string
		m      map[string]interface{}
		path   string
		want   interface{}
		wantOk bool
	}{
		{
			name: "get simple key",
			m: map[string]interface{}{
				"name": "myapp",
			},
			path:   "name",
			want:   "myapp",
			wantOk: true,
		},
		{
			name: "get nested path",
			m: map[string]interface{}{
				"app": map[string]interface{}{
					"repository": "myorg/payments-api",
				},
			},
			path:   "app.repository",
			want:   "myorg/payments-api",
			wantOk: true,
		},
		{
			name: "get deeply nested path",
			m: map[string]interface{}{
				"app": map[string]interface{}{
					"config": map[string]interface{}{
						"timeout": 30,
					},
				},
			},
			path:   "app.config.timeout",
			want:   30,
			wantOk: true,
		},
		{
			name: "path not found",
			m: map[string]interface{}{
				"name": "myapp",
			},
			path:   "missing",
			want:   nil,
			wantOk: false,
		},
		{
			name: "nested path not found",
			m: map[string]interface{}{
				"app": map[string]interface{}{
					"name": "myapp",
				},
			},
			path:   "app.missing",
			want:   nil,
			wantOk: false,
		},
		{
			name: "path segment not a map",
			m: map[string]interface{}{
				"app": "string-value",
			},
			path:   "app.repository",
			want:   nil,
			wantOk: false,
		},
		{
			name:   "empty path",
			m:      map[string]interface{}{},
			path:   "",
			want:   nil,
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := GetNestedPath(tt.m, tt.path)
			if ok != tt.wantOk {
				t.Errorf("GetNestedPath() ok = %v, want %v", ok, tt.wantOk)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetNestedPath() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteNestedPath(t *testing.T) {
	tests := []struct {
		name string
		obj  map[string]interface{}
		path string
		want map[string]interface{}
	}{
		{
			name: "delete simple key",
			obj: map[string]interface{}{
				"name": "myapp",
				"port": 8080,
			},
			path: "name",
			want: map[string]interface{}{
				"port": 8080,
			},
		},
		{
			name: "delete nested key",
			obj: map[string]interface{}{
				"app": map[string]interface{}{
					"repository": "myorg/payments-api",
					"version":    "v1",
				},
			},
			path: "app.repository",
			want: map[string]interface{}{
				"app": map[string]interface{}{
					"version": "v1",
				},
			},
		},
		{
			name: "delete deeply nested key",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"key1": "value1",
						"key2": "value2",
					},
				},
			},
			path: "metadata.annotations.key1",
			want: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"key2": "value2",
					},
				},
			},
		},
		{
			name: "delete non-existent key (no-op)",
			obj: map[string]interface{}{
				"name": "myapp",
			},
			path: "missing",
			want: map[string]interface{}{
				"name": "myapp",
			},
		},
		{
			name: "delete non-existent nested key (no-op)",
			obj: map[string]interface{}{
				"app": map[string]interface{}{
					"name": "myapp",
				},
			},
			path: "app.missing",
			want: map[string]interface{}{
				"app": map[string]interface{}{
					"name": "myapp",
				},
			},
		},
		{
			name: "path segment not a map (no-op)",
			obj: map[string]interface{}{
				"app": "string-value",
			},
			path: "app.repository",
			want: map[string]interface{}{
				"app": "string-value",
			},
		},
		{
			name: "empty path (no-op)",
			obj: map[string]interface{}{
				"name": "myapp",
			},
			path: "",
			want: map[string]interface{}{
				"name": "myapp",
			},
		},
		{
			name: "delete nested map completely",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"key": "value",
					},
					"labels": map[string]interface{}{
						"env": "prod",
					},
				},
			},
			path: "metadata.annotations",
			want: map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"env": "prod",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			DeleteNestedPath(tt.obj, tt.path)
			if !reflect.DeepEqual(tt.obj, tt.want) {
				t.Errorf("DeleteNestedPath() got = %v, want %v", tt.obj, tt.want)
			}
		})
	}
}

func TestIsNestedPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "simple key",
			path: "name",
			want: false,
		},
		{
			name: "one dot",
			path: "app.repository",
			want: true,
		},
		{
			name: "multiple dots",
			path: "metadata.annotations.internal-key",
			want: true,
		},
		{
			name: "empty path",
			path: "",
			want: false,
		},
		{
			name: "dot at start",
			path: ".start",
			want: true,
		},
		{
			name: "dot at end",
			path: "end.",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNestedPath(tt.path); got != tt.want {
				t.Errorf("IsNestedPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
