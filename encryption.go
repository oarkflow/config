package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const EncryptedPrefix = "enc:"

type Encryptor interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
	Name() string
}

type AESGCMEncryptor struct {
	key []byte
}

func NewAESGCMEncryptor(key []byte) (*AESGCMEncryptor, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, errors.New("config: AES key must be 16, 24, or 32 bytes")
	}
	return &AESGCMEncryptor{key: key}, nil
}

func NewAESGCMEncryptorFromPassphrase(passphrase string) *AESGCMEncryptor {
	h := sha256.Sum256([]byte(passphrase))
	return &AESGCMEncryptor{key: h[:]}
}

func (e *AESGCMEncryptor) Name() string { return "aes256-gcm" }

func (e *AESGCMEncryptor) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func (e *AESGCMEncryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("config: ciphertext too short")
	}
	return gcm.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], nil)
}

func EncodeEncrypted(data []byte, algo string) string {
	return EncryptedPrefix + algo + ":" + base64.RawStdEncoding.EncodeToString(data)
}

func IsEncryptedValue(v any) bool {
	s, ok := v.(string)
	return ok && strings.HasPrefix(s, EncryptedPrefix)
}

func DecodeEncrypted(s string) (algo string, data []byte, err error) {
	if !strings.HasPrefix(s, EncryptedPrefix) {
		return "", nil, fmt.Errorf("config: not an encrypted value")
	}
	rest := s[len(EncryptedPrefix):]
	colon := strings.IndexByte(rest, ':')
	if colon < 0 {
		return "", nil, fmt.Errorf("config: malformed encrypted value")
	}
	algo = rest[:colon]
	data, err = base64.RawStdEncoding.DecodeString(rest[colon+1:])
	return
}
