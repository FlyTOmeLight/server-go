package values

import "testing"

func TestGetValueOrDefault(t *testing.T) {
	// Test with string values
	t.Run("string", func(t *testing.T) {
		v, vDefault := "", "a"
		expected := "a"
		result := GetValueOrDefault(v, vDefault)
		if result != expected {
			t.Errorf("v = %v, vDefault = %v,  want %v, get %v", v, vDefault, expected, result)
		}
	})

	// Test with int values
	t.Run("int", func(t *testing.T) {
		v, vDefault := 10, 20
		expected := 10
		result := GetValueOrDefault(v, vDefault)
		if result != expected {
			t.Errorf("v = %v, vDefault = %v,  want %v, get %v", v, vDefault, expected, result)
		}
	})

	// Test with float64 values
	t.Run("float64", func(t *testing.T) {
		v, vDefault := 0.0, 3.14
		expected := 3.14
		result := GetValueOrDefault(v, vDefault)
		if result != expected {
			t.Errorf("v = %v, vDefault = %v,  want %v, get %v", v, vDefault, expected, result)
		}
	})
}

func TestGetFirstNonEmptyValue(t *testing.T) {
	// Test with string values
	t.Run("string", func(t *testing.T) {
		testCases := []struct {
			desc     string
			values   []string
			expected string
		}{
			{
				desc:     "empty value",
				values:   []string{},
				expected: "",
			},
			{
				desc:     "multi values",
				values:   []string{"", "a", " ", "b"},
				expected: "a",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.desc, func(t *testing.T) {
				result := GetFirstNonZeroValue(tc.values...)
				if result != tc.expected {
					t.Errorf("values = %v, want %v, get %v", tc.values, tc.expected, result)
				}
			})
		}
	})

	// Test with int values
	t.Run("int", func(t *testing.T) {
		values := []int{0, 0, 42, 200}
		expected := 42
		result := GetFirstNonZeroValue(values...)
		if result != expected {
			t.Errorf("values = %v, want %v, get %v", values, expected, result)
		}
	})

	// Test with float64 values
	t.Run("float64", func(t *testing.T) {
		values := []float64{0.0, 3.14, 2.0}
		expected := 3.14
		result := GetFirstNonZeroValue(values...)
		if result != expected {
			t.Errorf("values = %v, want %v, get %v", values, expected, result)
		}
	})
}

func TestFirst(t *testing.T) {
	t.Run("*int", func(t *testing.T) {
		a := 1
		b := 2

		testCases := []struct {
			desc     string
			values   []*int
			expected *int
		}{
			{
				desc:     "nil values",
				values:   nil,
				expected: nil,
			},
			{
				desc:     "empty values",
				values:   make([]*int, 0),
				expected: nil,
			},
			{
				desc:     "multi values",
				values:   []*int{&a, &b},
				expected: &a,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.desc, func(t *testing.T) {
				result := First(tc.values)
				if result != tc.expected {
					t.Errorf("values = %v, want %v, get %v", tc.values, tc.expected, result)
				}
			})
		}
	})

	t.Run("string", func(t *testing.T) {
		testCases := []struct {
			desc     string
			values   []string
			expected string
		}{
			{
				desc:     "empty values",
				values:   []string{},
				expected: "",
			},
			{
				desc:     "multi values",
				values:   []string{"a", "b"},
				expected: "a",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.desc, func(t *testing.T) {
				result := First(tc.values)
				if result != tc.expected {
					t.Errorf("values = %v, want %v, get %v", tc.values, tc.expected, result)
				}
			})
		}
	})
}

func TestFirstOrDefault(t *testing.T) {
	t.Run("*int", func(t *testing.T) {
		a := 1
		b := 2

		testCases := []struct {
			desc         string
			values       []*int
			defaultValue *int
			expected     *int
		}{
			{
				desc:     "nil values",
				values:   nil,
				expected: nil,
			},
			{
				desc:         "empty values",
				values:       make([]*int, 0),
				defaultValue: &a,
				expected:     &a,
			},
			{
				desc:         "multi values",
				values:       []*int{&a, &b},
				defaultValue: &b,
				expected:     &a,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.desc, func(t *testing.T) {
				result := FirstOrDefault(tc.values, tc.defaultValue)
				if result != tc.expected {
					t.Errorf("values = %v, want %v, get %v", tc.values, tc.expected, result)
				}
			})
		}
	})

	t.Run("string", func(t *testing.T) {
		testCases := []struct {
			desc         string
			values       []string
			defaultValue string
			expected     string
		}{
			{
				desc:         "empty values",
				values:       []string{},
				defaultValue: "a",
				expected:     "a",
			},
			{
				desc:         "multi values",
				values:       []string{"a", "b"},
				defaultValue: "b",
				expected:     "a",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.desc, func(t *testing.T) {
				result := FirstOrDefault(tc.values, tc.defaultValue)
				if result != tc.expected {
					t.Errorf("values = %v, want %v, get %v", tc.values, tc.expected, result)
				}
			})
		}
	})
}
