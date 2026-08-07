package agent

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"

	"github.com/futugyou/openclaw/core"
	"github.com/google/uuid"
)

const unixSocketPathBudget = 96

func CreateBridgeTransport(
	config core.BridgeTransportConfig,
	pluginID string,
	logger *slog.Logger,
	runtimeRoot string,
	metrics *core.RuntimeMetrics,
) (IBridgeTransport, *core.BridgeTransportRuntimeConfig, error) {

	mode := normalizeMode(config.Mode)
	var socketOpts *SocketTransportOptions
	var err error

	if mode != "stdio" {
		socketOpts, err = resolveSocketOptions(config.SocketPath, pluginID, runtimeRoot)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve socket options failed: %w", err)
		}
	}

	switch mode {
	case "stdio":
		transport := NewStdioBridgeTransport(logger)
		runtimeCfg := &core.BridgeTransportRuntimeConfig{Mode: mode}
		return transport, runtimeCfg, nil

	case "socket":
		transport := NewSocketBridgeTransport(
			socketOpts.SocketPath,
			socketOpts.SocketDirectory,
			socketOpts.OwnsSocketDirectory,
			socketOpts.AuthToken,
			logger,
			metrics,
		)
		return transport, createRuntimeConfig(mode, socketOpts), nil

	case "hybrid":
		transport := NewHybridBridgeTransport(
			socketOpts.SocketPath,
			socketOpts.SocketDirectory,
			socketOpts.OwnsSocketDirectory,
			socketOpts.AuthToken,
			logger,
			metrics,
		)
		return transport, createRuntimeConfig(mode, socketOpts), nil

	default:
		return nil, nil, fmt.Errorf("unsupported plugin bridge transport mode '%s'. Supported modes: stdio, socket, hybrid", config.Mode)
	}
}

func createRuntimeConfig(mode string, socketOptions *SocketTransportOptions) *core.BridgeTransportRuntimeConfig {
	return &core.BridgeTransportRuntimeConfig{
		Mode:            mode,
		SocketPath:      socketOptions.SocketPath,
		SocketDirectory: socketOptions.SocketDirectory,
		SocketAuthToken: socketOptions.AuthToken,
		SecurityMode:    "hardened_local_ipc",
	}
}

func normalizeMode(mode string) string {
	trimmed := strings.TrimSpace(mode)
	if trimmed == "" {
		return "stdio"
	}
	return strings.ToLower(trimmed)
}

func resolveSocketOptions(configuredPath, pluginID, runtimeRoot string) (*SocketTransportOptions, error) {
	if runtime.GOOS == "windows" {
		var pipePath string
		if strings.TrimSpace(configuredPath) != "" {
			pipePath = normalizePipePath(configuredPath)
		} else {
			pipePath = fmt.Sprintf(`\\.\pipe\openclaw-%s-%s`, sanitize(pluginID), strings.ReplaceAll(uuid.New().String(), "-", ""))
		}
		return &SocketTransportOptions{
			SocketPath:          pipePath,
			SocketDirectory:     "",
			OwnsSocketDirectory: false,
			AuthToken:           createAuthToken(),
		}, nil
	}

	if strings.TrimSpace(configuredPath) != "" {
		absPath, err := filepath.Abs(configuredPath)
		if err != nil {
			return nil, err
		}
		return &SocketTransportOptions{
			SocketPath:          absPath,
			SocketDirectory:     filepath.Dir(absPath),
			OwnsSocketDirectory: false,
			AuthToken:           createAuthToken(),
		}, nil
	}

	socketDirectory := createUnixSocketDirectory(pluginID, runtimeRoot)
	if err := os.MkdirAll(socketDirectory, 0700); err != nil {
		return nil, err
	}

	return &SocketTransportOptions{
		SocketPath:          filepath.Join(socketDirectory, "s"),
		SocketDirectory:     socketDirectory,
		OwnsSocketDirectory: true,
		AuthToken:           createAuthToken(),
	}, nil
}

func createUnixSocketDirectory(pluginID, runtimeRoot string) string {
	rawInput := fmt.Sprintf("%s:%s", pluginID, strings.ReplaceAll(uuid.New().String(), "-", ""))
	hashBytes := sha256.Sum256([]byte(rawInput))
	hash := strings.ToLower(hex.EncodeToString(hashBytes[:]))[:16]

	parent := resolveUnixSocketParent(runtimeRoot)
	socketDirectory := filepath.Join(parent, hash)

	if len(socketDirectory) > unixSocketPathBudget {
		shortenedParent := resolveShortUnixSocketParent()
		socketDirectory = filepath.Join(shortenedParent, hash)
	}

	return socketDirectory
}

func resolveUnixSocketParent(runtimeRoot string) string {
	if strings.TrimSpace(runtimeRoot) != "" {
		absRoot, err := filepath.Abs(runtimeRoot)
		if err == nil {
			return filepath.Join(absRoot, "pb")
		}
	}

	return resolveShortUnixSocketParent()
}

func resolveShortUnixSocketParent() string {
	tempRoot := "/tmp"
	userComponent := "user"

	currentUser, err := user.Current()
	if err == nil && currentUser.Username != "" {
		sanitized := sanitize(currentUser.Username)
		if sanitized != "" {
			userComponent = sanitized
		}
	}

	return filepath.Join(tempRoot, fmt.Sprintf(".openclaw-%s", userComponent), "pb")
}

func normalizePipePath(configuredPath string) string {
	prefix := `\\.\pipe\`
	if strings.HasPrefix(strings.ToLower(configuredPath), strings.ToLower(prefix)) {
		return configuredPath
	}
	trimmed := strings.Trim(configuredPath, `\/`)
	return prefix + trimmed
}

func createAuthToken() string {
	uuid1 := strings.ReplaceAll(uuid.New().String(), "-", "")
	uuid2 := strings.ReplaceAll(uuid.New().String(), "-", "")
	return strings.ToUpper(uuid1 + uuid2)
}

func sanitize(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))

	for _, ch := range value {
		if unicode.IsLetter(ch) || unicode.IsNumber(ch) {
			builder.WriteRune(unicode.ToLower(ch))
		} else {
			builder.WriteByte('-')
		}
	}

	return strings.Trim(builder.String(), "-")
}

func FindNodeExecutable() string {
	candidates := []string{"node"}
	if runtime.GOOS == "windows" {
		candidates = []string{"node.exe"}
	}

	// 1. 查找 PATH 中的候选可执行文件
	for _, candidate := range candidates {
		if path := FindExecutable(candidate); path != "" {
			return path
		}
	}

	// 2. 查找常见的固定路径/通配符路径
	var commonPaths []string
	homeDir, err := os.UserHomeDir()

	if runtime.GOOS == "windows" {
		commonPaths = []string{
			`C:\Program Files\nodejs\node.exe`,
			`C:\Program Files (x86)\nodejs\node.exe`,
		}
		if err == nil {
			commonPaths = append(commonPaths, filepath.Join(homeDir, `AppData\Roaming\nvm\v*\node.exe`))
		}
	} else {
		commonPaths = []string{
			"/usr/local/bin/node",
			"/usr/bin/node",
			"/opt/homebrew/bin/node",
		}
		if err == nil {
			commonPaths = append(commonPaths, filepath.Join(homeDir, ".nvm/versions/node/v*/bin/node"))
		}
	}

	for _, path := range commonPaths {
		if strings.Contains(path, "*") {
			// 处理带有通配符的路径（例如 nvm 版本目录）
			matches, err := filepath.Glob(path)
			if err == nil && len(matches) > 0 {
				for _, match := range matches {
					if info, err := os.Stat(match); err == nil && !info.IsDir() {
						return match
					}
				}
			}
		} else {
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				return path
			}
		}
	}

	return ""
}

func FindExecutable(name string) string {
	if path, err := exec.LookPath(name); err == nil {
		if absPath, err := filepath.Abs(path); err == nil {
			return absPath
		}
		return path
	}

	cmdName := "which"
	if runtime.GOOS == "windows" {
		cmdName = "where"
	}

	cmd := exec.Command(cmdName, name)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err == nil {
		output := strings.TrimSpace(stdout.String())
		if output != "" {
			// 按换行符分割，获取第一行结果
			lines := strings.FieldsFunc(output, func(r rune) bool {
				return r == '\n' || r == '\r'
			})
			if len(lines) > 0 {
				return strings.TrimSpace(lines[0])
			}
		}
	}

	return ""
}
