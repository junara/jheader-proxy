package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testToken = "test-token"

func newTestServer() *server {
	ctrl := NewController(testDeps(blockingServer{}), NewLogSink(0), DefaultRunConfig())
	return &server{ctrl: ctrl, token: testToken, version: "test"}
}

func TestStateRequiresToken(t *testing.T) {
	h := newTestServer().routes()

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/state", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rr.Code)
	}
}

func TestRejectsNonLocalHost(t *testing.T) {
	h := newTestServer().routes()

	// DNS リバインディングを模した、ドメイン名の Host。トークンが正しくても拒否する。
	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	req.Host = "attacker.example.com"
	req.Header.Set("X-Jheader-Token", testToken)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for domain-name host, got %d", rr.Code)
	}
}

func TestAllowsLiteralIPHost(t *testing.T) {
	h := newTestServer().routes()

	// --gui-listen で LAN アドレスにバインドし、別端末からリテラル IP で開く運用。
	// リテラル IP はリバインドできないため許可する(トークン検証は別途行われる)。
	for _, host := range []string{"192.168.1.5:9090", "[::1]:9090", "10.0.0.2"} {
		req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
		req.Host = host
		req.Header.Set("X-Jheader-Token", testToken)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 for literal IP host %q, got %d", host, rr.Code)
		}
	}
}

func TestStateWithToken(t *testing.T) {
	h := newTestServer().routes()

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/state", nil)
	req.Header.Set("X-Jheader-Token", testToken)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	var resp stateResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Running {
		t.Fatal("expected not running initially")
	}
}

func TestStartStopViaHTTP(t *testing.T) {
	useTempConfigDir(t) // handleStart が直近設定を保存するため実 HOME を汚さない

	s := newTestServer()
	h := s.routes()

	body := `{"listen":":0","domains":["x.test"],"headers":[{"name":"X","value":"1"}],"caCertPath":"c","caKeyPath":"k","duration":"0"}`
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/start", strings.NewReader(body))
	req.Header.Set("X-Jheader-Token", testToken)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("start expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if !s.ctrl.State().Running {
		t.Fatal("expected running after /api/start")
	}

	stopReq := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/stop", nil)
	stopReq.Header.Set("X-Jheader-Token", testToken)
	stopRR := httptest.NewRecorder()
	h.ServeHTTP(stopRR, stopReq)
	if stopRR.Code != http.StatusOK {
		t.Fatalf("stop expected 200, got %d", stopRR.Code)
	}
	if s.ctrl.State().Running {
		t.Fatal("expected stopped after /api/stop")
	}
}

func TestIndexServesTokenInjected(t *testing.T) {
	h := newTestServer().routes()

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for index, got %d", rr.Code)
	}
	body := rr.Body.String()
	if strings.Contains(body, tokenPlaceholder) {
		t.Fatal("token placeholder was not replaced")
	}
	if !strings.Contains(body, testToken) {
		t.Fatal("expected real token to be injected into HTML")
	}
}
