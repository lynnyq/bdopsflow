package rsautil

import (
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
