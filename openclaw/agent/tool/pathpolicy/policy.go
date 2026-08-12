package pathpolicy

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/futugyou/openclaw/core"
)

const maxSymlinkResolutionDepth = 64

type ToolPathPolicy struct{}

// 检查路径是否允许读取
func IsReadAllowed(config core.ToolingConfig, path string) bool {
	return isToolPathPolicyAllowed(config.AllowedReadRoots, path)
}

// 检查路径是否允许写入
func IsWriteAllowed(config core.ToolingConfig, path string) bool {
	return isToolPathPolicyAllowed(config.AllowedWriteRoots, path)
}

func isToolPathPolicyAllowed(roots []string, path string) bool {
	if len(roots) == 0 {
		return false
	}

	if len(roots) == 1 && roots[0] == "*" {
		return true
	}

	fullPath := ResolveRealPath(path)
	for _, root := range roots {
		if root == "*" {
			return true
		}

		fullRoot := ResolveRealPath(root)
		if isUnderRoot(fullPath, fullRoot) {
			return true
		}
	}

	return false
}

// ResolveRealPath 解析绝对真实路径（支持解析存在/不存在路径中的软链接）
func ResolveRealPath(path string) string {
	visited := make(map[string]struct{})
	return resolveRealPathInternal(path, visited, 0)
}

func resolveRealPathInternal(path string, visited map[string]struct{}, depth int) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	absPath = filepath.Clean(absPath)

	// 规范化 visited 集合里的 Key，应对 Windows 下的大小写差异
	key := normalizePathKey(absPath)
	if depth >= maxSymlinkResolutionDepth {
		return absPath
	}
	if _, exists := visited[key]; exists {
		return absPath
	}
	visited[key] = struct{}{}

	// 获取路径根节点（如 Windows 上的 "C:\" 或 Linux 上的 "/"）
	volName := filepath.VolumeName(absPath)
	var root string
	if volName != "" {
		root = volName + string(filepath.Separator)
	} else {
		root = string(filepath.Separator)
	}

	// 提取根路径之后的部分并切割为 Segments
	remaining := absPath[len(root):]
	if remaining == "" {
		return absPath
	}

	segments := strings.Split(remaining, string(filepath.Separator))
	current := root

	for _, segment := range segments {
		if segment == "" {
			continue
		}

		current = filepath.Join(current, segment)

		// 尝试解析软链接
		resolvedTarget, err := tryResolveLinkTarget(current)
		if err == nil && resolvedTarget != "" {
			// 递归解析真实路径
			current = resolveRealPathInternal(resolvedTarget, visited, depth+1)
		}
	}

	finalAbs, err := filepath.Abs(current)
	if err != nil {
		return current
	}
	return filepath.Clean(finalAbs)
}

// tryResolveLinkTarget 尝试获取软链接指向的最终目标绝对路径
func tryResolveLinkTarget(path string) (string, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return "", err
	}

	// 如果不是软链接，直接返回空
	if fi.Mode()&os.ModeSymlink == 0 {
		return "", nil
	}

	// 读取软链接目标
	target, err := os.Readlink(path)
	if err != nil {
		return "", err
	}

	// 如果是相对软链接，转换为基于当前目录的绝对路径
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}

	// 递归解析最终目标（等价于 C# 中 returnFinalTarget: true）
	finalTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return target, nil
		}
		return "", err
	}

	return finalTarget, nil
}

func isUnderRoot(fullPath, fullRoot string) bool {
	isWindows := runtime.GOOS == "windows"

	equal := func(a, b string) bool {
		if isWindows {
			return strings.EqualFold(a, b)
		}
		return a == b
	}

	startsWith := func(s, prefix string) bool {
		if isWindows {
			if len(s) < len(prefix) {
				return false
			}
			return strings.EqualFold(s[:len(prefix)], prefix)
		}
		return strings.HasPrefix(s, prefix)
	}

	fullPath = filepath.Clean(fullPath)
	fullRoot = filepath.Clean(fullRoot)

	if equal(fullPath, fullRoot) {
		return true
	}

	sep := string(filepath.Separator)
	if !strings.HasSuffix(fullRoot, sep) {
		fullRoot += sep
	}

	return startsWith(fullPath, fullRoot)
}

func normalizePathKey(path string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}
