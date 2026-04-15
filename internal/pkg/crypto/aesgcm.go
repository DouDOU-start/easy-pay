package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// AESGCM encrypts/decrypts arbitrary bytes with AES-256-GCM and returns
// a base64 string. The nonce is prepended to the ciphertext.
type AESGCM struct {
	gcm cipher.AEAD
}

func NewAESGCM(key string) (*AESGCM, error) {
	k := []byte(key)
	if len(k) != 32 {
		return nil, errors.New("crypto: key must be 32 bytes for AES-256")
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &AESGCM{gcm: gcm}, nil
}

func (a *AESGCM) Encrypt(plaintext []byte) (string, error) {
	nonce := make([]byte, a.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := a.gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func (a *AESGCM) Decrypt(ciphertext string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}
	ns := a.gcm.NonceSize()
	if len(raw) < ns {
		return nil, errors.New("crypto: ciphertext too short")
	}
	nonce, data := raw[:ns], raw[ns:]
	return a.gcm.Open(nil, nonce, data, nil)
}
