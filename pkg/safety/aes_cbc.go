package safety

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"regexp"
)

// CBCEncrypter is used to encryption with cbc mode
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

// CBCDecrypter is used to decryption with cbc mode
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
		return "", fmt.Errorf("CBC ciphertext must contain complete nonempty blocks")
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

// PKCS7Padding PKCS7 填充方式
func PKCS7Padding(text []byte, blockSize int) []byte {
	// 计算待填充的长度
	size := blockSize - len(text)%blockSize
	var paddingText []byte
	if size == 0 {
		// 已对齐，填充一整块数据，每个数据为 blockSize
		paddingText = bytes.Repeat([]byte{byte(blockSize)}, blockSize)
	} else {
		// 未对齐 填充 padding 个数据，每个数据为 padding
		paddingText = bytes.Repeat([]byte{byte(size)}, size)
	}
	return append(text, paddingText...)
}

// UnPKCS7Padding PKCS7 去除填充方式
func UnPKCS7Padding(text []byte) ([]byte, error) {
	if len(text) == 0 || len(text)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("invalid PKCS7 padded length")
	}
	padding := int(text[len(text)-1])
	if padding == 0 || padding > aes.BlockSize || padding > len(text) {
		return nil, fmt.Errorf("invalid PKCS7 padding length")
	}
	for _, value := range text[len(text)-padding:] {
		if int(value) != padding {
			return nil, fmt.Errorf("invalid PKCS7 padding bytes")
		}
	}
	return text[:len(text)-padding], nil
}

// IsBase64 判断字符串是否是base64，只能判断一定不是，目前条件下足够了
func IsBase64(text string) bool {
	pattern := "^([A-Za-z0-9+/]{4})*([A-Za-z0-9+/]{4}|[A-Za-z0-9+/]{3}=|[A-Za-z0-9+/]{2}==)$"
	matched, err := regexp.MatchString(pattern, text)
	if err != nil {
		return false
	}
	if !(len(text)%4 == 0 && matched) {
		return false
	}
	decodedText, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return false
	}
	encodedText := base64.StdEncoding.EncodeToString(decodedText)

	return text == encodedText
}
