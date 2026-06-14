package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/junara/jheader-proxy/internal/domain"
)

// RunProxyInput は RunProxy ユースケースの入力。
type RunProxyInput struct {
	Listen       string
	Domains      []string
	Headers      domain.Headers
	CACertPath   string
	CAKeyPath    string
	Allow        []string
	RedactValues bool
	// Duration は自動停止までの時間。0 以下なら無制限。
	Duration time.Duration
	// OnReady は待受開始に成功した直後に一度だけ呼ばれる(任意)。
	OnReady func()
}

// caExpiryWarnThreshold はCA証明書の残り有効期間がこれを下回ると起動ログで
// 警告する閾値。
const caExpiryWarnThreshold = 14 * 24 * time.Hour

// logCAExpiry はCA証明書の有効期限と残り日数を起動ログに出す。期限切れ・期限間近は
// 警告する。検証端末でCAを更新し忘れて MITM が突然失敗するのを未然に気付けるようにする。
func logCAExpiry(logger Logger, notAfter time.Time) {
	const day = 24 * time.Hour
	date := notAfter.Format("2006-01-02")
	remaining := time.Until(notAfter)
	switch {
	case remaining <= 0:
		logger.Printf("WARNING: CA certificate expired on %s; regenerate it with --gen-ca --force", date)
	case remaining <= caExpiryWarnThreshold:
		logger.Printf("WARNING: CA certificate expires soon: %s (in %d days)", date, int(remaining/day))
	default:
		logger.Printf("CA expires: %s (in %d days)", date, int(remaining/day))
	}
}

// RunProxy はCAを読み込み、ヘッダーを付与するプロキシを起動する。
type RunProxy struct {
	ca     CAProvider
	server ProxyServer
	logger Logger
}

// NewRunProxy は依存を注入して RunProxy を構築する。
func NewRunProxy(ca CAProvider, server ProxyServer, logger Logger) *RunProxy {
	return &RunProxy{ca: ca, server: server, logger: logger}
}

// Execute は入力を検証し、CAを読み込み、ctx がキャンセルされるか失敗するまで
// プロキシを提供する。
func (u *RunProxy) Execute(ctx context.Context, in RunProxyInput) error {
	if len(in.Domains) == 0 {
		return errors.New("at least one --domain is required")
	}
	if in.Headers.Len() == 0 {
		return errors.New("at least one --header is required")
	}
	if in.CACertPath == "" || in.CAKeyPath == "" {
		return errors.New("--ca-cert and --ca-key are required (generate one with --gen-ca)")
	}

	allow, err := domain.NewAllowList(in.Allow)
	if err != nil {
		return err
	}
	cert, err := u.ca.Load(in.CACertPath, in.CAKeyPath)
	if err != nil {
		return err
	}
	matcher := domain.NewMatcher(in.Domains)

	// 停止忘れ防止のため、指定時間が過ぎたら ctx をキャンセルして自動停止する。
	if in.Duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, in.Duration)
		defer cancel()
	}

	u.logger.Printf("proxy listening on %s", in.Listen)
	u.logger.Printf("target domains: %s", strings.Join(in.Domains, ", "))
	u.logger.Printf("CA certificate: %s", in.CACertPath)
	if cert.Leaf != nil {
		logCAExpiry(u.logger, cert.Leaf.NotAfter)
	}
	if in.Duration > 0 {
		u.logger.Printf("auto-stop after %s", in.Duration)
	}
	if !allow.AllowsAll() {
		u.logger.Printf("allowed clients: %s", strings.Join(in.Allow, ", "))
	}
	u.logger.Printf("headers:")
	in.Headers.Each(func(name, value string) {
		if in.RedactValues || domain.IsSensitiveHeader(name) {
			value = "***"
		}
		u.logger.Printf("  %s: %s", name, value)
	})

	return u.server.Serve(ctx, ProxyConfig{
		Listen:  in.Listen,
		Matcher: matcher,
		Headers: in.Headers,
		CA:      cert,
		Allow:   allow,
		OnReady: in.OnReady,
	})
}
