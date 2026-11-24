package core

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNewID(t *testing.T) {
	for _, tc := range []struct {
		name       string
		prefix     string
		wantPrefix string
	}{
		{name: "short prefix", prefix: "prefix", wantPrefix: "prefix"},
		{name: "long ASCII prefix", prefix: "12345678901234567", wantPrefix: "1234567890123456"},
		{name: "long UTF-8 prefix", prefix: strings.Repeat("测", 17), wantPrefix: strings.Repeat("测", 16)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := NewID(tc.prefix)
			suffix, ok := strings.CutPrefix(id, tc.wantPrefix+"-")
			if !ok || !utf8.ValidString(id) {
				t.Fatalf(
					"NewID(%q) = %q, want UTF-8 ID with prefix %q",
					tc.prefix,
					id,
					tc.wantPrefix+"-",
				)
			}
			checkRandomString(t, suffix, 8, defaultLetters)
		})
	}
}

func TestRandomString(t *testing.T) {
	const length = 16
	seen := make(map[string]struct{}, 1000)
	for range 1000 {
		value := RandomString(length)
		checkRandomString(t, value, length, defaultLetters)
		if value == strings.Repeat(value[:1], length) {
			t.Fatalf("RandomString returned a single repeated character: %q", value)
		}
		if _, ok := seen[value]; ok {
			t.Fatalf("RandomString returned a duplicate: %q", value)
		}
		seen[value] = struct{}{}
	}
}

func TestRandomStringOptions(t *testing.T) {
	checkRandomString(t, RandomString(20, "A#"), 20, "A#")
	for _, tc := range []struct {
		name    string
		length  int
		letters []string
	}{
		{name: "zero length"},
		{name: "negative length", length: -1},
		{name: "empty alphabet", length: 8, letters: []string{""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := RandomString(tc.length, tc.letters...); got != "" {
				t.Fatalf("RandomString(%d, %q) = %q, want empty string", tc.length, tc.letters, got)
			}
		})
	}
}

func checkRandomString(t *testing.T, value string, length int, letters string) {
	t.Helper()
	if len(value) != length || strings.Trim(value, letters) != "" {
		t.Fatalf("got %q, want %d characters from %q", value, length, letters)
	}
}

func BenchmarkRandomString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = RandomString(10)
	}
}
