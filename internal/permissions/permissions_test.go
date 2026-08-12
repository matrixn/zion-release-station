package permissions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckReportsReadyDirectory(t *testing.T) {
	path := t.TempDir()
	report, err := Check(path)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if report.Status != "ready" || !report.Readable || !report.Writable || !report.Deployable {
		t.Fatalf("unexpected report: %#v", report)
	}
}

func TestCheckReportsMissingDirectory(t *testing.T) {
	report, err := Check(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if report.Status != "missing" || report.Deployable {
		t.Fatalf("unexpected report: %#v", report)
	}
}

func TestCheckReportsReadOnlyDirectoryWhenSupported(t *testing.T) {
	path := t.TempDir()
	if err := os.Chmod(path, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	if os.Getuid() == 0 {
		t.Skip("root can write read-only directories")
	}
	report, err := Check(path)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if report.Status != "read_only" || report.Deployable {
		t.Fatalf("unexpected report: %#v", report)
	}
}
