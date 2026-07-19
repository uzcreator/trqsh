package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/rift/rift/internal/agent"
)

// AppSettings is the GUI settings surface (frontend/src/lib/agent.ts). Server
// and Insecure live in the frozen agent config (~/.rift/rift.yml, §10); the
// purely-cosmetic prefs live alongside it in gui.json so we never mutate the
// frozen schema.
type AppSettings struct {
	Server         string `json:"server"`
	Insecure       bool   `json:"insecure"`
	StartAtLogin   bool   `json:"start_at_login"`
	MinimizeToTray bool   `json:"minimize_to_tray"`
	Theme          string `json:"theme"` // system | light | dark
}

// guiPrefs is the GUI-only slice persisted outside the frozen config schema.
type guiPrefs struct {
	StartAtLogin   bool   `json:"start_at_login"`
	MinimizeToTray bool   `json:"minimize_to_tray"`
	Theme          string `json:"theme"`
}

func prefsPath() string {
	return filepath.Join(filepath.Dir(agent.DefaultConfigPath()), "gui.json")
}

func loadPrefs() guiPrefs {
	p := guiPrefs{MinimizeToTray: true, Theme: "system"}
	if data, err := os.ReadFile(prefsPath()); err == nil {
		_ = json.Unmarshal(data, &p)
	}
	if p.Theme == "" {
		p.Theme = "system"
	}
	return p
}

func savePrefs(p guiPrefs) error {
	if err := os.MkdirAll(filepath.Dir(prefsPath()), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(prefsPath(), data, 0o600)
}

// Settings reads the merged GUI settings.
func (s *AgentService) Settings() AppSettings {
	cfg, err := agent.Load(agent.DefaultConfigPath())
	if err != nil {
		cfg = agent.DefaultConfig()
	}
	p := loadPrefs()
	return AppSettings{
		Server:         cfg.Server,
		Insecure:       cfg.Insecure,
		StartAtLogin:   p.StartAtLogin,
		MinimizeToTray: p.MinimizeToTray,
		Theme:          p.Theme,
	}
}

// SaveSettings persists connection settings to the frozen config and the
// cosmetic prefs to gui.json.
func (s *AgentService) SaveSettings(in AppSettings) error {
	cfg, err := agent.Load(agent.DefaultConfigPath())
	if err != nil {
		cfg = agent.DefaultConfig()
	}
	cfg.Server = in.Server
	cfg.Insecure = in.Insecure
	if err := cfg.Save(agent.DefaultConfigPath()); err != nil {
		return err
	}
	return savePrefs(guiPrefs{
		StartAtLogin:   in.StartAtLogin,
		MinimizeToTray: in.MinimizeToTray,
		Theme:          in.Theme,
	})
}

// --- API key storage ---
//
// The agent core already persists the API key to ~/.rift/rift.yml on Login.
// That is the shared source of truth with the CLI. For a hardened desktop
// build the key should instead live in the OS keychain (Keychain / Windows
// Credential Manager / libsecret) via e.g. github.com/zalando/go-keyring — this
// pair of helpers is the single seam where that swap happens.

func hasStoredKey() bool {
	cfg, err := agent.Load(agent.DefaultConfigPath())
	return err == nil && cfg.APIKey != ""
}

func clearStoredKey() error {
	cfg, err := agent.Load(agent.DefaultConfigPath())
	if err != nil {
		return err
	}
	cfg.APIKey = ""
	return cfg.Save(agent.DefaultConfigPath())
}
