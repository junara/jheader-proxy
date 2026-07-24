package proxy

import (
	"crypto/tls"
	"fmt"
	"sync"
)

// certCache は goproxy.CertStorage の実装で、MITM 用に発行したサーバ証明書を
// ホスト名ごとに保持する。
//
// これが無いと goproxy は CONNECT のたびに証明書を発行し直す。発行処理には
// RSA-2048 の鍵生成が含まれ、1回あたり数十ミリ秒〜数百ミリ秒かかる。ブラウザは
// 1ページの表示で同一ホストへ並行に複数の接続を張るため、キャッシュしないと
// アセットの読み込みが直列化されて目に見えて遅くなる。
//
// goproxy の署名処理はCA秘密鍵とホスト名から決定的に鍵を導出するので、同じ
// ホストなら毎回同じ証明書になる。つまりキャッシュしても挙動は変わらない。
//
// 発行済み証明書の有効期間は365日で、ローカル開発用プロキシのプロセス寿命を
// 大きく超える。よって期限切れの考慮はしない。
//
// エントリはエビクションせず保持し続ける。件数は Matcher.IsTarget を通過した
// 異なるホスト名の数に等しく、通常の固定ドメイン運用では数個に収まる。対象ドメインを
// 広く取り、多数の異なるサブドメインへアクセスする場合はプロセス寿命の間だけ増え続ける。
type certCache struct {
	mu    sync.Mutex
	certs map[string]*certEntry
}

// certEntry は1ホスト分の発行結果。ready が閉じられるまで cert/err は未確定。
type certEntry struct {
	ready chan struct{}
	cert  *tls.Certificate
	err   error
}

// newCertCache は空の certCache を返す。
func newCertCache() *certCache {
	return &certCache{certs: make(map[string]*certEntry)}
}

// Fetch は hostname の証明書を返す。未発行なら gen で発行してから返す。
//
// 同一ホストへの並行呼び出しは1回の発行にまとめ、後続は結果を待つ。ブラウザは
// 同一ホストへ複数接続を同時に張るため、これが無いと最初の1ページで同じ鍵生成が
// 何回も走ってしまう。
func (c *certCache) Fetch(hostname string, gen func() (*tls.Certificate, error)) (*tls.Certificate, error) {
	c.mu.Lock()
	if e, ok := c.certs[hostname]; ok {
		c.mu.Unlock()
		<-e.ready
		return e.cert, e.err
	}
	e := &certEntry{ready: make(chan struct{})}
	c.certs[hostname] = e
	c.mu.Unlock()

	c.generate(hostname, e, gen)
	return e.cert, e.err
}

// generate は e の cert/err を確定し、待機中の Fetch を起こすため ready を必ず閉じる。
// 失敗したエントリはマップに残さず、次の接続で再試行できるようにする。
//
// gen が panic した場合もエラーとして扱い ready を閉じる。これが無いと、panic 時に
// ready が閉じられないまま残り、同一ホストの他の待機 goroutine が永久にブロックする。
// また単に close するだけでは cert が nil のエントリが残り、goproxy 側が nil 証明書を
// 参照してクラッシュするため、recover でエラーに変換して削除経路へ載せる。
//
// cert も err も nil のまま終わった場合(gen が契約に反して (nil, nil) を返した、
// または runtime.Goexit で戻らなかった)も同様に nil 証明書が残ってしまうため、
// エラー扱いにして削除経路へ載せる。
func (c *certCache) generate(hostname string, e *certEntry, gen func() (*tls.Certificate, error)) {
	defer func() {
		if r := recover(); r != nil {
			e.cert = nil
			e.err = fmt.Errorf("panic while signing certificate for %q: %v", hostname, r)
		}
		if e.err == nil && e.cert == nil {
			e.err = fmt.Errorf("certificate generator returned no certificate for %q", hostname)
		}
		if e.err != nil {
			c.mu.Lock()
			if c.certs[hostname] == e {
				delete(c.certs, hostname)
			}
			c.mu.Unlock()
		}
		close(e.ready)
	}()
	e.cert, e.err = gen()
}
