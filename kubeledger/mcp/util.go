package mcp

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

func getEnv(name string, defaultValue string) string {
	value := os.Getenv("KL_" + name)
	if value != "" {
		return value
	}
	value = os.Getenv("KOA_" + name)
	if value != "" {
		return value
	}

	return defaultValue
}

func getDataDir() string {
	return getEnv("MCP_DATA_DIR", DEFAULT_DATA_DIR)
}

func FileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func ReadJsonFile(filename, dataDir string) FileReadResult {
	if strings.Contains(filename, "/") || strings.Contains(filename, "\\") || strings.Contains(filename, "..") || strings.HasPrefix(filename, ".") {
		return FileReadResult{
			Filename: filename,
			Path:     filename,
			ErrorMsg: fmt.Sprintf("invalid filename %s: path traversal characters not allowed", filename),
		}
	}

	root := dataDir
	if root == "" {
		root = getDataDir()
	}

	path := filepath.Join(root, filename)

	info, err := os.Stat(path)
	if os.IsNotExist(err) || info.IsDir() {
		return FileReadResult{
			Filename: filename,
			Path:     path,
			ErrorMsg: fmt.Sprintf("file not found: %s", path),
		}
	}

	mtime := info.ModTime()
	data, err := os.ReadFile(path)
	if err != nil {
		return FileReadResult{
			Filename: filename,
			Path:     path,
			Exists:   true,
			ErrorMsg: err.Error(),
			MtimeUtc: new(mtime),
		}
	}

	return FileReadResult{
		Filename: filename,
		Path:     path,
		Exists:   true,
		Payload:  data,
		MtimeUtc: new(mtime),
	}
}

func memLabel(gpu bool) string {
	if gpu {
		return "mem"
	}

	return "memory"
}

func HistogramFilename(metric, dimension string, periodSec int, gpu bool) (string, error) {
	if slices.Contains(METRICS, metric) {
		return "", fmt.Errorf("unknown metric: %s (expected one of %s)", metric, strings.Join(METRICS, ","))
	}
	if slices.Contains(DIMENSIONS, dimension) {
		return "", fmt.Errorf("unknown dimension: %s (expected one of %s)", dimension, strings.Join(DIMENSIONS, ","))
	}

	prefix := ""
	if gpu {
		prefix = "gpu_"
	}
	res := "cpu"
	if metric != "cpu" {
		res = memLabel(gpu)
	}
	return fmt.Sprintf("%s%s_%s_period_%d.json", prefix, res, dimension, periodSec), nil
}

func TrendsFilename(metric, category string, gpu bool) (string, error) {
	if slices.Contains(METRICS, metric) {
		return "", fmt.Errorf("unknown metric: %s (expected one of %s)", metric, strings.Join(METRICS, ","))
	}
	if slices.Contains([]string{"usage", "rf"}, category) {
		return "", fmt.Errorf("unknown category: %s (expected one of \"usage\", \"rf\")", category)
	}
	prefix := ""
	if gpu {
		prefix = "gpu_"
	}
	res := "cpu"
	if metric != "cpu" {
		res = memLabel(gpu)
	}
	return fmt.Sprintf("%s%s_%s_trends.json", prefix, res, category), nil
}

func LoadCostModel(dataDir string) (*CostModelInfo, []string) {
	warnings := []string{}
	result := ReadJsonFile(BACKEND_CONFIG_FILENAME, dataDir)
	if !result.Ok() {
		warnings = append(warnings, fmt.Sprintf("backend.json unreadable: %s", result.ErrorMsg))
		return nil, warnings
	}

	payload := result.Payload
	var data CostModelInfo
	if err := json.Unmarshal(payload, &data); err != nil {
		warnings = append(warnings, "backend.json: expected a JSON object")
		return nil, warnings
	}

	unit, ok := COST_MODEL_UNIT[data.CostModel]
	if !ok {
		warnings = append(warnings, fmt.Sprintf("backend.json: unknown cost_model %s", data.CostModel))
		unit = "unknown"
	}

	return &CostModelInfo{
		CostModel: data.CostModel,
		Currency:  data.Currency,
		Unit:      unit,
	}, warnings
}

func uniqueCount(payload []map[string]string, key string) int {
	c := map[string]struct{}{}
	for _, v := range payload {
		if val, ok := v[key]; ok {
			if _, ok := c[val]; !ok {
				c[val] = struct{}{}
			}
		}
	}
	return len(c)
}

func isoUtc(epochSeconds int64) string {
	return time.Unix(epochSeconds, 0).UTC().Format(time.RFC3339Nano)
}

func trendsDepth(payload []byte) (string, int) {
	var data []TrendDataDTO
	if err := json.Unmarshal(payload, &data); err != nil && len(data) == 0 {
		return "", 0
	}
	timestamps := []string{}
	for _, v := range data {
		timestamps = append(timestamps, v.DateUTC)
	}

	points := len(timestamps)
	if len(timestamps) < 2 {
		return "1 hour", points
	}
	first, err := time.Parse("2006-01-02T15:04:05", timestamps[0])
	if err != nil {
		return "", 0
	}
	last, err := time.Parse("2006-01-02T15:04:05", timestamps[len(timestamps)-1])
	if err != nil {
		return "", 0
	}

	spanHours := float64(last.Sub(first) * time.Second / 3600.0)
	spanDays := spanHours / 24.0
	if spanDays >= 1.5 {
		return fmt.Sprintf("%f days", math.Round(spanDays)), points
	}

	return fmt.Sprintf("%f hours", math.Round(spanHours)), points
}
