package web

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/junara/jheader-proxy/internal/config"
	"github.com/junara/jheader-proxy/internal/domain"
	"github.com/junara/jheader-proxy/internal/usecase"
)

// ErrAlreadyRunning は稼働中にもう一度 Start を呼んだときに返る。
var ErrAlreadyRunning = errors.New("proxy is already running")

// Deps は Controller がユースケースを動かすために必要な依存。合成ルート
// (main.go)からインフラ実装を注入する。web 層は usecase/domain にのみ依存する。
type Deps struct {
	// NewProxyServer は quiet/verbose を反映したプロキシサーバを生成する。
	NewProxyServer func(logger usecase.Logger, quiet, verbose bool) usecase.ProxyServer
	CAProvider     usecase.CAProvider
	CAGenerator    usecase.CAGenerator
}

// Controller はプロキシの起動・停止を制御し、現在状態を保持する。
// GUI セッション中ずっと生存する単一インスタンス。
type Controller struct {
	deps Deps
	sink *LogSink

	mu        sync.Mutex
	running   bool
	cancel    context.CancelFunc
	done      chan struct{} // 実行 goroutine 終了時に close される
	startedAt time.Time
	duration  time.Duration
	cfg       RunConfig
	lastErr   string
}

// NewController は依存・ログシンク・初期フォーム設定で Controller を構築する。
func NewController(deps Deps, sink *LogSink, initial RunConfig) *Controller {
	return &Controller{deps: deps, sink: sink, cfg: initial}
}

// Sink は共有ログシンクを返す(SSE ハンドラ用)。
func (c *Controller) Sink() *LogSink { return c.sink }

// Start は設定を検証してプロキシを起動する。待受開始(成功)を確認できるまで
// ブロックし、起動前/起動時に失敗した場合はそのエラーを返す。
func (c *Controller) Start(cfg RunConfig) error {
	input, dur, err := buildInput(cfg)
	if err != nil {
		return err
	}

	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return ErrAlreadyRunning
	}

	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan struct{})
	var readyOnce sync.Once
	input.OnReady = func() { readyOnce.Do(func() { close(ready) }) }

	server := c.deps.NewProxyServer(c.sink, cfg.Quiet, cfg.Verbose)
	uc := usecase.NewRunProxy(c.deps.CAProvider, server, c.sink)

	done := make(chan struct{})
	errCh := make(chan error, 1)
	c.running = true
	c.cancel = cancel
	c.done = done
	c.startedAt = time.Now()
	c.duration = dur
	c.cfg = cfg
	c.lastErr = ""
	c.mu.Unlock()

	go func() {
		runErr := uc.Execute(ctx, input)
		c.mu.Lock()
		c.running = false
		c.cancel = nil
		if runErr != nil && !errors.Is(runErr, context.Canceled) {
			c.lastErr = runErr.Error()
		}
		c.mu.Unlock()
		cancel()
		close(done)
		errCh <- runErr
	}()

	// 待受開始(OnReady)で成功確定。OnReady 前に Execute が返ったら起動失敗。
	select {
	case <-ready:
		return nil
	case runErr := <-errCh:
		return runErr
	}
}

// Stop は稼働中のプロキシを穏当に停止し、完全停止まで待つ。停止済みなら何もしない。
func (c *Controller) Stop() {
	c.mu.Lock()
	cancel := c.cancel
	done := c.done
	c.mu.Unlock()

	if cancel == nil {
		return
	}
	cancel()
	if done != nil {
		<-done
	}
}

// GenerateCA は指定パスに CA を生成する。親ディレクトリは事前に作成する。
func (c *Controller) GenerateCA(certPath, keyPath string, force bool) error {
	for _, p := range []string{certPath, keyPath} {
		if p == "" {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			return fmt.Errorf("failed to create directory for %q: %w", p, err)
		}
	}
	in := usecase.GenerateCAInput{CertPath: certPath, KeyPath: keyPath, Force: force}
	return usecase.NewGenerateCA(c.deps.CAGenerator).Execute(in)
}

// StateView は /api/state のレスポンス。
type StateView struct {
	Running          bool      `json:"running"`
	StartedAt        string    `json:"startedAt,omitempty"`
	Duration         string    `json:"duration,omitempty"`
	RemainingSeconds int       `json:"remainingSeconds"` // -1 は無制限
	LastError        string    `json:"lastError,omitempty"`
	Config           RunConfig `json:"config"`
}

// State は現在状態のスナップショットを返す。
func (c *Controller) State() StateView {
	c.mu.Lock()
	defer c.mu.Unlock()

	view := StateView{
		Running:          c.running,
		RemainingSeconds: -1,
		LastError:        c.lastErr,
		Config:           c.cfg,
	}
	if c.running {
		view.StartedAt = c.startedAt.Format(time.RFC3339)
		if c.duration > 0 {
			view.Duration = c.duration.String()
			remaining := max(c.duration-time.Since(c.startedAt), 0)
			view.RemainingSeconds = int(remaining.Seconds())
		}
	}
	return view
}

// buildInput は RunConfig を usecase 入力へ変換する。CLI と同じく
// domain.ParseHeaders を再利用し、duration 文字列を解釈する。
func buildInput(cfg RunConfig) (usecase.RunProxyInput, time.Duration, error) {
	headers, err := domain.ParseHeaders(config.HeadersToSpecs(cfg.Headers))
	if err != nil {
		return usecase.RunProxyInput{}, 0, err
	}

	dur, err := config.ParseDuration(cfg.Duration)
	if err != nil {
		return usecase.RunProxyInput{}, 0, err
	}

	return usecase.RunProxyInput{
		Listen:       strings.TrimSpace(cfg.Listen),
		Domains:      trimNonEmpty(cfg.Domains),
		Headers:      headers,
		CACertPath:   cfg.CACertPath,
		CAKeyPath:    cfg.CAKeyPath,
		Allow:        trimNonEmpty(cfg.Allow),
		RedactValues: cfg.Redact,
		Duration:     dur,
	}, dur, nil
}

// trimNonEmpty は各要素を trim し、空要素を除いたスライスを返す。
func trimNonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
