package webstation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/matrixn/zion-release-station/internal/detection"
	"github.com/matrixn/zion-release-station/internal/pathsecurity"
	"github.com/matrixn/zion-release-station/internal/permissions"
)

type DiscoveredSite struct {
	Name           string                    `json:"name"`
	Hostname       string                    `json:"hostname"`
	ProjectRoot    string                    `json:"project_root"`
	WebRoot        string                    `json:"web_root"`
	Framework      string                    `json:"framework"`
	Detection      detection.DetectionResult `json:"detection"`
	Permissions    permissions.Report        `json:"permissions"`
	Source         string                    `json:"source"`
	AlreadyManaged bool                      `json:"already_managed"`
}

type WebStationAdapter interface {
	Available(context.Context) (bool, error)
	Discover(context.Context) ([]DiscoveredSite, error)
}

type FilesystemAdapter struct {
	roots    []string
	detector detection.FrameworkDetector
}

func NewFilesystemAdapter(roots []string, detector detection.FrameworkDetector) *FilesystemAdapter {
	return &FilesystemAdapter{roots: append([]string(nil), roots...), detector: detector}
}

func (a *FilesystemAdapter) Available(_ context.Context) (bool, error) {
	for _, root := range a.roots {
		info, err := os.Stat(root)
		if err == nil && info.IsDir() {
			return true, nil
		}
		if err != nil && !os.IsNotExist(err) {
			return false, fmt.Errorf("inspect Web Station root %q: %w", root, err)
		}
	}
	return false, nil
}

func (a *FilesystemAdapter) Discover(ctx context.Context) ([]DiscoveredSite, error) {
	seen := make(map[string]struct{})
	var discovered []DiscoveredSite
	var firstPermissionError error
	readableRoot := false
	for _, root := range a.roots {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entries, err := os.ReadDir(root)
		if os.IsNotExist(err) {
			continue
		}
		if os.IsPermission(err) {
			if firstPermissionError == nil {
				firstPermissionError = err
			}
			// Web Station may contain shared folders that are intentionally not
			// readable by the package user. Continue with the other configured
			// roots instead of failing the complete read-only discovery.
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read Web Station root %q: %w", root, err)
		}
		readableRoot = true
		for _, entry := range entries {
			if isReservedEntryName(entry.Name()) {
				continue
			}
			candidate := filepath.Join(root, entry.Name())
			info, err := os.Lstat(candidate)
			if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				continue
			}
			canonical, err := pathsecurity.CanonicalDirectory(candidate)
			if err != nil {
				continue
			}
			if _, ok := seen[canonical]; ok {
				continue
			}
			seen[canonical] = struct{}{}

			result, err := a.detector.Detect(canonical)
			if err != nil {
				return nil, fmt.Errorf("detect framework for %q: %w", canonical, err)
			}
			webRoot := canonical
			if result.DocumentRoot != "" {
				candidateRoot := filepath.Join(canonical, filepath.FromSlash(result.DocumentRoot))
				if resolved, resolveErr := pathsecurity.CanonicalDirectory(candidateRoot); resolveErr == nil && pathsecurity.IsWithin(canonical, resolved) {
					webRoot = resolved
				}
			}
			permission, err := permissions.Check(webRoot)
			if err != nil {
				return nil, fmt.Errorf("check permissions for %q: %w", webRoot, err)
			}
			discovered = append(discovered, DiscoveredSite{
				Name:        entry.Name(),
				Hostname:    entry.Name(),
				ProjectRoot: canonical,
				WebRoot:     webRoot,
				Framework:   result.Framework,
				Detection:   result,
				Permissions: permission,
				Source:      "filesystem-read-only",
			})
		}
	}
	if !readableRoot && firstPermissionError != nil {
		return nil, fmt.Errorf("read Web Station roots: %w", firstPermissionError)
	}
	sort.Slice(discovered, func(i, j int) bool { return discovered[i].Hostname < discovered[j].Hostname })
	return discovered, nil
}

// Synology uses marker-prefixed directories for metadata, recycle bins,
// snapshots and package internals. They are not hosted sites and must never
// be offered for import.
func isReservedEntryName(name string) bool {
	return name == "" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "@") || strings.HasPrefix(name, "#")
}
