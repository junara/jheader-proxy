// Package cli はコマンドライン引数をユースケースの入力へ変換するインターフェース
// アダプタ。フラグを config.RunConfig に組み立て、--config(設定ファイル)とマージした
// うえで config.ToRunProxyInput により GUI と同一の変換を通す。
package cli

import (
	"errors"
	"flag"
	"io"
	"strings"

	"github.com/junara/jheader-proxy/internal/config"
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
	// ModeGUI はローカル Web 管理画面を起動する。
	ModeGUI
)

// GUIOptions は GUI モードの起動オプション。
type GUIOptions struct {
	Listen string // 管理画面の待受アドレス
	NoOpen bool   // ブラウザを自動起動しない
}

// Command はコマンドラインから解析された意図を表す。
type Command struct {
	Mode    Mode
	Run     usecase.RunProxyInput
	GenCA   usecase.GenerateCAInput
	GUI     GUIOptions
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

// headerList は "Name=Value" を繰り返し受け取り HeaderKV として蓄積する flag.Value。
// GUI のフォーム入力と同じ表現に揃えることで、以降の変換を config.RunConfig 経由で
// 共通化できる。値の trim/重複解決は変換時(domain.ParseHeaders)に行う。
type headerList []config.HeaderKV

func (h *headerList) String() string { return strings.Join(config.HeadersToSpecs(*h), ",") }

func (h *headerList) Set(value string) error {
	// flag パッケージが "invalid value %q for flag -header:" を前置するため、
	// ここでは理由のみ返す。
	name, val, found := strings.Cut(value, "=")
	if !found {
		return errors.New("must be Name=Value")
	}
	if strings.TrimSpace(name) == "" {
		return errors.New("header name is empty")
	}
	*h = append(*h, config.HeaderKV{Name: name, Value: val})
	return nil
}

// applyConfig は path の設定ファイルを読み込み、コマンドラインで明示されなかった
// 項目にだけその値を反映する(精度: フラグ明示指定 > 設定ファイル > 既定値)。
// --domain/--header/--allow は1つでも指定されていれば設定ファイルのリストを
// 置き換える(マージはしない)。
func applyConfig(rc *config.RunConfig, path string, fs *flag.FlagSet) error {
	fc, err := config.Load(path)
	if err != nil {
		return err
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })

	if !set["listen"] {
		rc.Listen = fc.Listen
	}
	if !set["domain"] {
		rc.Domains = fc.Domains
	}
	if !set["header"] {
		rc.Headers = fc.Headers
	}
	if !set["allow"] {
		rc.Allow = fc.Allow
	}
	if !set["ca-cert"] {
		rc.CACertPath = fc.CACertPath
	}
	if !set["ca-key"] {
		rc.CAKeyPath = fc.CAKeyPath
	}
	if !set["duration"] {
		rc.Duration = fc.Duration
	}
	if !set["quiet"] {
		rc.Quiet = fc.Quiet
	}
	if !set["verbose"] {
		rc.Verbose = fc.Verbose
	}
	if !set["redact"] {
		rc.Redact = fc.Redact
	}
	return nil
}

// Parse は args を Command に解析する。フラグエラー(-h を含む)は、使用方法を
// output に書き出した上でそのまま返す。
func Parse(name string, args []string, output io.Writer) (*Command, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(output)

	var (
		rc          config.RunConfig
		domains     stringList
		headers     headerList
		allow       stringList
		genCA       bool
		force       bool
		showVersion bool
		gui         bool
		guiListen   string
		noOpen      bool
		configPath  string
	)
	fs.StringVar(&configPath, "config", "",
		"path to a JSON config file (GUI's config.json is compatible); command-line flags override its values")
	fs.StringVar(&rc.Listen, "listen", ":8080", "proxy listen address (e.g. :8080)")
	fs.Var(&domains, "domain", "target domain (repeatable; subdomains are included)")
	fs.Var(&headers, "header", `header to add in "Name=Value" form (repeatable)`)
	fs.Var(&allow, "allow", "allowed client IP or CIDR (repeatable; default allows all)")
	fs.StringVar(&rc.CACertPath, "ca-cert", "", "path to the CA certificate PEM used for HTTPS MITM (required)")
	fs.StringVar(&rc.CAKeyPath, "ca-key", "", "path to the CA private key PEM used for HTTPS MITM (required)")
	fs.StringVar(&rc.Duration, "duration", "10m", "auto-stop after this duration, e.g. 30m (0 to disable)")
	fs.BoolVar(&genCA, "gen-ca", false, "generate a new CA at --ca-cert/--ca-key and exit")
	fs.BoolVar(&force, "force", false, "with --gen-ca, overwrite existing files")
	fs.BoolVar(&rc.Quiet, "quiet", false, "suppress per-request logs")
	fs.BoolVar(&rc.Verbose, "verbose", false, "also log responses for target domains")
	fs.BoolVar(&rc.Redact, "redact", false, "mask all header values in the startup log")
	fs.BoolVar(&showVersion, "version", false, "print version and exit")
	fs.BoolVar(&gui, "gui", false, "launch the local web GUI to configure and control the proxy")
	fs.StringVar(&guiListen, "gui-listen", "127.0.0.1:9090", "with --gui, the management UI listen address")
	fs.BoolVar(&noOpen, "no-open", false, "with --gui, do not open the browser automatically")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if showVersion {
		return &Command{Mode: ModeVersion}, nil
	}

	// 繰り返しフラグを RunConfig へ移し、--config 指定時は明示しなかった項目だけ
	// 設定ファイルの値で埋める。
	rc.Domains = domains
	rc.Headers = headers
	rc.Allow = allow
	if configPath != "" {
		if err := applyConfig(&rc, configPath, fs); err != nil {
			return nil, err
		}
	}

	if gui {
		return &Command{
			Mode: ModeGUI,
			GUI:  GUIOptions{Listen: guiListen, NoOpen: noOpen},
		}, nil
	}
	if genCA {
		return &Command{
			Mode:  ModeGenCA,
			GenCA: usecase.GenerateCAInput{CertPath: rc.CACertPath, KeyPath: rc.CAKeyPath, Force: force},
		}, nil
	}

	input, err := config.ToRunProxyInput(rc)
	if err != nil {
		return nil, err
	}
	return &Command{
		Mode:    ModeRun,
		Quiet:   rc.Quiet,
		Verbose: rc.Verbose,
		Run:     input,
	}, nil
}
