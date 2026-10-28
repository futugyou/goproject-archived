package mcp

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"maps"
	"math"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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

func DescribeDataset(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, map[string]any, error) {
	info := DiscoverDataset("")

	scalesOut := map[string]map[string]any{}
	for scaleKey, scaleInfo := range info.Scales {
		scalesOut[scaleKey] = map[string]any{
			"granularity":          scaleInfo.Granularity,
			"depth":                scaleInfo.Depth,
			"points_per_namespace": scaleInfo.PointsPerNamespace,
		}
	}

	trendsOut := map[string]any{}
	if info.Trends != nil {
		trendsOut = map[string]any{
			"granularity":          info.Trends.Granularity,
			"depth":                info.Trends.Depth,
			"points_per_namespace": info.Trends.PointsPerNamespace,
		}
	}

	freshnessMtime := info.DataFreshnessUtc
	costModel := ""
	currency := ""
	if info.CostModel != nil {
		costModel = info.CostModel.CostModel
		currency = info.CostModel.Currency
	}

	return nil, map[string]any{
		"metrics_available": info.MetricsAvailable,
		"gpu_available":     info.GpuAvailable,
		"scales":            scalesOut,
		"trends":            trendsOut,
		"dimensions":        info.Dimensions,
		"efficiency": map[string]any{
			"aggregated":        info.Efficiency.Aggregated,
			"hourly_timeseries": info.Efficiency.HourlyTimeserie,
		},
		"cost_model":          costModel,
		"currency":            currency,
		"billing_hourly_rate": info.BillingHourlyRate,
		"data_freshness_utc":  info.DataFreshnessUtc,
		"metadata":            BuildMetadata(info.CostModel, info.Warnings, "", "", nil, "", freshnessMtime, ""),
	}, nil
}

type UsageInput struct {
	Metric         string `json:"metric" jsonschema:"get CPU or MEMORY Metric data"`
	Scale          string `json:"scale" jsonschema:"the scanning range, daily_14d or monthly_12m"`
	Namespace      string `json:"namespace" jsonschema:"namespace for metric"`
	ExcludeSpecial *bool  `json:"exclude_special" jsonschema:"whether it includes special items(non-allocatable/.billing-hourly-rate), default True"`
}

func GetUsage(ctx context.Context, req *mcp.CallToolRequest, input UsageInput) (*mcp.CallToolResult, map[string]any, error) {
	metric := input.Metric
	scale := input.Metric
	namespace := input.Namespace
	excludeSpecial := true
	if input.ExcludeSpecial != nil {
		excludeSpecial = *input.ExcludeSpecial
	}

	if err := validateMetric(metric); err != nil {
		return nil, nil, err
	}
	if err := validateScale(scale); err != nil {
		return nil, nil, err
	}

	costModel, warnings := LoadCostModel("")
	entries, fname, mtime, errMsg := loadHistogramPayload(metric, scale, "usage", false, "")

	if len(entries) == 0 {
		warnings = append(warnings, fmt.Sprintf("%s: %s", fname, errMsg))
		return nil, map[string]any{
			"metric":    metric,
			"scale":     scale,
			"namespace": namespace,
			"metadata": BuildMetadata(
				costModel,
				warnings,
				metric,
				scale,
				nil,
				fname,
				mtime,
				"",
			),
		}, nil
	}

	series := []map[string]any{}
	for _, entry := range entries {
		stack, ok := entry["stack"].(string)
		if !ok {
			continue
		}

		if excludeSpecial && isSpecial(stack) {
			continue
		}

		if namespace != "" && stack != namespace {
			continue
		}

		value := coerceValue(entry)
		dateKey, ok := entry["date"].(string)
		if value == nil || !ok {
			continue
		}

		series = append(series, map[string]any{
			"namespace": stack,
			"date":      dateKey,
			"value":     math.Round(*value),
		})
	}

	dates := uniqueDatesInOrder(entries)
	return nil, map[string]any{
		"metric":    metric,
		"scale":     scale,
		"namespace": namespace,
		"series":    series,
		"metadata": BuildMetadata(
			costModel,
			warnings,
			metric,
			scale,
			buildDataWindow(scale, dates, ""),
			fname,
			mtime,
			"",
		),
	}, nil
}

type ConsumerInput struct {
	Metric        string `json:"metric" jsonschema:"get CPU or MEMORY Metric data"`
	Scale         string `json:"scale" jsonschema:"the scanning range, daily_14d or monthly_12m"`
	Limit         int    `json:"limit" jsonschema:"scanning data limit"`
	ExcludeSystem *bool  `json:"exclude_system" jsonschema:"controls whether system namespaces appear, default True"`
	Date          string `json:"date" jsonschema:"selected date (last available by default)"`
}

func GetTopConsumers(ctx context.Context, req *mcp.CallToolRequest, input ConsumerInput) (*mcp.CallToolResult, map[string]any, error) {
	metric := input.Metric
	scale := input.Scale
	limit := input.Limit
	excludeSystem := true
	if input.ExcludeSystem != nil {
		excludeSystem = *input.ExcludeSystem
	}

	date := input.Date

	if limit < 1 {
		limit = 10
	}

	if err := validateMetric(metric); err != nil {
		return nil, nil, err
	}
	if err := validateScale(scale); err != nil {
		return nil, nil, err
	}

	costModel, warnings := LoadCostModel("")
	entries, fname, mtime, errMsg := loadHistogramPayload(metric, scale, "usage", false, "")
	if len(entries) == 0 {
		warnings = append(warnings, fmt.Sprintf("%s: %s", fname, errMsg))
		return nil, map[string]any{
			"metric": metric,
			"scale":  scale,
			"date":   date,
			"metadata": BuildMetadata(
				costModel,
				warnings,
				metric,
				scale,
				nil,
				fname,
				mtime,
				"",
			),
		}, nil
	}

	dates := uniqueDatesInOrder(entries)
	targetDate := date
	if targetDate == "" && len(dates) > 0 {
		targetDate = dates[len(dates)-1]
	}

	if targetDate == "" {
		warnings = append(warnings, fmt.Sprintf("no date available in %s", fname))
		return nil, map[string]any{
			"metric": metric,
			"scale":  scale,
			"date":   nil,
			"metadata": BuildMetadata(
				costModel,
				warnings,
				metric,
				scale,
				nil,
				fname,
				mtime,
				"",
			),
		}, nil
	}

	if date != "" && !slices.Contains(dates, date) {
		warnings = append(warnings, fmt.Sprintf("date %s not present in %s (available: %s)", date, fname, strings.Join(dates, ",")))
	}

	type candidate struct {
		stack string
		value float64
	}

	candidates := []candidate{}
	var total float64 = 0
	for _, entry := range entries {
		if d, ok := entry["date"].(string); !ok || d != targetDate {
			continue
		}

		stack, ok := entry["stack"].(string)
		if !ok {
			continue
		}

		if isSpecial(stack) {
			continue
		}
		if excludeSystem && isSystem(stack) {
			continue
		}
		value := coerceValue(entry)
		if value == nil {
			continue
		}
		total += *value
		candidates = append(candidates, candidate{
			stack: stack,
			value: *value,
		})
	}

	slices.SortFunc(candidates, func(a, b candidate) int {
		return cmp.Compare(b.value, a.value)
	})

	effectiveLimit := min(limit, len(candidates))

	ranking := []map[string]any{}
	for i := range effectiveLimit {
		idx := i + 1
		ns := candidates[i].stack
		value := candidates[i].value
		var sharePct float64 = 0
		if total > 0 {
			sharePct = (100.0 * value / total)
		}
		ranking = append(ranking, map[string]any{
			"rank":      idx,
			"namespace": ns,
			"value":     math.Round(value),
			"share_pct": math.Round(sharePct),
		})

	}

	return nil, map[string]any{
		"metric":  metric,
		"scale":   scale,
		"date":    targetDate,
		"ranking": ranking,
		"metadata": BuildMetadata(
			costModel,
			warnings,
			metric,
			scale,
			buildDataWindow(scale, dates, targetDate),
			fname,
			mtime,
			"",
		),
	}, nil
}

type NamespaceBreakdownInput struct {
	Metric        string `json:"metric" jsonschema:"get CPU or MEMORY Metric data"`
	Scale         string `json:"scale" jsonschema:"the scanning range, daily_14d or monthly_12m"`
	Date          string `json:"date" jsonschema:"selected date (last available by default)"`
	ExcludeSystem *bool  `json:"exclude_system" jsonschema:"controls whether system namespaces appear, default True"`
}

func GetNamespaceBreakdown(ctx context.Context, req *mcp.CallToolRequest, input NamespaceBreakdownInput) (*mcp.CallToolResult, map[string]any, error) {
	metric := input.Scale
	scale := input.Scale
	date := input.Date
	excludeSystem := true
	if input.ExcludeSystem != nil {
		excludeSystem = *input.ExcludeSystem
	}

	if err := validateMetric(metric); err != nil {
		return nil, nil, err
	}
	if err := validateScale(scale); err != nil {
		return nil, nil, err
	}

	costModel, warnings := LoadCostModel("")
	entries, fname, mtime, errMsg := loadHistogramPayload(metric, scale, "usage", false, "")

	if len(entries) == 0 {
		warnings = append(warnings, fmt.Sprintf("%s: %s", fname, errMsg))
		return nil, map[string]any{
			"metric":           metric,
			"scale":            scale,
			"date":             date,
			"breakdown":        []any{},
			"concentration":    map[string]float64{"top_3_share_pct": 0.0, "top_10_share_pct": 0.0, "total_namespaces": 0},
			"cluster_overhead": nil,
			"metadata": BuildMetadata(
				costModel,
				warnings,
				metric,
				scale,
				nil,
				fname,
				mtime,
				"",
			),
		}, nil
	}

	dates := uniqueDatesInOrder(entries)
	targetDate := date
	if targetDate == "" && len(dates) > 0 {
		targetDate = dates[len(dates)-1]
	}

	if targetDate == "" {
		warnings = append(warnings, fmt.Sprintf("no date available in %s", fname))
		return nil, map[string]any{
			"metric":           metric,
			"scale":            scale,
			"date":             nil,
			"breakdown":        []any{},
			"concentration":    map[string]float64{"top_3_share_pct": 0.0, "top_10_share_pct": 0.0, "total_namespaces": 0},
			"cluster_overhead": nil,
			"metadata": BuildMetadata(
				costModel,
				warnings,
				metric,
				scale,
				nil,
				fname,
				mtime,
				"",
			),
		}, nil
	}

	if date != "" && !slices.Contains(dates, date) {
		warnings = append(warnings, fmt.Sprintf("date %s not present in %s (available: %s)", date, fname, strings.Join(dates, ",")))
	}

	type namespaceValue struct {
		stack string
		value float64
	}

	var nonAllocatableValue *float64
	namespaceValues := []namespaceValue{}
	var namespaceTotal float64 = 0

	for _, entry := range entries {
		if d, ok := entry["date"].(string); !ok || d != targetDate {
			continue
		}

		stack, ok := entry["stack"].(string)
		if !ok {
			continue
		}
		value := coerceValue(entry)
		if value == nil {
			continue
		}

		if stack == "non-allocatable" {
			nonAllocatableValue = value
			continue
		}
		if isSpecial(stack) {
			continue
		}
		if excludeSystem && isSystem(stack) {
			continue
		}

		namespaceTotal += *value
		namespaceValues = append(namespaceValues, namespaceValue{
			stack: stack,
			value: *value,
		})
	}

	slices.SortFunc(namespaceValues, func(a, b namespaceValue) int {
		return cmp.Compare(b.value, a.value)
	})

	breakdown := []map[string]any{}
	var top_3_sum float64 = 0
	var top_10_sum float64 = 0
	for i, v := range namespaceValues {
		ns := v.stack
		value := v.value
		if i < 3 {
			top_3_sum += value
		}
		if i < 10 {
			top_10_sum += value
		}
		var sharePct float64 = 0
		if namespaceTotal > 0 {
			sharePct = (100.0 * value / namespaceTotal)
		}
		breakdown = append(breakdown, map[string]any{
			"namespace": ns,
			"value":     math.Round(value),
			"share_pct": math.Round(sharePct),
		})
	}

	top_3_share_pct := 0.0
	top_10_share_pct := 0.0
	if namespaceTotal != 0 {
		top_3_share_pct = math.Round(100.0 * top_3_sum / namespaceTotal)
		top_10_share_pct = math.Round(100.0 * top_10_sum / namespaceTotal)
	}

	concentration := map[string]float64{
		"top_3_share_pct":  top_3_share_pct,
		"top_10_share_pct": top_10_share_pct,
		"total_namespaces": float64(len(breakdown)),
	}

	clusterOverhead := map[string]any{}
	if nonAllocatableValue != nil {
		totalCluster := *nonAllocatableValue + namespaceTotal
		if totalCluster > 0 {
			clusterOverhead = map[string]any{
				"non_allocatable_value":     math.Round(*nonAllocatableValue),
				"non_allocatable_share_pct": math.Round(100.0 * *nonAllocatableValue / totalCluster),
				"allocatable_share_pct":     math.Round(100.0 * namespaceTotal / totalCluster),
			}
		} else {
			warnings = append(warnings, "non-allocatable present but total cluster value is zero; cluster_overhead omitted")
		}
	} else {
		warnings = append(warnings, fmt.Sprintf("non-allocatable absent from data for date %s; cluster_overhead omitted", targetDate))
	}
	return nil, map[string]any{
		"metric":           metric,
		"scale":            scale,
		"date":             targetDate,
		"breakdown":        breakdown,
		"concentration":    concentration,
		"cluster_overhead": clusterOverhead,
		"metadata": BuildMetadata(
			costModel,
			warnings,
			metric,
			scale,
			buildDataWindow(scale, dates, targetDate),
			fname,
			mtime,
			"",
		),
	}, nil
}

type EfficiencyInput struct {
	Scale     string `json:"scale" jsonschema:"the scanning range, daily_14d or monthly_12m"`
	Namespace string `json:"namespace" jsonschema:"k8s namespace"`
	Metric    string `json:"metric" jsonschema:"get CPU or MEMORY Metric data"`
}

func GetEfficiency(ctx context.Context, req *mcp.CallToolRequest, input EfficiencyInput) (*mcp.CallToolResult, map[string]any, error) {
	scale := input.Scale
	namespace := input.Namespace
	metric := input.Metric

	if err := validateScale(scale); err != nil {
		return nil, nil, err
	}

	if metric != "" {
		if err := validateMetric(metric); err != nil {
			return nil, nil, err
		}
	}

	metricsFilter := METRICS
	if metric != "" {
		metricsFilter = []string{metric}
	}

	costModel, warnings := LoadCostModel("")
	efficiency := []map[string]any{}
	sourceFiles := []string{}
	var latestMtime *time.Time
	dataWindow := map[string]any{}

	for _, currentMetric := range metricsFilter {
		usageEntries, usageFname, usageMtime, usageErr := loadHistogramPayload(currentMetric, scale, "usage", false, "")
		if len(usageEntries) == 0 {
			warnings = append(warnings, fmt.Sprintf("%s: %s", usageFname, usageErr))
			continue
		}

		sourceFiles = append(sourceFiles, usageFname)
		if usageMtime != nil {
			if latestMtime == nil {
				latestMtime = usageMtime
			}
			if usageMtime.After(*latestMtime) {
				latestMtime = usageMtime
			}
		}

		reqEntries, reqFname, reqMtime, reqErr := loadHistogramPayload(currentMetric, scale, "requests", false, "")
		if reqEntries == nil {
			warnings = append(warnings, fmt.Sprintf("%s: %s", reqFname, reqErr))
			continue
		}

		sourceFiles = append(sourceFiles, reqFname)
		if reqMtime != nil {
			if latestMtime == nil {
				latestMtime = reqMtime
			}
			if reqMtime.After(*latestMtime) {
				latestMtime = reqMtime
			}
		}

		if len(dataWindow) == 0 {
			dates := uniqueDatesInOrder(usageEntries)
			dataWindow = buildDataWindow(scale, dates, "")
		}

		usageTotals := aggregateByNamespace(usageEntries)
		requestsTotals := aggregateByNamespace(reqEntries)

		allNamespaces := []string{}
		for key := range usageTotals {
			if !slices.Contains(allNamespaces, key) {
				allNamespaces = append(allNamespaces, key)
			}
		}
		for key := range requestsTotals {
			if !slices.Contains(allNamespaces, key) {
				allNamespaces = append(allNamespaces, key)
			}
		}

		if len(allNamespaces) > 0 {
			if !slices.Contains(allNamespaces, namespace) {
				warnings = append(warnings, fmt.Sprintf("namespace %s not present in %s histograms", namespace, currentMetric))
				continue
			}
			allNamespaces = []string{namespace}
		}

		slices.Sort(allNamespaces)
		for _, ns := range allNamespaces {
			usageValue := usageTotals[ns]
			requestsValue := requestsTotals[ns]

			var ratio float64
			classification := ""
			if requestsValue < REQUESTS_ZERO_THRESHOLD {
				warnings = append(warnings, fmt.Sprintf("%s/%s: requests aggregate below {REQUESTS_ZERO_THRESHOLD:g} threshold;efficiency_ratio set to null", ns, currentMetric))
			} else {
				ratio = usageValue / requestsValue
				classification = classifyEfficiency(ratio)
			}

			efficiency = append(efficiency, map[string]any{
				"namespace":        ns,
				"metric":           currentMetric,
				"usage":            math.Round(usageValue),
				"requests":         math.Round(requestsValue),
				"efficiency_ratio": math.Round(ratio),
				"classification":   classification,
			})

		}
	}

	slices.SortFunc(efficiency, func(a, b map[string]any) int {
		r := cmp.Compare(a["namespace"].(string), b["namespace"].(string))
		if r != 0 {
			return r
		}

		return cmp.Compare(a["metric"].(string), b["metric"].(string))
	})

	sourceFile := strings.Join(sourceFiles, ", ")

	return nil, map[string]any{
		"scale":      scale,
		"namespace":  namespace,
		"metric":     metric,
		"efficiency": efficiency,
		"metadata": BuildMetadata(
			costModel,
			warnings,
			metric,
			scale,
			dataWindow,
			sourceFile,
			latestMtime,
			"efficiency_ratio",
		),
	}, nil
}

type TimeseriesInput struct {
	Namespace string `json:"namespace" jsonschema:"k8s namespace"`
	Metric    string `json:"metric" jsonschema:"get CPU or MEMORY Metric data"`
}

func GetTimeseries(ctx context.Context, req *mcp.CallToolRequest, input TimeseriesInput) (*mcp.CallToolResult, map[string]any, error) {
	return getInnerTimeseries(ctx, req, input, "usage", "")
}

func GetEfficiencyTimeseries(ctx context.Context, req *mcp.CallToolRequest, input TimeseriesInput) (*mcp.CallToolResult, map[string]any, error) {
	return getInnerTimeseries(ctx, req, input, "rf", "efficiency_ratio")
}

func getInnerTimeseries(_ context.Context, _ *mcp.CallToolRequest, input TimeseriesInput, category, unitOverride string) (*mcp.CallToolResult, map[string]any, error) {
	metric := input.Metric
	namespace := input.Namespace

	if err := validateMetric(metric); err != nil {
		return nil, nil, err
	}

	if namespace == "" {
		return nil, nil, errors.New("namespace can not be empty")
	}

	costModel, warnings := LoadCostModel("")
	entries, fname, mtime, errMsg := loadTrendsPayload(metric, category, false)
	if len(entries) == 0 {
		warnings = append(warnings, fmt.Sprintf("%s: %s", fname, errMsg))
		emptyStats := computeTrendStats(nil, nil)
		return nil, map[string]any{
			"namespace": namespace,
			"metric":    metric,
			"series":    []float64{},
			"stats":     emptyStats,
			"metadata": BuildMetadata(
				costModel,
				warnings,
				metric,
				"",
				nil,
				fname,
				mtime,
				unitOverride,
			),
		}, nil
	}

	series, values := filterTrendsNamespace(entries, namespace)
	if len(series) == 0 {
		warnings = append(warnings, fmt.Sprintf("namespace %s not present in %s (no data points)", namespace, fname))
	}

	stats := computeTrendStats(series, values)

	return nil, map[string]any{
		"namespace": namespace,
		"metric":    metric,
		"series":    series,
		"stats":     stats,
		"metadata": BuildMetadata(
			costModel,
			warnings,
			metric,
			"",
			trendsDataWindow(stats),
			fname,
			mtime,
			unitOverride,
		),
	}, nil
}

type ComparePeriodsInput struct {
	Namespace string `json:"namespace" jsonschema:"k8s namespace"`
	Metric    string `json:"metric" jsonschema:"get CPU or MEMORY Metric data"`
}

func ComparePeriods(ctx context.Context, req *mcp.CallToolRequest, input ComparePeriodsInput) (*mcp.CallToolResult, map[string]any, error) {
	metric := input.Metric
	namespace := input.Namespace

	if err := validateMetric(metric); err != nil {
		return nil, nil, err
	}

	if namespace == "" {
		return nil, nil, errors.New("namespace can not be empty")
	}

	costModel, warnings := LoadCostModel("")
	dailyEntries, dailyFname, dailyMtime, dailyErr := loadHistogramPayload(metric, SCALE_DAILY_14D, "usage", false, "")

	dailyBlock := map[string]any{}
	if len(dailyEntries) == 0 {
		warnings = append(warnings, fmt.Sprintf("%s: %s", dailyFname, dailyErr))
	} else {
		total, dates := namespacePeriodAggregate(dailyEntries, namespace)
		if len(dates) == 0 {
			warnings = append(warnings, fmt.Sprintf("namespace %s not present in %s", namespace, dailyFname))
		}
		start := ""
		end := ""
		if len(dates) > 0 {
			start = dates[0]
			end = dates[len(dates)-1]
		}
		dailyBlock = map[string]any{
			"aggregate":    math.Round(total),
			"window_start": start,
			"window_end":   end,
			"points":       len(dates),
		}
	}

	monthlyEntries, monthlyFname, monthlyMtime, monthlyErr := loadHistogramPayload(metric, SCALE_MONTHLY_12M, "usage", false, "")
	monthlyBlock := map[string]any{}
	if len(monthlyEntries) == 0 {
		warnings = append(warnings, fmt.Sprintf("%s: %s", monthlyFname, monthlyErr))
	} else {
		series, values := namespaceMonthlyTrajectory(monthlyEntries, namespace)
		if len(values) == 0 {
			warnings = append(warnings, fmt.Sprintf("namespace %s not present in %s", namespace, monthlyFname))
		}
		monthlyBlock = map[string]any{
			"series":          series,
			"trend_direction": classifyTrend(values),
		}
	}

	var latestMtime *time.Time
	for _, mt := range []*time.Time{dailyMtime, monthlyMtime} {
		if mt != nil {
			if latestMtime == nil {
				latestMtime = mt
			}

			if mt.After(*latestMtime) {
				latestMtime = mt
			}
		}
	}

	sourceFile := fmt.Sprintf("%s, %s", dailyFname, monthlyFname)

	return nil, map[string]any{
		"namespace":   namespace,
		"metric":      metric,
		"daily_14d":   dailyBlock,
		"monthly_12m": monthlyBlock,
		"metadata": BuildMetadata(
			costModel,
			warnings,
			metric,
			"",
			nil,
			sourceFile,
			latestMtime,
			"",
		),
	}, nil
}

type NamespacesInput struct {
	Metric  string `json:"metric" jsonschema:"get CPU or MEMORY Metric data"`
	Scale   string `json:"scale" jsonschema:"the scanning range, daily_14d or monthly_12m"`
	Pattern string `json:"pattern" jsonschema:"fatch pattern"`
	Date    string `json:"date" jsonschema:"target date"`
}

func GroupNamespaces(ctx context.Context, req *mcp.CallToolRequest, input NamespacesInput) (*mcp.CallToolResult, map[string]any, error) {
	metric := strings.TrimSpace(input.Metric)
	scale := strings.TrimSpace(input.Scale)
	pattern := strings.TrimSpace(input.Pattern)
	date := strings.TrimSpace(input.Date)

	if err := validateMetric(metric); err != nil {
		return nil, nil, err
	}
	if err := validateScale(scale); err != nil {
		return nil, nil, err
	}

	if pattern == "" {
		return nil, nil, errors.New("pattern must be a non-empty string")
	}
	if len(pattern) > GROUP_PATTERN_MAX_LENGTH {
		return nil, nil, fmt.Errorf("pattern length %d exceeds maximum of %d", len(pattern), GROUP_PATTERN_MAX_LENGTH)
	}

	costModel, warnings := LoadCostModel("")
	entries, fname, mtime, errMsg := loadHistogramPayload(metric, scale, "usage", false, "")
	if len(entries) == 0 {
		warnings = append(warnings, fmt.Sprintf("%s: %s", fname, errMsg))
		return nil, map[string]any{
			"metric":        metric,
			"scale":         scale,
			"pattern":       pattern,
			"date":          date,
			"group_total":   0.0,
			"matched_count": 0,
			"members":       map[string]any{},
			"metadata": BuildMetadata(
				costModel,
				warnings,
				metric,
				scale,
				nil,
				fname,
				mtime,
				"",
			),
		}, nil
	}

	dates := uniqueDatesInOrder(entries)
	targetDate := date
	if targetDate == "" && len(dates) > 0 {
		targetDate = dates[len(dates)-1]
	}
	if targetDate == "" {
		warnings = append(warnings, fmt.Sprintf("no date available in %s", fname))
		return nil, map[string]any{
			"metric":        metric,
			"scale":         scale,
			"pattern":       pattern,
			"date":          "",
			"group_total":   0.0,
			"matched_count": 0,
			"members":       []Member{},
			"metadata": BuildMetadata(
				costModel,
				warnings,
				metric,
				scale,
				nil,
				fname,
				mtime,
				"",
			),
		}, nil
	}

	if date != "" && !slices.Contains(dates, date) {
		warnings = append(warnings, fmt.Sprintf("date %s not present in %s (available: %s)", date, fname, strings.Join(dates, ",")))
	}

	membersMap := map[string]float64{}
	var groupTotal float64
	for _, entry := range entries {
		if d, ok := entry["date"].(string); !ok || d != targetDate {
			continue
		}
		stack, ok := entry["stack"].(string)
		if !ok {
			continue
		}

		if isSpecial(stack) {
			continue
		}

		// TODO:
		// matched, err := filepath.Match(pattern, stack)
		// if err != nil {
		// 	continue
		// }
		// if !matched {
		// 	continue
		// }

		value := coerceValue(entry)
		if value == nil {
			continue
		}

		membersMap[stack] = membersMap[stack] + *value
		groupTotal += *value
	}

	members := sortMembersByValue(membersMap)
	if len(members) == 0 {
		warnings = append(warnings, fmt.Sprintf("pattern %s matched no namespace at date %s", pattern, targetDate))
	}

	return nil, map[string]any{
		"metric":        metric,
		"scale":         scale,
		"pattern":       pattern,
		"date":          targetDate,
		"group_total":   math.Round(groupTotal),
		"matched_count": len(members),
		"members":       members,
		"metadata": BuildMetadata(
			costModel,
			warnings,
			metric,
			scale,
			buildDataWindow(scale, dates, targetDate),
			fname,
			mtime,
			"",
		),
	}, nil
}

func BuildMcpApp(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{Name: "list_namespaces", Description: "List available namespaces (application / system) and the known special entries."}, ListNamespaces)
	mcp.AddTool(server, &mcp.Tool{Name: "describe_dataset", Description: "Announce dataset capabilities: metrics, GPU availability, scales, efficiency, cost_model."}, DescribeDataset)
	mcp.AddTool(server, &mcp.Tool{Name: "get_usage", Description: "Usage per namespace for a given metric and scale (time series of points)."}, GetUsage)
	mcp.AddTool(server, &mcp.Tool{Name: "get_top_consumers", Description: "Rank namespaces by usage for a date (defaults to the most recent in the data)."}, GetTopConsumers)
	mcp.AddTool(server, &mcp.Tool{Name: "get_namespace_breakdown", Description: "Proportional breakdown + concentration + cluster_overhead (non-allocatable separated)."}, GetNamespaceBreakdown)
	mcp.AddTool(server, &mcp.Tool{Name: "get_efficiency", Description: "Return usage/requests ratio per (namespace, metric): descriptive classification."}, GetEfficiency)
	mcp.AddTool(server, &mcp.Tool{Name: "get_timeseries", Description: "Hourly usage series over the trends window for a single namespace, with min/max/mean."}, GetTimeseries)
	mcp.AddTool(server, &mcp.Tool{Name: "get_efficiency_timeseries", Description: "Hourly efficiency factor (rf = usage/requests) series: not bounded at 1."}, GetTimeseries)
	mcp.AddTool(server, &mcp.Tool{Name: "compare_periods", Description: "Compare 14-day aggregate against monthly trajectory; descriptive trend direction."}, ComparePeriods)
	mcp.AddTool(server, &mcp.Tool{Name: "group_namespaces", Description: "Aggregate namespaces matching a glob pattern (fnmatch): sum + per-member detail."}, GroupNamespaces)
}

func StartMcpServer() {
	server := mcp.NewServer(&mcp.Implementation{Name: "kubeledger", Version: "v1.0.0"}, nil)

	BuildMcpApp(server)

	handler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return server
	}, nil)

	url := os.Getenv("MCP_LISTEN_HOST")
	if url == "" {
		url = "localhost:8100"
	}

	log.Printf("MCP server listening on %s", url)

	// Start the HTTP server.
	if err := http.ListenAndServe(url, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
