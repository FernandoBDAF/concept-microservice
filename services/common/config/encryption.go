package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

var (
	ErrInvalidKeyLength = errors.New("invalid key length: must be 32 bytes")
	ErrInvalidData      = errors.New("invalid data")
)

// Encryptor handles configuration encryption and decryption
type Encryptor struct {
	key []byte
}

// NewEncryptor creates a new configuration encryptor
func NewEncryptor(key string) (*Encryptor, error) {
	// Convert key to bytes
	keyBytes := []byte(key)

	// Validate key length
	if len(keyBytes) != 32 {
		return nil, ErrInvalidKeyLength
	}

	return &Encryptor{
		key: keyBytes,
	}, nil
}

// Encrypt encrypts a value
func (e *Encryptor) Encrypt(value string) (string, error) {
	// Create cipher block
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Create nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt data
	ciphertext := gcm.Seal(nonce, nonce, []byte(value), nil)

	// Encode to base64
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a value
func (e *Encryptor) Decrypt(value string) (string, error) {
	// Decode from base64
	ciphertext, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	// Create cipher block
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Check ciphertext length
	if len(ciphertext) < gcm.NonceSize() {
		return "", ErrInvalidData
	}

	// Extract nonce
	nonce := ciphertext[:gcm.NonceSize()]
	ciphertext = ciphertext[gcm.NonceSize():]

	// Decrypt data
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// EncryptConfig encrypts sensitive configuration values
func (e *Encryptor) EncryptConfig(config *Config, sensitiveKeys []string) error {
	for _, key := range sensitiveKeys {
		if value, ok := config.Settings[key].(string); ok {
			encrypted, err := e.Encrypt(value)
			if err != nil {
				return fmt.Errorf("failed to encrypt %s: %w", key, err)
			}
			config.Settings[key] = encrypted
		}
	}
	return nil
}

// DecryptConfig decrypts sensitive configuration values
func (e *Encryptor) DecryptConfig(config *Config, sensitiveKeys []string) error {
	for _, key := range sensitiveKeys {
		if value, ok := config.Settings[key].(string); ok {
			decrypted, err := e.Decrypt(value)
			if err != nil {
				return fmt.Errorf("failed to decrypt %s: %w", key, err)
			}
			config.Settings[key] = decrypted
		}
	}
	return nil
}
