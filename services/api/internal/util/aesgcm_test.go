package util

import (
	"bytes"
	"errors"
	"testing"
)

const testAESKeyHex = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func TestEncryptDecryptAESGCM_RoundTrip(t *testing.T) {
	plaintext := []byte("wp application password")

	ciphertext, err := EncryptAESGCM(testAESKeyHex, plaintext)
	if err != nil {
		t.Fatalf("EncryptAESGCM: %v", err)
	}
	if bytes.Equal(ciphertext, plaintext) {
		t.Fatal("ciphertext must not equal plaintext")
	}

	decrypted, err := DecryptAESGCM(testAESKeyHex, ciphertext)
	if err != nil {
		t.Fatalf("DecryptAESGCM: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptAESGCM_MissingKey(t *testing.T) {
	_, err := EncryptAESGCM("", []byte("secret"))
	if !errors.Is(err, ErrEncKeyMissing) {
		t.Fatalf("err = %v, want ErrEncKeyMissing", err)
	}
}

func TestDecryptAESGCM_RejectsShortCiphertext(t *testing.T) {
	_, err := DecryptAESGCM(testAESKeyHex, []byte("short"))
	if err == nil {
		t.Fatal("expected error")
	}
}
