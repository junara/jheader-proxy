package web

import (
	"fmt"
	"sync"
)

// defaultLogCapacity はリングバッファに保持する最大行数。
const defaultLogCapacity = 1000

// LogSink は usecase.Logger を実装し、ログ行をリングバッファに蓄積しつつ
// 全 SSE 購読者へブロードキャストする。プロキシの起動・停止をまたいで
// GUI セッション中ずっと生存する。
type LogSink struct {
	mu       sync.Mutex
	capacity int
	buf      []string
	subs     map[int]chan string
	nextID   int
}

// NewLogSink は容量 capacity(<=0 なら既定値)の LogSink を返す。
func NewLogSink(capacity int) *LogSink {
	if capacity <= 0 {
		capacity = defaultLogCapacity
	}
	return &LogSink{
		capacity: capacity,
		subs:     make(map[int]chan string),
	}
}

// Printf は1行を整形して蓄積・配信する。usecase.Logger を満たす。
func (s *LogSink) Printf(format string, args ...any) {
	line := fmt.Sprintf(format, args...)

	s.mu.Lock()
	s.buf = append(s.buf, line)
	if len(s.buf) > s.capacity {
		s.buf = s.buf[len(s.buf)-s.capacity:]
	}
	subs := make([]chan string, 0, len(s.subs))
	for _, ch := range s.subs {
		subs = append(subs, ch)
	}
	s.mu.Unlock()

	// 配信はロック外で行う。遅い購読者がいてもブロックしないよう、
	// バッファが満杯なら最新行を捨てる(購読者側が追従できない場合の保険)。
	for _, ch := range subs {
		select {
		case ch <- line:
		default:
		}
	}
}

// Snapshot は現在保持しているログ行のコピーを返す。画面ロード時の初期表示用。
func (s *LogSink) Snapshot() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.buf))
	copy(out, s.buf)
	return out
}

// Subscribe は新規ログ行を受け取るチャネルと、購読解除用の関数を返す。
// 解除関数は必ず呼ぶこと(SSE ハンドラ終了時)。
func (s *LogSink) Subscribe() (<-chan string, func()) {
	ch := make(chan string, 256)

	s.mu.Lock()
	id := s.nextID
	s.nextID++
	s.subs[id] = ch
	s.mu.Unlock()

	cancel := func() {
		s.mu.Lock()
		if _, ok := s.subs[id]; ok {
			delete(s.subs, id)
			close(ch)
		}
		s.mu.Unlock()
	}
	return ch, cancel
}
