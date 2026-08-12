package detection

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryDetectsSupportedFrameworks(t *testing.T) {
	tests := []struct {
		name      string
		files     []string
		framework string
		document  string
	}{
		{"Laravel", []string{"artisan", "composer.json", "public/index.php"}, "laravel", "public"},
		{"Symfony", []string{"bin/console", "composer.json", "public/index.php"}, "symfony", "public"},
		{"WordPress", []string{"wp-config.php", "wp-admin/index.php", "wp-content/index.php"}, "wordpress", ""},
		{"Flarum", []string{"flarum", "composer.json", "public/index.php"}, "flarum", "public"},
		{"Node", []string{"package.json"}, "node", ""},
		{"PHP", []string{"composer.json"}, "php", ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			for _, relative := range test.files {
				path := filepath.Join(root, filepath.FromSlash(relative))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
					t.Fatalf("write: %v", err)
				}
			}
			result, err := (Registry{}).Detect(root)
			if err != nil {
				t.Fatalf("detect: %v", err)
			}
			if result.Framework != test.framework || result.DocumentRoot != test.document {
				t.Fatalf("unexpected result: %#v", result)
			}
		})
	}
}

func TestRegistryReturnsUnknownForEmptyDirectory(t *testing.T) {
	result, err := (Registry{}).Detect(t.TempDir())
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if result.Framework != "unknown" {
		t.Fatalf("expected unknown framework, got %q", result.Framework)
	}
}
