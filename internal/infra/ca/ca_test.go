package ca

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreGenerateAndLoad(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "jheader-proxy-ca-cert.pem")
	keyPath := filepath.Join(dir, "jheader-proxy-ca-key.pem")

	store := New()
	if err := store.Generate(certPath, keyPath, false); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	cert, err := store.Load(certPath, keyPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cert.Leaf == nil || !cert.Leaf.IsCA {
		t.Error("loaded certificate is not a CA")
	}
}

func TestStoreGenerateKeyPermission(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	if err := New().Generate(certPath, keyPath, false); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("failed to stat key file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("key file permission = %o, want 600", perm)
	}
}

func TestStoreGenerateNoOverwrite(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	store := New()
	if err := store.Generate(certPath, keyPath, false); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if err := store.Generate(certPath, keyPath, false); err == nil {
		t.Error("Generate overwrote existing files, want error")
	}
	// force ならば既存ファイルを上書きできる。
	if err := store.Generate(certPath, keyPath, true); err != nil {
		t.Errorf("Generate with force returned error: %v", err)
	}
}

func TestStoreGenerateForceResetsKeyPermission(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	// 緩い権限の既存鍵ファイルを用意する。
	if err := os.WriteFile(keyPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("seed key file: %v", err)
	}
	if err := os.WriteFile(certPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("seed cert file: %v", err)
	}

	if err := New().Generate(certPath, keyPath, true); err != nil {
		t.Fatalf("Generate with force returned error: %v", err)
	}

	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat key file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("key file permission after force = %o, want 600", perm)
	}
}

func TestStoreLoadMissing(t *testing.T) {
	if _, err := New().Load("/nonexistent/cert.pem", "/nonexistent/key.pem"); err == nil {
		t.Error("Load with missing files returned nil error, want error")
	}
}
