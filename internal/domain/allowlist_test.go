package domain

import (
	"net/netip"
	"testing"
)

func TestAllowListEmptyAllowsAll(t *testing.T) {
	a, err := NewAllowList(nil)
	if err != nil {
		t.Fatalf("NewAllowList: %v", err)
	}
	if !a.AllowsAll() {
		t.Error("empty AllowList should AllowsAll")
	}
	if !a.Allows(netip.MustParseAddr("203.0.113.9")) {
		t.Error("empty AllowList should allow any address")
	}
}

func TestAllowListIPAndCIDR(t *testing.T) {
	a, err := NewAllowList([]string{"192.168.1.5", "10.0.0.0/8"})
	if err != nil {
		t.Fatalf("NewAllowList: %v", err)
	}
	if a.AllowsAll() {
		t.Error("non-empty AllowList should not AllowsAll")
	}

	allowed := []string{"192.168.1.5", "10.1.2.3", "::ffff:192.168.1.5"}
	for _, s := range allowed {
		if !a.Allows(netip.MustParseAddr(s)) {
			t.Errorf("Allows(%s) = false, want true", s)
		}
	}
	denied := []string{"192.168.1.6", "172.16.0.1", "203.0.113.1"}
	for _, s := range denied {
		if a.Allows(netip.MustParseAddr(s)) {
			t.Errorf("Allows(%s) = true, want false", s)
		}
	}
}

func TestAllowListMappedIPv6Spec(t *testing.T) {
	// IPv4射影アドレスで指定しても、IPv4クライアントと一致する。
	a, err := NewAllowList([]string{"::ffff:10.0.0.7"})
	if err != nil {
		t.Fatalf("NewAllowList: %v", err)
	}
	if !a.Allows(netip.MustParseAddr("10.0.0.7")) {
		t.Error("mapped IPv6 spec should allow the equivalent IPv4 client")
	}
	if a.Allows(netip.MustParseAddr("10.0.0.8")) {
		t.Error("10.0.0.8 should not be allowed")
	}
}

func TestAllowListInvalid(t *testing.T) {
	for _, spec := range []string{"not-an-ip", "10.0.0.0/99", "999.1.1.1"} {
		if _, err := NewAllowList([]string{spec}); err == nil {
			t.Errorf("NewAllowList(%q) returned nil error, want error", spec)
		}
	}
}

func TestIsSensitiveHeader(t *testing.T) {
	sensitive := []string{"Authorization", "authorization", "Cookie", "Set-Cookie", "X-Api-Key", "Proxy-Authorization", " x-api-key "}
	for _, name := range sensitive {
		if !IsSensitiveHeader(name) {
			t.Errorf("IsSensitiveHeader(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"X-Debug-User", "Content-Type", ""} {
		if IsSensitiveHeader(name) {
			t.Errorf("IsSensitiveHeader(%q) = true, want false", name)
		}
	}
}
