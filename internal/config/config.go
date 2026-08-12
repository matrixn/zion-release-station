package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
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
	ConnectorStatePath   string
	PublicURL            string
	Version              string
}

func Load() Config {
	dataDir := envOrDefault("RS_DATA_DIR", "./var")
	statePath := filepath.Join(dataDir, "connector.json")
	state := readConnectorState(statePath)
	instanceID := os.Getenv("RS_INSTANCE_ID")
	if instanceID == "" {
		instanceID = state.InstanceID
	}
	if instanceID == "" {
		instanceID = newInstanceID()
	}
	credential := os.Getenv("RS_GITHUB_CONNECTOR_TOKEN")
	if credential == "" {
		credential = state.Credential
	}
	return Config{
		BindAddress:          envOrDefault("RS_BIND_ADDRESS", "127.0.0.1:24871"),
		DataDir:              dataDir,
		WebRoot:              envOrDefault("RS_WEB_ROOT", "./web"),
		WebStationRoots:      envListOrDefault("RS_WEB_STATION_ROOTS", []string{"/volume1/www", "/volume1/web"}),
		GitHubConnectorURL:   envOrDefault("RS_GITHUB_CONNECTOR_URL", "https://connector.raduta.synology.me"),
		GitHubConnectorToken: credential,
		InstanceID:           instanceID,
		ConnectorStatePath:   statePath,
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
	return c.persistConnectorState()
}

func (c Config) SaveConnectorCredential(credential string) error {
	return writeConnectorState(c.ConnectorStatePath, connectorState{InstanceID: c.InstanceID, Credential: credential})
}

type connectorState struct {
	InstanceID string `json:"instance_id"`
	Credential string `json:"credential,omitempty"`
}

func (c Config) persistConnectorState() error {
	return writeConnectorState(c.ConnectorStatePath, connectorState{InstanceID: c.InstanceID, Credential: c.GitHubConnectorToken})
}

func writeConnectorState(path string, state connectorState) error {
	encoded, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode connector state: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return fmt.Errorf("write connector state: %w", err)
	}
	return nil
}

func readConnectorState(path string) connectorState {
	content, err := os.ReadFile(path)
	if err != nil {
		return connectorState{}
	}
	var state connectorState
	if json.Unmarshal(content, &state) != nil {
		return connectorState{}
	}
	return state
}

func newInstanceID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "rs-local"
	}
	return "rs_" + hex.EncodeToString(value[:])
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
