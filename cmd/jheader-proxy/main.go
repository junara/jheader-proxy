// Command jheader-proxy は、指定ドメインへのリクエストに設定済みのリクエスト
// ヘッダーを付与するローカル HTTP/HTTPS プロキシ。iPhone の Wi-Fi プロキシ経由で
// 開発用サイトを検証する用途を想定する。
//
// このファイルは合成ルートであり、インフラのアダプタをユースケースへ結線し、
// 解析されたコマンドに応じてディスパッチする。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/junara/jheader-proxy/internal/adapter/cli"
	"github.com/junara/jheader-proxy/internal/adapter/web"
	"github.com/junara/jheader-proxy/internal/infra/ca"
	"github.com/junara/jheader-proxy/internal/infra/proxy"
	"github.com/junara/jheader-proxy/internal/usecase"
)

// version はビルド時に -ldflags "-X main.version=..." で埋め込む。
var version = "dev"

func main() {
	cmd, err := cli.Parse("jheader-proxy", os.Args[1:], os.Stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fatal(err)
	}

	switch cmd.Mode {
	case cli.ModeVersion:
		fmt.Println(versionString())
	case cli.ModeGenCA:
		if err := runGenCA(ca.New(), cmd.GenCA); err != nil {
			fatal(err)
		}
	case cli.ModeRun:
		if err := runProxy(cmd); err != nil {
			fatal(err)
		}
	case cli.ModeGUI:
		if err := runGUI(cmd); err != nil {
			fatal(err)
		}
	}
}

func runGUI(cmd *cli.Command) error {
	// SIGINT / SIGTERM で ctx をキャンセルし、管理画面とプロキシを穏当に停止する。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store := ca.New()
	deps := web.Deps{
		NewProxyServer: func(logger usecase.Logger, quiet, verbose bool) usecase.ProxyServer {
			return proxy.New(logger, proxy.Options{Quiet: quiet, Verbose: verbose})
		},
		CAProvider:  store,
		CAGenerator: store,
	}
	return web.Serve(ctx, deps, web.Options{
		Listen:  cmd.GUI.Listen,
		NoOpen:  cmd.GUI.NoOpen,
		Version: versionString(),
	})
}

func runProxy(cmd *cli.Command) error {
	// SIGINT / SIGTERM で ctx をキャンセルし、プロキシを穏当に停止する。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := log.New(os.Stderr, "", log.LstdFlags)
	server := proxy.New(logger, proxy.Options{Quiet: cmd.Quiet, Verbose: cmd.Verbose})
	return usecase.NewRunProxy(ca.New(), server, logger).Execute(ctx, cmd.Run)
}

func runGenCA(store usecase.CAGenerator, in usecase.GenerateCAInput) error {
	if err := usecase.NewGenerateCA(store).Execute(in); err != nil {
		return err
	}
	fmt.Printf("generated CA certificate: %s\n", in.CertPath)
	fmt.Printf("generated CA private key: %s\n", in.KeyPath)
	fmt.Println("install the certificate on your iPhone and enable trust to use HTTPS MITM.")
	fmt.Println("keep the private key out of version control.")
	return nil
}

// versionString はビルド埋め込みのバージョン、無ければモジュール情報を返す。
func versionString() string {
	if version != "dev" {
		return "jheader-proxy " + version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return "jheader-proxy " + v
		}
	}
	return "jheader-proxy dev"
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
