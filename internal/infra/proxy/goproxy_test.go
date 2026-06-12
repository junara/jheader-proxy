package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/junara/jheader-proxy/internal/domain"
	"github.com/junara/jheader-proxy/internal/infra/ca"
	"github.com/junara/jheader-proxy/internal/usecase"
)

type nopLogger struct{}

func (nopLogger) Printf(string, ...any) {}

func TestCleanShutdownErr(t *testing.T) {
	if err := cleanShutdownErr(nil); err != nil {
		t.Errorf("cleanShutdownErr(nil) = %v, want nil", err)
	}
	// 期限切れ（閉じない接続が残った）は正常停止扱いにする。
	if err := cleanShutdownErr(context.DeadlineExceeded); err != nil {
		t.Errorf("cleanShutdownErr(DeadlineExceeded) = %v, want nil", err)
	}
	// それ以外のエラーはそのまま返す。
	want := errors.New("boom")
	if err := cleanShutdownErr(want); !errors.Is(err, want) {
		t.Errorf("cleanShutdownErr(boom) = %v, want boom", err)
	}
}

// testCA は一時CAを生成・ロードして返す。
func testCA(t *testing.T) *tls.Certificate {
	t.Helper()
	dir := t.TempDir()
	cert := filepath.Join(dir, "ca.pem")
	key := filepath.Join(dir, "key.pem")
	store := ca.New()
	if err := store.Generate(cert, key, false); err != nil {
		t.Fatalf("generate CA: %v", err)
	}
	loaded, err := store.Load(cert, key)
	if err != nil {
		t.Fatalf("load CA: %v", err)
	}
	return loaded
}

// freeAddr は使用可能な 127.0.0.1 のアドレスを返す。
func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

// startProxy はプロキシを起動し、待ち受け開始を待ってアドレスを返す。
func startProxy(t *testing.T, opts Options, cfg usecase.ProxyConfig) (proxyURL string, stop func()) {
	t.Helper()
	addr := freeAddr(t)
	cfg.Listen = addr
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := New(nopLogger{}, opts).Serve(ctx, cfg); err != nil {
			t.Errorf("Serve returned error: %v", err)
		}
	}()

	// 待ち受け開始までポーリング。
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.Dial("tcp", addr)
		if err == nil {
			_ = c.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	return "http://" + addr, func() {
		cancel()
		<-done
	}
}

func TestServeAddsHeadersToTargetOverHTTP(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Echo-Debug", r.Header.Get("X-Debug-User"))
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	upHost := mustHost(t, upstream.URL)

	headers, _ := domain.ParseHeaders([]string{"X-Debug-User=jun"})
	proxyURL, stop := startProxy(t, Options{}, usecase.ProxyConfig{
		Matcher: domain.NewMatcher([]string{upHost}),
		Headers: headers,
		CA:      testCA(t),
	})
	defer stop()

	header := proxyGet(t, proxyURL, upstream.URL)
	if got := header.Get("X-Echo-Debug"); got != "jun" {
		t.Errorf("target request: X-Debug-User echoed = %q, want jun", got)
	}
}

func TestServeDoesNotAddHeadersToNonTarget(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Echo-Debug", r.Header.Get("X-Debug-User"))
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	headers, _ := domain.ParseHeaders([]string{"X-Debug-User=jun"})
	proxyURL, stop := startProxy(t, Options{}, usecase.ProxyConfig{
		Matcher: domain.NewMatcher([]string{"other.test"}), // 上流は対象外
		Headers: headers,
		CA:      testCA(t),
	})
	defer stop()

	header := proxyGet(t, proxyURL, upstream.URL)
	if got := header.Get("X-Echo-Debug"); got != "" {
		t.Errorf("non-target request: X-Debug-User should be absent, got %q", got)
	}
}

func TestServeDeniesDisallowedClient(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	// 127.0.0.1 のクライアントを許可しないリストにする。
	allow, _ := domain.NewAllowList([]string{"10.0.0.1"})
	headers, _ := domain.ParseHeaders([]string{"X-Debug-User=jun"})
	proxyURL, stop := startProxy(t, Options{}, usecase.ProxyConfig{
		Matcher: domain.NewMatcher([]string{mustHost(t, upstream.URL)}),
		Headers: headers,
		CA:      testCA(t),
		Allow:   allow,
	})
	defer stop()

	client := proxyClient(t, proxyURL)
	client.Timeout = 2 * time.Second
	resp, err := client.Get(upstream.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Error("request from disallowed client succeeded, want failure")
	}
}

func mustHost(t *testing.T, rawURL string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	return u.Hostname()
}

func proxyClient(t *testing.T, proxyURL string) *http.Client {
	t.Helper()
	pu, err := url.Parse(proxyURL)
	if err != nil {
		t.Fatalf("parse proxy url: %v", err)
	}
	return &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(pu)},
		Timeout:   3 * time.Second,
	}
}

// proxyGet はプロキシ経由で target を GET し、レスポンスヘッダーを返す。
// ボディはこの関数内で読み切って閉じる。
func proxyGet(t *testing.T, proxyURL, target string) http.Header {
	t.Helper()
	resp, err := proxyClient(t, proxyURL).Get(target)
	if err != nil {
		t.Fatalf("proxied GET: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return resp.Header
}
