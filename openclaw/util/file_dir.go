package util

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

func TryResolveLinkTarget(path string) (string, bool) {
	finalPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return "", false
		}

		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrInvalid) {
			return "", false
		}

		return "", false
	}

	// EvalSymlinks 如果传入普通路径，会直接返回原路径。
	// 我们检查原路径是否真的是一个符号链接。
	if IsLstatSame(path, finalPath) {
		return "", false
	}

	return finalPath, true
}

// 辅助函数：判断原路径是否本身就是最终路径（排除非链接的情况）
func IsLstatSame(original, final string) bool {
	// 获取原路径的 Lstat（不追踪链接本身）
	origFi, err1 := os.Lstat(original)
	// 获取最终路径的 Stat
	finalFi, err2 := os.Stat(final)

	if err1 != nil || err2 != nil {
		return true
	}

	// 如果原路径的模式不是 Symlink，说明它本来就不是链接
	if origFi.Mode()&os.ModeSymlink == 0 {
		return true
	}

	// 比较它们是否指向同一个文件系统实体
	return os.SameFile(origFi, finalFi)
}

// isUnresolvedLink 判断路径是否是一个无法解析的死链接
func IsUnresolvedLink(path string) bool {
	// 1. 获取路径自身的元数据（Lstat 不会追踪符号链接目标）
	fi, err := os.Lstat(path)
	if err != nil {
		// 如果路径本身就不存在或无法读取，它就谈不上是一个“未解析的链接”，返回 false
		return false
	}

	// 2. 检查它是否是符号链接
	if fi.Mode()&os.ModeSymlink == 0 {
		return false
	}

	// 3. 如果 tryResolveLinkTarget 返回 false，说明链接断开或目标不可达
	_, ok := TryResolveLinkTarget(path)
	return !ok
}

// 判断两个路径在当前操作系统下是否相等
func PathEqual(path1, path2 string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(path1, path2)
	}
	return path1 == path2
}

// 判断 path 是否以 prefix 为前缀（考虑操作系统大小写）
func PathHasPrefix(path, prefix string) bool {
	if runtime.GOOS == "windows" {
		return strings.HasPrefix(strings.ToLower(path), strings.ToLower(prefix))
	}
	return strings.HasPrefix(path, prefix)
}

func GetFileNameWithoutExtension(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

func PathGetFullPath(path string) string {
	p, _ := filepath.Abs(path)
	return p
}

// LoadAllFile 遍历目录下所有的 .json 文件并反序列化为对象切片
func LoadAllFile[T any](ctx context.Context, directory string) ([]T, error) {
	files, err := os.ReadDir(directory)
	if err != nil {
		return []T{}, nil // C# 中 catch 块返回空数组
	}

	var results []T
	for _, file := range files {
		// 检查 Context 是否已取消
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		path := filepath.Join(directory, file.Name())
		item, err := LoadOneFile[T](ctx, path)
		if err == nil && item != nil {
			results = append(results, *item)
		}
	}

	return results, nil
}

func AppendAllText(path, text string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(text + "\n")
	return err
}

// LoadOneFile 反序列化单个文件
func LoadOneFile[T any](ctx context.Context, path string) (*T, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil // 文件不存在，明确返回 nil 指针
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file failed: %w", err)
	}

	var item T
	if err := json.Unmarshal(data, &item); err != nil {
		return nil, fmt.Errorf("unmarshal json failed: %w", err)
	}

	return &item, nil
}

// saveOneFile 安全写入文件（先写临时文件再重命名，以保证原子性）
func SaveOneFile(ctx context.Context, path string, item any) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	data, err := json.Marshal(item)
	if err != nil {
		return err
	}

	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}

	// 重命名（在 Go 中跨平台覆盖行为略有区别，os.Rename 在 Linux/Unix 下支持覆盖，Windows 下建议先移除）
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}

	return os.Rename(tempPath, path)
}

func SaveFile(ctx context.Context, path string, content string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, []byte(content), 0644); err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}

	return os.Rename(tempPath, path)
}

// 定义一个结构体来存放分组数据
type FileGroup struct {
	BaseName     string
	Files        []string
	MaxWriteTime time.Time
}

func GetGroupByFilename(dirPath string) ([]FileGroup, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	groupMap := make(map[string]*FileGroup)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fullPath := filepath.Join(dirPath, entry.Name())

		info, err := entry.Info()
		if err != nil {
			continue
		}
		writeTime := info.ModTime().UTC()

		ext := filepath.Ext(entry.Name())
		baseName := strings.TrimSuffix(entry.Name(), ext)

		key := strings.ToLower(baseName)

		if group, exists := groupMap[key]; exists {
			group.Files = append(group.Files, fullPath)
			if writeTime.After(group.MaxWriteTime) {
				group.MaxWriteTime = writeTime
			}
		} else {
			groupMap[key] = &FileGroup{
				BaseName:     baseName,
				Files:        []string{fullPath},
				MaxWriteTime: writeTime,
			}
		}
	}

	var groups []FileGroup
	for _, group := range groupMap {
		groups = append(groups, *group)
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].MaxWriteTime.After(groups[j].MaxWriteTime)
	})

	return groups, nil
}

// 计算指定目录的总大小（字节）
func GetDirectorySize(path string) int64 {
	if path == "" {
		return 0
	}

	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return 0
	}

	var totalSize int64

	_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			return nil
		}

		fileInfo, err := d.Info()
		if err != nil {
			return nil
		}

		totalSize += fileInfo.Size()
		return nil
	})

	return totalSize
}

func DeleteOneFile(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func DeleteDirectory(path string) {
	_ = os.RemoveAll(path)
}

func FileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func DirectoryExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

// 判断路径是否是符号链接/重定向点
func IsReparsePoint(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}

	return info.Mode()&os.ModeSymlink != 0, nil
}

func FindDirectoriesCantainsFileName(candidatePath string, filename string) ([]string, error) {
	uniquePaths := make(map[string]bool)

	err := filepath.WalkDir(candidatePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}

		if !d.IsDir() && d.Name() == filename {
			dir := filepath.Dir(path)

			if strings.TrimSpace(dir) != "" {
				uniquePaths[dir] = true
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	matches := make([]string, 0, len(uniquePaths))
	for path := range uniquePaths {
		matches = append(matches, path)
	}

	return matches, nil
}

func EnumerateTopFiles(root string) []string {
	var files []string

	// os.ReadDir 只读取 root 目录下的第一层内容（非递归）
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		// 过滤掉子目录，只保留文件
		if !entry.IsDir() {
			fullPath := filepath.Join(root, entry.Name())
			files = append(files, fullPath)
		}
	}

	return files
}

// 递归获取目录下所有文件的绝对路径
func EnumerateAllFiles(root string) []string {
	var files []string

	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return err
		}

		if !d.IsDir() {
			absPath, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			files = append(files, absPath)
		}

		return nil
	})

	return files
}

func ReadAllLines(ctx context.Context, path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}

func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// os.Create 会默认清空并覆盖已存在的文件（对应 overwrite: true）
	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func CopyFileWithContext(ctx context.Context, sourcePath, destPath string) error {
	src, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer src.Close()

	dest, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dest.Close()

	// 使用 32KB 缓冲区，分块读取并检查 Context 是否已取消
	buf := make([]byte, 32*1024)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// 读取一块数据
			nr, readErr := src.Read(buf)
			if nr > 0 {
				// 写入目标文件
				nw, writeErr := dest.Write(buf[0:nr])
				if writeErr != nil {
					return writeErr
				}
				if nr != nw {
					return io.ErrShortWrite
				}
			}
			if readErr != nil {
				if readErr == io.EOF {
					return nil // 复制完成
				}
				return readErr
			}
		}
	}
}
