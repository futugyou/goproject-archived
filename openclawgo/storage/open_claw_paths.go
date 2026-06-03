package storage

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// StorageRootSource
type StorageRootSource int

const (
	SourceDefault StorageRootSource = iota
	SourceEnvironmentVariable
)

const (
	EnvironmentVariableName       = "OPENCLAWNET_STORAGE_ROOT"
	LegacyEnvironmentVariableName = "OPENCLAW_STORAGE_DIR"
)

var (
	safeSegmentRegex       = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	safeUserFolderRegex    = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	safeModelFileNameRegex = regexp.MustCompile(`(?i)^[a-z0-9][a-z0-9._-]{0,127}\.(gguf|safetensors|onnx|bin)$`)
)

var reservedNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

var (
	ErrInvalidName  = errors.New("unsafe path: invalid name or path traversal detected")
	ErrReservedName = errors.New("unsafe path: uses a reserved Windows device name")
)

func DefaultRoot() string {
	if runtime.GOOS == "windows" {
		return `C:\openclawnet`
	}
	u, err := user.Current()
	if err != nil {
		return filepath.Join(".", "openclawnet")
	}
	return filepath.Join(u.HomeDir, "openclawnet")
}

func Normalize(raw string) string {
	trimmed := strings.TrimSpace(raw)
	return filepath.Clean(trimmed)
}

func ResolveRoot() (string, StorageRootSource) {
	if envValue := os.Getenv(EnvironmentVariableName); strings.TrimSpace(envValue) != "" {
		return Normalize(envValue), SourceEnvironmentVariable
	}

	return Normalize(DefaultRoot()), SourceDefault
}

func validateName(name string, paramName string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%s must be non-empty", paramName)
	}

	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return fmt.Errorf("%w: %s contains separators or traversal", ErrInvalidName, paramName)
	}

	if name[0] == '.' || name[0] == ' ' || name[len(name)-1] == '.' || name[len(name)-1] == ' ' {
		return fmt.Errorf("%w: %s has leading/trailing dot or space", ErrInvalidName, paramName)
	}

	if !safeSegmentRegex.MatchString(name) {
		return fmt.Errorf("%w: %s violates regex policy", ErrInvalidName, paramName)
	}

	stem := strings.Split(name, ".")[0]
	if reservedNames[strings.ToUpper(stem)] {
		return fmt.Errorf("%w: %s uses reserved device name", ErrReservedName, paramName)
	}

	return nil
}

func applyWindowsRestrictiveDacl(path string) {
	username := os.Getenv("USERNAME")
	if username == "" {
		return
	}

	_ = exec.Command("icacls", path, "/inheritance:r").Run()
	_ = exec.Command("icacls", path, "/grant:r", username+":(OI)(CI)F").Run()
}

func EnsureDirectoryWithRestrictiveAcl(path string) error {
	err := os.MkdirAll(path, 0700)
	if err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		applyWindowsRestrictiveDacl(path)
	}
	return nil
}

func ResolveAgentRoot(agentName string) (string, error) {
	if err := validateName(agentName, "agentName"); err != nil {
		return "", err
	}
	root, _ := ResolveRoot()
	agentRoot := filepath.Join(root, "agents", agentName)
	if err := EnsureDirectoryWithRestrictiveAcl(agentRoot); err != nil {
		return "", err
	}
	return agentRoot, nil
}

func ResolveModelsRoot() (string, error) {
	root, _ := ResolveRoot()
	modelsRoot := filepath.Join(root, "models")
	if err := EnsureDirectoryWithRestrictiveAcl(modelsRoot); err != nil {
		return "", err
	}
	return modelsRoot, nil
}

func ResolveSafeModelPath(fileName string) (string, error) {
	if strings.TrimSpace(fileName) == "" {
		return "", errors.New("model file name must be non-empty")
	}

	if strings.ContainsAny(fileName, `/\`) || strings.Contains(fileName, "..") {
		return "", fmt.Errorf("%w: model file name contains separators", ErrInvalidName)
	}

	if !safeModelFileNameRegex.MatchString(fileName) {
		return "", fmt.Errorf("%w: model file name violates regex policy", ErrInvalidName)
	}

	stem, _, _ := strings.Cut(fileName, ".")
	if reservedNames[strings.ToUpper(stem)] {
		return "", ErrReservedName
	}

	modelsRoot, err := ResolveModelsRoot()
	if err != nil {
		return "", err
	}

	combined, err := filepath.Abs(filepath.Join(modelsRoot, fileName))
	if err != nil {
		return "", err
	}

	lowerCombined := strings.ToLower(combined)
	lowerRoot := strings.ToLower(modelsRoot) + string(filepath.Separator)
	if !strings.HasPrefix(lowerCombined, lowerRoot) && runtime.GOOS == "windows" {
		return "", errors.New("unsafe path: escaped models root")
	} else if !strings.HasPrefix(combined, modelsRoot+string(filepath.Separator)) && runtime.GOOS != "windows" {
		return "", errors.New("unsafe path: escaped models root")
	}

	return combined, nil
}
