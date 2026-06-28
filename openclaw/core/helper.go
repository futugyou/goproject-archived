package core

import (
	"crypto/rand"
	"encoding/json"
	"io/fs"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

func findDirectoriesCantainsFileName(candidatePath string, filename string) ([]string, error) {
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

func isBlank(s string) bool {
	return strings.TrimSpace(s) == ""
}

func isBlankP(s *string) bool {
	if s == nil {
		return true
	}
	return strings.TrimSpace(*s) == ""
}

func readStringArray(raw json.RawMessage) []string {
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr
	}
	return nil
}

func containsIgnoreCase(slice []string, val string) bool {
	target := strings.ToLower(val)
	for _, item := range slice {
		if strings.ToLower(item) == target {
			return true
		}
	}
	return false
}

func isLoopbackBind(bindAddress string) bool {
	// 1. 排除常见的通配符（绑定到所有接口，非 loopback）
	if bindAddress == "*" || bindAddress == "+" || bindAddress == "[::]" || bindAddress == ":" || bindAddress == "0.0.0.0" {
		return false
	}

	// 2. 尝试解析为 IP 地址并判断是否为环回地址
	if ip := net.ParseIP(bindAddress); ip != nil {
		return ip.IsLoopback()
	}

	// 3. 不区分大小写判断是否为 "localhost"
	return strings.EqualFold(bindAddress, "localhost")
}

func generateCode(min, max int64) string {
	rangeSize := big.NewInt(max - min)

	randomNum, err := rand.Int(rand.Reader, rangeSize)
	if err != nil {
		panic("critical system error: failed to generate secure random number: " + err.Error())
	}

	codeInt := randomNum.Int64() + min
	code := strconv.FormatInt(codeInt, 10)
	return code
}
