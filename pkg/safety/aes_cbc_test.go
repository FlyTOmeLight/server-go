package safety

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	keyString  = "6kY8uyQEl6XV37KR"
	ivString   = "E5ZKizMERreD3HX8"
	text       = "oooooo11111113442"
	ciphertext = "qg5lFlIxLWKUtXUKi9FVoLfmiTNDTRVjQfOh7k/ti+o="
)

func TestCBCEncrypter(t *testing.T) {
	t.Run("aes cbc encrypter", func(t *testing.T) {
		key := []byte(keyString)
		iv := []byte(ivString)

		encryptionText, err := CBCEncrypter(text, key, iv)
		assert.Nil(t, err)
		assert.Equal(t, encryptionText, ciphertext)
	})
}

func TestCBCDecrypter(t *testing.T) {
	t.Run("aes cbc decrypter", func(t *testing.T) {
		key := []byte(keyString)
		iv := []byte(ivString)

		plaintext, err := CBCDecrypter(ciphertext, key, iv)
		assert.Nil(t, err)
		assert.Equal(t, plaintext, text)
	})
}

func TestIsBase64(t *testing.T) {
	t.Run("judge base64", func(t *testing.T) {
		is := IsBase64(ciphertext)
		not := IsBase64(text)
		assert.Equal(t, is, true)
		assert.Equal(t, not, false)
	})
}

func BenchmarkIsBase64(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		is := IsBase64(ciphertext)
		assert.Equal(b, is, true)
	}
}
