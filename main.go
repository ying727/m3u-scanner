package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"m3u-scanner/internal/ui"
)

func main() {
	port := flag.Int("port", 8080, "Port to run the web server on")
	parentPID := flag.Int("parent-pid", 0, "Watch the given parent PID and exit when it disappears (sidecar mode)")
	flag.Parse()

	if *parentPID > 0 {
		startParentWatchdog(*parentPID)
	}

	server := ui.NewServer()
	if err := server.Run(*port); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// startParentWatchdog exits the process when the parent (the desktop client)
// is gone, so a crashed or force-killed UI never leaves an orphaned backend.
func startParentWatchdog(parentPID int) {
	if runtime.GOOS != "windows" {
		return
	}
	go func() {
		want := []byte(fmt.Sprintf(`"%d"`, parentPID))
		for {
			time.Sleep(5 * time.Second)
			out, err := execTasklist(parentPID)
			if err != nil || !bytes.Contains(out, want) {
				os.Exit(0)
			}
		}
	}()
}
