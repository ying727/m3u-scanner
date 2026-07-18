package scanner

import (
	"context"
	"testing"
	"time"

	"m3u-scanner/internal/parser"
)

func TestScanEmptyPlaylistClosesChannels(t *testing.T) {
	s := NewScanner(1, time.Second, true, "")
	progress := make(chan ScanProgress, 1)
	results := make(chan ScanResult, 1)

	if err := s.Scan(context.Background(), &parser.M3UPlaylist{}, progress, results); err != nil {
		t.Fatalf("Scan returned unexpected error: %v", err)
	}
	if _, ok := <-progress; ok {
		t.Fatal("progress channel should be closed for an empty playlist")
	}
	if _, ok := <-results; ok {
		t.Fatal("result channel should be closed for an empty playlist")
	}
}

func TestScanPreCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := NewScanner(1, time.Second, true, "")
	progress := make(chan ScanProgress, 1)
	results := make(chan ScanResult, 1)
	playlist := &parser.M3UPlaylist{Channels: []parser.Channel{{Name: "test", URL: "http://127.0.0.1:1/"}}}

	err := s.Scan(ctx, playlist, progress, results)
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if _, ok := <-progress; ok {
		t.Fatal("progress channel should be closed after cancellation")
	}
	if _, ok := <-results; ok {
		t.Fatal("result channel should be closed after cancellation")
	}
}
