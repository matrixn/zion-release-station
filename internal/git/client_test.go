package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRepositoryAcceptsSSHAndHTTPSRemotes(t *testing.T) {
	sshRemote, err := ValidateRepository("git@github.com:matrixn/zion-release-station.git", "main")
	if err != nil {
		t.Fatalf("validate SSH remote: %v", err)
	}
	if sshRemote.Transport != "ssh" || sshRemote.Host != "github.com" {
		t.Fatalf("unexpected SSH remote: %#v", sshRemote)
	}
	httpsRemote, err := ValidateRepository("https://github.com/matrixn/zion-release-station.git", "feature/deploy")
	if err != nil {
		t.Fatalf("validate HTTPS remote: %v", err)
	}
	if httpsRemote.Transport != "https" || httpsRemote.Host != "github.com" {
		t.Fatalf("unexpected HTTPS remote: %#v", httpsRemote)
	}
}

func TestValidateRepositoryRejectsShellInputAndInvalidBranches(t *testing.T) {
	for _, test := range []struct {
		remote string
		branch string
	}{
		{remote: "https://github.com/org/repo.git;touch /tmp/pwned", branch: "main"},
		{remote: "https://user:password@github.com/org/repo.git", branch: "main"},
		{remote: "file:///tmp/repository", branch: "main"},
		{remote: "https://github.com/org/repo.git", branch: "../../main"},
		{remote: "https://github.com/org/repo.git", branch: "main..next"},
	} {
		if _, err := ValidateRepository(test.remote, test.branch); err == nil {
			t.Fatalf("expected validation error for %#v", test)
		}
	}
}

func TestKeyStoreEncryptsPrivateKeyAndReturnsOpenSSHPublicKey(t *testing.T) {
	dataDir := t.TempDir()
	store := NewKeyStore(dataDir)
	publicKey, err := store.Generate()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	if !strings.HasPrefix(publicKey, "ssh-ed25519 ") {
		t.Fatalf("unexpected public key: %q", publicKey)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "git", "keys", "deploy-key.enc")); err != nil {
		t.Fatalf("encrypted key missing: %v", err)
	}
	privateBytes, err := os.ReadFile(filepath.Join(dataDir, "git", "keys", "deploy-key.enc"))
	if err != nil {
		t.Fatalf("read encrypted key: %v", err)
	}
	if strings.Contains(string(privateBytes), "PRIVATE KEY") {
		t.Fatal("private key is stored in plaintext")
	}
	readBack, err := store.PublicKey()
	if err != nil || readBack != publicKey {
		t.Fatalf("read public key: %q, %v", readBack, err)
	}
	keyPath, cleanup, err := store.TemporaryPrivateKey()
	if err != nil {
		t.Fatalf("temporary private key: %v", err)
	}
	if info, err := os.Stat(keyPath); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("temporary key permissions: %v, %v", info, err)
	}
	cleanup()
	if _, err := os.Stat(keyPath); !os.IsNotExist(err) {
		t.Fatalf("temporary key was not removed: %v", err)
	}
}

func TestClientTestUsesArgumentSeparatedGitCommand(t *testing.T) {
	client := NewClient(t.TempDir())
	if _, err := client.KeyStore.Generate(); err != nil {
		t.Fatalf("generate test key: %v", err)
	}
	var captured []string
	client.CommandRunner = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		captured = append([]string{name}, args...)
		return exec.CommandContext(ctx, "true")
	}
	if err := client.Test(context.Background(), "git@github.com:org/repo.git", "main"); err != nil {
		t.Fatalf("test remote: %v", err)
	}
	joined := strings.Join(captured, "\x00")
	if !strings.Contains(joined, "ls-remote") || !strings.Contains(joined, "refs/heads/main") {
		t.Fatalf("unexpected git arguments: %#v", captured)
	}
}
