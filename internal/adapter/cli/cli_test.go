package cli

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeConfig はテスト用の一時設定ファイルを書き出してパスを返す。
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	return path
}

func TestParseConfigFile(t *testing.T) {
	path := writeConfig(t, `{
		"listen": ":7000",
		"domains": ["example.test", "example.dev"],
		"headers": [{"name": "X-Debug-User", "value": "jun"}],
		"allow": ["192.168.1.5"],
		"duration": "30s",
		"quiet": true,
		"verbose": true,
		"redact": true,
		"caCertPath": "cert.pem",
		"caKeyPath": "key.pem"
	}`)

	cmd, err := Parse("jheader-proxy", []string{"--config", path}, io.Discard)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if cmd.Mode != ModeRun {
		t.Fatalf("Mode = %v, want ModeRun", cmd.Mode)
	}
	if cmd.Run.Listen != ":7000" {
		t.Errorf("Listen = %q, want :7000", cmd.Run.Listen)
	}
	if len(cmd.Run.Domains) != 2 {
		t.Errorf("Domains = %v, want 2 entries", cmd.Run.Domains)
	}
	if cmd.Run.Headers.Len() != 1 {
		t.Errorf("Headers.Len() = %d, want 1", cmd.Run.Headers.Len())
	}
	if cmd.Run.Duration != 30*time.Second {
		t.Errorf("Duration = %s, want 30s", cmd.Run.Duration)
	}
	if !cmd.Quiet || !cmd.Verbose || !cmd.Run.RedactValues {
		t.Errorf("Quiet=%v Verbose=%v Redact=%v, want all true", cmd.Quiet, cmd.Verbose, cmd.Run.RedactValues)
	}
	if cmd.Run.CACertPath != "cert.pem" || cmd.Run.CAKeyPath != "key.pem" {
		t.Errorf("CA paths = (%q, %q), want (cert.pem, key.pem)", cmd.Run.CACertPath, cmd.Run.CAKeyPath)
	}
}

func TestParseConfigFlagsOverride(t *testing.T) {
	path := writeConfig(t, `{
		"listen": ":7000",
		"domains": ["from-file.test"],
		"headers": [{"name": "X-From-File", "value": "1"}],
		"duration": "30s",
		"caCertPath": "file-cert.pem",
		"caKeyPath": "file-key.pem"
	}`)

	// listen と domain と header はコマンドラインで上書きする。
	cmd, err := Parse("jheader-proxy", []string{
		"--config", path,
		"--listen", ":9999",
		"--domain", "from-flag.test",
		"--header", "X-From-Flag=2",
	}, io.Discard)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if cmd.Run.Listen != ":9999" {
		t.Errorf("Listen = %q, want :9999 (flag overrides file)", cmd.Run.Listen)
	}
	if len(cmd.Run.Domains) != 1 || cmd.Run.Domains[0] != "from-flag.test" {
		t.Errorf("Domains = %v, want [from-flag.test] (flag replaces file list)", cmd.Run.Domains)
	}
	// 上書きしなかった項目はファイルの値が残る。
	if cmd.Run.Duration != 30*time.Second {
		t.Errorf("Duration = %s, want 30s (from file)", cmd.Run.Duration)
	}
	if cmd.Run.CACertPath != "file-cert.pem" {
		t.Errorf("CACertPath = %q, want file-cert.pem (from file)", cmd.Run.CACertPath)
	}
}

func TestParseConfigMissingFile(t *testing.T) {
	_, err := Parse("jheader-proxy", []string{
		"--config", filepath.Join(t.TempDir(), "does-not-exist.json"),
	}, io.Discard)
	if err == nil {
		t.Error("Parse with missing --config file returned nil error, want error")
	}
}

func TestParseRunMode(t *testing.T) {
	cmd, err := Parse("jheader-proxy", []string{
		"--listen", ":9090",
		"--domain", "example.test",
		"--domain", "example.dev",
		"--header", "X-Debug-User=jun",
		"--ca-cert", "cert.pem",
		"--ca-key", "key.pem",
	}, io.Discard)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if cmd.Mode != ModeRun {
		t.Fatalf("Mode = %v, want ModeRun", cmd.Mode)
	}
	if cmd.Run.Listen != ":9090" {
		t.Errorf("Listen = %q, want :9090", cmd.Run.Listen)
	}
	if len(cmd.Run.Domains) != 2 {
		t.Errorf("Domains = %v, want 2 entries", cmd.Run.Domains)
	}
	if cmd.Run.Headers.Len() != 1 {
		t.Errorf("Headers.Len() = %d, want 1", cmd.Run.Headers.Len())
	}
	if cmd.Run.CACertPath != "cert.pem" || cmd.Run.CAKeyPath != "key.pem" {
		t.Errorf("CA paths = (%q, %q), want (cert.pem, key.pem)", cmd.Run.CACertPath, cmd.Run.CAKeyPath)
	}
}

func TestParseGenCAMode(t *testing.T) {
	cmd, err := Parse("jheader-proxy", []string{
		"--gen-ca",
		"--ca-cert", "cert.pem",
		"--ca-key", "key.pem",
	}, io.Discard)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if cmd.Mode != ModeGenCA {
		t.Fatalf("Mode = %v, want ModeGenCA", cmd.Mode)
	}
	if cmd.GenCA.CertPath != "cert.pem" || cmd.GenCA.KeyPath != "key.pem" {
		t.Errorf("GenCA paths = (%q, %q), want (cert.pem, key.pem)", cmd.GenCA.CertPath, cmd.GenCA.KeyPath)
	}
	if cmd.GenCA.Force {
		t.Error("GenCA.Force = true without --force, want false")
	}
}

func TestParseGenCAForce(t *testing.T) {
	cmd, err := Parse("jheader-proxy", []string{
		"--gen-ca", "--force",
		"--ca-cert", "cert.pem",
		"--ca-key", "key.pem",
	}, io.Discard)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !cmd.GenCA.Force {
		t.Error("GenCA.Force = false with --force, want true")
	}
}

func TestParseVersionMode(t *testing.T) {
	cmd, err := Parse("jheader-proxy", []string{"--version"}, io.Discard)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if cmd.Mode != ModeVersion {
		t.Errorf("Mode = %v, want ModeVersion", cmd.Mode)
	}
}

func TestParseRuntimeOptions(t *testing.T) {
	cmd, err := Parse("jheader-proxy", []string{
		"--domain", "example.test",
		"--header", "X-Debug-User=jun",
		"--ca-cert", "cert.pem",
		"--ca-key", "key.pem",
		"--allow", "192.168.1.5",
		"--allow", "10.0.0.0/8",
		"--quiet",
		"--verbose",
		"--redact",
	}, io.Discard)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !cmd.Quiet || !cmd.Verbose {
		t.Errorf("Quiet=%v Verbose=%v, want both true", cmd.Quiet, cmd.Verbose)
	}
	if !cmd.Run.RedactValues {
		t.Error("RedactValues = false, want true")
	}
	if len(cmd.Run.Allow) != 2 {
		t.Errorf("Allow = %v, want 2 entries", cmd.Run.Allow)
	}
}

func TestParseDefaultListen(t *testing.T) {
	cmd, err := Parse("jheader-proxy", []string{
		"--domain", "example.test",
		"--header", "X-Debug-User=jun",
		"--ca-cert", "cert.pem",
		"--ca-key", "key.pem",
	}, io.Discard)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if cmd.Run.Listen != ":8080" {
		t.Errorf("default Listen = %q, want :8080", cmd.Run.Listen)
	}
	if cmd.Run.Duration != 10*time.Minute {
		t.Errorf("default Duration = %s, want 10m", cmd.Run.Duration)
	}
}

func TestParseDuration(t *testing.T) {
	cmd, err := Parse("jheader-proxy", []string{
		"--domain", "example.test",
		"--header", "X-Debug-User=jun",
		"--ca-cert", "cert.pem",
		"--ca-key", "key.pem",
		"--duration", "30s",
	}, io.Discard)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if cmd.Run.Duration != 30*time.Second {
		t.Errorf("Duration = %s, want 30s", cmd.Run.Duration)
	}
}

func TestParseInvalidHeader(t *testing.T) {
	_, err := Parse("jheader-proxy", []string{
		"--domain", "example.test",
		"--header", "X-Debug-User", // = がない
		"--ca-cert", "cert.pem",
		"--ca-key", "key.pem",
	}, io.Discard)
	if err == nil {
		t.Error("Parse with malformed --header returned nil error, want error")
	}
}
