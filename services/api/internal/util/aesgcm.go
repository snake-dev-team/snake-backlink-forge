package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
)

var ErrEncKeyMissing = errors.New("encryption key missing")

func EncryptAESGCM(masterHex string, plaintext []byte) ([]byte, error) {
	gcm, err := newAESGCM(masterHex)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("aesgcm: nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func DecryptAESGCM(masterHex string, ciphertext []byte) ([]byte, error) {
	gcm, err := newAESGCM(masterHex)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("aesgcm: ciphertext too short")
	}
	return gcm.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], nil)
}

func newAESGCM(masterHex string) (cipher.AEAD, error) {
	if masterHex == "" {
		return nil, ErrEncKeyMissing
	}
	key, err := hex.DecodeString(masterHex)
	if err != nil {
		return nil, fmt.Errorf("aesgcm: decode key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("aesgcm: key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aesgcm: cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("aesgcm: gcm: %w", err)
	}
	return gcm, nil
}
