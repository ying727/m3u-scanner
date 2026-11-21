package main

import (
	"fmt"
	"os/exec"
)

func execTasklist(pid int) ([]byte, error) {
	return exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH", "/FO", "CSV").Output()
}
