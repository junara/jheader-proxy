package web

import (
	"testing"
	"time"
)

func TestLogSinkRingBuffer(t *testing.T) {
	s := NewLogSink(3)
	for i := range 5 {
		s.Printf("line%d", i)
	}
	snap := s.Snapshot()
	if len(snap) != 3 {
		t.Fatalf("expected 3 retained lines, got %d", len(snap))
	}
	if snap[0] != "line2" || snap[2] != "line4" {
		t.Fatalf("unexpected window: %v", snap)
	}
}

func TestLogSinkSubscribeBroadcast(t *testing.T) {
	s := NewLogSink(10)
	ch, cancel := s.Subscribe()
	defer cancel()

	s.Printf("hello %s", "world")
	select {
	case line := <-ch:
		if line != "hello world" {
			t.Fatalf("unexpected line: %q", line)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for broadcast")
	}
}

func TestLogSinkCancelStopsDelivery(t *testing.T) {
	s := NewLogSink(10)
	ch, cancel := s.Subscribe()
	cancel()
	// 解除後はチャネルが閉じられている。
	if _, ok := <-ch; ok {
		t.Fatal("expected channel closed after cancel")
	}
	// 解除後の Printf がパニックしないこと。
	s.Printf("after cancel")
}
