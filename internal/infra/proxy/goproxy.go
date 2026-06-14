// Package proxy は usecase.ProxyServer ポートのインフラ実装で、
// github.com/elazarl/goproxy を用いる。
package proxy

import (
	"bytes"
	"context"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	_ "embed"

	"github.com/elazarl/goproxy"

	"github.com/junara/jheader-proxy/internal/domain"
	"github.com/junara/jheader-proxy/internal/usecase"
)

// caPortalHost はプロキシ経由で CA 証明書を配布するための特別なホスト名。
// プロキシを設定した端末で http://jheader.proxy を開くと、証明書をダウンロード
// できる(mitmproxy の mitm.it と同じ仕組み)。秘密鍵は配信しない。
const caPortalHost = "jheader.proxy"

// caPortalHTML は CA ポータルの案内ページ。
//
//go:embed ca_portal.html
var caPortalHTML string

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

	// CA ポータル(http://jheader.proxy)で配布する CA 証明書を PEM 化しておく。
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cfg.CA.Certificate[0]})

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

	//nolint:bodyclose // 返す *http.Response は goproxy が書き出して閉じる
	proxy.OnRequest().DoFunc(s.handleRequest(cfg, caPEM))

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

	// 待受開始に成功した。GUI 等が起動成功を検知できるよう通知する。
	if cfg.OnReady != nil {
		cfg.OnReady()
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

// handleRequest はプロキシに届いた HTTP リクエストの処理関数を返す。CA ポータル宛なら
// 証明書を配布し、対象ドメイン宛ならヘッダーを付与する。
func (s *Server) handleRequest(
	cfg usecase.ProxyConfig, caPEM []byte,
) func(*http.Request, *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	return func(req *http.Request, _ *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		// CA ポータル: プロキシ経由で CA 証明書を配布する(モバイル端末向け)。
		if isCAPortalHost(req.Host) {
			s.requestLogf("[CA PORTAL] %s %s", req.Method, req.URL.Path)
			return req, caPortalResponse(req, caPEM)
		}
		if cfg.Matcher.IsTarget(req.Host) {
			cfg.Headers.Each(func(name, value string) {
				req.Header.Set(name, value)
			})
			s.requestLogf("[ADD HEADER] %s %s", req.Method, req.URL)
		}
		return req, nil
	}
}

// isCAPortalHost は要求先が CA ポータルのホストかどうかを返す(ポートは無視)。
func isCAPortalHost(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.EqualFold(host, caPortalHost)
}

// caPortalResponse は CA ポータルのレスポンスを返す。/cert 配下は証明書本体を、
// それ以外は案内ページを返す。
func caPortalResponse(req *http.Request, caPEM []byte) *http.Response {
	if strings.HasPrefix(req.URL.Path, "/cert") {
		// iOS はこの Content-Type でプロファイルのインストールを促す。Android では
		// ダウンロード後に「CA証明書」としてインストールする。
		return newProxyResponse(req, http.StatusOK, "application/x-x509-ca-cert", caPEM, map[string]string{
			"Content-Disposition": `attachment; filename="jheader-proxy-ca.crt"`,
		})
	}
	return newProxyResponse(req, http.StatusOK, "text/html; charset=utf-8", []byte(caPortalHTML), nil)
}

// newProxyResponse は goproxy が上流へ転送せず返す簡易レスポンスを組み立てる。
func newProxyResponse(
	req *http.Request, status int, contentType string, body []byte, extra map[string]string,
) *http.Response {
	resp := &http.Response{
		StatusCode:    status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}
	resp.Header.Set("Content-Type", contentType)
	for k, v := range extra {
		resp.Header.Set(k, v)
	}
	return resp
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
