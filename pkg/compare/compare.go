package compare

import "strconv"

// Ordered 是一个自定义约束，表示可以进行顺序比较的类型
type Ordered interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64 | ~string
}

// IsIncreasing 递归函数检查数组是否递增
func IsIncreasing[T Ordered](arr []T, index int) bool {
	// 基本情况：如果到达数组末尾，返回 true
	if index >= len(arr)-1 {
		return true
	}

	// 检查当前元素是否小于下一个元素
	if !Less(arr[index], arr[index+1]) {
		return false
	}

	// 递归调用，检查下一个元素
	return IsIncreasing(arr, index+1)
}

// Less 比较两个元素 a是否小于b
func Less[T Ordered](a, b T) bool {
	switch aVal := any(a).(type) {
	case string:
		bVal := any(b).(string)
		// 尝试将字符串转换为浮点数进行比较
		numA, errA := strconv.ParseFloat(aVal, 64)
		numB, errB := strconv.ParseFloat(bVal, 64)
		if errA == nil && errB == nil {
			return numA < numB
		}
		// 如果转换失败，按字符串比较
		return aVal < bVal

	default:
		// 对于其他支持的数字类型，直接比较
		return a < b
	}
}
