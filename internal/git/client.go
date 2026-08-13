package git

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	branchPattern  = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
	sshHostPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]+$`)
)

type Remote struct {
	URL       string `json:"url"`
	Host      string `json:"host"`
	User      string `json:"user,omitempty"`
	Transport string `json:"transport"`
}

type Client struct {
	KeyStore      *KeyStore
	KnownHosts    string
	GitBinary     string
	CommandRunner func(context.Context, string, ...string) *exec.Cmd
}

func NewClient(dataDir string) *Client {
	return &Client{
		KeyStore:   NewKeyStore(dataDir),
		KnownHosts: filepath.Join(dataDir, "git", "known_hosts"),
		GitBinary:  "git",
	}
}

func ValidateRepository(remoteURL, branch string) (Remote, error) {
	remoteURL = strings.TrimSpace(remoteURL)
	if remoteURL == "" {
		return Remote{}, errors.New("repository URL is required")
	}
	if strings.IndexFunc(remoteURL, func(r rune) bool { return r < 0x20 || r == '\x7f' }) >= 0 {
		return Remote{}, errors.New("repository URL contains control characters")
	}
	if strings.ContainsAny(remoteURL, "\"';&|$`<>\n\r") {
		return Remote{}, errors.New("repository URL contains unsupported characters")
	}
	if strings.HasPrefix(remoteURL, "-") || strings.HasPrefix(remoteURL, "/") || strings.HasPrefix(remoteURL, "./") || strings.HasPrefix(remoteURL, "../") {
		return Remote{}, errors.New("repository URL must be a remote Git URL")
	}
	if strings.Contains(remoteURL, "@") && strings.HasPrefix(remoteURL, "https://") {
		return Remote{}, errors.New("repository URL must not contain embedded credentials")
	}

	remote := Remote{URL: remoteURL}
	switch {
	case strings.HasPrefix(remoteURL, "git@"):
		parts := strings.SplitN(remoteURL, ":", 2)
		if len(parts) != 2 || !strings.Contains(parts[0], "@") || strings.Trim(parts[1], "/") == "" {
			return Remote{}, errors.New("invalid SSH repository URL")
		}
		userHost := strings.SplitN(parts[0], "@", 2)
		if userHost[0] == "" || !sshHostPattern.MatchString(userHost[1]) || strings.Contains(parts[1], "..") {
			return Remote{}, errors.New("invalid SSH repository URL")
		}
		remote.User, remote.Host, remote.Transport = userHost[0], userHost[1], "ssh"
	case strings.HasPrefix(remoteURL, "ssh://"):
		parsed, err := url.Parse(remoteURL)
		if err != nil || parsed.Hostname() == "" || parsed.User == nil || parsed.User.Username() == "" || parsed.Path == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return Remote{}, errors.New("invalid SSH repository URL")
		}
		if _, hasPassword := parsed.User.Password(); hasPassword {
			return Remote{}, errors.New("SSH repository URL must not contain a password")
		}
		remote.User, remote.Host, remote.Transport = parsed.User.Username(), parsed.Hostname(), "ssh"
	case strings.HasPrefix(remoteURL, "https://") || strings.HasPrefix(remoteURL, "http://"):
		parsed, err := url.Parse(remoteURL)
		if err != nil || parsed.Hostname() == "" || parsed.Path == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return Remote{}, errors.New("invalid HTTP repository URL")
		}
		remote.Host, remote.Transport = parsed.Hostname(), "https"
	default:
		return Remote{}, errors.New("repository URL must use SSH or HTTP(S)")
	}
	if err := validateBranch(branch); err != nil {
		return Remote{}, err
	}
	return remote, nil
}

func (c *Client) Test(ctx context.Context, remoteURL, branch string) error {
	remote, err := ValidateRepository(remoteURL, branch)
	if err != nil {
		return err
	}
	args := []string{"ls-remote", "--exit-code", "--heads", remote.URL, "refs/heads/" + branch}
	_, err = c.run(ctx, remote, args...)
	if err != nil {
		return fmt.Errorf("test repository connectivity: %w", err)
	}
	return nil
}

func (c *Client) Branches(ctx context.Context, remoteURL string) ([]string, error) {
	remote, err := ValidateRepository(remoteURL, "main")
	if err != nil {
		return nil, err
	}
	output, err := c.run(ctx, remote, "ls-remote", "--heads", remote.URL)
	if err != nil {
		return nil, fmt.Errorf("list repository branches: %w", err)
	}
	branches := make([]string, 0)
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 || !strings.HasPrefix(fields[1], "refs/heads/") {
			continue
		}
		branches = append(branches, strings.TrimPrefix(fields[1], "refs/heads/"))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read repository branches: %w", err)
	}
	return branches, nil
}

func (c *Client) Clone(ctx context.Context, remoteURL, branch, target string) error {
	remote, err := ValidateRepository(remoteURL, branch)
	if err != nil {
		return err
	}
	if strings.TrimSpace(target) == "" || filepath.IsAbs(target) == false {
		return errors.New("clone target must be an absolute path")
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return errors.New("clone target already exists")
		}
		return fmt.Errorf("inspect clone target: %w", err)
	}
	_, err = c.run(ctx, remote, "clone", "--no-checkout", "--single-branch", "--branch", branch, remote.URL, target)
	if err != nil {
		return fmt.Errorf("clone repository: %w", err)
	}
	return nil
}

func (c *Client) Fetch(ctx context.Context, repositoryDir string) error {
	repositoryDir = filepath.Clean(strings.TrimSpace(repositoryDir))
	if repositoryDir == "." || repositoryDir == string(filepath.Separator) || repositoryDir == "" {
		return errors.New("repository directory is required")
	}
	if _, err := os.Stat(filepath.Join(repositoryDir, ".git")); err != nil {
		return errors.New("repository directory is not a Git checkout")
	}
	remote := Remote{Transport: "ssh"}
	_, err := c.runIn(ctx, remote, repositoryDir, "fetch", "--prune", "origin")
	if err != nil {
		return fmt.Errorf("fetch repository: %w", err)
	}
	return nil
}

func (c *Client) TestSSH(ctx context.Context, remoteURL, branch string) error {
	remote, err := ValidateRepository(remoteURL, branch)
	if err != nil {
		return err
	}
	if remote.Transport != "ssh" {
		return errors.New("SSH test requires an SSH repository URL")
	}
	if _, err := os.Stat(c.KnownHosts); err != nil {
		return fmt.Errorf("known_hosts is not configured: %w", err)
	}
	return c.Test(ctx, remoteURL, branch)
}

func (c *Client) run(ctx context.Context, remote Remote, args ...string) (string, error) {
	return c.runIn(ctx, remote, "", args...)
}

func (c *Client) runIn(ctx context.Context, remote Remote, dir string, args ...string) (string, error) {
	commandRunner := c.CommandRunner
	if commandRunner == nil {
		commandRunner = func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, name, args...)
		}
	}
	cmd := commandRunner(ctx, c.GitBinary, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_NOSYSTEM=1")
	cleanup := func() {}
	if remote.Transport == "ssh" {
		keyFile, remove, err := c.KeyStore.TemporaryPrivateKey()
		if err != nil {
			return "", fmt.Errorf("prepare SSH deploy key: %w", err)
		}
		cleanup = remove
		cmd.Env = append(cmd.Env, "GIT_SSH_COMMAND=ssh -i "+shellQuote(keyFile)+" -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o UserKnownHostsFile="+shellQuote(c.KnownHosts))
	}
	defer cleanup()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(output)), fmt.Errorf("git command failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func validateBranch(branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" || !branchPattern.MatchString(branch) || strings.HasPrefix(branch, "/") || strings.HasPrefix(branch, "-") || strings.HasSuffix(branch, "/") || strings.HasPrefix(branch, ".") || strings.Contains(branch, "..") || strings.Contains(branch, "//") || strings.Contains(branch, "@{") || strings.HasSuffix(branch, ".lock") {
		return errors.New("invalid Git branch")
	}
	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
