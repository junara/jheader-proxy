package usecase

import (
	"context"
	"crypto/tls"
	"errors"
	"testing"

	"github.com/junara/jheader-proxy/internal/domain"
)

type fakeCAProvider struct {
	cert *tls.Certificate
	err  error
}

func (f *fakeCAProvider) Load(_, _ string) (*tls.Certificate, error) {
	return f.cert, f.err
}

type fakeProxyServer struct {
	called bool
	err    error
}

func (f *fakeProxyServer) Serve(_ context.Context, _ ProxyConfig) error {
	f.called = true
	return f.err
}

type nopLogger struct{}

func (nopLogger) Printf(string, ...any) {}

func validInput(t *testing.T) RunProxyInput {
	t.Helper()
	h, err := domain.ParseHeaders([]string{"X-Debug-User=jun"})
	if err != nil {
		t.Fatalf("ParseHeaders: %v", err)
	}
	return RunProxyInput{
		Listen:     ":8080",
		Domains:    []string{"example.test"},
		Headers:    h,
		CACertPath: "cert.pem",
		CAKeyPath:  "key.pem",
	}
}

func TestRunProxyExecuteSuccess(t *testing.T) {
	server := &fakeProxyServer{}
	uc := NewRunProxy(&fakeCAProvider{cert: &tls.Certificate{}}, server, nopLogger{})

	if err := uc.Execute(context.Background(), validInput(t)); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !server.called {
		t.Error("Serve was not called")
	}
}

func TestRunProxyValidation(t *testing.T) {
	base := validInput(t)

	noDomain := base
	noDomain.Domains = nil

	noHeader := base
	noHeader.Headers = domain.Headers{}

	noCA := base
	noCA.CACertPath = ""

	badAllow := base
	badAllow.Allow = []string{"not-an-ip"}

	for name, in := range map[string]RunProxyInput{
		"no domain":     noDomain,
		"no header":     noHeader,
		"no ca":         noCA,
		"invalid allow": badAllow,
	} {
		server := &fakeProxyServer{}
		uc := NewRunProxy(&fakeCAProvider{cert: &tls.Certificate{}}, server, nopLogger{})
		if err := uc.Execute(context.Background(), in); err == nil {
			t.Errorf("%s: Execute returned nil error, want error", name)
		}
		if server.called {
			t.Errorf("%s: Serve should not be called on invalid input", name)
		}
	}
}

func TestRunProxyCALoadError(t *testing.T) {
	server := &fakeProxyServer{}
	uc := NewRunProxy(&fakeCAProvider{err: errors.New("boom")}, server, nopLogger{})

	if err := uc.Execute(context.Background(), validInput(t)); err == nil {
		t.Fatal("Execute returned nil error, want CA load error")
	}
	if server.called {
		t.Error("Serve should not be called when CA load fails")
	}
}
