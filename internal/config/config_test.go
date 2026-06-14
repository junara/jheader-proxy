package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadAppliesDefaults(t *testing.T) {
	// duration を省いた設定。既定値("10m")で補完されること。
	path := filepath.Join(t.TempDir(), "config.json")
	writeFile(t, path, `{"listen": ":7000", "domains": ["example.test"]}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Listen != ":7000" {
		t.Errorf("Listen = %q, want :7000", cfg.Listen)
	}
	if cfg.Duration != "10m" {
		t.Errorf("Duration = %q, want 10m (default)", cfg.Duration)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Error("Load of missing file returned nil error, want error")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	writeFile(t, path, `{not valid json`)
	if _, err := Load(path); err == nil {
		t.Error("Load of invalid JSON returned nil error, want error")
	}
}

func TestHeadersToSpecs(t *testing.T) {
	specs := HeadersToSpecs([]HeaderKV{
		{Name: "  X-A  ", Value: "1"},
		{Name: "", Value: "skip"}, // 名前が空 → 無視
		{Name: "X-B", Value: ""},  // 空の値は許可
	})
	want := []string{"X-A=1", "X-B="}
	if len(specs) != len(want) {
		t.Fatalf("specs = %v, want %v", specs, want)
	}
	for i := range want {
		if specs[i] != want[i] {
			t.Errorf("specs[%d] = %q, want %q", i, specs[i], want[i])
		}
	}
}

func TestTrimNonEmpty(t *testing.T) {
	got := TrimNonEmpty([]string{"  a ", "", "  ", "b"})
	want := []string{"a", "b"}
	if len(got) != len(want) {
		t.Fatalf("TrimNonEmpty = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("TrimNonEmpty[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"30s", 30 * time.Second, false},
		{"10m", 10 * time.Minute, false},
		{"", 0, false},     // 未指定 → 無制限
		{"  0 ", 0, false}, // 0 → 無制限
		{"nope", 0, true},
	}
	for _, c := range cases {
		got, err := ParseDuration(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseDuration(%q) err = nil, want error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseDuration(%q) returned error: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("ParseDuration(%q) = %s, want %s", c.in, got, c.want)
		}
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("failed to write %q: %v", path, err)
	}
}
