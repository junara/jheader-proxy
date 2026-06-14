package web

import (
	"os"
	"path/filepath"
	"testing"
)

// useTempConfigDir は UserConfigDir がテスト用一時ディレクトリへ解決されるよう
// 環境変数を差し替える(macOS は HOME、Linux は XDG_CONFIG_HOME を見る)。
func useTempConfigDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
}

func TestConfigRoundTrip(t *testing.T) {
	useTempConfigDir(t)

	cfg := DefaultRunConfig()
	cfg.Domains = []string{"a.test", "b.test"}
	cfg.Headers = []HeaderKV{{Name: "X-Debug", Value: "secret"}}
	cfg.Verbose = true

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	got, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(got.Domains) != 2 || got.Domains[1] != "b.test" {
		t.Fatalf("domains round-trip failed: %v", got.Domains)
	}
	if len(got.Headers) != 1 || got.Headers[0].Value != "secret" {
		t.Fatalf("headers round-trip failed: %v", got.Headers)
	}
	if !got.Verbose {
		t.Fatal("verbose round-trip failed")
	}

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("expected config perm 0600, got %o", perm)
	}
}

func TestLoadConfigMissingReturnsDefault(t *testing.T) {
	useTempConfigDir(t)

	got, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig on missing file: %v", err)
	}
	if got.Listen != ":8080" {
		t.Fatalf("expected default listen :8080, got %q", got.Listen)
	}
	if got.CACertPath == "" {
		t.Fatal("expected default CA cert path to be set")
	}
}
