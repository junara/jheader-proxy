package usecase

import (
	"errors"
	"testing"
)

type fakeCAGenerator struct {
	called    bool
	gotCert   string
	gotKey    string
	gotForce  bool
	returnErr error
}

func (f *fakeCAGenerator) Generate(certPath, keyPath string, force bool) error {
	f.called = true
	f.gotCert = certPath
	f.gotKey = keyPath
	f.gotForce = force
	return f.returnErr
}

func TestGenerateCAExecute(t *testing.T) {
	gen := &fakeCAGenerator{}
	uc := NewGenerateCA(gen)

	err := uc.Execute(GenerateCAInput{CertPath: "cert.pem", KeyPath: "key.pem", Force: true})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !gen.called || gen.gotCert != "cert.pem" || gen.gotKey != "key.pem" || !gen.gotForce {
		t.Errorf("Generate called with (%q, %q, force=%v), want (cert.pem, key.pem, true)", gen.gotCert, gen.gotKey, gen.gotForce)
	}
}

func TestGenerateCAMissingPaths(t *testing.T) {
	gen := &fakeCAGenerator{}
	uc := NewGenerateCA(gen)

	if err := uc.Execute(GenerateCAInput{CertPath: "", KeyPath: "key.pem"}); err == nil {
		t.Error("Execute with empty cert path returned nil error, want error")
	}
	if gen.called {
		t.Error("Generate should not be called on invalid input")
	}
}

func TestGenerateCAPropagatesError(t *testing.T) {
	gen := &fakeCAGenerator{returnErr: errors.New("boom")}
	uc := NewGenerateCA(gen)

	if err := uc.Execute(GenerateCAInput{CertPath: "cert.pem", KeyPath: "key.pem"}); err == nil {
		t.Error("Execute returned nil error, want propagated generator error")
	}
}
