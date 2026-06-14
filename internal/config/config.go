// Package config は jheader-proxy の設定ファイルスキーマと、その読み込み・
// ユースケース入力への変換を提供する。CLI(--config)と GUI(config.json)で
// 同一の JSON 形式・同一の変換ロジックを共有するため、両アダプタがこのパッケージを
// 参照する。依存は内側(domain / usecase)にのみ向く。
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/junara/jheader-proxy/internal/domain"
	"github.com/junara/jheader-proxy/internal/usecase"
)

// defaultDuration は duration 未指定時の既定値(自動停止までの時間)。
const defaultDuration = "10m"

// HeaderKV は付与するヘッダー1件(Name/Value)。GUI フォームの1行に対応する。
type HeaderKV struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// RunConfig は設定ファイル(JSON)の全項目。GUI の直近設定 config.json と、
// CLI の --config はこの同一スキーマを共有する。
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

// Default は CA パスに依存しない普遍的な既定値を返す。
// CA の既定パスはアプリ固有のため、必要な呼び出し側(GUI)が上書きする。
func Default() RunConfig {
	return RunConfig{
		Listen:   ":8080",
		Duration: defaultDuration,
	}
}

// Load は path の JSON 設定ファイルを読み込む。明示されなかった項目は既定値で
// 埋まる(将来フィールドが増えても既定で補完される)。
func Load(path string) (RunConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Default(), fmt.Errorf("failed to read config %q: %w", path, err)
	}
	cfg := Default()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), fmt.Errorf("failed to parse config %q: %w", path, err)
	}
	return cfg, nil
}

// HeadersToSpecs は []HeaderKV を "Name=Value" 形式のスライスへ変換する。
// 名前が空の行は無視し、名前は前後の空白を除去する(値はそのまま)。
func HeadersToSpecs(headers []HeaderKV) []string {
	specs := make([]string, 0, len(headers))
	for _, h := range headers {
		name := strings.TrimSpace(h.Name)
		if name == "" {
			continue
		}
		specs = append(specs, name+"="+h.Value)
	}
	return specs
}

// ToRunProxyInput は RunConfig を RunProxy ユースケースの入力へ変換する正準ロジック。
// CLI と GUI の両方がこの関数を通すことで、ヘッダー解釈・duration 解釈・trim/空要素
// 除去のルールが一元化される。OnReady は呼び出し側で設定する。
func ToRunProxyInput(cfg RunConfig) (usecase.RunProxyInput, error) {
	headers, err := domain.ParseHeaders(HeadersToSpecs(cfg.Headers))
	if err != nil {
		return usecase.RunProxyInput{}, err
	}
	dur, err := ParseDuration(cfg.Duration)
	if err != nil {
		return usecase.RunProxyInput{}, err
	}
	return usecase.RunProxyInput{
		Listen:       strings.TrimSpace(cfg.Listen),
		Domains:      TrimNonEmpty(cfg.Domains),
		Headers:      headers,
		CACertPath:   cfg.CACertPath,
		CAKeyPath:    cfg.CAKeyPath,
		Allow:        TrimNonEmpty(cfg.Allow),
		RedactValues: cfg.Redact,
		Duration:     dur,
	}, nil
}

// TrimNonEmpty は各要素を前後の空白除去し、空要素を取り除いたスライスを返す。
// ドメインや許可リストの「空・空白だけの項目」を入口(CLI/GUI)に依らず一様に
// 落とすために使う。
func TrimNonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// ParseDuration は設定の duration 文字列を解釈する。"" / "0" は無制限(0)とする。
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}
	return d, nil
}
