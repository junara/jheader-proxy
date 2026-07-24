package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeHeaderFile はテスト用のヘッダーファイルを書き出してパスを返す。
func writeHeaderFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "headers.txt")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("failed to write header file: %v", err)
	}
	return path
}

func TestParseHeaderFile(t *testing.T) {
	path := writeHeaderFile(t, `
# 検証用の秘匿ヘッダー
Authorization=Bearer secret-token

X-Api-Key = abc123
`)
	cmd, err := Parse("jheader-proxy", []string{
		"run",
		"--domain", "example.test",
		"--header-file", path,
		"--ca-cert", "cert.pem",
		"--ca-key", "key.pem",
	}, io.Discard)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if cmd.Run.Headers.Len() != 2 {
		t.Fatalf("Headers.Len() = %d, want 2", cmd.Run.Headers.Len())
	}
	got := map[string]string{}
	cmd.Run.Headers.Each(func(name, value string) { got[name] = value })
	if got["Authorization"] != "Bearer secret-token" {
		t.Errorf("Authorization = %q, want %q", got["Authorization"], "Bearer secret-token")
	}
	if got["X-Api-Key"] != "abc123" {
		t.Errorf("X-Api-Key = %q, want %q (name/value trimmed)", got["X-Api-Key"], "abc123")
	}
}

func TestParseHeaderFileInlineHeaderWins(t *testing.T) {
	// 同名ヘッダーは --header の明示指定がファイルの値より勝つ。
	path := writeHeaderFile(t, "X-Debug-User=from-file\n")
	cmd, err := Parse("jheader-proxy", []string{
		"run",
		"--domain", "example.test",
		"--header-file", path,
		"--header", "X-Debug-User=from-flag",
		"--ca-cert", "cert.pem",
		"--ca-key", "key.pem",
	}, io.Discard)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	var got string
	cmd.Run.Headers.Each(func(name, value string) {
		if name == "X-Debug-User" {
			got = value
		}
	})
	if got != "from-flag" {
		t.Errorf("X-Debug-User = %q, want from-flag (inline --header wins)", got)
	}
}

func TestParseHeaderFileReplacesConfigHeaders(t *testing.T) {
	// --header-file を指定したら、--header と同様に設定ファイルのヘッダーを置き換える。
	headerPath := writeHeaderFile(t, "X-From-Header-File=1\n")
	configPath := writeConfig(t, `{
		"domains": ["example.test"],
		"headers": [{"name": "X-From-Config", "value": "1"}],
		"caCertPath": "cert.pem",
		"caKeyPath": "key.pem"
	}`)
	cmd, err := Parse("jheader-proxy", []string{
		"run",
		"--config", configPath,
		"--header-file", headerPath,
	}, io.Discard)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if cmd.Run.Headers.Len() != 1 {
		t.Fatalf("Headers.Len() = %d, want 1", cmd.Run.Headers.Len())
	}
	cmd.Run.Headers.Each(func(name, _ string) {
		if name != "X-From-Header-File" {
			t.Errorf("header name = %q, want X-From-Header-File", name)
		}
	})
}

func TestParseHeaderFileInvalidLine(t *testing.T) {
	path := writeHeaderFile(t, "X-Valid=1\nnot-a-header\n")
	_, err := Parse("jheader-proxy", []string{
		"run",
		"--domain", "example.test",
		"--header-file", path,
		"--ca-cert", "cert.pem",
		"--ca-key", "key.pem",
	}, io.Discard)
	if err == nil {
		t.Fatal("Parse with invalid header line returned nil error, want error")
	}
	if !strings.Contains(err.Error(), ":2") {
		t.Errorf("error %q does not mention line number 2", err)
	}
}

func TestParseHeaderFileMissing(t *testing.T) {
	_, err := Parse("jheader-proxy", []string{
		"run",
		"--domain", "example.test",
		"--header-file", filepath.Join(t.TempDir(), "does-not-exist.txt"),
		"--ca-cert", "cert.pem",
		"--ca-key", "key.pem",
	}, io.Discard)
	if err == nil {
		t.Error("Parse with missing header file returned nil error, want error")
	}
}
