package agent

import (
	"fmt"
	"os"
	"path/filepath"
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

// Config is the agent configuration, matching ~/.trqsh/trqsh.yml (§10).
// Precedence when loaded by the CLI: flags > env (TRQSH_*) > file > defaults.
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
	APIBase     string `yaml:"api_base,omitempty"`     // control-plane API base (desktop cloud calls)
	Insecure    bool   `yaml:"insecure,omitempty"`     // skip TLS verify (dev)
}

// DefaultConfig returns the agent defaults (dev-friendly).
func DefaultConfig() Config {
	return Config{
		Version:     1,
		Server:      DefaultServer,
		Region:      "auto",
		Transport:   "auto",
		Tunnels:     map[string]TunnelConfig{},
		Inspector:   InspectorConfig{Enabled: true, Addr: "127.0.0.1:4040"},
		LogLevel:    "info",
		ControlAddr: "127.0.0.1:4041",
		APIBase:     DefaultAPIBase,
	}
}

// DefaultConfigPath is ~/.trqsh/trqsh.yml (override with TRQSH_CONFIG).
func DefaultConfigPath() string {
	if p := os.Getenv("TRQSH_CONFIG"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "trqsh.yml"
	}
	return filepath.Join(home, ".trqsh", "trqsh.yml")
}

// Load builds a Config from defaults, overlaid with the file at path (if it
// exists), overlaid with TRQSH_* environment variables.
func Load(path string) (Config, error) {
	c := DefaultConfig()
	if path == "" {
		path = DefaultConfigPath()
	}
	if data, err := os.ReadFile(path); err == nil { // #nosec G304 -- path is DefaultConfigPath() or the caller's own --config/TRQSH_CONFIG, never remote/attacker input
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
	envStr(&c.APIKey, "TRQSH_API_KEY")
	envStr(&c.Server, "TRQSH_SERVER")
	envStr(&c.Region, "TRQSH_REGION")
	envStr(&c.Transport, "TRQSH_TRANSPORT")
	envStr(&c.LogLevel, "TRQSH_LOG_LEVEL")
	envStr(&c.Inspector.Addr, "TRQSH_INSPECTOR_ADDR")
	envStr(&c.ControlAddr, "TRQSH_CONTROL_ADDR")
	envStr(&c.APIBase, "TRQSH_API_URL")
	if v, ok := os.LookupEnv("TRQSH_INSECURE"); ok {
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
	data, err := yaml.Marshal(c) // #nosec G117 -- the API key belongs in this config file by design (it's how the CLI persists a signed-in session)
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
