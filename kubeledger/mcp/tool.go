package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"slices"

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

func BuildMcpApp() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "greeter", Version: "v1.0.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{Name: "list_namespaces", Description: "List available namespaces (application / system) and the known special entries."}, ListNamespaces)
	mcp.AddTool(server, &mcp.Tool{Name: "describe_dataset", Description: "Announce dataset capabilities: metrics, GPU availability, scales, efficiency, cost_model."}, DescribeDataset)
	mcp.AddTool(server, &mcp.Tool{Name: "get_usage", Description: "Usage per namespace for a given metric and scale (time series of points)."}, GetUsage)

	return server
}
