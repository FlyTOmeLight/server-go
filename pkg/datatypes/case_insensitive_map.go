package datatypes

import (
	"net/http"
	"strings"
)

// CaseInsensitiveMap is a map with string keys and generic type values.
// T is the generic type for the map's values.
type CaseInsensitiveMap[T any] map[string]T

// Lookup searches the map for the given key in a case-insensitive manner.
// It returns the value associated with the key and a boolean indicating
// whether the key was found.
func (m CaseInsensitiveMap[T]) Lookup(key string) (T, bool) {
	var zero T // zero value for the generic type T
	if len(m) == 0 {
		return zero, false
	}
	for _, k := range []string{
		key,
		strings.ToLower(key),
		http.CanonicalHeaderKey(key),
	} {
		if v, ok := m[k]; ok {
			return v, true
		}
	}
	return zero, false
}

// Delete removes the entry associated with the key from the map, if present.
// It checks for original, lowercased and the canonical versions of the key.
func (m CaseInsensitiveMap[T]) Delete(key string) {
	for _, k := range []string{
		key,
		strings.ToLower(key),
		http.CanonicalHeaderKey(key),
	} {
		delete(m, k)
	}
}
