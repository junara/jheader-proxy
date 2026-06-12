package cli

import (
	"io"
	"testing"
	"time"
)

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
