package proxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
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
func startProxy(t *testing.T, cfg usecase.ProxyConfig) (proxyURL string, stop func()) {
	t.Helper()
	addr := freeAddr(t)
	cfg.Listen = addr
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := New(nopLogger{}, Options{}).Serve(ctx, cfg); err != nil {
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
	proxyURL, stop := startProxy(t, usecase.ProxyConfig{
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
	proxyURL, stop := startProxy(t, usecase.ProxyConfig{
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

// TestServeTunnelsNonTargetOverHTTPSWithoutHeader は、対象外ドメインへの HTTPS は
// MITM せず素のトンネルとして通し、ヘッダーが付与されない(復号もしない)ことを確認する。
func TestServeTunnelsNonTargetOverHTTPSWithoutHeader(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Echo-Debug", r.Header.Get("X-Debug-User"))
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	caCert := testCA(t)
	headers, _ := domain.ParseHeaders([]string{"X-Debug-User=jun"})
	proxyURL, stop := startProxy(t, usecase.ProxyConfig{
		Matcher: domain.NewMatcher([]string{"other.test"}), // 上流(127.0.0.1)は対象外 → トンネル
		Headers: headers,
		CA:      caCert,
	})
	defer stop()

	// トンネルなので MITM されない。クライアントは上流の実サーバ証明書を直接検証する。
	pool := x509.NewCertPool()
	pool.AddCert(upstream.Certificate())
	client := httpsProxyClient(t, proxyURL, pool)
	resp, err := client.Get(upstream.URL)
	if err != nil {
		t.Fatalf("tunneled HTTPS GET: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("X-Echo-Debug"); got != "" {
		t.Errorf("tunneled (non-target): header should be absent, got %q", got)
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
	proxyURL, stop := startProxy(t, usecase.ProxyConfig{
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

func TestIsCAPortalHost(t *testing.T) {
	cases := map[string]bool{
		"jheader.proxy":    true,
		"jheader.proxy:80": true,
		"JHEADER.PROXY":    true, // ホスト名は大文字小文字を区別しない
		"example.test":     false,
		"":                 false,
	}
	for host, want := range cases {
		if got := isCAPortalHost(host); got != want {
			t.Errorf("isCAPortalHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestServeCAPortal(t *testing.T) {
	headers, _ := domain.ParseHeaders([]string{"X-Debug-User=jun"})
	caCert := testCA(t)
	proxyURL, stop := startProxy(t, usecase.ProxyConfig{
		Matcher: domain.NewMatcher([]string{"example.test"}),
		Headers: headers,
		CA:      caCert,
	})
	defer stop()

	client := proxyClient(t, proxyURL)

	// 案内ページ
	resp, err := client.Get("http://" + caPortalHost + "/")
	if err != nil {
		t.Fatalf("portal landing: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("landing status = %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "/cert.crt") {
		t.Error("landing page missing download link")
	}

	// 証明書本体
	certResp, err := client.Get("http://" + caPortalHost + "/cert.crt")
	if err != nil {
		t.Fatalf("portal cert: %v", err)
	}
	certBody, _ := io.ReadAll(certResp.Body)
	_ = certResp.Body.Close()
	if ct := certResp.Header.Get("Content-Type"); ct != "application/x-x509-ca-cert" {
		t.Errorf("cert content-type = %q, want application/x-x509-ca-cert", ct)
	}
	block, _ := pem.Decode(certBody)
	if block == nil {
		t.Fatal("served cert is not PEM")
	}
	if !bytes.Equal(block.Bytes, caCert.Certificate[0]) {
		t.Error("served cert does not match the CA")
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

// httpsProxyClient はプロキシ経由で、roots を信頼して TLS 検証する HTTP クライアントを返す。
func httpsProxyClient(t *testing.T, proxyURL string, roots *x509.CertPool) *http.Client {
	t.Helper()
	pu, err := url.Parse(proxyURL)
	if err != nil {
		t.Fatalf("parse proxy url: %v", err)
	}
	return &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(pu),
			TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12},
		},
		Timeout: 5 * time.Second,
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
