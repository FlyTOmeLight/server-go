package values

func GetValueOrDefault[T comparable](value, defaultValue T) T {
	var zeroValue T
	if value != zeroValue {
		return value
	}
	return defaultValue
}

func GetFirstNonZeroValue[T comparable](values ...T) T {
	var zeroValue T
	for _, v := range values {
		if v != zeroValue {
			return v
		}
	}
	return zeroValue
}

func First[T any](values []T) T {
	var zeroValue T
	if len(values) == 0 {
		return zeroValue
	}
	return values[0]
}

func FirstOrDefault[T any](values []T, defaultValue T) T {
	if len(values) == 0 {
		return defaultValue
	}
	return values[0]
}
