// Package usecase はアプリケーションのビジネスルールを保持する。インフラ層が実装する
// ポート(interface)を通じて domain 層を協調動作させるため、このパッケージは domain
// 層と標準ライブラリにのみ依存する。
package usecase

import (
	"context"
	"crypto/tls"

	"github.com/junara/jheader-proxy/internal/domain"
)

// CAProvider は永続ストレージから HTTPS MITM 用のCAを読み込む。
type CAProvider interface {
	Load(certPath, keyPath string) (*tls.Certificate, error)
}

// CAGenerator は新しいCA証明書・秘密鍵を生成して永続化する。
// force が true の場合は既存ファイルを上書きする。
type CAGenerator interface {
	Generate(certPath, keyPath string, force bool) error
}

// ProxyConfig はプロキシ起動に必要な設定一式。
type ProxyConfig struct {
	Listen  string
	Matcher *domain.Matcher
	Headers domain.Headers
	CA      *tls.Certificate
	Allow   domain.AllowList
}

// ProxyServer は cfg に従ってプロキシを実行する。ctx がキャンセルされると
// 穏当に停止する。
type ProxyServer interface {
	Serve(ctx context.Context, cfg ProxyConfig) error
}

// Logger は人間可読の進捗メッセージを受け取る。
type Logger interface {
	Printf(format string, args ...any)
}
