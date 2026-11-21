//go:build !windows

package ffprobe

import (
	"context"
	"os/exec"
)

// newCmd 构建 ffprobe 子进程命令。非 Windows 平台无控制台窗口问题。
func newCmd(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

func newCmdSimple(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}
