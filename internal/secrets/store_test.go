package secrets

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreSealsAndOpensSecrets(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "runtime"))
	sealed, err := store.Seal([]byte("webhook-secret"))
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if bytes.Equal(sealed, []byte("webhook-secret")) {
		t.Fatal("secret was not encrypted")
	}
	opened, err := store.Open(sealed)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if string(opened) != "webhook-secret" {
		t.Fatalf("unexpected secret: %q", opened)
	}
	info, err := os.Stat(store.masterKey)
	if err != nil {
		t.Fatalf("stat master key: %v", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("master key is not private: %o", info.Mode().Perm())
	}
}

func TestStoreRejectsTamperedCiphertext(t *testing.T) {
	store := NewStore(t.TempDir())
	sealed, err := store.Seal([]byte("secret"))
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	sealed[len(sealed)-1] ^= 1
	if _, err := store.Open(sealed); err == nil {
		t.Fatal("expected tampered ciphertext to be rejected")
	}
}
