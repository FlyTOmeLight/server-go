package compare

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsIncreasing(t *testing.T) {
	// 定义测试用例结构体
	type testCase struct {
		name     string
		input    any
		expected bool
	}

	// 创建测试用例
	testCases := []testCase{
		{
			name:     "Increasing Numbers",
			input:    []float64{1.0, 2.0, 3.5, 4.1},
			expected: true,
		},
		{
			name:     "Increasing Strings",
			input:    []string{"apple", "banana", "cherry", "date"},
			expected: true,
		},
		{
			name:     "Numeric Strings",
			input:    []string{"1", "2", "3", "4"},
			expected: true,
		},
		{
			name:     "Single Element",
			input:    []string{"1"},
			expected: true,
		},
		{
			name:     "Non-Increasing Numbers",
			input:    []float64{1.0, 3.0, 2.5, 4.1},
			expected: false,
		},
		{
			name:     "Non-Increasing Strings",
			input:    []string{"apple", "banana", "banana", "date"},
			expected: false,
		},
	}

	// 迭代测试用例
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			switch v := tc.input.(type) {
			case []float64:
				assert.Equal(t, tc.expected, IsIncreasing(v, 0))
			case []string:
				assert.Equal(t, tc.expected, IsIncreasing(v, 0))
			default:
				t.Fatalf("unsupported type %T", tc.input)
			}
		})
	}
}
