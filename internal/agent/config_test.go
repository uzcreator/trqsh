package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	if c.Server == "" || c.Inspector.Addr == "" || c.ControlAddr == "" {
		t.Fatalf("defaults incomplete: %+v", c)
	}
	if !c.Inspector.Enabled {
		t.Error("inspector should default enabled")
	}
}

func TestLoadFileOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trqsh.yml")
	yaml := "version: 1\nserver: edge.example:4443\nregion: eu\ntransport: tcp\n" +
		"tunnels:\n  web:\n    proto: http\n    addr: localhost:3000\n    subdomain: myapp\n"
	if err := writeFile(t, path, yaml); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Server != "edge.example:4443" || c.Region != "eu" || c.Transport != "tcp" {
		t.Fatalf("file not applied: %+v", c)
	}
	if tn, ok := c.Tunnels["web"]; !ok || tn.Addr != "localhost:3000" || tn.Subdomain != "myapp" {
		t.Fatalf("tunnel not parsed: %+v", c.Tunnels)
	}
}

func TestEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trqsh.yml")
	if err := writeFile(t, path, "version: 1\nserver: file.example:4443\n"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRQSH_SERVER", "env.example:4443")
	t.Setenv("TRQSH_API_KEY", "tq_from_env")
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Server != "env.example:4443" {
		t.Errorf("env should override file: got %q", c.Server)
	}
	if c.APIKey != "tq_from_env" {
		t.Errorf("api key from env not applied: %q", c.APIKey)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "trqsh.yml")
	c := DefaultConfig()
	c.APIKey = "tq_saved"
	c.Server = "roundtrip:4443"
	if err := c.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.APIKey != "tq_saved" || got.Server != "roundtrip:4443" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func writeFile(t *testing.T, path, content string) error {
	t.Helper()
	return os.WriteFile(path, []byte(content), 0o600)
}
