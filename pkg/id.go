package core

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/cespare/xxhash/v2"
	"github.com/google/uuid"
)

const defaultLetters = "abcdefghijkmnpqrstuvwxyz0123456789"

func NewID(prefix string) string {
	const maxPrefixRunes = 16
	runeCount := 0
	for i := range prefix {
		if runeCount == maxPrefixRunes {
			prefix = prefix[:i]
			break
		}
		runeCount++
	}
	return fmt.Sprintf("%s-%s", prefix, RandomString(8))
}

// RandomString returns a random string with a fixed length
func RandomString(n int, allowedChars ...string) string {
	letters := defaultLetters
	if len(allowedChars) > 0 {
		letters = allowedChars[0]
	}
	if n <= 0 || len(letters) == 0 {
		return ""
	}

	alphabetSize := big.NewInt(int64(len(letters)))
	b := make([]byte, n)
	for i := range b {
		index, err := rand.Int(rand.Reader, alphabetSize)
		if err != nil {
			panic(err)
		}
		b[i] = letters[index.Int64()]
	}

	return string(b)
}

func NewHashID(raw []byte) string {
	hash64 := xxhash.Sum64(raw)
	id := fmt.Sprintf("%x", hash64)
	return id
}

func NewHashIDByObject(obj any) (string, error) {
	raw, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}

	id := NewHashID(raw)
	return id, nil
}

func NewUUID() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}
