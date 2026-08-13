package systemchecks

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
)

// Definition describes a deployment tool that can be verified on the NAS.
type Definition struct {
	ID          string
	Label       string
	Command     string
	Description string
	InstallHint string
	VersionArgs []string
}

type Result struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Command     string `json:"command"`
	Description string `json:"description"`
	InstallHint string `json:"install_hint"`
	State       string `json:"state"`
	Detail      string `json:"detail"`
	Version     string `json:"version,omitempty"`
}

var definitions = []Definition{
	{ID: "php", Label: "PHP", Command: "php", Description: "PHP runtime used by PHP, WordPress, Laravel and Symfony sites.", InstallHint: "Install a PHP package from DSM Package Center, then verify with: php -v", VersionArgs: []string{"-v"}},
	{ID: "composer", Label: "Composer", Command: "composer", Description: "Dependency manager used by most modern PHP applications.", InstallHint: "Install Composer for the selected PHP runtime and verify with: composer --version", VersionArgs: []string{"--version"}},
	{ID: "node", Label: "Node.js", Command: "node", Description: "JavaScript runtime used for frontend builds and Node-based applications.", InstallHint: "Install Node.js from DSM Package Center, then verify with: node --version", VersionArgs: []string{"--version"}},
	{ID: "npm", Label: "npm", Command: "npm", Description: "Node package manager used by Vite, Next.js and other frontend builds.", InstallHint: "Install Node.js from DSM Package Center; npm is normally bundled. Verify with: npm --version", VersionArgs: []string{"--version"}},
	{ID: "git", Label: "Git transport", Command: "git", Description: "Git client required for repository checks and non-archive workflows.", InstallHint: "Install Git from DSM Package Center or Synology's official package source, then verify with: git --version", VersionArgs: []string{"--version"}},
	{ID: "rsync", Label: "rsync", Command: "rsync", Description: "Efficient file synchronization utility useful for larger in-place deployments.", InstallHint: "Enable/install rsync from DSM services or the official Synology package source, then verify with: rsync --version", VersionArgs: []string{"--version"}},
	{ID: "unzip", Label: "unzip", Command: "unzip", Description: "Archive utility used by many PHP and deployment workflows.", InstallHint: "Use the DSM-provided unzip binary or install the official package that provides it. Verify with: unzip -v", VersionArgs: []string{"-v"}},
	{ID: "tar", Label: "tar", Command: "tar", Description: "Archive utility used to stage and inspect release files.", InstallHint: "tar is normally included with DSM. Verify with: tar --version", VersionArgs: []string{"--version"}},
	{ID: "curl", Label: "curl", Command: "curl", Description: "HTTP client used by health checks, webhooks and deployment scripts.", InstallHint: "curl is normally included with DSM. Verify with: curl --version", VersionArgs: []string{"--version"}},
	{ID: "mysql", Label: "MariaDB/MySQL client", Command: "mysql", Description: "Optional database client used by PHP applications for migrations and diagnostics.", InstallHint: "Install the matching MariaDB package/client from DSM Package Center, then verify with: mysql --version", VersionArgs: []string{"--version"}},
}

func Definitions() []Definition {
	result := make([]Definition, len(definitions))
	copy(result, definitions)
	return result
}

func DefaultIDs() []string {
	result := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		result = append(result, definition.ID)
	}
	return result
}

func NormalizeIDs(ids []string) []string {
	known := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		known[definition.ID] = struct{}{}
	}
	seen := make(map[string]struct{}, len(ids))
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if _, ok := known[id]; !ok {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func Run(ctx context.Context, enabled []string) []Result {
	selected := make(map[string]struct{}, len(enabled))
	for _, id := range NormalizeIDs(enabled) {
		selected[id] = struct{}{}
	}
	result := make([]Result, 0, len(selected))
	for _, definition := range definitions {
		if _, ok := selected[definition.ID]; !ok {
			continue
		}
		item := Result{ID: definition.ID, Label: definition.Label, Command: definition.Command, Description: definition.Description, InstallHint: definition.InstallHint, State: "error", Detail: "Not installed"}
		commandPath := resolveCommand(definition.Command)
		if commandPath == "" {
			result = append(result, item)
			continue
		}
		item.State = "ready"
		item.Detail = "Available at " + commandPath
		command := exec.CommandContext(ctx, commandPath, definition.VersionArgs...)
		output, err := command.Output()
		if err == nil {
			version := strings.TrimSpace(strings.SplitN(string(output), "\n", 2)[0])
			if version != "" {
				item.Version = version
				item.Detail = version + " · " + commandPath
			}
		}
		result = append(result, item)
	}
	return result
}

func resolveCommand(name string) string {
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	for _, directory := range []string{"/usr/local/bin", "/usr/bin", "/bin", "/usr/syno/bin", "/opt/bin"} {
		path := filepath.Join(directory, name)
		if command, err := exec.LookPath(path); err == nil {
			return command
		}
	}
	return ""
}
