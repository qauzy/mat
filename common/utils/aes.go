package utils

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"io"
	mrand "math/rand"
)

// Encrypt 加密函数
func Encrypt(key []byte, text []byte) ([]byte, error) {
	if len(text) == 0 || text == nil {
		return text, nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	plaintext := PKCS7Padding(text, aes.BlockSize)
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	return ciphertext, nil
}

// Decrypt 解密函数
func Decrypt(key []byte, text []byte) ([]byte, error) {
	if len(text) == 0 || text == nil {
		return text, nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	iv := text[:aes.BlockSize]
	text = text[aes.BlockSize:]

	decrypted := make([]byte, len(text))
	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(decrypted, text)

	decrypted = PKCS7UnPadding(decrypted)

	return decrypted, nil
}

// PKCS7Padding PKCS7填充
func PKCS7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

// PKCS7UnPadding PKCS7去除填充
func PKCS7UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

func IsAESEncrypted(data []byte) bool {
	block, err := aes.NewCipher(data)
	if err != nil {
		return false
	}
	// Check if the size of data is a multiple of the block size
	if len(data)%block.BlockSize() != 0 {
		return false
	}
	return true
}

func MD5(input string) string {
	// 创建MD5哈希对象
	hasher := md5.New()

	// 将字符串转换为字节数组并计算MD5哈希值
	hasher.Write([]byte(input))

	// 获取MD5哈希值的字节数组
	hashBytes := hasher.Sum(nil)

	// 将字节数组转换为十六进制字符串
	hashString := hex.EncodeToString(hashBytes)

	return hashString
}

const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandStr(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[mrand.Intn(len(letters))]
	}
	return string(b)
}
