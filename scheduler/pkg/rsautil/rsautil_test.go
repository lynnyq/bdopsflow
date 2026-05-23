package rsautil

import (
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	pubB64, privB64, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}
	if pubB64 == "" {
		t.Fatal("public key should not be empty")
	}
	if privB64 == "" {
		t.Fatal("private key should not be empty")
	}
}

func TestNewFromConfig(t *testing.T) {
	pubB64, privB64, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	u, err := NewFromConfig(pubB64, privB64)
	if err != nil {
		t.Fatalf("NewFromConfig failed: %v", err)
	}
	if u.PublicKeyB64() != pubB64 {
		t.Fatal("PublicKeyB64 mismatch")
	}
	if !u.HasPrivateKey() {
		t.Fatal("should have private key")
	}
}

func TestNewFromConfig_PublicKeyOnly(t *testing.T) {
	pubB64, _, _ := GenerateKeyPair()

	u, err := NewFromConfig(pubB64, "")
	if err != nil {
		t.Fatalf("NewFromConfig with public key only failed: %v", err)
	}
	if u.HasPrivateKey() {
		t.Fatal("should not have private key")
	}

	plaintext := "testpassword"
	ciphertext, err := u.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if ciphertext == "" {
		t.Fatal("ciphertext should not be empty")
	}

	_, err = u.Decrypt(ciphertext)
	if err == nil {
		t.Fatal("expected error when decrypting without private key")
	}
}

func TestNewFromConfig_EmptyPublicKey(t *testing.T) {
	_, err := NewFromConfig("", "")
	if err == nil {
		t.Fatal("expected error for empty public key")
	}
}

func TestNewFromConfig_InvalidPublicKey(t *testing.T) {
	_, privB64, _ := GenerateKeyPair()
	_, err := NewFromConfig("invalidbase64!!!", privB64)
	if err == nil {
		t.Fatal("expected error for invalid public key")
	}
}

func TestNewFromConfig_InvalidPrivateKey(t *testing.T) {
	pubB64, _, _ := GenerateKeyPair()
	_, err := NewFromConfig(pubB64, "invalidbase64!!!")
	if err == nil {
		t.Fatal("expected error for invalid private key")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	pubB64, privB64, _ := GenerateKeyPair()
	u, _ := NewFromConfig(pubB64, privB64)

	plaintext := "mypassword123"
	ciphertext, err := u.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if ciphertext == "" {
		t.Fatal("ciphertext should not be empty")
	}
	if ciphertext == plaintext {
		t.Fatal("ciphertext should differ from plaintext")
	}

	decrypted, err := u.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("decrypted mismatch: got %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptDecrypt_ChinesePassword(t *testing.T) {
	pubB64, privB64, _ := GenerateKeyPair()
	u, _ := NewFromConfig(pubB64, privB64)

	plaintext := "中文密码测试"
	ciphertext, err := u.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := u.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("decrypted mismatch: got %q, want %q", decrypted, plaintext)
	}
}

func TestDecrypt_InvalidCiphertext(t *testing.T) {
	pubB64, privB64, _ := GenerateKeyPair()
	u, _ := NewFromConfig(pubB64, privB64)

	_, err := u.Decrypt("notvalidhexciphertextzz")
	if err == nil {
		t.Fatal("expected error for invalid ciphertext")
	}
}

func TestEncrypt_ReturnsHex(t *testing.T) {
	pubB64, privB64, _ := GenerateKeyPair()
	u, _ := NewFromConfig(pubB64, privB64)

	ciphertext, err := u.Encrypt("testpassword")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	for _, c := range ciphertext {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Fatalf("ciphertext should be hex encoded, found: %c", c)
		}
	}
}

func TestDecryptConfigPassword_PlainText(t *testing.T) {
	pubB64, privB64, _ := GenerateKeyPair()
	u, _ := NewFromConfig(pubB64, privB64)

	plain := "mypassword"
	result, err := u.DecryptConfigPassword(plain)
	if err != nil {
		t.Fatalf("DecryptConfigPassword failed: %v", err)
	}
	if result != plain {
		t.Fatalf("plaintext should pass through: got %q, want %q", result, plain)
	}
}

func TestDecryptConfigPassword_Encrypted(t *testing.T) {
	pubB64, privB64, _ := GenerateKeyPair()
	u, _ := NewFromConfig(pubB64, privB64)

	plain := "redis_password_123"
	ciphertext, _ := u.Encrypt(plain)

	prefixed := "RSA_ENCRYPTED:" + ciphertext
	result, err := u.DecryptConfigPassword(prefixed)
	if err != nil {
		t.Fatalf("DecryptConfigPassword failed: %v", err)
	}
	if result != plain {
		t.Fatalf("decrypted mismatch: got %q, want %q", result, plain)
	}
}

func TestDecryptConfigPassword_InvalidEncrypted(t *testing.T) {
	pubB64, privB64, _ := GenerateKeyPair()
	u, _ := NewFromConfig(pubB64, privB64)

	prefixed := "RSA_ENCRYPTED:invalidciphertext"
	_, err := u.DecryptConfigPassword(prefixed)
	if err == nil {
		t.Fatal("expected error for invalid encrypted config password")
	}
}

func TestDecryptConfigPassword_EmptyString(t *testing.T) {
	pubB64, privB64, _ := GenerateKeyPair()
	u, _ := NewFromConfig(pubB64, privB64)

	result, err := u.DecryptConfigPassword("")
	if err != nil {
		t.Fatalf("DecryptConfigPassword failed: %v", err)
	}
	if result != "" {
		t.Fatalf("empty string should pass through: got %q", result)
	}
}

func TestEncryptDecrypt_DifferentInstances(t *testing.T) {
	pubB64, privB64, _ := GenerateKeyPair()

	encryptor, _ := NewFromConfig(pubB64, "")
	decryptor, _ := NewFromConfig(pubB64, privB64)

	plaintext := "testpassword"
	ciphertext, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := decryptor.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("decrypted mismatch: got %q, want %q", decrypted, plaintext)
	}
}
