//go:build !windows

package core

import (
	"os/exec"
	"syscall"
)

// 在 Linux / macOS 下，通过杀死进程组（负的 PID）来消灭整个进程树
func killProcessTree(cmd *exec.Cmd) {
	if cmd.Process != nil {
		// -PID 代表向整个进程组发送信号
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// 在 Linux / macOS 下，启动前需要配置 Setpgid
func configureSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
