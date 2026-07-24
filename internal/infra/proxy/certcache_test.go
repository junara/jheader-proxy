package proxy

import (
	"crypto/tls"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elazarl/goproxy"
)

func TestCertCacheGeneratesOncePerHost(t *testing.T) {
	c := newCertCache()
	var calls atomic.Int32
	gen := func() (*tls.Certificate, error) {
		calls.Add(1)
		return &tls.Certificate{}, nil
	}

	for range 5 {
		if _, err := c.Fetch("example.test", gen); err != nil {
			t.Fatalf("Fetch: %v", err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("gen called %d times, want 1", got)
	}

	if _, err := c.Fetch("other.test", gen); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("gen called %d times after second host, want 2", got)
	}
}

// TestCertCacheConcurrentSameHost は、ブラウザが同一ホストへ同時に複数接続を張る
// 状況で発行が1回にまとまることを確かめる。
func TestCertCacheConcurrentSameHost(t *testing.T) {
	c := newCertCache()
	var calls atomic.Int32
	release := make(chan struct{})
	// error を返さない実装だが、シグネチャは Fetch の引数に合わせる必要がある。
	gen := func() (*tls.Certificate, error) { //nolint:unparam // Fetch のシグネチャに合わせる
		calls.Add(1)
		<-release // 後続の Fetch が待ち合わせに入るまで発行を終わらせない
		return &tls.Certificate{}, nil
	}

	const n = 16
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			if _, err := c.Fetch("example.test", gen); err != nil {
				t.Errorf("Fetch: %v", err)
			}
		}()
	}
	close(release)
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Errorf("gen called %d times, want 1", got)
	}
}

func TestCertCacheDoesNotCacheFailure(t *testing.T) {
	c := newCertCache()
	wantErr := errors.New("sign failed")
	var calls atomic.Int32
	gen := func() (*tls.Certificate, error) {
		calls.Add(1)
		return nil, wantErr
	}

	for range 2 {
		if _, err := c.Fetch("example.test", gen); !errors.Is(err, wantErr) {
			t.Fatalf("err = %v, want %v", err, wantErr)
		}
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("gen called %d times, want 2 (failures must be retried)", got)
	}
}

// TestCertCacheGenPanicReturnsErrorAndRetries は、gen が panic しても Fetch が
// デッドロックせずエラーを返し、失敗エントリを残さず次回は再発行できることを確かめる。
func TestCertCacheGenPanicReturnsErrorAndRetries(t *testing.T) {
	c := newCertCache()

	_, err := c.Fetch("example.test", func() (*tls.Certificate, error) {
		panic("boom")
	})
	if err == nil {
		t.Fatal("Fetch with panicking gen returned nil error, want error")
	}

	// 失敗エントリは残らないので、次は正常に発行できる。
	var calls atomic.Int32
	cert, err := c.Fetch("example.test", func() (*tls.Certificate, error) {
		calls.Add(1)
		return &tls.Certificate{}, nil
	})
	if err != nil {
		t.Fatalf("retry after panic: %v", err)
	}
	if cert == nil {
		t.Error("retry after panic returned nil cert")
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("retry gen called %d times, want 1", got)
	}
}

// TestCertCacheNilCertWithoutErrorIsRejected は、gen が契約に反して (nil, nil) を
// 返しても nil 証明書をキャッシュせず(goproxy が nil 参照でクラッシュするため)、
// エラーを返して次回は再発行できることを確かめる。
func TestCertCacheNilCertWithoutErrorIsRejected(t *testing.T) {
	c := newCertCache()

	if _, err := c.Fetch("example.test", func() (*tls.Certificate, error) {
		return nil, nil //nolint:nilnil // この契約違反への耐性こそがテスト対象
	}); err == nil {
		t.Fatal("Fetch with gen returning (nil, nil) returned nil error, want error")
	}

	// エントリは残らないので、次は正常に発行できる。
	cert, err := c.Fetch("example.test", func() (*tls.Certificate, error) {
		return &tls.Certificate{}, nil
	})
	if err != nil {
		t.Fatalf("retry after (nil, nil): %v", err)
	}
	if cert == nil {
		t.Error("retry after (nil, nil) returned nil cert")
	}
}

// TestCertCacheConcurrentGenPanic は、勝者の gen が panic しても同一ホストへの
// 並行 Fetch が全て解放される(永久ブロックしない)ことを確かめる。
func TestCertCacheConcurrentGenPanic(t *testing.T) {
	c := newCertCache()
	start := make(chan struct{})
	gen := func() (*tls.Certificate, error) {
		<-start // 全 goroutine が Fetch に入るまで発行を終わらせない
		panic("boom")
	}

	const n = 8
	var wg sync.WaitGroup
	wg.Add(n)
	errs := make([]error, n)
	for i := range n {
		go func() {
			defer wg.Done()
			_, errs[i] = c.Fetch("example.test", gen)
		}()
	}
	close(start)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Fetch deadlocked after gen panic")
	}

	for i, err := range errs {
		if err == nil {
			t.Errorf("goroutine %d: got nil error, want error", i)
		}
	}
}

func TestTuneTransportRaisesIdleConnsPerHost(t *testing.T) {
	p := goproxy.NewProxyHttpServer()
	tuneTransport(p.Tr)

	if p.Tr.MaxIdleConnsPerHost <= 2 {
		t.Errorf("MaxIdleConnsPerHost = %d, want > 2 (Go's default)", p.Tr.MaxIdleConnsPerHost)
	}
	if p.Tr.TLSClientConfig == nil {
		t.Error("tuneTransport dropped goproxy's TLSClientConfig")
	}
	if p.Tr.DialContext == nil {
		t.Error("DialContext not set")
	}
}

// BenchmarkCertSigning は MITM 用証明書の発行コストを測る。キャッシュの有無で
// 1回あたりの所要時間がどれだけ違うかを確認するためのもの。
func BenchmarkCertSigning(b *testing.B) {
	p := goproxy.NewProxyHttpServer()
	newTLSConfig := goproxy.TLSConfigFromCA(testCA(b))
	pctx := &goproxy.ProxyCtx{Proxy: p}

	b.Run("uncached", func(b *testing.B) {
		for b.Loop() {
			if _, err := newTLSConfig("example.test:443", pctx); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("cached", func(b *testing.B) {
		c := newCertCache()
		gen := func() (*tls.Certificate, error) {
			conf, err := newTLSConfig("example.test:443", pctx)
			if err != nil {
				return nil, err
			}
			return &conf.Certificates[0], nil
		}
		for b.Loop() {
			if _, err := c.Fetch("example.test", gen); err != nil {
				b.Fatal(err)
			}
		}
	})
}
