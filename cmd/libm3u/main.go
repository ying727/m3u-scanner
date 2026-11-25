// Command libm3u builds the scanner core as a C shared library for desktop
// clients that link it in-process.
//
// Build (Windows, needs a C toolchain for cgo):
//
//	go build -buildmode=c-shared -o m3u_core.dll ./cmd/libm3u
//
// The plain web server in the repository root is unaffected: this is a second
// wrapper around the same internal packages.
package main

/*
#include <stdlib.h>

// Callback type: receives a JSON event string, which the host must free.
typedef void (*m3u_progress_cb)(char* event_json);

static void m3u_call_cb(m3u_progress_cb cb, char* event_json) {
	cb(event_json);
}
*/
import "C"

import (
	"os"
	"path/filepath"
	"unsafe"

	"m3u-scanner/internal/ffiapi"
	"m3u-scanner/internal/ffprobe"
)

//export M3USetProgressCallback
func M3USetProgressCallback(cb C.m3u_progress_cb) {
	if cb == nil {
		ffiapi.SetProgressCallback(nil)
		return
	}
	ffiapi.SetProgressCallback(func(eventJSON string) {
		C.m3u_call_cb(cb, C.CString(eventJSON))
	})
}

//export M3UParse
func M3UParse(reqJSON *C.char) *C.char {
	return C.CString(ffiapi.Parse(C.GoString(reqJSON)))
}

//export M3UScan
func M3UScan(reqJSON *C.char) *C.char {
	return C.CString(ffiapi.Scan(C.GoString(reqJSON)))
}

//export M3UCancelScan
func M3UCancelScan() {
	ffiapi.CancelScan()
}

//export M3UScanState
func M3UScanState() *C.char {
	return C.CString(ffiapi.ScanState())
}

//export M3UReleaseHandle
func M3UReleaseHandle(handle C.longlong) {
	ffiapi.ReleaseHandle(int64(handle))
}

//export M3USetFFprobePath
func M3USetFFprobePath(path *C.char) {
	p := C.GoString(path)
	if p == "" {
		return
	}
	if info, err := os.Stat(p); err != nil || info.IsDir() {
		return
	}
	ffprobe.SetPath(filepath.Clean(p))
}

//export M3UFFprobeAvailable
func M3UFFprobeAvailable() C.int {
	if ffprobe.IsFFprobeAvailable() {
		return 1
	}
	return 0
}

//export M3UFreeString
func M3UFreeString(s *C.char) {
	C.free(unsafe.Pointer(s))
}

func main() {}
