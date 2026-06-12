// Package ca は crypto/x509 とファイルシステムを用いた、CAの読み込みと生成の
// インフラ実装。usecase.CAProvider と usecase.CAGenerator ポートを満たす。
package ca

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"time"
)

// Store はローカルファイルシステム上でCA素材の読み込み・生成を行う。
type Store struct{}

// New はファイルシステムを用いる CA Store を返す。
func New() *Store { return &Store{} }

// Load はPEMファイルからCA証明書と秘密鍵を読み込み、Leaf を解析した
// tls.Certificate を返す。証明書がCA証明書でない場合はエラーとする。
func (*Store) Load(certPath, keyPath string) (*tls.Certificate, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA cert %q: %w", certPath, err)
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA key %q: %w", keyPath, err)
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to load CA key pair: %w", err)
	}
	if cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0]); err != nil {
		return nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}
	if !cert.Leaf.IsCA {
		return nil, fmt.Errorf("certificate %q is not a CA certificate (CA:TRUE required)", certPath)
	}
	return &cert, nil
}

// Generate は新しい自己署名CAを生成し、証明書と秘密鍵を指定パスへ書き出す。
// 既存ファイルは上書きしない。秘密鍵ファイルは 0600 で書き出す。
func (*Store) Generate(certPath, keyPath string, force bool) error {
	if !force {
		for _, p := range []string{certPath, keyPath} {
			if _, err := os.Stat(p); err == nil {
				return fmt.Errorf("%q already exists; refusing to overwrite (use --force)", p)
			}
		}
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "jheader-proxy local CA",
			Organization: []string{"jheader-proxy"},
		},
		NotBefore:             now.Add(-1 * time.Hour),
		NotAfter:              now.AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	if err := writePEMFile(certPath, 0o644, force, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		return fmt.Errorf("failed to write cert: %w", err)
	}
	if err := writePEMFile(keyPath, 0o600, force, &pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}); err != nil {
		return fmt.Errorf("failed to write key: %w", err)
	}
	return nil
}

// writePEMFile は block を path へ書き出す。Close 時のフラッシュ失敗も検知する。
// 常に O_EXCL で新規作成し、perm を確実に適用する。force の場合は既存ファイルを
// 先に削除する（O_TRUNC では既存ファイルの権限が維持され、秘密鍵が緩い権限の
// まま残り得るため）。
func writePEMFile(path string, perm os.FileMode, force bool, block *pem.Block) error {
	if force {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if err := pem.Encode(f, block); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
