package domain

import "testing"

func TestParseHeaders(t *testing.T) {
	h, err := ParseHeaders([]string{
		"X-Debug-User=jun",
		"X-From-iPhone-Proxy=true",
		"X-Empty=",
		"Authorization=Bearer dummy-token",
	})
	if err != nil {
		t.Fatalf("ParseHeaders returned error: %v", err)
	}

	got := map[string]string{}
	var order []string
	h.Each(func(name, value string) {
		got[name] = value
		order = append(order, name)
	})

	want := map[string]string{
		"X-Debug-User":        "jun",
		"X-From-iPhone-Proxy": "true",
		"X-Empty":             "",
		"Authorization":       "Bearer dummy-token",
	}
	for name, value := range want {
		if got[name] != value {
			t.Errorf("header[%q] = %q, want %q", name, got[name], value)
		}
	}

	wantOrder := []string{"X-Debug-User", "X-From-iPhone-Proxy", "X-Empty", "Authorization"}
	if len(order) != len(wantOrder) {
		t.Fatalf("order length = %d, want %d", len(order), len(wantOrder))
	}
	for i, name := range wantOrder {
		if order[i] != name {
			t.Errorf("order[%d] = %q, want %q", i, order[i], name)
		}
	}
}

func TestParseHeadersLastWins(t *testing.T) {
	h, err := ParseHeaders([]string{
		"X-Debug-User=jun",
		"X-Debug-User=taro",
	})
	if err != nil {
		t.Fatalf("ParseHeaders returned error: %v", err)
	}
	if h.Len() != 1 {
		t.Errorf("Len() = %d, want 1", h.Len())
	}
	h.Each(func(name, value string) {
		if value != "taro" {
			t.Errorf("header[%q] = %q, want %q", name, value, "taro")
		}
	})
}

func TestParseHeadersErrors(t *testing.T) {
	invalid := []string{
		"X-Debug-User", // = がない
		"=value",       // ヘッダー名が空
		" =value",      // TrimSpace後にヘッダー名が空
	}
	for _, spec := range invalid {
		if _, err := ParseHeaders([]string{spec}); err == nil {
			t.Errorf("ParseHeaders(%q) returned nil error, want error", spec)
		}
	}
}

func TestZeroHeaders(t *testing.T) {
	var h Headers
	if h.Len() != 0 {
		t.Errorf("zero Headers Len() = %d, want 0", h.Len())
	}
	h.Each(func(name, value string) {
		t.Errorf("zero Headers iterated %q=%q, want none", name, value)
	})
}
