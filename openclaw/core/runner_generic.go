//go:build !windows

package core

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
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

func getDiskFreeSpace(path string) (uint64, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, err
	}
	// 可用字节数 = 可用块数 * 块大小
	return stat.Bavail * uint64(stat.Bsize), nil
}
