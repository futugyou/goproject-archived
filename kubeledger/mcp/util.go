package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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

func DiscoverDataset(dataDir string) DatasetInfo {
	root := dataDir
	if root == "" {
		root = getDataDir()
	}
	costModel, warnings := LoadCostModel(root)
	probed := map[string]FileReadResult{}

	probe := func(filename string) FileReadResult {
		if _, ok := probed[filename]; !ok {
			probed[filename] = ReadJsonFile(filename, dataDir)
		}

		return probed[filename]
	}

	metricsAvailable := []string{}
	for _, metric := range METRICS {
		for _, scale := range []string{SCALE_DAILY_14D, SCALE_MONTHLY_12M} {
			fname, _ := HistogramFilename(metric, "usage", SCALE_TO_PERIOD[scale], false)
			if len(fname) > 0 && probe(fname).Exists {
				if !slices.Contains(metricsAvailable, metric) {
					metricsAvailable = append(metricsAvailable, metric)
				}
				break
			}
		}
	}

	gpuAvailable := false
	for _, metric := range METRICS {
		for _, scale := range []string{SCALE_DAILY_14D, SCALE_MONTHLY_12M} {
			fname, err := HistogramFilename(metric, "usage", SCALE_TO_PERIOD[scale], true)
			if err != nil {
				continue
			}
			r := probe(fname)
			if r.Ok() && len(r.Payload) > 0 {
				gpuAvailable = true
				break
			}
		}
		if gpuAvailable {
			break
		}
	}
	dimensions := []string{}
	if len(metricsAvailable) > 0 {
	outer:
		for _, m := range metricsAvailable {
			for _, s := range []string{SCALE_DAILY_14D, SCALE_MONTHLY_12M} {
				filename, err := HistogramFilename(m, "usage", SCALE_TO_PERIOD[s], false)
				if err != nil {
					continue
				}
				if probe(filename).Exists {
					dimensions = append(dimensions, "usage")
					break outer
				}
			}
		}
	outer2:
		for _, m := range metricsAvailable {
			for _, s := range []string{SCALE_DAILY_14D, SCALE_MONTHLY_12M} {
				filename, err := HistogramFilename(m, "requests", SCALE_TO_PERIOD[s], false)
				if err != nil {
					continue
				}
				if probe(filename).Exists {
					dimensions = append(dimensions, "requests")
					break outer2
				}
			}
		}
	}

	scales := map[string]ScaleInfo{}
	for _, scale := range []string{SCALE_DAILY_14D, SCALE_MONTHLY_12M} {
		var sample *FileReadResult
		for _, metric := range append(metricsAvailable, METRICS...) {
			filename, err := HistogramFilename(metric, "usage", SCALE_TO_PERIOD[scale], false)
			if err != nil {
				continue
			}
			r := probe(filename)
			if r.Ok() && len(r.Payload) > 0 {
				sample = &r
				break
			}
		}
		if sample == nil {
			continue
		}

		var data []map[string]string
		if err := json.Unmarshal(sample.Payload, &data); err != nil {
			continue
		}
		points := uniqueCount(data, "date")
		scales[scale] = ScaleInfo{
			Granularity:        SCALE_GRANULARITY[scale],
			Depth:              SCALE_DEFAULT_DEPTH[scale],
			PointsPerNamespace: points,
		}
	}

	var trends *TrendsInfo
	for _, metric := range append(metricsAvailable, METRICS...) {
		fielname, err := TrendsFilename(metric, "usage", false)
		if err != nil {
			continue
		}

		r := probe(fielname)
		if r.Ok() && len(r.Payload) > 0 {
			depth, points := trendsDepth(r.Payload)
			if depth == "" {
				depth = "unknown"
			}
			trends = &TrendsInfo{
				Granularity:        "hourly",
				Depth:              depth,
				PointsPerNamespace: points,
			}
			break
		}
	}

	aggregatedEfficiency := slices.Contains(dimensions, "requests")
	hourlyRf := false

	for _, m := range append(metricsAvailable, METRICS...) {
		filename, err := TrendsFilename(m, "rf", false)
		if err != nil {
			continue
		}
		if probe(filename).Exists {
			hourlyRf = true
			break
		}
	}

	efficiency := EfficiencyInfo{
		Aggregated:      aggregatedEfficiency,
		HourlyTimeserie: hourlyRf,
	}

	mtimes := []time.Time{}
	for fname, r := range probed {
		if r.MtimeUtc != nil && fname != BACKEND_CONFIG_FILENAME {
			mtimes = append(mtimes, *r.MtimeUtc)
		}
	}

	slices.SortFunc(mtimes, func(a, b time.Time) int {
		return a.Compare(b)
	})

	var dataFreshnessUtc *time.Time
	if len(mtimes) > 0 {
		dataFreshnessUtc = &mtimes[len(mtimes)-1]
	}

	var billingHourlyRate float64 = 0
	if costModel != nil && costModel.CostModel == "costs" {
		warnings = append(warnings, "billing_hourly_rate is not exposed in the current static data set; backend.json carries only cost_model and currency.")
	}

	return DatasetInfo{
		MetricsAvailable:  metricsAvailable,
		GpuAvailable:      gpuAvailable,
		Scales:            scales,
		Trends:            trends,
		Dimensions:        dimensions,
		Efficiency:        efficiency,
		CostModel:         costModel,
		BillingHourlyRate: billingHourlyRate,
		DataFreshnessUtc:  dataFreshnessUtc,
		Warnings:          warnings,
	}
}

func ClassifyNamespace(name string) string {
	if _, ok := SPECIAL_ENTRIES[name]; ok {
		return "special"
	}

	for _, prefix := range SYSTEM_NAMESPACE_PREFIXES {
		if strings.HasPrefix(name, prefix) {
			return "system"
		}
	}

	return "application"
}

func iterDataFilenames(includeGpu *bool) []string {
	if includeGpu == nil {
		includeGpu = new(true)
	}
	out := []string{}
	gpuVariants := []bool{false}
	if *includeGpu {
		gpuVariants = []bool{false, true}
	}

	for _, metric := range METRICS {
		for _, period := range []int{PERIOD_14_DAYS_SEC, PERIOD_YEAR_SEC} {
			for _, dimension := range DIMENSIONS {
				for _, gpu := range gpuVariants {
					filename, err := HistogramFilename(metric, dimension, period, gpu)
					if err != nil {
						continue
					}

					out = append(out, filename)
				}
			}
		}

		for _, category := range []string{"usage", "rf"} {
			for _, gpu := range gpuVariants {
				if gpu && category == "rf" {
					continue
				}
				filename, err := TrendsFilename(metric, category, gpu)
				if err != nil {
					continue
				}
				out = append(out, filename)
			}
		}
	}

	return out
}

func collectNamespaceNames(dataDir string) (map[string]struct{}, []string, *time.Time) {
	root := dataDir
	if root != "" {
		root = getDataDir()
	}

	names := map[string]struct{}{}
	warnings := []string{}
	mtimes := []time.Time{}

	for _, fname := range iterDataFilenames(nil) {
		result := ReadJsonFile(fname, root)
		if !result.Exists {
			continue
		}

		if result.MtimeUtc != nil {
			mtimes = append(mtimes, *result.MtimeUtc)
		}

		if !result.Ok() {
			warnings = append(warnings, fmt.Sprintf("%s: %s", fname, result.ErrorMsg))
			continue
		}

		var list []map[string]string
		if err := json.Unmarshal(result.Payload, &list); err != nil {
			continue
		}

		for _, entry := range list {
			if stack, ok := entry["stack"]; ok {
				names[stack] = struct{}{}
			}
			if name, ok := entry["name"]; ok {
				names[name] = struct{}{}
			}
		}
	}

	slices.SortFunc(mtimes, func(a, b time.Time) int {
		return a.Compare(b)
	})

	if len(mtimes) > 0 {
		return names, warnings, &mtimes[len(mtimes)-1]
	}
	return names, warnings, nil
}

func BuildMetadata(costModel *CostModelInfo, warnings []string, metric string, scale string, dataWindow map[string]any, sourceFile string, generatedAtMtime *time.Time, unitOverride string) map[string]any {
	outWarnings := warnings
	generatedAtUtc := ""
	if generatedAtMtime != nil {
		generatedAtUtc = isoUtc(generatedAtMtime.Unix())
		ageSec := time.Until(*generatedAtMtime) * time.Second
		if ageSec > FRESHNESS_WARN_AFTER_SECONDS {
			outWarnings = append(outWarnings, fmt.Sprintf("stale data: most recent file is %d minutes old", ageSec/60))
		}
	}

	costM := ""
	currency := ""
	unit := ""
	if costModel != nil {
		costM = costModel.CostModel
		currency = costModel.Currency
		unit = costModel.Unit
	}
	if unit == "" {
		unit = unitOverride
	}

	return map[string]any{
		"cost_model":       costM,
		"currency":         currency,
		"unit":             unit,
		"metric":           metric,
		"scale":            scale,
		"data_window":      dataWindow,
		"source_file":      sourceFile,
		"generated_at_utc": generatedAtUtc,
		"warnings":         outWarnings,
	}
}

func ListNamespaces(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, map[string]any, error) {
	root := getDataDir()
	names, warnings, latestMtime := collectNamespaceNames(root)
	costModel, cmWarnings := LoadCostModel(root)
	warnings = append(warnings, cmWarnings...)

	namespaces := []map[string]string{}
	counts := map[string]int{"application": 0, "system": 0, "special": 0}

	namekeys := slices.Collect(maps.Keys(names))
	slices.Sort(namekeys)
	for _, name := range namekeys {
		kind := ClassifyNamespace(name)
		if kind == "special" {
			continue
		}
		namespaces = append(namespaces, map[string]string{"name": name, "type": kind})
		counts[kind] += 1
	}

	specialEntries := []map[string]string{}
	for entryName, meta := range SPECIAL_ENTRIES {
		specialEntries = append(specialEntries, map[string]string{
			"name":        entryName,
			"role":        meta["role"],
			"description": meta["description"],
		})
	}
	counts["special"] = len(specialEntries)

	return nil, map[string]any{
		"namespaces":     namespaces,
		"specialEntries": specialEntries,
		"counts":         counts,
		"metadata": BuildMetadata(
			costModel,
			warnings,
			"",
			"",
			nil,
			"",
			latestMtime,
			"",
		),
	}, nil
}

func BuildMcpApp() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "greeter", Version: "v1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "list_namespaces", Description: "List available namespaces (application / system) and the known special entries."}, ListNamespaces)

	return server
}
