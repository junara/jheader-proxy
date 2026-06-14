// Package cli はコマンドライン引数をユースケースの入力へ変換するインターフェース
// アダプタ。フラグを config.RunConfig に組み立て、--config(設定ファイル)とマージした
// うえで config.ToRunProxyInput により GUI と同一の変換を通す。
package cli

import (
	"errors"
	"flag"
	"fmt"
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

// usageTemplate は使い方の概要・代表的な実行例の書式。%[1]s はプログラム名。
const usageTemplate = `%[1]s - 対象ドメインへのリクエストに HTTP ヘッダーを付与するローカル HTTP/HTTPS プロキシ。

使い方:
  %[1]s --domain <ホスト> --header <Name=Value> --ca-cert <パス> --ca-key <パス> [オプション]
  %[1]s --gen-ca --ca-cert <パス> --ca-key <パス>   # 自分専用のCAを生成（HTTPSに必須）
  %[1]s --gui                                       # ブラウザの管理画面で操作
  %[1]s --config <ファイル>                          # 設定をJSONファイルから読む

例:
  %[1]s --gen-ca --ca-cert ca-cert.pem --ca-key ca-key.pem
  %[1]s --domain example.test --header "X-Debug-User=jun" --ca-cert ca-cert.pem --ca-key ca-key.pem

オプション:
`

// writeUsage は使い方の概要・代表的な実行例・オプション一覧を出力する。
// 引数なし実行時と -h/--help 時の両方で使う。
func writeUsage(fs *flag.FlagSet, name string) {
	_, _ = io.WriteString(fs.Output(), fmt.Sprintf(usageTemplate, name))
	fs.PrintDefaults()
}

// parseFlags は Parse がフラグ束縛先としてまとめて使う値の集合。
type parseFlags struct {
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
}

// bindFlags は fs に全フラグを登録し、束縛先 f を結びつける。
func bindFlags(fs *flag.FlagSet, f *parseFlags) {
	fs.StringVar(&f.configPath, "config", "",
		"設定をまとめた JSON ファイルのパス（GUIの config.json と互換。コマンドライン引数が優先）")
	fs.StringVar(&f.rc.Listen, "listen", ":8080", "プロキシの待受アドレス（例: :8080）")
	fs.Var(&f.domains, "domain", "ヘッダー付与の対象ドメイン（複数指定可・サブドメインも対象）")
	fs.Var(&f.headers, "header", "付与するヘッダー（Name=Value 形式・複数指定可）")
	fs.Var(&f.allow, "allow", "接続を許可するクライアントの IP / CIDR（複数指定可・未指定で全許可）")
	fs.StringVar(&f.rc.CACertPath, "ca-cert", "", "HTTPS MITM に使う CA 証明書 PEM のパス（必須）")
	fs.StringVar(&f.rc.CAKeyPath, "ca-key", "", "HTTPS MITM に使う CA 秘密鍵 PEM のパス（必須）")
	fs.StringVar(&f.rc.Duration, "duration", "10m", "この時間が過ぎると自動停止（例: 30m。0 で無制限）")
	fs.BoolVar(&f.genCA, "gen-ca", false, "--ca-cert/--ca-key に新しい CA を生成して終了する")
	fs.BoolVar(&f.force, "force", false, "--gen-ca 時に既存ファイルを上書きする")
	fs.BoolVar(&f.rc.Quiet, "quiet", false, "リクエストごとのログを抑制する")
	fs.BoolVar(&f.rc.Verbose, "verbose", false, "対象ドメインのレスポンスもログ出力する")
	fs.BoolVar(&f.rc.Redact, "redact", false, "起動ログで全ヘッダー値をマスクする")
	fs.BoolVar(&f.showVersion, "version", false, "バージョンを表示して終了する")
	fs.BoolVar(&f.gui, "gui", false, "ブラウザで操作するローカル Web 管理画面を起動する")
	fs.StringVar(&f.guiListen, "gui-listen", "127.0.0.1:9090", "--gui 時の管理画面の待受アドレス")
	fs.BoolVar(&f.noOpen, "no-open", false, "--gui 時にブラウザを自動起動しない")
}

// Parse は args を Command に解析する。フラグエラー(-h を含む)は、使用方法を
// output に書き出した上でそのまま返す。引数なしの場合も使い方を表示する。
func Parse(name string, args []string, output io.Writer) (*Command, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(output)
	fs.Usage = func() { writeUsage(fs, name) }

	var f parseFlags
	bindFlags(fs, &f)

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	// 引数なしで実行されたら、エラーにせず使い方とオプション一覧を表示して終了する。
	if len(args) == 0 {
		fs.Usage()
		return nil, flag.ErrHelp
	}

	if f.showVersion {
		return &Command{Mode: ModeVersion}, nil
	}

	// 繰り返しフラグを RunConfig へ移し、--config 指定時は明示しなかった項目だけ
	// 設定ファイルの値で埋める。
	f.rc.Domains = f.domains
	f.rc.Headers = f.headers
	f.rc.Allow = f.allow
	if f.configPath != "" {
		if err := applyConfig(&f.rc, f.configPath, fs); err != nil {
			return nil, err
		}
	}

	if f.gui {
		return &Command{
			Mode: ModeGUI,
			GUI:  GUIOptions{Listen: f.guiListen, NoOpen: f.noOpen},
		}, nil
	}
	if f.genCA {
		return &Command{
			Mode:  ModeGenCA,
			GenCA: usecase.GenerateCAInput{CertPath: f.rc.CACertPath, KeyPath: f.rc.CAKeyPath, Force: f.force},
		}, nil
	}

	input, err := config.ToRunProxyInput(f.rc)
	if err != nil {
		return nil, err
	}
	return &Command{
		Mode:    ModeRun,
		Quiet:   f.rc.Quiet,
		Verbose: f.rc.Verbose,
		Run:     input,
	}, nil
}
