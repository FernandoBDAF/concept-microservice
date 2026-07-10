package config

import (
	"testing"
)

func TestEncryptor(t *testing.T) {
	// Create a 32-byte key
	key := "12345678901234567890123456789012"

	// Create encryptor
	encryptor, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// Test encryption and decryption
	plaintext := "sensitive-data"
	encrypted, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	decrypted, err := encryptor.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("Expected %s, got %s", plaintext, decrypted)
	}
}

func TestEncryptor_InvalidKey(t *testing.T) {
	// Test with invalid key length
	key := "invalid-key"
	_, err := NewEncryptor(key)
	if err != ErrInvalidKeyLength {
		t.Errorf("Expected ErrInvalidKeyLength, got %v", err)
	}
}

func TestEncryptor_InvalidData(t *testing.T) {
	// Create encryptor
	key := "12345678901234567890123456789012"
	encryptor, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// Test with invalid data
	_, err = encryptor.Decrypt("invalid-data")
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestEncryptor_Config(t *testing.T) {
	// Create a 32-byte key
	key := "12345678901234567890123456789012"

	// Create encryptor
	encryptor, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// Create test config
	config := &Config{
		Environment: "test",
		Version:     "1.0.0",
		Settings: map[string]interface{}{
			"password": "secret-password",
			"api_key":  "secret-api-key",
			"port":     8080,
		},
	}

	// Define sensitive keys
	sensitiveKeys := []string{"password", "api_key"}

	// Encrypt sensitive values
	if err := encryptor.EncryptConfig(config, sensitiveKeys); err != nil {
		t.Fatalf("Failed to encrypt config: %v", err)
	}

	// Verify encrypted values
	for _, key := range sensitiveKeys {
		value, ok := config.Settings[key].(string)
		if !ok {
			t.Errorf("Expected string value for %s", key)
			continue
		}

		// Try to decrypt
		decrypted, err := encryptor.Decrypt(value)
		if err != nil {
			t.Errorf("Failed to decrypt %s: %v", key, err)
			continue
		}

		// Verify decrypted value
		switch key {
		case "password":
			if decrypted != "secret-password" {
				t.Errorf("Expected 'secret-password', got %s", decrypted)
			}
		case "api_key":
			if decrypted != "secret-api-key" {
				t.Errorf("Expected 'secret-api-key', got %s", decrypted)
			}
		}
	}

	// Verify non-sensitive value
	if port, ok := config.Settings["port"].(int); !ok || port != 8080 {
		t.Errorf("Expected port 8080, got %v", config.Settings["port"])
	}

	// Decrypt config
	if err := encryptor.DecryptConfig(config, sensitiveKeys); err != nil {
		t.Fatalf("Failed to decrypt config: %v", err)
	}

	// Verify decrypted values
	if password, ok := config.Settings["password"].(string); !ok || password != "secret-password" {
		t.Errorf("Expected 'secret-password', got %v", config.Settings["password"])
	}

	if apiKey, ok := config.Settings["api_key"].(string); !ok || apiKey != "secret-api-key" {
		t.Errorf("Expected 'secret-api-key', got %v", config.Settings["api_key"])
	}
}
