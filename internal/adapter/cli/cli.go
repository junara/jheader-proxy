// Package cli はコマンドライン引数をユースケースの入力へ変換するインターフェース
// アダプタ。ユースケース層と domain 層に依存するが、インフラには依存しない。
package cli

import (
	"flag"
	"io"
	"strings"
	"time"

	"github.com/junara/jheader-proxy/internal/config"
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

// runFlags は run / gen-ca モードに関わるフラグ値をまとめる。--config のマージ対象。
type runFlags struct {
	listen   string
	domains  stringList
	headers  stringList
	allow    stringList
	caCert   string
	caKey    string
	duration time.Duration
	quiet    bool
	verbose  bool
	redact   bool
}

// applyConfig は path の設定ファイルを読み込み、コマンドラインで明示されなかった
// 項目にだけその値を反映する(精度: フラグ明示指定 > 設定ファイル > 既定値)。
// --domain/--header/--allow は1つでも指定されていれば設定ファイルのリストを
// 置き換える(マージはしない)。
func (r *runFlags) applyConfig(path string, fs *flag.FlagSet) error {
	fc, err := config.Load(path)
	if err != nil {
		return err
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })

	if !set["listen"] {
		r.listen = fc.Listen
	}
	if !set["domain"] {
		r.domains = fc.Domains
	}
	if !set["header"] {
		r.headers = config.HeadersToSpecs(fc.Headers)
	}
	if !set["allow"] {
		r.allow = fc.Allow
	}
	if !set["ca-cert"] {
		r.caCert = fc.CACertPath
	}
	if !set["ca-key"] {
		r.caKey = fc.CAKeyPath
	}
	if !set["duration"] {
		d, err := config.ParseDuration(fc.Duration)
		if err != nil {
			return err
		}
		r.duration = d
	}
	if !set["quiet"] {
		r.quiet = fc.Quiet
	}
	if !set["verbose"] {
		r.verbose = fc.Verbose
	}
	if !set["redact"] {
		r.redact = fc.Redact
	}
	return nil
}

// Parse は args を Command に解析する。フラグエラー(-h を含む)は、使用方法を
// output に書き出した上でそのまま返す。
func Parse(name string, args []string, output io.Writer) (*Command, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(output)

	var (
		rf          runFlags
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
	fs.StringVar(&rf.listen, "listen", ":8080", "proxy listen address (e.g. :8080)")
	fs.Var(&rf.domains, "domain", "target domain (repeatable; subdomains are included)")
	fs.Var(&rf.headers, "header", `header to add in "Name=Value" form (repeatable)`)
	fs.Var(&rf.allow, "allow", "allowed client IP or CIDR (repeatable; default allows all)")
	fs.StringVar(&rf.caCert, "ca-cert", "", "path to the CA certificate PEM used for HTTPS MITM (required)")
	fs.StringVar(&rf.caKey, "ca-key", "", "path to the CA private key PEM used for HTTPS MITM (required)")
	fs.DurationVar(&rf.duration, "duration", 10*time.Minute, "auto-stop after this duration (0 to disable)")
	fs.BoolVar(&genCA, "gen-ca", false, "generate a new CA at --ca-cert/--ca-key and exit")
	fs.BoolVar(&force, "force", false, "with --gen-ca, overwrite existing files")
	fs.BoolVar(&rf.quiet, "quiet", false, "suppress per-request logs")
	fs.BoolVar(&rf.verbose, "verbose", false, "also log responses for target domains")
	fs.BoolVar(&rf.redact, "redact", false, "mask all header values in the startup log")
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

	// --config 指定時は、コマンドラインで明示しなかった項目だけ設定ファイルの値で埋める。
	if configPath != "" {
		if err := rf.applyConfig(configPath, fs); err != nil {
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
			GenCA: usecase.GenerateCAInput{CertPath: rf.caCert, KeyPath: rf.caKey, Force: force},
		}, nil
	}

	parsedHeaders, err := domain.ParseHeaders(rf.headers)
	if err != nil {
		return nil, err
	}
	return &Command{
		Mode:    ModeRun,
		Quiet:   rf.quiet,
		Verbose: rf.verbose,
		Run: usecase.RunProxyInput{
			// 入口(フラグ/--config/GUI)に依らず挙動を揃えるため、listen は trim し、
			// domains/allow は空・空白だけの項目を落とす。
			Listen:       strings.TrimSpace(rf.listen),
			Domains:      config.TrimNonEmpty(rf.domains),
			Headers:      parsedHeaders,
			CACertPath:   rf.caCert,
			CAKeyPath:    rf.caKey,
			Allow:        config.TrimNonEmpty(rf.allow),
			RedactValues: rf.redact,
			Duration:     rf.duration,
		},
	}, nil
}
