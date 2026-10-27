package mcp

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"

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
	var data []map[string]any
	if err := json.Unmarshal(entries, &data); err != nil {
		return nil, nil, err
	}
	for _, entry := range data {
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

	dates := uniqueDatesInOrder(data)
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

	var data []map[string]any
	if err := json.Unmarshal(entries, &data); err != nil {
		return nil, nil, err
	}

	dates := uniqueDatesInOrder(data)
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
	for _, entry := range data {
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

	var data []map[string]any
	if err := json.Unmarshal(entries, &data); err != nil {
		return nil, nil, err
	}

	dates := uniqueDatesInOrder(data)
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

	for _, entry := range data {
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

func BuildMcpApp() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "greeter", Version: "v1.0.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{Name: "list_namespaces", Description: "List available namespaces (application / system) and the known special entries."}, ListNamespaces)
	mcp.AddTool(server, &mcp.Tool{Name: "describe_dataset", Description: "Announce dataset capabilities: metrics, GPU availability, scales, efficiency, cost_model."}, DescribeDataset)
	mcp.AddTool(server, &mcp.Tool{Name: "get_usage", Description: "Usage per namespace for a given metric and scale (time series of points)."}, GetUsage)
	mcp.AddTool(server, &mcp.Tool{Name: "get_top_consumers", Description: "Rank namespaces by usage for a date (defaults to the most recent in the data)."}, GetTopConsumers)
	mcp.AddTool(server, &mcp.Tool{Name: "get_namespace_breakdown", Description: "Proportional breakdown + concentration + cluster_overhead (non-allocatable separated)."}, GetNamespaceBreakdown)

	return server
}
