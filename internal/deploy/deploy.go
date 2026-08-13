package deploy

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/matrixn/zion-release-station/internal/githubconnector"
	"github.com/matrixn/zion-release-station/internal/sites"
)

const maxArchiveSize = 512 * 1024 * 1024

type ArchiveDownloader interface {
	DownloadArchive(ctx context.Context, installationID int64, fullName, ref string, target io.Writer) error
}

type CommitResolver interface {
	ResolveCommit(ctx context.Context, installationID int64, fullName, ref string) (githubconnector.Commit, error)
}

type Runner struct {
	db       *sql.DB
	github   ArchiveDownloader
	resolver CommitResolver
}

type Result struct {
	DeploymentID string `json:"deployment_id"`
	ReleaseID    string `json:"release_id"`
	ReleasePath  string `json:"release_path"`
	Commit       string `json:"commit_sha,omitempty"`
	Status       string `json:"status"`
}

type Deployment struct {
	ID               string `json:"id"`
	SiteID           string `json:"site_id"`
	TriggerType      string `json:"trigger_type"`
	Branch           string `json:"branch,omitempty"`
	CommitSHA        string `json:"commit_sha,omitempty"`
	CommitMessage    string `json:"commit_message,omitempty"`
	CommitURL        string `json:"commit_url,omitempty"`
	DeploymentMethod string `json:"deployment_method"`
	Status           string `json:"status"`
	ErrorCode        string `json:"error_code,omitempty"`
	ErrorSummary     string `json:"error_summary,omitempty"`
	QueuedAt         string `json:"queued_at"`
	StartedAt        string `json:"started_at,omitempty"`
	FinishedAt       string `json:"finished_at,omitempty"`
	DurationMS       int64  `json:"duration_ms,omitempty"`
	CreatedAt        string `json:"created_at"`
	BuildLog         string `json:"build_log,omitempty"`
	DeploymentLog    string `json:"deployment_log,omitempty"`
}

type Commit struct {
	SHA          string `json:"sha"`
	Message      string `json:"message"`
	Branch       string `json:"branch"`
	Author       string `json:"author,omitempty"`
	URL          string `json:"url,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
	Deployed     bool   `json:"deployed"`
	DeploymentID string `json:"deployment_id,omitempty"`
	Status       string `json:"status"`
}

type Page struct {
	Items      []Deployment `json:"items"`
	Page       int          `json:"page"`
	PerPage    int          `json:"per_page"`
	Total      int          `json:"total"`
	TotalPages int          `json:"total_pages"`
}

func NewRunner(db *sql.DB, github ArchiveDownloader) *Runner {
	runner := &Runner{db: db, github: github}
	if resolver, ok := github.(CommitResolver); ok {
		runner.resolver = resolver
	}
	return runner
}

func (r *Runner) DeployGitHub(ctx context.Context, site sites.Site) (Result, error) {
	return r.DeployGitHubRef(ctx, site, "")
}

func (r *Runner) DeployGitHubRef(ctx context.Context, site sites.Site, ref string) (result Result, err error) {
	if site.Strategy != "atomic" {
		return Result{}, fmt.Errorf("site %q is not configured for atomic deployment", site.ID)
	}
	if site.Repository == nil || strings.ToLower(site.Repository.Provider) != "github" {
		return Result{}, fmt.Errorf("site %q does not have a GitHub repository", site.ID)
	}
	if site.Repository.GitHubInstallationID == nil || site.Repository.GitHubFullName == "" {
		return Result{}, fmt.Errorf("site %q has incomplete GitHub repository metadata", site.ID)
	}
	if filepath.Clean(site.WebRoot) == filepath.Clean(site.ProjectRoot) {
		return Result{}, fmt.Errorf("atomic deployment requires a document root below the project root, such as %q", filepath.Join(site.ProjectRoot, "current"))
	}
	if err := os.MkdirAll(site.ProjectRoot, 0o755); err != nil {
		return Result{}, fmt.Errorf("create project root: %w", err)
	}
	zionRoot := filepath.Join(site.ProjectRoot, ".zion")
	releasesRoot := filepath.Join(zionRoot, "releases")
	if err := os.MkdirAll(releasesRoot, 0o755); err != nil {
		return Result{}, fmt.Errorf("create release root: %w", err)
	}
	lock, err := acquireLock(filepath.Join(zionRoot, "deploy.lock"))
	if err != nil {
		return Result{}, err
	}
	defer releaseLock(lock)

	deploymentID, err := newID("dep_")
	if err != nil {
		return Result{}, err
	}
	releaseID, err := newID("rel_")
	if err != nil {
		return Result{}, err
	}
	branch := strings.TrimSpace(site.Repository.Branch)
	if branch == "" {
		branch = site.Repository.GitHubDefaultBranch
	}
	if branch == "" {
		return Result{}, fmt.Errorf("site %q has no GitHub branch configured", site.ID)
	}
	archiveRef := branch
	if strings.TrimSpace(ref) != "" {
		archiveRef = strings.TrimSpace(ref)
	}
	var commit githubconnector.Commit
	if r.resolver != nil {
		commit, err = r.resolver.ResolveCommit(ctx, *site.Repository.GitHubInstallationID, site.Repository.GitHubFullName, archiveRef)
		if err != nil {
			return Result{}, err
		}
	}
	logs := newDeploymentLogs()
	logs.add("build", "Preparing deployment for "+site.Name)
	logs.add("build", "Resolving commit "+archiveRef)
	if commit.SHA != "" {
		logs.add("build", "Resolved "+commit.SHA+" — "+strings.Split(commit.Message, "\n")[0])
	}
	if err := r.createDeployment(ctx, deploymentID, site.ID, branch, commit); err != nil {
		return Result{}, err
	}
	failed := true
	defer func() {
		if err != nil {
			logs.add("deployment", "ERROR: "+err.Error())
		}
		_ = r.saveDeploymentLogs(context.Background(), deploymentID, logs)
		if failed {
			_ = r.finishDeployment(context.Background(), deploymentID, "failed", "DEPLOYMENT_FAILED")
		} else {
			_ = r.finishDeployment(context.Background(), deploymentID, "deployed", "")
		}
	}()

	archiveFile, err := os.CreateTemp(zionRoot, "archive-*.tar.gz")
	if err != nil {
		return Result{}, fmt.Errorf("create archive staging file: %w", err)
	}
	archivePath := archiveFile.Name()
	defer os.Remove(archivePath)
	logs.add("deployment", "Downloading GitHub archive for "+archiveRef)
	if err := r.github.DownloadArchive(ctx, *site.Repository.GitHubInstallationID, site.Repository.GitHubFullName, archiveRef, archiveFile); err != nil {
		archiveFile.Close()
		return Result{}, err
	}
	if err := archiveFile.Close(); err != nil {
		return Result{}, fmt.Errorf("close archive staging file: %w", err)
	}

	stagePath := filepath.Join(releasesRoot, releaseID+".staging")
	defer os.RemoveAll(stagePath)
	if err := os.MkdirAll(stagePath, 0o700); err != nil {
		return Result{}, fmt.Errorf("create release staging directory: %w", err)
	}
	if err := extractArchive(archivePath, stagePath); err != nil {
		return Result{}, err
	}
	logs.add("deployment", "Archive extracted and staged")
	releasePath := filepath.Join(releasesRoot, releaseID)
	if err := os.Rename(stagePath, releasePath); err != nil {
		return Result{}, fmt.Errorf("finalize release: %w", err)
	}
	if err := r.createRelease(ctx, releaseID, site.ID, deploymentID, releasePath, commit.SHA); err != nil {
		return Result{}, err
	}

	currentStagingPath := filepath.Join(site.ProjectRoot, ".current")
	nextStagingPath := currentStagingPath + ".next-" + releaseID
	if err := os.Symlink(filepath.Join(".zion", "releases", releaseID), nextStagingPath); err != nil {
		return Result{}, fmt.Errorf("prepare current staging link: %w", err)
	}
	if err := replaceSymlink(nextStagingPath, currentStagingPath); err != nil {
		return Result{}, fmt.Errorf("activate current staging link: %w", err)
	}
	logs.add("deployment", "Prepared .current from "+releaseID)
	if err := runDeploymentScript(ctx, site, releasePath, currentStagingPath, deploymentID, releaseID, commit.SHA, logs); err != nil {
		return Result{}, err
	}
	logs.add("deployment", "Copied .current into "+site.WebRoot)
	if err := r.activateRelease(ctx, site.ID, releaseID); err != nil {
		return Result{}, err
	}
	failed = false
	return Result{DeploymentID: deploymentID, ReleaseID: releaseID, ReleasePath: releasePath, Commit: commit.SHA, Status: "deployed"}, nil
}

func runDeploymentScript(ctx context.Context, site sites.Site, releasePath, currentPath, deploymentID, releaseID, commitSHA string, logs *deploymentLogs) error {
	script := sites.EffectiveDeployScript(site.Strategy, site.DeployScript)
	if strings.TrimSpace(script) == "" {
		return fmt.Errorf("site %q has no deployment script", site.ID)
	}
	scriptKind := "custom"
	if strings.TrimSpace(script) == strings.TrimSpace(sites.DefaultAtomicDeployScript) {
		scriptKind = "default"
	}
	logs.add("build", "Using "+scriptKind+" deployment script")
	command := exec.CommandContext(ctx, "/bin/sh", "-c", script)
	command.Dir = releasePath
	command.Env = append(os.Environ(),
		"PROJECT_ROOT="+site.ProjectRoot,
		"WEB_ROOT="+site.WebRoot,
		"CURRENT_DIR="+currentPath,
		"RELEASE_DIR="+releasePath,
		"RELEASE_ID="+releaseID,
		"DEPLOYMENT_ID="+deploymentID,
		"COMMIT_SHA="+commitSHA,
	)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		if text := strings.TrimSpace(output.String()); text != "" {
			for _, line := range strings.Split(text, "\n") {
				logs.add("deployment", line)
			}
		}
		return fmt.Errorf("deployment script failed: %w", err)
	}
	if text := strings.TrimSpace(output.String()); text != "" {
		for _, line := range strings.Split(text, "\n") {
			logs.add("deployment", line)
		}
	}
	return nil
}

func (r *Runner) createDeployment(ctx context.Context, id, siteID, branch string, commit githubconnector.Commit) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `INSERT INTO deployments(id, site_id, trigger_type, branch, commit_sha, commit_message, commit_url, deployment_method, status, queued_at, started_at, created_at) VALUES (?, ?, 'manual', ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), 'manual', 'running', ?, ?, ?)`, id, siteID, branch, commit.SHA, commit.Message, commit.URL, now, now, now)
	if err != nil {
		return fmt.Errorf("create deployment record: %w", err)
	}
	return nil
}

func (r *Runner) createRelease(ctx context.Context, id, siteID, deploymentID, releasePath, commitSHA string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `INSERT INTO releases(id, site_id, deployment_id, release_name, release_path, commit_sha, active, created_at) VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), 0, ?)`, id, siteID, deploymentID, id, releasePath, commitSHA, now)
	if err != nil {
		return fmt.Errorf("create release record: %w", err)
	}
	return nil
}

func (r *Runner) activateRelease(ctx context.Context, siteID, releaseID string) error {
	if _, err := r.db.ExecContext(ctx, `UPDATE releases SET active = 0 WHERE site_id = ?`, siteID); err != nil {
		return fmt.Errorf("deactivate previous releases: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := r.db.ExecContext(ctx, `UPDATE releases SET active = 1, health_status = 'not_checked', activated_at = ? WHERE id = ? AND site_id = ?`, now, releaseID, siteID); err != nil {
		return fmt.Errorf("activate release record: %w", err)
	}
	return nil
}

func (r *Runner) finishDeployment(ctx context.Context, id, status, code string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `UPDATE deployments SET status = ?, error_code = NULLIF(?, ''), finished_at = ?, duration_ms = CAST((julianday(?) - julianday(COALESCE(started_at, queued_at))) * 86400000 AS INTEGER) WHERE id = ?`, status, code, now, now, id)
	if err != nil {
		return fmt.Errorf("finish deployment record: %w", err)
	}
	return nil
}

func acquireLock(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("another deployment is already running for this site")
		}
		return nil, fmt.Errorf("acquire deployment lock: %w", err)
	}
	_, _ = io.WriteString(file, time.Now().UTC().Format(time.RFC3339Nano))
	return file, nil
}

func releaseLock(file *os.File) {
	name := file.Name()
	_ = file.Close()
	_ = os.Remove(name)
}

func replaceSymlink(nextPath, currentPath string) error {
	if info, err := os.Lstat(currentPath); err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("current path exists and is not a symlink")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(nextPath, currentPath)
}

func extractArchive(archivePath, destination string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open release archive: %w", err)
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("read release archive: %w", err)
	}
	defer compressed.Close()
	reader := tar.NewReader(compressed)
	var topLevel string
	var total int64
	entries := 0
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read release archive entry: %w", err)
		}
		entries++
		if entries > 100000 {
			return fmt.Errorf("release archive contains too many entries")
		}
		name := strings.TrimPrefix(strings.ReplaceAll(header.Name, "\\", "/"), "./")
		parts := strings.SplitN(name, "/", 2)
		if topLevel == "" {
			topLevel = parts[0]
		}
		relative := name
		if name == topLevel {
			continue
		}
		prefix := topLevel + "/"
		if strings.HasPrefix(name, prefix) {
			relative = strings.TrimPrefix(name, prefix)
		}
		clean := path.Clean(relative)
		if clean == "." || clean == "" || clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
			return fmt.Errorf("unsafe path in release archive: %q", header.Name)
		}
		target := filepath.Join(destination, filepath.FromSlash(clean))
		if !within(destination, target) {
			return fmt.Errorf("release archive entry escapes staging directory")
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create release directory: %w", err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("create release parent: %w", err)
			}
			output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, normalizedMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create release file: %w", err)
			}
			written, copyErr := io.Copy(output, io.LimitReader(reader, maxArchiveSize-total+1))
			closeErr := output.Close()
			if copyErr != nil {
				return fmt.Errorf("extract release file: %w", copyErr)
			}
			if closeErr != nil {
				return fmt.Errorf("close release file: %w", closeErr)
			}
			total += written
			if total > maxArchiveSize {
				return fmt.Errorf("expanded release exceeds the 512 MB safety limit")
			}
		default:
			return fmt.Errorf("unsupported archive entry type for %q", header.Name)
		}
	}
	if entries == 0 {
		return fmt.Errorf("release archive is empty")
	}
	return nil
}

func normalizedMode(mode int64) os.FileMode {
	result := os.FileMode(mode) & 0o777
	if result == 0 {
		return 0o644
	}
	return result
}

func within(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func newID(prefix string) (string, error) {
	var value [12]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate deployment id: %w", err)
	}
	return prefix + fmt.Sprintf("%x", value), nil
}
