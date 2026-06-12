// Package proxy は usecase.ProxyServer ポートのインフラ実装で、
// github.com/elazarl/goproxy を用いる。
package proxy

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"time"

	"github.com/elazarl/goproxy"

	"github.com/junara/jheader-proxy/internal/domain"
	"github.com/junara/jheader-proxy/internal/usecase"
)

// Logger は人間可読の進捗メッセージを受け取る。
type Logger interface {
	Printf(format string, args ...any)
}

// Options は Server のログ挙動を設定する。
type Options struct {
	Quiet   bool // リクエストごとのログを抑制する
	Verbose bool // レスポンスのログも出力する
}

// Server は対象ドメインのみ MITM する HTTP/HTTPS プロキシを実行する。
type Server struct {
	logger Logger
	opts   Options
}

// New は goproxy ベースの Server を返す。
func New(logger Logger, opts Options) *Server {
	return &Server{logger: logger, opts: opts}
}

// Serve はプロキシを構築し、ctx がキャンセルされるか失敗するまで提供をブロックする。
//
// matcher が受理したホストのみ(cfg.CA を用いて)MITM し、ヘッダーを付与する。
// それ以外の HTTPS 接続は素の CONNECT トンネルとして素通しする。
// goproxy のパッケージグローバルは変更せず、CONNECT アクションに直接 TLS 設定を持たせる。
func (s *Server) Serve(ctx context.Context, cfg usecase.ProxyConfig) error {
	proxy := goproxy.NewProxyHttpServer()

	// ロード済みCAから動的にサーバ証明書を発行する TLS 設定。
	tlsConfig := goproxy.TLSConfigFromCA(cfg.CA)
	mitm := &goproxy.ConnectAction{Action: goproxy.ConnectMitm, TLSConfig: tlsConfig}
	tunnel := &goproxy.ConnectAction{Action: goproxy.ConnectAccept, TLSConfig: tlsConfig}

	proxy.OnRequest().HandleConnectFunc(func(host string, _ *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		if cfg.Matcher.IsTarget(host) {
			s.requestLogf("[MITM] %s", host)
			return mitm, host
		}
		s.requestLogf("[TUNNEL] %s", host)
		return tunnel, host
	})

	proxy.OnRequest().DoFunc(func(req *http.Request, _ *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		if cfg.Matcher.IsTarget(req.Host) {
			cfg.Headers.Each(func(name, value string) {
				req.Header.Set(name, value)
			})
			s.requestLogf("[ADD HEADER] %s %s", req.Method, req.URL)
		}
		return req, nil
	})

	if s.opts.Verbose {
		proxy.OnResponse().DoFunc(func(resp *http.Response, pctx *goproxy.ProxyCtx) *http.Response {
			if resp != nil && pctx.Req != nil && cfg.Matcher.IsTarget(pctx.Req.Host) {
				s.logger.Printf("[RESP] %d %s", resp.StatusCode, pctx.Req.URL)
			}
			return resp
		})
	}

	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", cfg.Listen)
	if err != nil {
		return err
	}
	if !cfg.Allow.AllowsAll() {
		listener = &filteredListener{Listener: listener, allow: cfg.Allow, logger: s.logger}
	}

	srv := &http.Server{
		Handler:           proxy,
		ReadHeaderTimeout: 30 * time.Second,
	}

	// Serve がどの理由で終了しても goroutine を確実に終わらせるためのキャンセル。
	serveCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	shutdownErr := make(chan error, 1)
	go func() {
		<-serveCtx.Done()
		if ctx.Err() == nil {
			// 停止要求ではなく Serve 自体が終了した（cancel() による）。何もしない。
			shutdownErr <- nil
			return
		}
		s.logger.Printf("shutting down...")
		shutdownCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		err := srv.Shutdown(shutdownCtx) //nolint:contextcheck // 停止用に親ctxとは独立したタイムアウトを使う
		if errors.Is(err, context.DeadlineExceeded) {
			// 期限内に閉じない接続が残った。強制クローズして正常終了扱いにする。
			_ = srv.Close()
		}
		shutdownErr <- cleanShutdownErr(err)
	}()

	err = srv.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		// ctx キャンセルによる正常終了。Shutdown の完了を待つ。
		return <-shutdownErr
	}
	return err
}

// cleanShutdownErr は Shutdown の結果を正常終了かどうかに正規化する。
// 期限切れ（閉じない接続が残ったため強制クローズした）は、停止忘れ防止の
// 自動停止や Ctrl+C による正常な停止なので nil として扱う。
func cleanShutdownErr(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return nil
	}
	return err
}

// requestLogf は quiet でないときだけリクエストごとのログを出す。
func (s *Server) requestLogf(format string, args ...any) {
	if s.opts.Quiet {
		return
	}
	s.logger.Printf(format, args...)
}

// filteredListener は許可リストにないクライアントの接続を受理時に拒否する。
type filteredListener struct {
	net.Listener

	allow  domain.AllowList
	logger Logger
}

func (l *filteredListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		if l.allowed(conn) {
			return conn, nil
		}
		l.logger.Printf("[DENY] %s", conn.RemoteAddr())
		_ = conn.Close()
	}
}

func (l *filteredListener) allowed(conn net.Conn) bool {
	host, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		return false
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	return l.allow.Allows(addr)
}
