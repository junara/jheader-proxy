package usecase

import "errors"

// GenerateCAInput は GenerateCA ユースケースの入力。
type GenerateCAInput struct {
	CertPath string
	KeyPath  string
	Force    bool
}

// GenerateCA は要求されたパスに新しい自己署名CAを生成する。
type GenerateCA struct {
	gen CAGenerator
}

// NewGenerateCA は依存を注入して GenerateCA を構築する。
func NewGenerateCA(gen CAGenerator) *GenerateCA {
	return &GenerateCA{gen: gen}
}

// Execute は入力を検証してCAを生成する。
func (u *GenerateCA) Execute(in GenerateCAInput) error {
	if in.CertPath == "" || in.KeyPath == "" {
		return errors.New("--gen-ca requires both --ca-cert and --ca-key (output paths)")
	}
	return u.gen.Generate(in.CertPath, in.KeyPath, in.Force)
}
