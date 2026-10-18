package structured

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
)

func TestToStringSet(t *testing.T) {
	tests := []struct {
		name string
		ops  []string
		want map[string]bool
	}{
		{
			name: "valid slice",
			ops:  []string{"read", "write", "execute"},
			want: map[string]bool{
				"read":    true,
				"write":   true,
				"execute": true,
			},
		},
		{
			name: "slice with duplicates",
			ops:  []string{"read", "read", "write", "write"},
			want: map[string]bool{
				"read":  true,
				"write": true,
			},
		},
		{
			name: "empty slice",
			ops:  []string{},
			want: map[string]bool{},
		},
		{
			name: "nil slice",
			ops:  nil,
			want: map[string]bool{},
		},
		{
			name: "single element",
			ops:  []string{"admin"},
			want: map[string]bool{
				"admin": true,
			},
		},
		{
			name: "with special characters",
			ops:  []string{"key.with.dots", "key-with-dashes", "key_with_underscores"},
			want: map[string]bool{
				"key.with.dots":        true,
				"key-with-dashes":      true,
				"key_with_underscores": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToStringSet(tt.ops)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ToStringSet() = %v, want %v", got, tt.want)
			}
			// Verify all values are true
			for key, val := range got {
				if !val {
					t.Errorf("ToStringSet() key %q has value false, expected true", key)
				}
			}
		})
	}
}

func TestSetContains(t *testing.T) {
	tests := []struct {
		name string
		s    map[string]struct{}
		key  string
		want bool
	}{
		{
			name: "key exists",
			s: map[string]struct{}{
				"read":  {},
				"write": {},
			},
			key:  "read",
			want: true,
		},
		{
			name: "key does not exist",
			s: map[string]struct{}{
				"read":  {},
				"write": {},
			},
			key:  "execute",
			want: false,
		},
		{
			name: "nil map",
			s:    nil,
			key:  "read",
			want: false,
		},
		{
			name: "empty map",
			s:    map[string]struct{}{},
			key:  "read",
			want: false,
		},
		{
			name: "empty string key",
			s: map[string]struct{}{
				"": {},
			},
			key:  "",
			want: true,
		},
		{
			name: "key with special characters",
			s: map[string]struct{}{
				"key.with.dots": {},
				"key-with-dash": {},
			},
			key:  "key.with.dots",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SetContains(tt.s, tt.key)
			if got != tt.want {
				t.Errorf("SetContains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMapContains(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]interface{}
		key  string
		want bool
	}{
		{
			name: "key exists with string value",
			m: map[string]interface{}{
				"name": "myapp",
				"port": 8080,
			},
			key:  "name",
			want: true,
		},
		{
			name: "key exists with int value",
			m: map[string]interface{}{
				"name": "myapp",
				"port": 8080,
			},
			key:  "port",
			want: true,
		},
		{
			name: "key exists with nil value",
			m: map[string]interface{}{
				"name": "myapp",
				"nil":  nil,
			},
			key:  "nil",
			want: true,
		},
		{
			name: "key does not exist",
			m: map[string]interface{}{
				"name": "myapp",
				"port": 8080,
			},
			key:  "missing",
			want: false,
		},
		{
			name: "nil map",
			m:    nil,
			key:  "name",
			want: false,
		},
		{
			name: "empty map",
			m:    map[string]interface{}{},
			key:  "name",
			want: false,
		},
		{
			name: "key with special characters",
			m: map[string]interface{}{
				"key.with.dots": "value1",
				"key-with-dash": "value2",
			},
			key:  "key.with.dots",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapContains(tt.m, tt.key)
			if got != tt.want {
				t.Errorf("MapContains() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Additional tests for generic MapContains with different value types
func TestMapContainsGeneric(t *testing.T) {
	// Test with int values
	t.Run("int values", func(t *testing.T) {
		m := map[string]int{
			"one": 1,
			"two": 2,
		}
		if !MapContains(m, "one") {
			t.Errorf("MapContains() with int map should return true for existing key")
		}
		if MapContains(m, "three") {
			t.Errorf("MapContains() with int map should return false for missing key")
		}
	})

	// Test with bool values
	t.Run("bool values", func(t *testing.T) {
		m := map[string]bool{
			"enabled":  true,
			"disabled": false,
		}
		if !MapContains(m, "enabled") {
			t.Errorf("MapContains() with bool map should return true for existing key")
		}
		if MapContains(m, "missing") {
			t.Errorf("MapContains() with bool map should return false for missing key")
		}
	})

	// Test with struct values
	t.Run("struct values", func(t *testing.T) {
		type Person struct {
			Name string
			Age  int
		}
		m := map[string]Person{
			"alice": {Name: "Alice", Age: 30},
			"bob":   {Name: "Bob", Age: 25},
		}
		if !MapContains(m, "alice") {
			t.Errorf("MapContains() with struct map should return true for existing key")
		}
		if MapContains(m, "charlie") {
			t.Errorf("MapContains() with struct map should return false for missing key")
		}
	})

	// Test with slice values
	t.Run("slice values", func(t *testing.T) {
		m := map[string][]string{
			"fruits": {"apple", "banana"},
			"colors": {"red", "blue"},
		}
		if !MapContains(m, "fruits") {
			t.Errorf("MapContains() with slice map should return true for existing key")
		}
		if MapContains(m, "vegetables") {
			t.Errorf("MapContains() with slice map should return false for missing key")
		}
	})
}

// Benchmark tests
func BenchmarkToStringSet(b *testing.B) {
	ops := []string{"read", "write", "execute", "delete", "create", "update"}
	for i := 0; i < b.N; i++ {
		ToStringSet(ops)
	}
}

func BenchmarkSetContains(b *testing.B) {
	s := map[string]struct{}{
		"read":    {},
		"write":   {},
		"execute": {},
		"delete":  {},
		"create":  {},
		"update":  {},
	}
	for i := 0; i < b.N; i++ {
		SetContains(s, "read")
	}
}

func BenchmarkMapContains(b *testing.B) {
	m := map[string]interface{}{
		"read":    true,
		"write":   true,
		"execute": true,
		"delete":  true,
		"create":  true,
		"update":  true,
	}
	for i := 0; i < b.N; i++ {
		MapContains(m, "read")
	}
}

// Example usage tests
func ExampleToStringSet() {
	ops := []string{"read", "write", "execute"}
	set := ToStringSet(ops)
	if set["read"] {
		fmt.Println("read permission exists")
	}
	// Output: read permission exists
}

func ExampleSetContains() {
	permissions := map[string]struct{}{
		"read":  {},
		"write": {},
	}
	if SetContains(permissions, "read") {
		fmt.Println("has read permission")
	}
	// Output: has read permission
}

func ExampleMapContains() {
	config := map[string]interface{}{
		"port":  8080,
		"debug": true,
	}
	if MapContains(config, "port") {
		fmt.Println("port is configured")
	}
	// Output: port is configured
}

func TestSortedKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected []string
	}{
		{
			name:     "nil map",
			input:    nil,
			expected: []string{},
		},
		{
			name:     "empty map",
			input:    map[string]any{},
			expected: []string{},
		},
		{
			name:     "single key",
			input:    map[string]any{"foo": 42},
			expected: []string{"foo"},
		},
		{
			name: "multiple keys - already sorted",
			input: map[string]any{
				"a": 1,
				"b": 2,
				"c": 3,
			},
			expected: []string{"a", "b", "c"},
		},
		{
			name: "multiple keys - unsorted",
			input: map[string]any{
				"z": 26,
				"a": 1,
				"m": 13,
			},
			expected: []string{"a", "m", "z"},
		},
		{
			name: "keys with different lengths",
			input: map[string]any{
				"apple":      "fruit",
				"banana":     "fruit",
				"cherry":     "fruit",
				"date":       "fruit",
				"elderberry": "fruit",
			},
			expected: []string{"apple", "banana", "cherry", "date", "elderberry"},
		},
		{
			name: "numeric strings as keys",
			input: map[string]any{
				"100": "hundred",
				"1":   "one",
				"10":  "ten",
				"2":   "two",
			},
			expected: []string{"1", "10", "100", "2"}, // Lexicographic, not numeric
		},
		{
			name: "mixed case keys",
			input: map[string]any{
				"Zebra": "animal",
				"apple": "fruit",
				"Apple": "fruit",
				"zoo":   "place",
			},
			expected: []string{"Apple", "Zebra", "apple", "zoo"}, // Lexicographic by Unicode
		},
		{
			name: "keys with special characters",
			input: map[string]any{
				"!":     "exclamation",
				"*":     "asterisk",
				"@":     "at",
				"[":     "bracket",
				"hello": "world",
			},
			expected: []string{"!", "*", "@", "[", "hello"}, // ASCII order
		},
		{
			name: "large number of keys",
			input: func() map[string]any {
				m := make(map[string]any)
				for i := 0; i < 100; i++ {
					m[string(rune('a'+i%26))+string(rune('0'+i/26))] = i
				}
				return m
			}(),
			expected: func() []string {
				keys := make([]string, 0, 100)
				for i := 0; i < 100; i++ {
					keys = append(keys, string(rune('a'+i%26))+string(rune('0'+i/26)))
				}
				sort.Strings(keys)
				return keys
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SortedKeys(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("SortedKeys() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSortedKeysWithDifferentValueTypes(t *testing.T) {
	// Test with int values
	t.Run("int values", func(t *testing.T) {
		m := map[string]int{"b": 2, "a": 1, "c": 3}
		expected := []string{"a", "b", "c"}
		result := SortedKeys(m)
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("SortedKeys() with int values = %v, want %v", result, expected)
		}
	})

	// Test with bool values
	t.Run("bool values", func(t *testing.T) {
		m := map[string]bool{"yes": true, "no": false, "maybe": true}
		expected := []string{"maybe", "no", "yes"}
		result := SortedKeys(m)
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("SortedKeys() with bool values = %v, want %v", result, expected)
		}
	})

	// Test with struct values
	t.Run("struct values", func(t *testing.T) {
		type Person struct{ Name string }
		m := map[string]Person{"alice": {"Alice"}, "bob": {"Bob"}}
		expected := []string{"alice", "bob"}
		result := SortedKeys(m)
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("SortedKeys() with struct values = %v, want %v", result, expected)
		}
	})

	// Test with slice values
	t.Run("slice values", func(t *testing.T) {
		m := map[string][]int{"z": {26}, "a": {1}, "m": {13}}
		expected := []string{"a", "m", "z"}
		result := SortedKeys(m)
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("SortedKeys() with slice values = %v, want %v", result, expected)
		}
	})
}

func TestSortedKeysStability(t *testing.T) {
	// Ensure the function always returns the same order for the same input
	m := map[string]int{
		"delta":   4,
		"alpha":   1,
		"gamma":   3,
		"beta":    2,
		"epsilon": 5,
	}

	// Run multiple times and verify consistency
	for i := 0; i < 10; i++ {
		result1 := SortedKeys(m)
		result2 := SortedKeys(m)
		if !reflect.DeepEqual(result1, result2) {
			t.Errorf("SortedKeys() returned different results on consecutive calls: %v vs %v", result1, result2)
		}
	}
}

func TestSortedKeysDoesNotModifyMap(t *testing.T) {
	m := map[string]int{"z": 1, "a": 2, "b": 3}
	originalKeys := make([]string, 0, len(m))
	for k := range m {
		originalKeys = append(originalKeys, k)
	}

	// Call SortedKeys
	_ = SortedKeys(m) // ignore result

	// Verify map unchanged
	if len(m) != 3 {
		t.Errorf("Map length changed from 3 to %d", len(m))
	}

	for k := range m {
		found := false
		for _, orig := range originalKeys {
			if k == orig {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Map key %q was modified", k)
		}
	}
}

func TestDedupStrings(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "nil slice",
			input: nil,
			want:  []string{},
		},
		{
			name:  "empty slice",
			input: []string{},
			want:  []string{},
		},
		{
			name:  "single element",
			input: []string{"a"},
			want:  []string{"a"},
		},
		{
			name:  "no duplicates",
			input: []string{"a", "b", "c"},
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "consecutive duplicates",
			input: []string{"a", "a", "b", "b", "c"},
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "non-consecutive duplicates",
			input: []string{"a", "b", "a", "c", "b", "d"},
			want:  []string{"a", "b", "c", "d"},
		},
		{
			name:  "all duplicates",
			input: []string{"x", "x", "x", "x"},
			want:  []string{"x"},
		},
		{
			name:  "mixed case preserved",
			input: []string{"a", "A", "a", "A"},
			want:  []string{"a", "A"}, // case-sensitive
		},
		{
			name:  "empty strings",
			input: []string{"", "", "a", ""},
			want:  []string{"", "a"},
		},
		{
			name:  "large dataset",
			input: []string{"z", "a", "z", "b", "c", "a", "z"},
			want:  []string{"z", "a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DedupStrings(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DedupStrings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDedupStringsDoesNotModifyInput(t *testing.T) {
	input := []string{"a", "b", "a", "c"}
	original := make([]string, len(input))
	copy(original, input)

	DedupStrings(input)

	if !reflect.DeepEqual(input, original) {
		t.Errorf("DedupStrings() modified input = %v, original %v", input, original)
	}
}

func BenchmarkDedupStrings(b *testing.B) {
	ss := make([]string, 10000)
	for i := 0; i < 10000; i++ {
		ss[i] = string(rune('a' + i%26))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DedupStrings(ss)
	}
}
