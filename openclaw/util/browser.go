package util

import (
	"os/exec"
	"runtime"
)

func TryOpenBrowser(url string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command(
			"rundll32",
			"url.dll,FileProtocolHandler",
			url,
		)

	case "darwin":
		cmd = exec.Command("open", url)

	default:
		cmd = exec.Command("xdg-open", url)
	}

	if err := cmd.Start(); err != nil {
		return
	}

	// Do not leave a child process unreaped.
	go func() {
		_ = cmd.Wait()
	}()
}
