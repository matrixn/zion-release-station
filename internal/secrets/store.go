package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Store encrypts small runtime secrets with the package-scoped master key.
// The key and ciphertext live below the package data directory and are never
// returned by an API handler.
type Store struct {
	dataDir   string
	masterKey string
}

func NewStore(dataDir string) *Store {
	return &Store{dataDir: dataDir, masterKey: filepath.Join(dataDir, "master.key")}
}

func (s *Store) Seal(plain []byte) ([]byte, error) {
	key, err := s.key()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create secret cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create secret encryption: %w", err)
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate secret nonce: %w", err)
	}
	return append(nonce, aead.Seal(nil, nonce, plain, nil)...), nil
}

func (s *Store) Open(sealed []byte) ([]byte, error) {
	key, err := s.key()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create secret cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create secret encryption: %w", err)
	}
	if len(sealed) < aead.NonceSize() {
		return nil, errors.New("encrypted secret is truncated")
	}
	nonce, ciphertext := sealed[:aead.NonceSize()], sealed[aead.NonceSize():]
	plain, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt secret: %w", err)
	}
	return plain, nil
}

func (s *Store) key() ([]byte, error) {
	key, err := os.ReadFile(s.masterKey)
	if errors.Is(err, os.ErrNotExist) {
		key = make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			return nil, fmt.Errorf("generate secret master key: %w", err)
		}
		if err := os.MkdirAll(s.dataDir, 0o700); err != nil {
			return nil, fmt.Errorf("create secret data directory: %w", err)
		}
		if err := os.WriteFile(s.masterKey, key, 0o600); err != nil {
			return nil, fmt.Errorf("store secret master key: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("read secret master key: %w", err)
	}
	if len(key) != 32 {
		return nil, errors.New("secret master key must be 32 bytes")
	}
	return key, nil
}
