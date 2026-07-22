//go:build windows

package core

import (
	"fmt"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
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

func getDiskFreeSpace(path string) (uint64, error) {
	var freeBytesAvailable uint64
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	err = windows.GetDiskFreeSpaceEx(pathPtr, &freeBytesAvailable, nil, nil)
	if err != nil {
		return 0, err
	}
	return freeBytesAvailable, nil
}
