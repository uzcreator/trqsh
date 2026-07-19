package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// TunnelConfig is a persisted tunnel definition (see §10 of 00-ARCHITECTURE.md).
type TunnelConfig struct {
	Proto        string `yaml:"proto"`
	Addr         string `yaml:"addr"`
	Subdomain    string `yaml:"subdomain,omitempty"`
	CustomDomain string `yaml:"custom_domain,omitempty"`
	BasicAuth    string `yaml:"basic_auth,omitempty"`
	HostHeader   string `yaml:"host_header,omitempty"`
	RemotePort   int    `yaml:"remote_port,omitempty"`
}

// InspectorConfig configures the local request inspector.
type InspectorConfig struct {
	Enabled bool   `yaml:"enabled"`
	Addr    string `yaml:"addr"`
}

// Config is the agent configuration, matching ~/.rift/rift.yml (§10).
// Precedence when loaded by the CLI: flags > env (RIFT_*) > file > defaults.
type Config struct {
	Version   int                     `yaml:"version"`
	APIKey    string                  `yaml:"api_key,omitempty"`
	Server    string                  `yaml:"server"`
	Region    string                  `yaml:"region"`
	Transport string                  `yaml:"transport"` // auto | quic | tcp
	Tunnels   map[string]TunnelConfig `yaml:"tunnels,omitempty"`
	Inspector InspectorConfig         `yaml:"inspector"`
	LogLevel  string                  `yaml:"log_level"`

	// Operational, not part of the frozen schema.
	ControlAddr string `yaml:"control_addr,omitempty"` // local control API (GUI)
	Insecure    bool   `yaml:"insecure,omitempty"`     // skip TLS verify (dev)
}

// DefaultConfig returns the agent defaults (dev-friendly).
func DefaultConfig() Config {
	return Config{
		Version:     1,
		Server:      "localhost:4443",
		Region:      "auto",
		Transport:   "auto",
		Tunnels:     map[string]TunnelConfig{},
		Inspector:   InspectorConfig{Enabled: true, Addr: "127.0.0.1:4040"},
		LogLevel:    "info",
		ControlAddr: "127.0.0.1:4041",
	}
}

// DefaultConfigPath is ~/.rift/rift.yml (override with RIFT_CONFIG).
func DefaultConfigPath() string {
	if p := os.Getenv("RIFT_CONFIG"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "rift.yml"
	}
	return filepath.Join(home, ".rift", "rift.yml")
}

// Load builds a Config from defaults, overlaid with the file at path (if it
// exists), overlaid with RIFT_* environment variables.
func Load(path string) (Config, error) {
	c := DefaultConfig()
	if path == "" {
		path = DefaultConfigPath()
	}
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, &c); err != nil {
			return c, fmt.Errorf("agent: parse %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return c, fmt.Errorf("agent: read %s: %w", path, err)
	}
	c.applyEnv()
	if c.Tunnels == nil {
		c.Tunnels = map[string]TunnelConfig{}
	}
	return c, nil
}

func (c *Config) applyEnv() {
	envStr(&c.APIKey, "RIFT_API_KEY")
	envStr(&c.Server, "RIFT_SERVER")
	envStr(&c.Region, "RIFT_REGION")
	envStr(&c.Transport, "RIFT_TRANSPORT")
	envStr(&c.LogLevel, "RIFT_LOG_LEVEL")
	envStr(&c.Inspector.Addr, "RIFT_INSPECTOR_ADDR")
	envStr(&c.ControlAddr, "RIFT_CONTROL_ADDR")
	if v, ok := os.LookupEnv("RIFT_INSECURE"); ok {
		c.Insecure = truthy(v)
	}
}

// Save writes the config as YAML, creating the parent directory.
func (c Config) Save(path string) error {
	if path == "" {
		path = DefaultConfigPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("agent: mkdir config dir: %w", err)
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("agent: marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("agent: write config: %w", err)
	}
	return nil
}

// PreferredKind maps the transport setting to a tunnel.Kind override (""=auto).
func (c Config) forcedTransport() string {
	switch strings.ToLower(c.Transport) {
	case "quic":
		return "quic"
	case "tcp":
		return "tcp"
	default:
		return ""
	}
}

func envStr(dst *string, key string) {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		*dst = strings.TrimSpace(v)
	}
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// atoiPort parses a port-only or host:port addr's port; used by the CLI.
func atoiPort(s string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 || n > 65535 {
		return 0, false
	}
	return n, true
}
