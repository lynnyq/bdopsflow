package rsautil

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

type RSAUtil struct {
	publicKey     *rsa.PublicKey
	privateKey    *rsa.PrivateKey
	publicKeyB64  string
	privateKeyB64 string
}

func NewFromConfig(publicKeyB64, privateKeyB64 string) (*RSAUtil, error) {
	u := &RSAUtil{}

	if publicKeyB64 == "" {
		return nil, fmt.Errorf("RSA公钥不能为空")
	}

	pubKey, err := parsePublicKey(publicKeyB64)
	if err != nil {
		return nil, fmt.Errorf("解析RSA公钥失败: %w", err)
	}
	u.publicKey = pubKey
	u.publicKeyB64 = publicKeyB64

	if privateKeyB64 != "" {
		privKey, err := parsePrivateKey(privateKeyB64)
		if err != nil {
			return nil, fmt.Errorf("解析RSA私钥失败: %w", err)
		}
		u.privateKey = privKey
		u.privateKeyB64 = privateKeyB64
	}

	return u, nil
}

func (u *RSAUtil) PublicKeyB64() string {
	return u.publicKeyB64
}

func (u *RSAUtil) HasPrivateKey() bool {
	return u.privateKey != nil
}

func (u *RSAUtil) Encrypt(plaintext string) (string, error) {
	ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, u.publicKey, []byte(plaintext))
	if err != nil {
		return "", fmt.Errorf("RSA加密失败: %w", err)
	}
	return hex.EncodeToString(ciphertext), nil
}

// EncryptLarge encrypts large plaintext using AES-GCM with a random key,
// then encrypts the AES key with RSA. Output format: hex(rsa_encrypted_aes_key) + "." + base64(aes_ciphertext).
// This overcomes the RSA plaintext size limit (~245 bytes for 2048-bit keys).
func (u *RSAUtil) EncryptLarge(plaintext string) (string, error) {
	// Generate random AES-256 key
	aesKey := make([]byte, 32)
	if _, err := rand.Read(aesKey); err != nil {
		return "", fmt.Errorf("生成AES密钥失败: %w", err)
	}

	// Encrypt AES key with RSA
	encryptedAESKey, err := rsa.EncryptPKCS1v15(rand.Reader, u.publicKey, aesKey)
	if err != nil {
		return "", fmt.Errorf("RSA加密AES密钥失败: %w", err)
	}

	// Encrypt plaintext with AES-GCM
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", fmt.Errorf("创建AES cipher失败: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建AES-GCM失败: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("生成nonce失败: %w", err)
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)

	// Format: hex(rsa_encrypted_aes_key) + "." + base64(aes_gcm_ciphertext)
	return hex.EncodeToString(encryptedAESKey) + "." + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptLarge decrypts ciphertext produced by EncryptLarge.
func (u *RSAUtil) DecryptLarge(ciphertext string) (string, error) {
	if u.privateKey == nil {
		return "", fmt.Errorf("RSA私钥未配置，无法解密")
	}

	parts := strings.SplitN(ciphertext, ".", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("密文格式无效")
	}

	// Decrypt AES key with RSA
	encryptedAESKey, err := hex.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("RSA密文hex解码失败: %w", err)
	}

	aesKey, err := rsa.DecryptPKCS1v15(rand.Reader, u.privateKey, encryptedAESKey)
	if err != nil {
		return "", fmt.Errorf("RSA解密AES密钥失败: %w", err)
	}

	// Decrypt ciphertext with AES-GCM
	ciphertextBytes, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("AES密文base64解码失败: %w", err)
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", fmt.Errorf("创建AES cipher失败: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建AES-GCM失败: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertextBytes) < nonceSize {
		return "", fmt.Errorf("AES密文长度不足")
	}

	nonce, ciphertextData := ciphertextBytes[:nonceSize], ciphertextBytes[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertextData, nil)
	if err != nil {
		return "", fmt.Errorf("AES-GCM解密失败: %w", err)
	}

	return string(plaintext), nil
}

func (u *RSAUtil) Decrypt(ciphertextHex string) (string, error) {
	if u.privateKey == nil {
		return "", fmt.Errorf("RSA私钥未配置，无法解密")
	}

	ciphertext, err := hex.DecodeString(ciphertextHex)
	if err != nil {
		return "", fmt.Errorf("RSA密文hex解码失败: %w", err)
	}

	plaintext, err := rsa.DecryptPKCS1v15(rand.Reader, u.privateKey, ciphertext)
	if err != nil {
		return "", fmt.Errorf("RSA解密失败: %w", err)
	}
	return string(plaintext), nil
}

func (u *RSAUtil) DecryptConfigPassword(password string) (string, error) {
	if !strings.HasPrefix(password, "RSA_ENCRYPTED:") {
		return password, nil
	}
	ciphertext := strings.TrimPrefix(password, "RSA_ENCRYPTED:")
	plaintext, err := u.Decrypt(ciphertext)
	if err != nil {
		return "", fmt.Errorf("配置密码解密失败: %w", err)
	}
	return plaintext, nil
}

func GenerateKeyPair() (publicKeyB64, privateKeyB64 string, err error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("生成RSA密钥对失败: %w", err)
	}

	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("序列化公钥失败: %w", err)
	}

	privKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", "", fmt.Errorf("序列化私钥失败: %w", err)
	}

	publicKeyB64 = base64.StdEncoding.EncodeToString(pubKeyBytes)
	privateKeyB64 = base64.StdEncoding.EncodeToString(privKeyBytes)

	return publicKeyB64, privateKeyB64, nil
}

func parsePublicKey(b64 string) (*rsa.PublicKey, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("Base64解码失败: %w", err)
	}

	pub, err := x509.ParsePKIXPublicKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("PKCS#8公钥解析失败: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("密钥类型不是RSA公钥")
	}

	return rsaPub, nil
}

func parsePrivateKey(b64 string) (*rsa.PrivateKey, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("Base64解码失败: %w", err)
	}

	key, err := x509.ParsePKCS8PrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("PKCS#8私钥解析失败: %w", err)
	}

	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("密钥类型不是RSA私钥")
	}

	return rsaKey, nil
}
