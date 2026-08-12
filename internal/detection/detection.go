package detection

import (
	"fmt"
	"os"
	"path/filepath"
)

type DetectionResult struct {
	Framework    string   `json:"framework"`
	Confidence   string   `json:"confidence"`
	Evidence     []string `json:"evidence"`
	DocumentRoot string   `json:"document_root"`
}

type FrameworkDetector interface {
	Detect(root string) (DetectionResult, error)
}

type Registry struct{}

func (Registry) Detect(root string) (DetectionResult, error) {
	if root == "" {
		return DetectionResult{}, fmt.Errorf("project root is required")
	}
	if info, err := os.Stat(root); err != nil {
		return DetectionResult{}, fmt.Errorf("stat project root: %w", err)
	} else if !info.IsDir() {
		return DetectionResult{}, fmt.Errorf("project root is not a directory")
	}

	tests := []struct {
		framework string
		document  string
		evidence  []string
	}{
		{"wordpress", "", []string{"wp-config.php", "wp-admin", "wp-content"}},
		{"laravel", "public", []string{"artisan", "composer.json", "public/index.php"}},
		{"symfony", "public", []string{"bin/console", "composer.json", "public/index.php"}},
		{"flarum", "public", []string{"flarum", "composer.json", "public/index.php"}},
		{"node", "", []string{"package.json"}},
		{"php", "", []string{"composer.json"}},
	}

	for _, test := range tests {
		matched := true
		for _, relative := range test.evidence {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); err != nil {
				matched = false
				break
			}
		}
		if matched {
			return DetectionResult{
				Framework:    test.framework,
				Confidence:   "high",
				Evidence:     append([]string(nil), test.evidence...),
				DocumentRoot: test.document,
			}, nil
		}
	}

	return DetectionResult{Framework: "unknown", Confidence: "low"}, nil
}
