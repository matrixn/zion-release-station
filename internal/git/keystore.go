package git

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrDeployKeyNotFound = errors.New("deploy key has not been generated")

// KeyStore keeps the deploy key encrypted at rest. The temporary private-key
// file exists only for the lifetime of a Git command and is never returned by
// an API handler.
type KeyStore struct {
	root       string
	masterPath string
	keyPath    string
}

func NewKeyStore(dataDir string) *KeyStore {
	root := filepath.Join(dataDir, "git", "keys")
	return &KeyStore{
		root:       root,
		masterPath: filepath.Join(dataDir, "master.key"),
		keyPath:    filepath.Join(root, "deploy-key.enc"),
	}
}

func (s *KeyStore) Generate() (string, error) {
	if err := s.ensureDirectories(); err != nil {
		return "", err
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", fmt.Errorf("generate ed25519 deploy key: %w", err)
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", fmt.Errorf("encode deploy key: %w", err)
	}
	sealed, err := s.seal(encoded)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(s.keyPath, sealed, 0o600); err != nil {
		return "", fmt.Errorf("store deploy key: %w", err)
	}
	return authorizedKey(publicKey), nil
}

func (s *KeyStore) PublicKey() (string, error) {
	privateKey, err := s.privateKey()
	if err != nil {
		return "", err
	}
	publicKey, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok {
		return "", fmt.Errorf("stored deploy key is not ed25519")
	}
	return authorizedKey(publicKey), nil
}

func (s *KeyStore) Fingerprint() (string, error) {
	privateKey, err := s.privateKey()
	if err != nil {
		return "", err
	}
	publicKey, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok {
		return "", fmt.Errorf("stored deploy key is not ed25519")
	}
	digest := sha256.Sum256(publicKey)
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(digest[:]), nil
}

// TemporaryPrivateKey creates a 0600 PKCS#8 key file for one Git command.
// The caller must invoke the returned cleanup function immediately after the
// command exits.
func (s *KeyStore) TemporaryPrivateKey() (string, func(), error) {
	privateKey, err := s.privateKey()
	if err != nil {
		return "", func() {}, err
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", func() {}, fmt.Errorf("encode temporary deploy key: %w", err)
	}
	file, err := os.CreateTemp(s.root, "deploy-key-*.pem")
	if err != nil {
		return "", func() {}, fmt.Errorf("create temporary deploy key: %w", err)
	}
	name := file.Name()
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(name)
	}
	if err := file.Chmod(0o600); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("protect temporary deploy key: %w", err)
	}
	if err := pem.Encode(file, &pem.Block{Type: "PRIVATE KEY", Bytes: encoded}); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("write temporary deploy key: %w", err)
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("close temporary deploy key: %w", err)
	}
	return name, cleanup, nil
}

func (s *KeyStore) ensureDirectories() error {
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return fmt.Errorf("create Git key directory: %w", err)
	}
	if info, err := os.Stat(s.root); err == nil && info.Mode().Perm()&0o077 != 0 {
		if err := os.Chmod(s.root, 0o700); err != nil {
			return fmt.Errorf("protect Git key directory: %w", err)
		}
	}
	return nil
}

func (s *KeyStore) privateKey() (ed25519.PrivateKey, error) {
	sealed, err := os.ReadFile(s.keyPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrDeployKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read deploy key: %w", err)
	}
	plain, err := s.open(sealed)
	if err != nil {
		return nil, err
	}
	key, err := x509.ParsePKCS8PrivateKey(plain)
	if err != nil {
		return nil, fmt.Errorf("parse deploy key: %w", err)
	}
	privateKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("stored deploy key is not ed25519")
	}
	return privateKey, nil
}

func (s *KeyStore) masterKey() ([]byte, error) {
	key, err := os.ReadFile(s.masterPath)
	if errors.Is(err, os.ErrNotExist) {
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("generate Git encryption key: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(s.masterPath), 0o700); err != nil {
			return nil, fmt.Errorf("create encryption key directory: %w", err)
		}
		if err := os.WriteFile(s.masterPath, key, 0o600); err != nil {
			return nil, fmt.Errorf("store Git encryption key: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("read Git encryption key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("Git encryption key must be 32 bytes")
	}
	return key, nil
}

func (s *KeyStore) seal(plain []byte) ([]byte, error) {
	key, err := s.masterKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create Git encryption cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create Git encryption: %w", err)
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate Git encryption nonce: %w", err)
	}
	return append(nonce, aead.Seal(nil, nonce, plain, nil)...), nil
}

func (s *KeyStore) open(sealed []byte) ([]byte, error) {
	key, err := s.masterKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create Git encryption cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create Git encryption: %w", err)
	}
	if len(sealed) < aead.NonceSize() {
		return nil, fmt.Errorf("stored deploy key is truncated")
	}
	nonce, ciphertext := sealed[:aead.NonceSize()], sealed[aead.NonceSize():]
	plain, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt deploy key: %w", err)
	}
	return plain, nil
}

func authorizedKey(publicKey ed25519.PublicKey) string {
	var encoded []byte
	appendString := func(value []byte) {
		var length [4]byte
		binary.BigEndian.PutUint32(length[:], uint32(len(value)))
		encoded = append(encoded, length[:]...)
		encoded = append(encoded, value...)
	}
	appendString([]byte("ssh-ed25519"))
	appendString(publicKey)
	return "ssh-ed25519 " + base64.StdEncoding.EncodeToString(encoded) + " zion-releasestation"
}

func keyFingerprint(publicKey string) string {
	return fmt.Sprintf("SHA256:%x", sha256.Sum256([]byte(publicKey)))
}
