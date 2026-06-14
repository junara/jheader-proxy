package web

import (
	"context"
	"crypto/tls"
	"errors"
	"strings"
	"testing"

	"github.com/junara/jheader-proxy/internal/usecase"
)

type fakeCA struct{}

func (fakeCA) Load(_, _ string) (*tls.Certificate, error) { return &tls.Certificate{}, nil }
func (fakeCA) Generate(_, _ string, _ bool) error         { return nil }

// blockingServer は待受開始を通知し、ctx がキャンセルされるまでブロックする。
type blockingServer struct{}

func (blockingServer) Serve(ctx context.Context, cfg usecase.ProxyConfig) error {
	if cfg.OnReady != nil {
		cfg.OnReady()
	}
	<-ctx.Done()
	return nil
}

// failingServer は待受開始前にエラーを返す(ポート衝突などを模す)。
type failingServer struct{ err error }

func (f failingServer) Serve(_ context.Context, _ usecase.ProxyConfig) error { return f.err }

func testDeps(server usecase.ProxyServer) Deps {
	return Deps{
		NewProxyServer: func(_ usecase.Logger, _, _ bool) usecase.ProxyServer { return server },
		CAProvider:     fakeCA{},
		CAGenerator:    fakeCA{},
	}
}

func validCfg() RunConfig {
	return RunConfig{
		Listen:     ":0",
		Domains:    []string{"example.test"},
		Headers:    []HeaderKV{{Name: "X-Debug", Value: "1"}},
		CACertPath: "cert.pem",
		CAKeyPath:  "key.pem",
		Duration:   "0",
	}
}

func TestControllerStartStop(t *testing.T) {
	c := NewController(testDeps(blockingServer{}), NewLogSink(0), RunConfig{})
	if err := c.Start(validCfg()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !c.State().Running {
		t.Fatal("expected running after Start")
	}
	c.Stop()
	if c.State().Running {
		t.Fatal("expected stopped after Stop")
	}
}

func TestControllerDoubleStart(t *testing.T) {
	c := NewController(testDeps(blockingServer{}), NewLogSink(0), RunConfig{})
	if err := c.Start(validCfg()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Stop()
	if err := c.Start(validCfg()); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("expected ErrAlreadyRunning, got %v", err)
	}
}

func TestControllerStartupFailure(t *testing.T) {
	boom := errors.New("listen: address already in use")
	c := NewController(testDeps(failingServer{err: boom}), NewLogSink(0), RunConfig{})
	err := c.Start(validCfg())
	if err == nil || !strings.Contains(err.Error(), "address already in use") {
		t.Fatalf("expected startup failure, got %v", err)
	}
	if c.State().Running {
		t.Fatal("expected not running after startup failure")
	}
}

func TestControllerStartValidationError(t *testing.T) {
	c := NewController(testDeps(blockingServer{}), NewLogSink(0), RunConfig{})
	cfg := validCfg()
	cfg.Domains = nil // usecase 側で「at least one --domain」エラーになる
	err := c.Start(cfg)
	if err == nil || !strings.Contains(err.Error(), "domain") {
		t.Fatalf("expected domain validation error, got %v", err)
	}
	if c.State().Running {
		t.Fatal("expected not running after validation error")
	}
}

func TestControllerRemaining(t *testing.T) {
	c := NewController(testDeps(blockingServer{}), NewLogSink(0), RunConfig{})
	cfg := validCfg()
	cfg.Duration = "10m"
	if err := c.Start(cfg); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Stop()
	st := c.State()
	if st.RemainingSeconds <= 0 || st.RemainingSeconds > 600 {
		t.Fatalf("unexpected remaining: %d", st.RemainingSeconds)
	}
}
