package safety

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
)

var base64Regex = regexp.MustCompile(
	"^([A-Za-z0-9+/]{4})*([A-Za-z0-9+/]{4}|[A-Za-z0-9+/]{3}=|[A-Za-z0-9+/]{2}==)$",
)

var errInvalidPKCS7Padding = errors.New("invalid PKCS7 padding")

// CBCEncrypter encrypts using legacy unauthenticated AES-CBC. New code should prefer AEAD such as AES-GCM.
func CBCEncrypter(text string, key []byte, iv []byte) (string, error) {
	// key is secret key
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	if len(iv) != block.BlockSize() {
		return "", fmt.Errorf("invalid CBC IV length: %d", len(iv))
	}

	textBuf := []byte(text)
	// 填充
	padding := PKCS7Padding(textBuf, block.BlockSize())

	// iv is initialization vector
	blockMode := cipher.NewCBCEncrypter(block, iv)

	// 加密
	encryptedBuf := make([]byte, len(padding))
	blockMode.CryptBlocks(encryptedBuf, padding)

	result := base64.StdEncoding.EncodeToString(encryptedBuf)

	return result, nil
}

// CBCDecrypter decrypts legacy unauthenticated AES-CBC. New code should prefer AEAD such as AES-GCM.
func CBCDecrypter(ciphertext string, key []byte, iv []byte) (string, error) {
	cipherBuf, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	if len(iv) != block.BlockSize() {
		return "", fmt.Errorf("invalid CBC IV length: %d", len(iv))
	}
	if len(cipherBuf) == 0 || len(cipherBuf)%block.BlockSize() != 0 {
		return "", errors.New("CBC ciphertext must contain complete nonempty blocks")
	}
	blockMode := cipher.NewCBCDecrypter(block, iv)

	// 解密
	decryptedBuf := make([]byte, len(cipherBuf))
	blockMode.CryptBlocks(decryptedBuf, cipherBuf)

	// 去除填充
	resultBuf, err := UnPKCS7Padding(decryptedBuf)
	if err != nil {
		return "", err
	}
	return string(resultBuf), nil
}

// PKCS7Padding pads a copy of text, using the AES block size if blockSize is invalid.
func PKCS7Padding(text []byte, blockSize int) []byte {
	if blockSize <= 0 || blockSize > 255 {
		blockSize = aes.BlockSize
	}
	size := blockSize - len(text)%blockSize
	padded := make([]byte, len(text)+size)
	copy(padded, text)
	for index := len(text); index < len(padded); index++ {
		padded[index] = byte(size)
	}
	return padded
}

// UnPKCS7Padding PKCS7 去除填充方式
func UnPKCS7Padding(text []byte) ([]byte, error) {
	if len(text) == 0 || len(text)%aes.BlockSize != 0 {
		return nil, errInvalidPKCS7Padding
	}
	lastBlock := text[len(text)-aes.BlockSize:]
	padding := int(lastBlock[aes.BlockSize-1])
	valid := subtle.ConstantTimeLessOrEq(1, padding) & subtle.ConstantTimeLessOrEq(padding, aes.BlockSize)
	for index, value := range lastBlock {
		isPadding := subtle.ConstantTimeLessOrEq(aes.BlockSize-index, padding)
		matches := subtle.ConstantTimeByteEq(value, byte(padding))
		valid &= subtle.ConstantTimeSelect(isPadding, matches, 1)
	}
	if valid != 1 {
		return nil, errInvalidPKCS7Padding
	}
	return text[:len(text)-padding], nil
}

// IsBase64 判断字符串是否是base64，只能判断一定不是，目前条件下足够了
func IsBase64(text string) bool {
	if len(text)%4 != 0 || !base64Regex.MatchString(text) {
		return false
	}
	decodedText, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return false
	}
	encodedText := base64.StdEncoding.EncodeToString(decodedText)

	return text == encodedText
}
