package util

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"time"
)

func ReadLimited(r io.Reader, maxBytes int64) (string, error) {
	// 1. 创建一个只读取 maxBytes 的 Reader
	limitedReader := io.LimitReader(r, maxBytes)

	var buf bytes.Buffer
	// 读取前 maxBytes 字节
	n, err := buf.ReadFrom(limitedReader)
	if err != nil {
		return "", err
	}

	result := buf.String()

	// 2. 如果读取的字节数达到了上限，说明可能还有剩余输出
	if n == maxBytes { // 这里可能刚好相等，多的一次io.copy可能会堵塞
		// 继续读取并丢弃剩余所有内容，防止管道堵塞导致子进程死锁
		written, _ := io.Copy(io.Discard, r)
		if written > 0 {
			result += "\n... (output truncated)"
		}
	}

	return result, nil
}

type ProcessResult struct {
	Error       string
	ExitCode    int
	StdoutText  string
	StdoutError error
	StderrText  string
	StderrError error
}

func RunProcess(ctx context.Context, exe string, args []string, cwd string, timeoutSec, stdoutMaxbytes, stderrMaxbytes int64) ProcessResult {
	result := ProcessResult{}
	if stdoutMaxbytes <= 0 {
		stdoutMaxbytes = 8192
	}
	if stdoutMaxbytes <= 0 {
		stdoutMaxbytes = 8192
	}
	// 创建带有超时的 Context
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	// 创建 Cmd 命令
	cmd := exec.CommandContext(timeoutCtx, exe, args...)

	if cwd != "" {
		workDir := filepath.Dir(cwd)
		if workDir == "" {
			workDir = "."
		}
		cmd.Dir = workDir
	}

	// 获取 stdout 和 stderr 的管道
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		result.Error = fmt.Sprintf("Error: Failed to start execution process (%T).", err)
		return result
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		result.Error = fmt.Sprintf("Error: Failed to start execution process (%T).", err)
		return result
	}

	// 启动进程
	if err := cmd.Start(); err != nil {
		result.Error = fmt.Sprintf("Error: Failed to start execution process (%T).", err)
		return result
	}

	// 并发读取 stdout 和 stderr
	type readResult struct {
		text string
		err  error
	}

	stdoutChan := make(chan readResult, 1)
	stderrChan := make(chan readResult, 1)

	go func() {
		out, truncated, err := ReadLimitedClean(stdoutPipe, stdoutMaxbytes)
		if truncated {
			out += "\n... (output truncated)"
		}
		stdoutChan <- readResult{out, err}
	}()

	go func() {
		errOut, truncated, err := ReadLimitedClean(stderrPipe, stderrMaxbytes)
		if truncated {
			errOut += "\n... (output truncated)"
		}
		stderrChan <- readResult{errOut, err}
	}()

	// 等待读取完成
	stdoutRes := <-stdoutChan
	stderrRes := <-stderrChan

	// 等待进程结束
	err = cmd.Wait()

	// 检查是否因为超时导致 context 被取消
	if timeoutCtx.Err() == context.DeadlineExceeded {
		result.Error = "Error: Execution timed out."
		return result
	}

	result.ExitCode = cmd.ProcessState.ExitCode()
	result.StdoutError = stdoutRes.err
	result.StdoutText = stdoutRes.text
	result.StderrError = stderrRes.err
	result.StderrText = stderrRes.text

	return result
}

func ReadLimitedClean(r io.Reader, maxBytes int64) (string, bool, error) {
	// 多读 1 个字节，用于精确检测是否超出 maxBytes，避免误判
	lr := io.LimitReader(r, maxBytes+1)
	b, err := io.ReadAll(lr)
	if err != nil {
		return "", false, err
	}

	truncated := int64(len(b)) > maxBytes
	if truncated {
		b = b[:maxBytes]
		// 丢弃后续剩余流，防止堵塞子进程/管道
		io.Copy(io.Discard, r)
	}

	return string(b), truncated, nil
}
