package storage

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
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
	safeModelFileNameRegex = regexp.MustCompile(`(?i)^[A-Za-z0-9][A-Za-z0-9._-]{0,127}\.(gguf|safetensors|onnx|bin)$`)
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

func readerWithContext(ctx context.Context, r io.Reader) io.Reader {
	return readerFunc(func(p []byte) (int, error) {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		return r.Read(p)
	})
}

type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) { return f(p) }

func copyToWithContext(ctx context.Context, destPath string, sourceStream io.Reader, bufferSize int) (int64, error) {
	dest, err := os.OpenFile(destPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return 0, err
	}

	defer dest.Close()

	buf := make([]byte, bufferSize)

	cancelableSource := io.NopCloser(struct{ io.Reader }{
		Reader: readerWithContext(ctx, sourceStream),
	})

	bytesWritten, err := io.CopyBuffer(dest, cancelableSource, buf)
	if err != nil {

		if ctx.Err() != nil {

			return bytesWritten, nil
		}
		return bytesWritten, err
	}

	return bytesWritten, nil
}

func openFileForRead(tempPath string) (*bufio.Reader, *os.File, error) {
	file, err := os.Open(tempPath)
	if err != nil {
		return nil, nil, err
	}

	bufferSize := 64 * 1024
	bufferedReader := bufio.NewReaderSize(file, bufferSize)

	return bufferedReader, file, nil
}

func MoveFile(src, dest string) error {
	err := os.Rename(src, dest)
	if err == nil {
		return nil
	}
	return moveCrossDevice(src, dest)
}

func moveCrossDevice(src, dest string) error {
	inputFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer inputFile.Close()

	outputFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	_, err = io.Copy(outputFile, inputFile)
	if err != nil {
		return err
	}

	inputFile.Close()
	outputFile.Close()

	return os.Remove(src)
}

func TryParseVaultReference(value string) (string, bool) {
	prefix := "vault://"

	if len(value) == 0 || !strings.HasPrefix(value, prefix) {
		return "", false
	}

	name := strings.TrimSpace(value[:min(len(prefix), len(value))])
	return name, len(name) > 0
}
