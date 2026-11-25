// Package ffiapi exposes the scanner core as a C shared library so desktop
// clients can call it in-process over FFI instead of running an HTTP backend.
//
// It is consumed by cmd/libm3u, which re-exports these entry points from its
// main package (c-shared only exports symbols of the main package).
//
// Conventions:
//   - inputs are JSON-encoded strings
//   - outputs are JSON-encoded strings; call FreeString on anything returned
//   - failures travel in-band as {"error":"..."} instead of panicking
//
// Parsing returns a handle that Scan consumes, so a large playlist crosses the
// FFI boundary once (for display) rather than once per scan.
package ffiapi

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"m3u-scanner/internal/parser"
	"m3u-scanner/internal/scanner"
)

// ProgressFunc receives JSON events from scanner goroutines. The host must
// free the string.
type ProgressFunc func(eventJSON string)

var (
	cbMu sync.RWMutex
	cb   ProgressFunc

	playlistsMu sync.RWMutex
	playlists   = map[int64]*parser.M3UPlaylist{}
	nextHandle  atomic.Int64

	scanMu     sync.Mutex
	scanCancel context.CancelFunc
	scanState  atomic.Value // string: latest scan state JSON
)

func init() {
	scanState.Store(`{"running":false}`)
}

// SetProgressCallback registers the callback invoked from scanner goroutines.
// Passing nil clears the registration.
func SetProgressCallback(f ProgressFunc) {
	cbMu.Lock()
	cb = f
	cbMu.Unlock()
}

func emit(event any) {
	cbMu.RLock()
	f := cb
	cbMu.RUnlock()
	if f == nil {
		return
	}
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	f(string(data))
}

// ParseRequest carries the source to load.
type ParseRequest struct {
	URL       string `json:"url"`
	FilePath  string `json:"file_path"`
	UserAgent string `json:"user_agent"`
}

// ParseResponse returns a reusable handle plus the parsed playlist.
type ParseResponse struct {
	Handle   int64              `json:"handle"`
	Playlist parser.M3UPlaylist `json:"playlist"`
}

// Parse loads an m3u source from a URL or a local file and returns a handle.
func Parse(reqJSON string) string {
	var req ParseRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		return errorJSON(err)
	}

	var (
		playlist *parser.M3UPlaylist
		err      error
	)
	switch {
	case req.FilePath != "":
		playlist, err = parser.ParseFile(req.FilePath)
	case req.URL != "":
		playlist, err = parser.ParseURLWithUA(req.URL, req.UserAgent)
	default:
		return errorJSON(errString("需要 url 或 file_path"))
	}
	if err != nil {
		return errorJSON(err)
	}

	handle := nextHandle.Add(1)
	playlistsMu.Lock()
	playlists[handle] = playlist
	playlistsMu.Unlock()

	return jsonOf(ParseResponse{Handle: handle, Playlist: *playlist})
}

// ScanRequest references a parsed playlist plus scan settings.
type ScanRequest struct {
	Handle      int64  `json:"handle"`
	Concurrency int    `json:"concurrency"`
	TimeoutSec  int    `json:"timeout_sec"`
	QuickCheck  bool   `json:"quick_check"`
	UserAgent   string `json:"user_agent"`
}

// Scan starts a scan and returns immediately with {"started":true}.
// Progress and the final result arrive through the callback as
// {"type":"progress"...} / {"type":"complete","results":[...]} / {"type":"cancelled"}.
// A second call cancels the previous scan.
func Scan(reqJSON string) string {
	var req ScanRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		return errorJSON(err)
	}

	playlistsMu.RLock()
	playlist, ok := playlists[req.Handle]
	playlistsMu.RUnlock()
	if !ok {
		return errorJSON(errString("播放列表不存在,请重新加载"))
	}

	cancelPreviousScan()

	ctx, cancel := context.WithCancel(context.Background())
	scanMu.Lock()
	scanCancel = cancel
	scanMu.Unlock()

	go runScan(ctx, playlist, req)
	return jsonOf(map[string]bool{"started": true})
}

func runScan(ctx context.Context, playlist *parser.M3UPlaylist, req ScanRequest) {
	defer func() {
		scanMu.Lock()
		if scanCancel != nil {
			scanCancel()
			scanCancel = nil
		}
		scanMu.Unlock()
	}()

	s := scanner.NewScanner(
		req.Concurrency,
		time.Duration(req.TimeoutSec)*time.Second,
		req.QuickCheck,
		req.UserAgent,
	)

	progressCh := make(chan scanner.ScanProgress, 8)
	go func() {
		for p := range progressCh {
			scanState.Store(stateJSON(p))
			emit(map[string]any{
				"type":      "progress",
				"total":     p.Total,
				"completed": p.Completed,
				"available": p.Available,
				"failed":    p.Failed,
			})
		}
	}()

	results, err := s.ScanSync(ctx, playlist, progressCh)
	if ctx.Err() != nil {
		scanState.Store(`{"running":false}`)
		emit(map[string]string{"type": "cancelled"})
		return
	}
	if err != nil {
		scanState.Store(`{"running":false}`)
		emit(map[string]string{"type": "error", "error": err.Error()})
		return
	}

	scanState.Store(`{"running":false}`)
	emit(map[string]any{"type": "complete", "results": results})
}

// CancelScan aborts an in-flight scan; safe when idle.
func CancelScan() {
	cancelPreviousScan()
}

func cancelPreviousScan() {
	scanMu.Lock()
	if scanCancel != nil {
		scanCancel()
		scanCancel = nil
	}
	scanMu.Unlock()
}

// ScanState returns the most recent scan state, letting the host recover after
// a missed callback.
func ScanState() string {
	if s, ok := scanState.Load().(string); ok {
		return s
	}
	return `{"running":false}`
}

// ReleaseHandle drops a parsed playlist from memory.
func ReleaseHandle(handle int64) {
	playlistsMu.Lock()
	delete(playlists, handle)
	playlistsMu.Unlock()
}

func stateJSON(p scanner.ScanProgress) string {
	data, _ := json.Marshal(map[string]any{
		"running":   true,
		"total":     p.Total,
		"completed": p.Completed,
		"available": p.Available,
		"failed":    p.Failed,
	})
	return string(data)
}

func jsonOf(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return errorJSON(err)
	}
	return string(data)
}

func errorJSON(err error) string {
	data, _ := json.Marshal(map[string]string{"error": err.Error()})
	return string(data)
}

type errString string

func (e errString) Error() string { return string(e) }
