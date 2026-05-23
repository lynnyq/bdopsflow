package datasource

import (
	"strings"
	"testing"
)

func TestNewCrypto_Success(t *testing.T) {
	key := strings.Repeat("a", 32)
	crypto, err := NewCrypto(key)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if crypto == nil {
		t.Fatal("expected crypto instance, got nil")
	}
}

func TestNewCrypto_InvalidKeySize(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"30 bytes", strings.Repeat("a", 30)},
		{"16 bytes", strings.Repeat("a", 16)},
		{"0 bytes", ""},
		{"64 bytes", strings.Repeat("a", 64)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCrypto(tt.key)
			if err == nil {
				t.Errorf("expected error for key size %d, got nil", len(tt.key))
			}
		})
	}
}

func TestCrypto_EncryptDecrypt(t *testing.T) {
	key := strings.Repeat("a", 32)
	crypto, err := NewCrypto(key)
	if err != nil {
		t.Fatalf("failed to create crypto: %v", err)
	}

	plaintext := "hello world"
	encrypted, err := crypto.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}

	if encrypted == plaintext {
		t.Error("encrypted text should not equal plaintext")
	}

	decrypted, err := crypto.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("failed to decrypt: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestCrypto_EncryptDecryptEmptyString(t *testing.T) {
	key := strings.Repeat("a", 32)
	crypto, err := NewCrypto(key)
	if err != nil {
		t.Fatalf("failed to create crypto: %v", err)
	}

	plaintext := ""
	encrypted, err := crypto.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}

	decrypted, err := crypto.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("failed to decrypt: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestCrypto_EncryptDecryptLongString(t *testing.T) {
	key := strings.Repeat("a", 32)
	crypto, err := NewCrypto(key)
	if err != nil {
		t.Fatalf("failed to create crypto: %v", err)
	}

	plaintext := strings.Repeat("x", 10000)
	encrypted, err := crypto.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}

	decrypted, err := crypto.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("failed to decrypt: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("expected %d chars, got %d chars", len(plaintext), len(decrypted))
	}
}

func TestCrypto_DecryptWithRotatedKey(t *testing.T) {
	oldKey := strings.Repeat("a", 32)
	crypto, err := NewCrypto(oldKey)
	if err != nil {
		t.Fatalf("failed to create crypto: %v", err)
	}

	plaintext := "secret data"
	encrypted, err := crypto.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}

	newKey := strings.Repeat("b", 32)
	err = crypto.RotateKey(newKey)
	if err != nil {
		t.Fatalf("failed to rotate key: %v", err)
	}

	decrypted, err := crypto.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("failed to decrypt with rotated key: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestCrypto_RotateKey(t *testing.T) {
	key := strings.Repeat("a", 32)
	crypto, err := NewCrypto(key)
	if err != nil {
		t.Fatalf("failed to create crypto: %v", err)
	}

	newKey := strings.Repeat("b", 32)
	err = crypto.RotateKey(newKey)
	if err != nil {
		t.Fatalf("failed to rotate key: %v", err)
	}

	plaintext := "after rotation"
	encrypted, err := crypto.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("failed to encrypt after rotation: %v", err)
	}

	decrypted, err := crypto.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("failed to decrypt after rotation: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestCrypto_RotateKeyInvalidSize(t *testing.T) {
	key := strings.Repeat("a", 32)
	crypto, err := NewCrypto(key)
	if err != nil {
		t.Fatalf("failed to create crypto: %v", err)
	}

	err = crypto.RotateKey(strings.Repeat("c", 16))
	if err == nil {
		t.Error("expected error for invalid key size, got nil")
	}
}

func TestCrypto_DecryptInvalidCiphertext(t *testing.T) {
	key := strings.Repeat("a", 32)
	crypto, err := NewCrypto(key)
	if err != nil {
		t.Fatalf("failed to create crypto: %v", err)
	}

	_, err = crypto.Decrypt("not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid ciphertext, got nil")
	}

	_, err = crypto.Decrypt("aGVsbG8=")
	if err == nil {
		t.Error("expected error for short ciphertext, got nil")
	}
}

func TestGenerateEncryptionKey(t *testing.T) {
	key, err := GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(key) != 32 {
		t.Errorf("expected key length 32, got %d", len(key))
	}

	key2, err := GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	same := true
	for i := range key {
		if key[i] != key2[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("two generated keys should not be identical")
	}
}
