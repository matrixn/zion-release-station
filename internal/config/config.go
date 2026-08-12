package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	BindAddress          string
	DataDir              string
	WebRoot              string
	WebStationRoots      []string
	GitHubConnectorURL   string
	GitHubConnectorToken string
	InstanceID           string
	PublicURL            string
	Version              string
}

func Load() Config {
	return Config{
		BindAddress:          envOrDefault("RS_BIND_ADDRESS", "127.0.0.1:24871"),
		DataDir:              envOrDefault("RS_DATA_DIR", "./var"),
		WebRoot:              envOrDefault("RS_WEB_ROOT", "./web"),
		WebStationRoots:      envListOrDefault("RS_WEB_STATION_ROOTS", []string{"/volume1/www", "/volume1/web"}),
		GitHubConnectorURL:   os.Getenv("RS_GITHUB_CONNECTOR_URL"),
		GitHubConnectorToken: os.Getenv("RS_GITHUB_CONNECTOR_TOKEN"),
		InstanceID:           envOrDefault("RS_INSTANCE_ID", "local"),
		PublicURL:            os.Getenv("RS_PUBLIC_URL"),
		Version:              envOrDefault("RS_VERSION", "0.1.0"),
	}
}

func (c Config) DatabasePath() string {
	return filepath.Join(c.DataDir, "releasestation.db")
}

func (c Config) DeploymentLogDir() string {
	return filepath.Join(c.DataDir, "logs", "deployments")
}

func (c Config) EnsureDataDirectories() error {
	for _, path := range []string{
		c.DataDir,
		filepath.Join(c.DataDir, "git", "keys"),
		c.DeploymentLogDir(),
		filepath.Join(c.DataDir, "locks"),
		filepath.Join(c.DataDir, "cache"),
		filepath.Join(c.DataDir, "runtime"),
	} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return fmt.Errorf("create runtime directory %q: %w", path, err)
		}
	}
	return nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envListOrDefault(name string, fallback []string) []string {
	value := os.Getenv(name)
	if value == "" {
		return append([]string(nil), fallback...)
	}

	var values []string
	for _, item := range filepath.SplitList(value) {
		if item != "" {
			values = append(values, item)
		}
	}
	if len(values) == 0 {
		return append([]string(nil), fallback...)
	}
	return values
}
