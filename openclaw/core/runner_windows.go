//go:build windows

package core

import (
	"fmt"
	"os/exec"
	"syscall"
)

// 在 Windows 下，利用系统的 taskkill /T /F 斩草除根
func killProcessTree(cmd *exec.Cmd) {
	if cmd.Process != nil {
		killCmd := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", cmd.Process.Pid))
		_ = killCmd.Run()
	}
}

// 在 Windows 下，配置隐藏窗口（对应 C# 的 CreateNoWindow = true）
func configureSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
}
