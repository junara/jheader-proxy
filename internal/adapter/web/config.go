package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/junara/jheader-proxy/internal/config"
)

// appDirName は設定・CA を置くアプリ専用サブディレクトリ名。
// macOS では ~/Library/Application Support/jheader-proxy に解決される。
const appDirName = "jheader-proxy"

// 設定スキーマは CLI(--config)と共有する(internal/config)。GUI が保存する
// config.json をそのまま CLI の --config で読み込めるよう、同一型を別名で公開する。
type (
	// HeaderKV は付与するヘッダー1件(Name/Value)。
	HeaderKV = config.HeaderKV
	// RunConfig は GUI フォームの値一式。直近設定として JSON 永続化される。
	RunConfig = config.RunConfig
)

// ConfigDir はアプリ専用ディレクトリのパスを返す(未作成でも返す)。
func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve user config dir: %w", err)
	}
	return filepath.Join(base, appDirName), nil
}

// DefaultRunConfig は共通の既定値に、CA の既定パス(アプリ固有)を加えて返す。
func DefaultRunConfig() RunConfig {
	cfg := config.Default()
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
