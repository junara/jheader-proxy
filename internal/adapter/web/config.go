package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// appDirName は設定・CA を置くアプリ専用サブディレクトリ名。
// macOS では ~/Library/Application Support/jheader-proxy に解決される。
const appDirName = "jheader-proxy"

// HeaderKV はフォームから受け取るヘッダー1件(Name/Value)。
type HeaderKV struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// RunConfig は GUI フォームの値一式。直近設定として JSON 永続化される。
type RunConfig struct {
	Listen     string     `json:"listen"`
	Domains    []string   `json:"domains"`
	Headers    []HeaderKV `json:"headers"`
	Allow      []string   `json:"allow"`
	Duration   string     `json:"duration"` // Go の duration 文字列(例 "10m")。"" / "0" で無制限。
	Quiet      bool       `json:"quiet"`
	Verbose    bool       `json:"verbose"`
	Redact     bool       `json:"redact"`
	CACertPath string     `json:"caCertPath"`
	CAKeyPath  string     `json:"caKeyPath"`
}

// ConfigDir はアプリ専用ディレクトリのパスを返す(未作成でも返す)。
func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve user config dir: %w", err)
	}
	return filepath.Join(base, appDirName), nil
}

// DefaultRunConfig は CA の既定パスを埋めた初期設定を返す。
func DefaultRunConfig() RunConfig {
	cfg := RunConfig{
		Listen:   ":8080",
		Duration: "10m",
	}
	if dir, err := ConfigDir(); err == nil {
		cfg.CACertPath = filepath.Join(dir, "ca-cert.pem")
		cfg.CAKeyPath = filepath.Join(dir, "ca-key.pem")
	}
	return cfg
}

// configPath は設定ファイルのパスを返す。
func configPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// LoadConfig は直近設定を読み込む。ファイルが無ければ既定設定を返す。
func LoadConfig() (RunConfig, error) {
	path, err := configPath()
	if err != nil {
		return DefaultRunConfig(), err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultRunConfig(), nil
	}
	if err != nil {
		return DefaultRunConfig(), fmt.Errorf("failed to read config %q: %w", path, err)
	}

	// 既定値の上に保存値を重ねる(将来フィールドが増えても既定で埋まる)。
	cfg := DefaultRunConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultRunConfig(), fmt.Errorf("failed to parse config %q: %w", path, err)
	}
	return cfg, nil
}

// SaveConfig は設定を 0600 で保存する。ヘッダー値に認証トークン等が入りうるため
// 権限を絞る。ディレクトリは 0700 で作成する。
func SaveConfig(cfg RunConfig) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create config dir %q: %w", dir, err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write config %q: %w", path, err)
	}
	return nil
}
