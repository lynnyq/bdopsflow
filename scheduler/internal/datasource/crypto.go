package datasource

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"sync"
)

type Crypto struct {
	currentKey []byte
	oldKeys    [][]byte
	mutex      sync.RWMutex
}

func NewCrypto(key string) (*Crypto, error) {
	keyBytes := []byte(key)
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(keyBytes))
	}
	return &Crypto{
		currentKey: keyBytes,
		oldKeys:    nil,
	}, nil
}

func (c *Crypto) Encrypt(plaintext string) (string, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	block, err := aes.NewCipher(c.currentKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (c *Crypto) Decrypt(ciphertext string) (string, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	plain, err := c.decryptWithKey(c.currentKey, data)
	if err == nil {
		return plain, nil
	}

	for _, key := range c.oldKeys {
		plain, err = c.decryptWithKey(key, data)
		if err == nil {
			return plain, nil
		}
	}

	return "", fmt.Errorf("all keys failed to decrypt")
}

func (c *Crypto) decryptWithKey(key []byte, data []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func (c *Crypto) RotateKey(newKey string) error {
	keyBytes := []byte(newKey)
	if len(keyBytes) != 32 {
		return fmt.Errorf("encryption key must be 32 bytes, got %d", len(keyBytes))
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.currentKey != nil {
		c.oldKeys = append([][]byte{c.currentKey}, c.oldKeys...)
	}
	c.currentKey = keyBytes
	return nil
}

func GenerateEncryptionKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}
