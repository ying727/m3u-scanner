package main

import (
	"flag"
	"fmt"
	"os"

	"m3u-scanner/internal/ui"
)

func main() {
	port := flag.Int("port", 8080, "Port to run the web server on")
	flag.Parse()

	server := ui.NewServer()
	if err := server.Run(*port); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
