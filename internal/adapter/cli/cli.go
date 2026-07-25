// Package cli はコマンドライン引数をユースケースの入力へ変換するインターフェース
// アダプタ。サブコマンド(run / gen-ca / gui / version)を解析し、フラグを
// config.RunConfig に組み立て、--config(設定ファイル)とマージしたうえで
// config.ToRunProxyInput により GUI と同一の変換を通す。
//
// 従来のフラグ形式(--gen-ca / --gui やサブコマンドなしの run 相当)も後方互換の
// ため受け付けるが、非推奨警告を出す。--version はフラグとしても慣習的なので
// 警告なしで受け付ける。
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
	if !set["header"] && !set["header-file"] {
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
	// quiet / verbose は矛盾しうるペアなので、どちらかがフラグで明示されたら設定
	// ファイルの両方の値を無視する(設定に quiet があっても --verbose 明示が勝つ)。
	if !set["quiet"] && !set["verbose"] {
		rc.Quiet = fc.Quiet
		rc.Verbose = fc.Verbose
	}
	if !set["redact"] {
		rc.Redact = fc.Redact
	}
	return nil
}

// rootUsageTemplate はコマンド一覧つきの全体の使い方。%[1]s はプログラム名。
const rootUsageTemplate = `%[1]s - 対象ドメインへのリクエストに HTTP ヘッダーを付与するローカル HTTP/HTTPS プロキシ。

使い方:
  %[1]s <コマンド> [オプション]

コマンド:
  run      プロキシを起動する
  gen-ca   自分専用の CA 証明書・秘密鍵を生成する（HTTPS に必須）
  gui      ブラウザで操作するローカル Web 管理画面を起動する
  version  バージョンを表示する
  help     使い方を表示する（'help <コマンド>' で各コマンドの詳細）

例:
  %[1]s gen-ca --cert ca-cert.pem --key ca-key.pem
  %[1]s run --domain example.test --header "X-Debug-User=jun" --ca-cert ca-cert.pem --ca-key ca-key.pem
  %[1]s gui

各コマンドの詳細は '%[1]s <コマンド> --help' を参照してください。

詳しいマニュアル: https://junara.github.io/jheader-proxy/
不具合報告:       https://github.com/junara/jheader-proxy/issues
`

// runUsageTemplate は run サブコマンドの使い方。%[1]s はプログラム名。
const runUsageTemplate = `%[1]s run - ヘッダーを付与するプロキシを起動する。

使い方:
  %[1]s run --domain <ホスト> --header <Name=Value> --ca-cert <パス> --ca-key <パス> [オプション]
  %[1]s run --config <ファイル>   # 設定を JSON ファイルから読む

オプション:
`

// genCAUsageTemplate は gen-ca サブコマンドの使い方。%[1]s はプログラム名。
const genCAUsageTemplate = `%[1]s gen-ca - HTTPS MITM に使う自分専用の CA 証明書と秘密鍵を生成する。

使い方:
  %[1]s gen-ca --cert <証明書の出力先> --key <秘密鍵の出力先> [--force]

オプション:
`

// guiUsageTemplate は gui サブコマンドの使い方。%[1]s はプログラム名。
const guiUsageTemplate = `%[1]s gui - ブラウザで操作するローカル Web 管理画面を起動する。

使い方:
  %[1]s gui [--listen 127.0.0.1:9090] [--no-open]

オプション:
`

// サブコマンド名。
const (
	cmdRun     = "run"
	cmdGenCA   = "gen-ca"
	cmdGUI     = "gui"
	cmdVersion = "version"
	cmdHelp    = "help"
)

// helpFlag は明示的なヘルプ要求のロングフラグ表記。
const helpFlag = "--help"

// commands は認識するサブコマンド名の一覧。タイポ時の提案にも使う。
var commands = []string{cmdRun, cmdGenCA, cmdGUI, cmdVersion, cmdHelp}

// writeRootUsage はコマンド一覧つきの全体の使い方を w に書き出す。
func writeRootUsage(w io.Writer, name string) {
	_, _ = io.WriteString(w, fmt.Sprintf(rootUsageTemplate, name))
}

// writeSubUsage はサブコマンドの使い方の概要とオプション一覧を w に書き出す。
func writeSubUsage(fs *flag.FlagSet, template, name string, w io.Writer) {
	fs.SetOutput(w)
	_, _ = io.WriteString(w, fmt.Sprintf(template, name))
	fs.PrintDefaults()
}

// parseSubFlags は fs で args を解析し、ヘルプとエラーの出力先を使い分ける共通
// 処理。明示的に要求されたヘルプ(-h/--help)は stdout へ書き出し(clig.dev:
// grep やページャに渡せるように)、フラグエラーは使い方を示すヒントを添えて
// 返す(flag パッケージ自身の出力は抑制し、呼び出し元の error 表示に一本化する)。
func parseSubFlags(fs *flag.FlagSet, args []string, usage func(io.Writer), stdout io.Writer, helpCmd string) error {
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	err := fs.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		usage(stdout)
		return flag.ErrHelp
	}
	if err != nil {
		return fmt.Errorf("%w; run '%s --help' for usage", err, helpCmd)
	}
	return nil
}

// Parse は args を Command に解析する。先頭引数をサブコマンドとして解釈し、
// フラグで始まる場合は従来のフラグ形式(非推奨)として解析する。明示的に要求
// されたヘルプ(help / -h / --help)は stdout へ、引数なし実行時の使い方表示や
// 非推奨警告は stderr へ書き出す。
func Parse(name string, args []string, stdout, stderr io.Writer) (*Command, error) {
	// 引数なしで実行されたら、エラーにせず使い方とコマンド一覧を表示して終了する。
	// これは誤用の通知なので stderr へ出す。
	if len(args) == 0 {
		writeRootUsage(stderr, name)
		return nil, flag.ErrHelp
	}

	switch args[0] {
	case cmdHelp, "-h", "-help", helpFlag:
		return parseHelp(name, args[1:], stdout)
	case cmdVersion:
		return &Command{Mode: ModeVersion}, nil
	case cmdRun:
		return parseRun(name, args[1:], stdout)
	case cmdGenCA:
		return parseGenCA(name, args[1:], stdout)
	case cmdGUI:
		return parseGUI(name, args[1:], stdout)
	}

	// フラグで始まる場合は従来のフラグ形式(非推奨)として受け付ける。
	if strings.HasPrefix(args[0], "-") {
		return parseLegacy(name, args, stdout, stderr)
	}
	return nil, unknownCommandError(name, args[0])
}

// parseHelp は help コマンドを解析する。引数なしなら全体の使い方を、コマンド名
// 付き(help run 等)ならそのコマンドの使い方を stdout へ書き出す。
func parseHelp(name string, args []string, stdout io.Writer) (*Command, error) {
	if len(args) == 0 {
		writeRootUsage(stdout, name)
		return nil, flag.ErrHelp
	}
	switch args[0] {
	case cmdRun:
		return parseRun(name, []string{helpFlag}, stdout)
	case cmdGenCA:
		return parseGenCA(name, []string{helpFlag}, stdout)
	case cmdGUI:
		return parseGUI(name, []string{helpFlag}, stdout)
	case cmdVersion, cmdHelp:
		writeRootUsage(stdout, name)
		return nil, flag.ErrHelp
	}
	return nil, unknownCommandError(name, args[0])
}

// runFlags は run サブコマンド(および従来形式)のフラグ束縛先。
type runFlags struct {
	rc          config.RunConfig
	domains     stringList
	headers     headerList
	headerFiles stringList
	allow       stringList
	configPath  string
}

// bindRunFlags は fs にプロキシ実行に関わるフラグを登録する。
func bindRunFlags(fs *flag.FlagSet, f *runFlags) {
	fs.StringVar(&f.configPath, "config", "",
		"設定をまとめた JSON ファイルのパス（GUIの config.json と互換。コマンドライン引数が優先）")
	fs.StringVar(&f.rc.Listen, "listen", ":8080", "プロキシの待受アドレス（例: :8080）")
	fs.Var(&f.domains, "domain", "ヘッダー付与の対象ドメイン（複数指定可・サブドメインも対象）")
	fs.Var(&f.headers, "header", "付与するヘッダー（Name=Value 形式・複数指定可）")
	fs.Var(&f.headerFiles, "header-file",
		"付与するヘッダーを書いたファイルのパス（1行1件の Name=Value。空行と # 始まりの行は無視。複数指定可）。"+
			"トークン等の秘匿値はシェル履歴に残る --header ではなくこちらで渡す")
	fs.Var(&f.allow, "allow", "接続を許可するクライアントの IP / CIDR（複数指定可・未指定で全許可）")
	fs.StringVar(&f.rc.CACertPath, "ca-cert", "", "HTTPS MITM に使う CA 証明書 PEM のパス（必須）")
	fs.StringVar(&f.rc.CAKeyPath, "ca-key", "", "HTTPS MITM に使う CA 秘密鍵 PEM のパス（必須）")
	fs.StringVar(&f.rc.Duration, "duration", "10m", "この時間が過ぎると自動停止（例: 30m。0 で無制限）")
	fs.BoolVar(&f.rc.Quiet, "quiet", false, "リクエストごとのログを抑制する")
	fs.BoolVar(&f.rc.Verbose, "verbose", false, "対象ドメインのレスポンスもログ出力する")
	fs.BoolVar(&f.rc.Redact, "redact", false, "起動ログで全ヘッダー値をマスクする")
}

// buildRunCommand は解析済みの run フラグを設定ファイルとマージし、実行コマンドへ
// 変換する。run サブコマンドと従来形式が共通で通る経路。
func buildRunCommand(fs *flag.FlagSet, f *runFlags) (*Command, error) {
	// 繰り返しフラグを RunConfig へ移し、--config 指定時は明示しなかった項目だけ
	// 設定ファイルの値で埋める。--header-file のヘッダーを先に置き、同名は
	// --header の明示指定が勝つようにする(重複解決は後勝ち)。
	fileHeaders, err := readHeaderFiles(f.headerFiles)
	if err != nil {
		return nil, err
	}
	headers := make([]config.HeaderKV, 0, len(fileHeaders)+len(f.headers))
	headers = append(headers, fileHeaders...)
	headers = append(headers, f.headers...)
	f.rc.Domains = f.domains
	f.rc.Headers = headers
	f.rc.Allow = f.allow
	if f.configPath != "" {
		if err := applyConfig(&f.rc, f.configPath, fs); err != nil {
			return nil, err
		}
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

// parseRun は run サブコマンドを解析する。
func parseRun(name string, args []string, stdout io.Writer) (*Command, error) {
	fs := flag.NewFlagSet(name+" run", flag.ContinueOnError)
	var f runFlags
	bindRunFlags(fs, &f)

	usage := func(w io.Writer) { writeSubUsage(fs, runUsageTemplate, name, w) }
	if err := parseSubFlags(fs, args, usage, stdout, name+" run"); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("unexpected argument %q for 'run'", fs.Arg(0))
	}
	return buildRunCommand(fs, &f)
}

// parseGenCA は gen-ca サブコマンドを解析する。--cert/--key が主で、run と揃えた
// --ca-cert/--ca-key も別名として受け付ける。
func parseGenCA(name string, args []string, stdout io.Writer) (*Command, error) {
	fs := flag.NewFlagSet(name+" gen-ca", flag.ContinueOnError)
	var cert, key string
	var force bool
	fs.StringVar(&cert, "cert", "", "生成する CA 証明書 PEM の出力先パス（必須）")
	fs.StringVar(&cert, "ca-cert", "", "--cert の別名")
	fs.StringVar(&key, "key", "", "生成する CA 秘密鍵 PEM の出力先パス（必須）")
	fs.StringVar(&key, "ca-key", "", "--key の別名")
	fs.BoolVar(&force, "force", false, "既存ファイルを上書きする")

	usage := func(w io.Writer) { writeSubUsage(fs, genCAUsageTemplate, name, w) }
	if err := parseSubFlags(fs, args, usage, stdout, name+" gen-ca"); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("unexpected argument %q for 'gen-ca'", fs.Arg(0))
	}
	return &Command{
		Mode:  ModeGenCA,
		GenCA: usecase.GenerateCAInput{CertPath: cert, KeyPath: key, Force: force},
	}, nil
}

// parseGUI は gui サブコマンドを解析する。
func parseGUI(name string, args []string, stdout io.Writer) (*Command, error) {
	fs := flag.NewFlagSet(name+" gui", flag.ContinueOnError)
	var f GUIOptions
	fs.StringVar(&f.Listen, "listen", "127.0.0.1:9090", "管理画面の待受アドレス")
	fs.BoolVar(&f.NoOpen, "no-open", false, "ブラウザを自動起動しない")

	usage := func(w io.Writer) { writeSubUsage(fs, guiUsageTemplate, name, w) }
	if err := parseSubFlags(fs, args, usage, stdout, name+" gui"); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("unexpected argument %q for 'gui'", fs.Arg(0))
	}
	return &Command{Mode: ModeGUI, GUI: f}, nil
}

// legacyFlags は従来のフラグ形式(サブコマンドなし)の束縛先。run のフラグに、
// モード切替フラグ(--gen-ca / --gui / --version)とその付随フラグを加えたもの。
type legacyFlags struct {
	runFlags

	genCA       bool
	force       bool
	showVersion bool
	gui         bool
	guiListen   string
	noOpen      bool
}

// parseLegacy は従来のフラグ形式を解析する。--version 以外のモードフラグと
// サブコマンドなしの run 相当には非推奨警告を stderr へ出す。
func parseLegacy(name string, args []string, stdout, stderr io.Writer) (*Command, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	var f legacyFlags
	bindRunFlags(fs, &f.runFlags)
	fs.BoolVar(&f.genCA, cmdGenCA, false, "非推奨: 'gen-ca' サブコマンドを使う")
	fs.BoolVar(&f.force, "force", false, "--gen-ca 時に既存ファイルを上書きする")
	fs.BoolVar(&f.showVersion, "version", false, "バージョンを表示して終了する")
	fs.BoolVar(&f.gui, "gui", false, "非推奨: 'gui' サブコマンドを使う")
	fs.StringVar(&f.guiListen, "gui-listen", "127.0.0.1:9090", "非推奨: 'gui --listen' を使う")
	fs.BoolVar(&f.noOpen, "no-open", false, "非推奨: 'gui --no-open' を使う")

	usage := func(w io.Writer) {
		writeRootUsage(w, name)
		_, _ = io.WriteString(w, "\n従来のフラグ形式（非推奨）のオプション:\n")
		fs.SetOutput(w)
		fs.PrintDefaults()
	}
	if err := parseSubFlags(fs, args, usage, stdout, name); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}

	// モードフラグの同時指定は、これまで暗黙の優先順位で片方が黙殺されていた。
	// どれを意図したか判断できないためエラーにする。
	modes := 0
	for _, on := range []bool{f.genCA, f.gui, f.showVersion} {
		if on {
			modes++
		}
	}
	if modes > 1 {
		return nil, errors.New("cannot combine --gen-ca, --gui, and --version; specify only one")
	}

	// --version はフラグとしても慣習的(clig.dev 推奨)なので警告なしで受け付ける。
	if f.showVersion {
		return &Command{Mode: ModeVersion}, nil
	}
	if f.gui {
		deprecationWarning(stderr, name, "--gui", "gui")
		return &Command{
			Mode: ModeGUI,
			GUI:  GUIOptions{Listen: f.guiListen, NoOpen: f.noOpen},
		}, nil
	}
	if f.genCA {
		deprecationWarning(stderr, name, "--gen-ca", cmdGenCA)
		return &Command{
			Mode:  ModeGenCA,
			GenCA: usecase.GenerateCAInput{CertPath: f.rc.CACertPath, KeyPath: f.rc.CAKeyPath, Force: f.force},
		}, nil
	}

	deprecationWarning(stderr, name, "サブコマンドなしの実行", "run")
	return buildRunCommand(fs, &f.runFlags)
}

// deprecationWarning は従来形式の利用者に移行先のサブコマンドを案内する。
func deprecationWarning(w io.Writer, name, old, sub string) {
	_, _ = fmt.Fprintf(w, "警告: %s は非推奨です。今後は '%s %s' を使ってください。\n", old, name, sub)
}

// unknownCommandError は未知のサブコマンドに対するエラーを組み立てる。
// 綴りが近いコマンドがあれば候補として提案する。
func unknownCommandError(name, input string) error {
	if s := suggestCommand(input); s != "" {
		return fmt.Errorf("unknown command %q (did you mean %q?); run '%s help' for usage", input, s, name)
	}
	return fmt.Errorf("unknown command %q; run '%s help' for usage", input, name)
}

// suggestCommand は input に綴りが近いコマンド名を返す。候補がなければ空文字を
// 返す。許容する編集距離はコマンド名の半分まで(短い名前ほど厳しく)とし、
// 無関係な入力への見当違いな提案を避ける。
func suggestCommand(input string) string {
	input = strings.ToLower(input)
	best, bestDist := "", -1
	for _, c := range commands {
		d := editDistance(input, c)
		if d <= len(c)/2 && (bestDist < 0 || d < bestDist) {
			best, bestDist = c, d
		}
	}
	return best
}

// editDistance は2つの文字列のレーベンシュタイン距離を返す。
func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, min(cur[j-1]+1, prev[j-1]+cost))
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}
