package detection

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DetectHTTPServer identifies the HTTP server most likely serving Web Station.
// Synology can run nginx as the front proxy and Apache as a backend, so the
// running process is preferred over a generic Web Station label. Configuration
// files are used as a fallback when /proc is restricted.
func DetectHTTPServer(projectRoot, webRoot string) string {
	if server := runningHTTPServer(); server != "" {
		return server
	}

	for _, path := range []string{
		"/etc/nginx/nginx.conf",
		"/usr/local/etc/nginx/nginx.conf",
		"/var/packages/WebStation/target/etc/nginx/nginx.conf",
	} {
		if configMentionsRoots(path, projectRoot, webRoot) {
			return "Nginx"
		}
	}
	for _, path := range []string{
		"/etc/httpd/conf/httpd.conf",
		"/etc/apache2/httpd.conf",
		"/var/packages/Apache2.4/target/usr/local/etc/apache24/httpd.conf",
		"/var/packages/WebStation/target/usr/local/etc/apache24/httpd.conf",
	} {
		if configMentionsRoots(path, projectRoot, webRoot) {
			return "Apache"
		}
	}
	return "Unknown"
}

func runningHTTPServer() string {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return ""
	}
	foundNginx := false
	foundApache := false
	for _, entry := range entries {
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		command, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "comm"))
		if err != nil {
			continue
		}
		switch strings.TrimSpace(strings.ToLower(string(command))) {
		case "nginx":
			foundNginx = true
		case "httpd", "apache2":
			foundApache = true
		}
	}
	// nginx is the public Web Station front proxy when both services exist.
	if foundNginx {
		return "Nginx"
	}
	if foundApache {
		return "Apache"
	}
	return ""
}

func configMentionsRoots(path, projectRoot, webRoot string) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	text := string(content)
	if projectRoot != "" && strings.Contains(text, projectRoot) {
		return true
	}
	return webRoot != "" && strings.Contains(text, webRoot)
}
