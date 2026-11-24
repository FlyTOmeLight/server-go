package safety

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
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
	for _, input := range []string{ciphertext, "YQ==", "YWI=", "YWJj", "+/8="} {
		assert.True(t, IsBase64(input), "%q", input)
	}
	for _, input := range []string{text, "", "YQ", "YQ=", "YR==", "-_8=", "YQ==\n", "Y=Q="} {
		assert.False(t, IsBase64(input), "%q", input)
	}
}

func TestPKCS7PaddingCopiesInput(t *testing.T) {
	backing := []byte("testXXXXXXXXXXXX")
	input := backing[:4]
	for _, blockSize := range []int{aes.BlockSize, 0, -1, 256} {
		padded := PKCS7Padding(input, blockSize)
		assert.Equal(t, "testXXXXXXXXXXXX", string(backing))
		assert.Equal(t, "test\x0c\x0c\x0c\x0c\x0c\x0c\x0c\x0c\x0c\x0c\x0c\x0c", string(padded))
		padded[0] = 'X'
		assert.Equal(t, "test", string(input))
	}
}

func TestUnPKCS7Padding(t *testing.T) {
	for size := range aes.BlockSize {
		input := make([]byte, size)
		unpadded, err := UnPKCS7Padding(PKCS7Padding(input, aes.BlockSize))
		assert.NoError(t, err)
		assert.Equal(t, input, unpadded)
	}

	wrongBytes := make([]byte, aes.BlockSize)
	wrongBytes[aes.BlockSize-1] = 2
	wrongLength := make([]byte, aes.BlockSize)
	wrongLength[aes.BlockSize-1] = 17
	for _, input := range [][]byte{nil, {1}, make([]byte, aes.BlockSize), wrongLength, wrongBytes} {
		got, err := UnPKCS7Padding(input)
		assert.Nil(t, got)
		assert.EqualError(t, err, "invalid PKCS7 padding")
	}
}

func BenchmarkIsBase64(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		is := IsBase64(ciphertext)
		assert.Equal(b, is, true)
	}
}

func TestCBCRejectsMalformedInputs(t *testing.T) {
	key, iv := []byte(keyString), []byte(ivString)
	for _, input := range []string{"", "YQ==", "!!!"} {
		_, err := CBCDecrypter(input, key, iv)
		assert.Error(t, err)
	}
	for _, badIV := range [][]byte{nil, []byte("short"), make([]byte, 17)} {
		_, err := CBCEncrypter("plain", key, badIV)
		assert.Error(t, err)
		_, err = CBCDecrypter(ciphertext, key, badIV)
		assert.Error(t, err)
	}
	block, err := aes.NewCipher(key)
	assert.NoError(t, err)
	for _, padding := range []byte{0, 17, 2} {
		plain := make([]byte, aes.BlockSize)
		plain[len(plain)-1] = padding
		// padding=2 has a preceding zero byte, which must also be rejected.
		cipher.NewCBCEncrypter(block, iv).CryptBlocks(plain, plain)
		_, err = CBCDecrypter(base64.StdEncoding.EncodeToString(plain), key, iv)
		assert.EqualError(t, err, "invalid PKCS7 padding")
	}
}
