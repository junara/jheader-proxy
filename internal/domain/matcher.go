// Package domain は jheader-proxy のエンタープライズビジネスルール、すなわち
// どのホストを対象とし、どのヘッダーを付与するかを保持する。標準ライブラリにのみ
// 依存する。
package domain

import (
	"net"
	"strings"
)

// Matcher は、ホストが設定された対象ドメイン集合に属するかを判定する。
type Matcher struct {
	domains []string
}

// NewMatcher は与えられたドメインを正規化して Matcher を返す。
func NewMatcher(domains []string) *Matcher {
	normalized := make([]string, 0, len(domains))
	for _, d := range domains {
		normalized = append(normalized, strings.ToLower(strings.TrimSpace(d)))
	}
	return &Matcher{domains: normalized}
}

// IsTarget は host が対象ドメインと一致するか、そのサブドメインかを返す。
//
// strings.Contains は意図的に使わない。"example.test" に対して
// "example.test.evil.com" のようなホストを誤って対象としないためである。
func (m *Matcher) IsTarget(host string) bool {
	host = NormalizeHost(host)
	for _, d := range m.domains {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

// NormalizeHost は host を小文字化し、ポート番号と前後の空白を除去する。
func NormalizeHost(host string) string {
	host = strings.TrimSpace(host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.ToLower(host)
}
