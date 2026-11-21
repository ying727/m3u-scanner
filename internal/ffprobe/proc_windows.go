//go:build windows

package ffprobe

import (
	"context"
	"os/exec"
	"syscall"
)

// CREATE_NO_WINDOW: 后端以 GUI 子系统运行(桌面客户端内嵌模式)时自身没有
// 控制台,不给子进程加这个标志的话,Windows 会为每个 ffprobe 进程新开一个
// 终端窗口——并发扫描时会瞬间弹出一排。
const createNoWindowFlag = 0x08000000

// newCmd 构建 ffprobe 子进程命令,Windows 下附带隐藏控制台属性。
func newCmd(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindowFlag}
	return cmd
}

func newCmdSimple(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindowFlag}
	return cmd
}
