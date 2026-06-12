// Package cli はコマンドライン引数をユースケースの入力へ変換するインターフェース
// アダプタ。ユースケース層と domain 層に依存するが、インフラには依存しない。
package cli

import (
	"flag"
	"io"
	"strings"
	"time"

	"github.com/junara/jheader-proxy/internal/domain"
	"github.com/junara/jheader-proxy/internal/usecase"
)

// Mode は、解析されたコマンドがどのユースケースを実行すべきかを選択する。
type Mode int

const (
	// ModeRun はプロキシを実行する。
	ModeRun Mode = iota
	// ModeGenCA はCAを生成して終了する。
	ModeGenCA
	// ModeVersion はバージョンを表示して終了する。
	ModeVersion
)

// Command はコマンドラインから解析された意図を表す。
type Command struct {
	Mode    Mode
	Run     usecase.RunProxyInput
	GenCA   usecase.GenerateCAInput
	Quiet   bool
	Verbose bool
}

// stringList は繰り返し指定された値を蓄積する flag.Value。
type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// Parse は args を Command に解析する。フラグエラー(-h を含む)は、使用方法を
// output に書き出した上でそのまま返す。
func Parse(name string, args []string, output io.Writer) (*Command, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(output)

	var (
		listen      string
		domains     stringList
		headers     stringList
		allow       stringList
		caCert      string
		caKey       string
		duration    time.Duration
		genCA       bool
		force       bool
		quiet       bool
		verbose     bool
		redact      bool
		showVersion bool
	)
	fs.StringVar(&listen, "listen", ":8080", "proxy listen address (e.g. :8080)")
	fs.Var(&domains, "domain", "target domain (repeatable; subdomains are included)")
	fs.Var(&headers, "header", `header to add in "Name=Value" form (repeatable)`)
	fs.Var(&allow, "allow", "allowed client IP or CIDR (repeatable; default allows all)")
	fs.StringVar(&caCert, "ca-cert", "", "path to the CA certificate PEM used for HTTPS MITM (required)")
	fs.StringVar(&caKey, "ca-key", "", "path to the CA private key PEM used for HTTPS MITM (required)")
	fs.DurationVar(&duration, "duration", 10*time.Minute, "auto-stop after this duration (0 to disable)")
	fs.BoolVar(&genCA, "gen-ca", false, "generate a new CA at --ca-cert/--ca-key and exit")
	fs.BoolVar(&force, "force", false, "with --gen-ca, overwrite existing files")
	fs.BoolVar(&quiet, "quiet", false, "suppress per-request logs")
	fs.BoolVar(&verbose, "verbose", false, "also log responses for target domains")
	fs.BoolVar(&redact, "redact", false, "mask all header values in the startup log")
	fs.BoolVar(&showVersion, "version", false, "print version and exit")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if showVersion {
		return &Command{Mode: ModeVersion}, nil
	}
	if genCA {
		return &Command{
			Mode:  ModeGenCA,
			GenCA: usecase.GenerateCAInput{CertPath: caCert, KeyPath: caKey, Force: force},
		}, nil
	}

	parsedHeaders, err := domain.ParseHeaders(headers)
	if err != nil {
		return nil, err
	}
	return &Command{
		Mode:    ModeRun,
		Quiet:   quiet,
		Verbose: verbose,
		Run: usecase.RunProxyInput{
			Listen:       listen,
			Domains:      domains,
			Headers:      parsedHeaders,
			CACertPath:   caCert,
			CAKeyPath:    caKey,
			Allow:        allow,
			RedactValues: redact,
			Duration:     duration,
		},
	}, nil
}
