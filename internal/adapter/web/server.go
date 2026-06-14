// Package web は GUI(ローカル Web 管理画面)のインターフェースアダプタ。
// ブラウザからの操作を usecase 入力へ変換してプロキシを制御する。usecase/domain
// にのみ依存し、インフラには依存しない(実装は合成ルートから Deps で注入される)。
package web

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "embed"
)

//go:embed static/index.html
var indexHTML string

// 標準出力・標準エラー(テストで差し替え可能にするため変数化)。
var (
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

// tokenPlaceholder は index.html 内で実トークンに置換されるプレースホルダ。
const tokenPlaceholder = "__JHEADER_TOKEN__" //nolint:gosec // 認証情報ではなく、配信時に置換するテンプレート用の目印

// Options は GUI サーバの起動オプション。
type Options struct {
	Listen  string // 管理画面の待受アドレス(例 127.0.0.1:9090)
	NoOpen  bool   // true ならブラウザを自動起動しない
	Version string // バージョン表示用
}

// server は HTTP ハンドラと依存をまとめる。
type server struct {
	ctrl    *Controller
	token   string
	version string
}

// Serve は管理画面を起動し、ctx がキャンセルされるまで提供する。
// 指定ポートが使用中なら(フォールバックせず)明確なエラーを返す。
func Serve(ctx context.Context, deps Deps, opts Options) error {
	token, err := newToken()
	if err != nil {
		return err
	}

	initial, err := LoadConfig()
	if err != nil {
		// 壊れた設定でも既定で起動できるよう、警告だけ出して続行する。
		_, _ = fmt.Fprintf(stderr, "warning: %v\n", err)
		initial = DefaultRunConfig()
	}

	ctrl := NewController(deps, NewLogSink(0), initial)
	srv := &server{ctrl: ctrl, token: token, version: opts.Version}

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", opts.Listen)
	if err != nil {
		return fmt.Errorf("failed to bind management UI to %s: %w (change it with --gui-listen)", opts.Listen, err)
	}

	url := displayURL(ln.Addr())
	httpSrv := &http.Server{
		Handler:           srv.routes(),
		ReadHeaderTimeout: 30 * time.Second,
	}

	// ctx キャンセルで管理画面とプロキシの両方を穏当に停止する。
	go func() {
		<-ctx.Done()
		ctrl.Stop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx) //nolint:contextcheck // 停止用に親ctxとは独立したタイムアウトを使う
	}()

	_, _ = fmt.Fprintf(stdout, "jheader-proxy GUI: %s\n", url)
	if !opts.NoOpen {
		openBrowser(ctx, url)
	}

	if err := httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/state", s.requireToken(s.handleState))
	mux.HandleFunc("/api/start", s.requireToken(s.handleStart))
	mux.HandleFunc("/api/stop", s.requireToken(s.handleStop))
	mux.HandleFunc("/api/gen-ca", s.requireToken(s.handleGenCA))
	mux.HandleFunc("/api/reveal", s.requireToken(s.handleReveal))
	mux.HandleFunc("/api/logs", s.handleLogs) // SSE: トークンはクエリで検証
	return requireSafeHost(mux)
}

// requireSafeHost は Host ヘッダが localhost かリテラル IP のときだけ通す。
// DNS リバインディング(攻撃者ドメインを 127.0.0.1 に解決させ同一オリジン化して
// トークンを盗む)を防ぐ。リバインディングは必ず「ドメイン名」を使うため、
// リテラル IP の Host は安全に許可できる。これにより --gui-listen で LAN アドレス
// (例 0.0.0.0 / 192.168.x.x)にバインドして別端末から開く運用も壊さない。
func requireSafeHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if host == "localhost" || net.ParseIP(host) != nil {
			next.ServeHTTP(w, r)
			return
		}
		writeErr(w, http.StatusForbidden, "invalid host")
	})
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// トークンはプロセス毎に再生成され、HTML も更新されうるため、古い内容を
	// ブラウザがキャッシュしないようにする。
	w.Header().Set("Cache-Control", "no-store")
	page := strings.ReplaceAll(indexHTML, tokenPlaceholder, s.token)
	_, _ = io.WriteString(w, page)
}

func (s *server) handleState(w http.ResponseWriter, _ *http.Request) {
	view := s.ctrl.State()
	view.Config.Domains = ensureSlice(view.Config.Domains)
	view.Config.Allow = ensureSlice(view.Config.Allow)
	writeJSON(w, http.StatusOK, stateResponse{StateView: view, LANIP: lanIPv4(), Version: s.version})
}

func (s *server) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var cfg RunConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := s.ctrl.Start(cfg); err != nil { //nolint:contextcheck // プロキシは HTTP リクエストとは独立したライフサイクルで動く
		if errors.Is(err, ErrAlreadyRunning) {
			writeErr(w, http.StatusConflict, err.Error())
			return
		}
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	// 起動成功時のみ直近設定として保存する。
	if err := SaveConfig(cfg); err != nil {
		_, _ = fmt.Fprintf(stderr, "warning: %v\n", err)
	}
	s.handleState(w, r)
}

func (s *server) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	s.ctrl.Stop()
	s.handleState(w, r)
}

func (s *server) handleGenCA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req struct {
		CertPath string `json:"certPath"`
		KeyPath  string `json:"keyPath"`
		Force    bool   `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := s.ctrl.GenerateCA(req.CertPath, req.KeyPath, req.Force); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"certPath": req.CertPath,
		"keyPath":  req.KeyPath,
	})
}

func (s *server) handleReveal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Path == "" {
		writeErr(w, http.StatusBadRequest, "path is required")
		return
	}
	// OS のファイルマネージャでファイルの場所を開く。パスはローカルユーザ自身の入力で、
	// 引数として直接渡す(シェル経由ではない)。リクエスト終了で kill されないよう
	// 独立した context で起動する。
	revealCmd := revealCommand(context.Background(), req.Path) //nolint:contextcheck
	if err := revealCmd.Start(); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to open file location: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// revealCommand はファイルの場所を OS のファイルマネージャで開くコマンドを返す。
// macOS は Finder で当該ファイルを選択表示、Windows はエクスプローラーで選択表示し、
// その他(Linux 等)は含まれるフォルダを既定のファイルマネージャ(xdg-open)で開く。
func revealCommand(ctx context.Context, path string) *exec.Cmd {
	switch runtime.GOOS {
	case "darwin":
		return exec.CommandContext(ctx, "open", "-R", path) //nolint:gosec // パスはローカルユーザ入力・シェル非経由
	case "windows":
		return exec.CommandContext(ctx, "explorer", "/select,"+path) //nolint:gosec // 同上
	default:
		return exec.CommandContext(ctx, "xdg-open", filepath.Dir(path)) //nolint:gosec // フォルダを開く
	}
}

func (s *server) handleLogs(w http.ResponseWriter, r *http.Request) {
	// EventSource はヘッダを付けられないため、SSE はクエリでトークンを検証する。
	if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("token")), []byte(s.token)) != 1 {
		writeErr(w, http.StatusUnauthorized, "invalid token")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch, cancel := s.ctrl.Sink().Subscribe()
	defer cancel()

	for _, line := range s.ctrl.Sink().Snapshot() {
		writeSSE(w, line)
	}
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case line, ok := <-ch:
			if !ok {
				return
			}
			writeSSE(w, line)
			flusher.Flush()
		}
	}
}

// requireToken は /api/* に対し X-JHeader-Token ヘッダを検証するミドルウェア。
func (s *server) requireToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Jheader-Token")), []byte(s.token)) != 1 {
			writeErr(w, http.StatusUnauthorized, "invalid token")
			return
		}
		next(w, r)
	}
}

type stateResponse struct {
	StateView

	LANIP   string `json:"lanIp"`
	Version string `json:"version"`
}

func writeSSE(w io.Writer, line string) {
	// data フィールドに改行を含めない(改行はイベント境界になるため)。
	_, _ = fmt.Fprintf(w, "data: %s\n\n", strings.ReplaceAll(line, "\n", " "))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		_, _ = fmt.Fprintf(stderr, "warning: failed to encode response: %v\n", err)
	}
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// ensureSlice は nil を空スライスに正規化する(JSON で null ではなく [] にする)。
func ensureSlice(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// displayURL は実際にバインドされたアドレスからブラウザで開く URL を組み立てる。
func displayURL(addr net.Addr) string {
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return "http://" + addr.String()
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port)
}

// openBrowser はプラットフォームに応じて既定ブラウザで url を開く。失敗は無視する
// (URL は stdout にも出しているため)。
func openBrowser(ctx context.Context, url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}
	// コマンドは固定、URL は自前生成。失敗は無視する。
	_ = exec.CommandContext(ctx, cmd, args...).Start() //nolint:gosec // G204
}
