package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"slices"
	"strings"
	"time"
)

func dumpTrendAnalytics(ctx context.Context, db *sql.DB, subCategory string, prefix string, categories []string) {
	sum_cpu_usage := []TrendDataDTO{}
	sum_mem_usage := []TrendDataDTO{}
	for _, category := range categories {
		if category == KOA_CONFIG.DbBillingHourlyRate && !KOA_CONFIG.EnableDebug {
			continue
		}
		rrd, err := NewPartman(db, category, PERIOD_1_HOUR_SEC)
		if err != nil {
			KOA_LOGGER.Error(err.Error())
			continue
		}

		cpu_usage, mem_usage, err := rrd.DumpTrendData(ctx, PERIOD_7_DAYS_SEC, 0)
		if err != nil {
			KOA_LOGGER.Error(err.Error())
			continue
		}
		KOA_LOGGER.Debug(fmt.Sprintf("[dumpTrendAnalytics] category=%s, cpu_data_len=%d, mem_data_len=%d", category, len(cpu_usage), len(mem_usage)))
		sum_cpu_usage = append(sum_cpu_usage, cpu_usage...)
		sum_mem_usage = append(sum_mem_usage, mem_usage...)
	}

	mem_label := "mem"
	if prefix == "" {
		mem_label = "memory"
	}
	cpupath := fmt.Sprintf("%s/%scpu_%s_trends.json", KOA_CONFIG.FrontendDataLocation, prefix, subCategory)
	mempath := fmt.Sprintf("%s/%s%s_%s_trends.json", KOA_CONFIG.FrontendDataLocation, prefix, mem_label, subCategory)

	cpudata, err := json.Marshal(sum_cpu_usage)
	if err == nil && len(cpudata) > 0 {
		os.WriteFile(cpupath, cpudata, 0644)
	}

	memdata, err := json.Marshal(sum_mem_usage)
	if err == nil && len(memdata) > 0 {
		os.WriteFile(mempath, memdata, 0644)
	}
}

func calculateHistogramCost(value, total, billingRate float64, costModel string) float64 {
	if costModel != "RATIO" && costModel != "CHARGE_BACK" {
		return round(value, KOA_CONFIG.DbRoundDecimals)
	}

	if total <= 0 {
		return 0
	}

	ratio := value / total

	if costModel == "CHARGE_BACK" {
		return round(
			ratio*billingRate,
			KOA_CONFIG.DbRoundDecimals,
		)
	}

	return round(
		100*ratio,
		KOA_CONFIG.DbRoundDecimals,
	)
}

func writeAnalyticsJSON(path string, data []AnalyticsTDO) {
	content, err := json.Marshal(data)
	if err != nil {
		KOA_LOGGER.Error(
			fmt.Sprintf("Failed to marshal analytics %s: %v", path, err),
		)
		return
	}

	if len(content) == 0 {
		return
	}

	if err := os.WriteFile(path, content, 0644); err != nil {
		KOA_LOGGER.Error(
			fmt.Sprintf("Failed to write analytics %s: %v", path, err),
		)
	}
}

func dumpHistogramAnalytics(ctx context.Context, db *sql.DB, period int, cost_model, prefix string, categories []string) {
	now := time.Now()
	currentDate := getDateGroup(now, period)

	cpu := newHistogramMetrics()
	mem := newHistogramMetrics()

	billingCategory := KOA_CONFIG.DbBillingHourlyRate

	// 1. Collect and aggregate CPU and memory data.
	for _, category := range categories {
		rrd, err := NewPartman(db, category, PERIOD_1_HOUR_SEC)
		if err != nil {
			KOA_LOGGER.Error(err.Error())
			continue
		}

		cpuUsage, memUsage, err := rrd.DumpHistogramData(ctx, period, 0)
		if err != nil {
			KOA_LOGGER.Error(err.Error())
			continue
		}

		rrd, err = NewPartman(
			db,
			UsageEfficiencyDb(category),
			PERIOD_1_HOUR_SEC,
		)
		if err != nil {
			KOA_LOGGER.Error(err.Error())
			continue
		}

		cpuFactors, memFactors, err := rrd.DumpHistogramData(ctx, period, 0)
		if err != nil {
			KOA_LOGGER.Error(err.Error())
			continue
		}

		excludeFromTotals := category == billingCategory

		for date, usage := range cpuUsage {
			cpu.add(
				date,
				category,
				usage,
				cpuFactors[date],
				excludeFromTotals,
			)
		}

		for date, usage := range memUsage {
			mem.add(
				date,
				category,
				usage,
				memFactors[date],
				excludeFromTotals,
			)
		}
	}

	// 2. Calculate costs, export analytics and update Prometheus.
	exportResource := func(
		metrics *histogramMetrics,
		resourceType string,
	) ([]AnalyticsTDO, []AnalyticsTDO) {
		usageExport := make([]AnalyticsTDO, 0)
		requestsExport := make([]AnalyticsTDO, 0)

		for date, categoriesByDate := range metrics.usageByDate {
			billingRate := categoriesByDate[billingCategory]

			for category, usage := range categoriesByDate {
				if category == billingCategory {
					continue
				}

				usageCost := calculateHistogramCost(
					usage,
					metrics.totalUsage[date],
					billingRate,
					cost_model,
				)

				requestCost := calculateHistogramCost(
					metrics.requestsByDate[date][category],
					metrics.totalRequests[date],
					billingRate,
					cost_model,
				)

				usageExport = append(usageExport, AnalyticsTDO{
					Stack: category,
					Usage: usageCost,
					Date:  date,
				})

				requestsExport = append(requestsExport, AnalyticsTDO{
					Stack: category,
					Usage: requestCost,
					Date:  date,
				})

				if date != currentDate {
					continue
				}

				switch resourceType {
				case "CPU":
					PROMETHEUS_PERIODIC_USAGE_EXPORTERS[period].
						WithLabelValues(category, "CPU").
						Set(usageCost)

					PROMETHEUS_PERIODIC_REQUESTS_EXPORTERS[period].
						WithLabelValues(category, "CPU").
						Set(requestCost)

				case "MEMORY":
					PROMETHEUS_PERIODIC_USAGE_EXPORTERS[period].
						WithLabelValues(category, "MEMORY").
						Set(usageCost)

					PROMETHEUS_PERIODIC_REQUESTS_EXPORTERS[period].
						WithLabelValues(category, "MEMORY").
						Set(requestCost)
				}
			}
		}

		return usageExport, requestsExport
	}

	cpuUsageExport, cpuRequestsExport := exportResource(cpu, "CPU")
	memUsageExport, memRequestsExport := exportResource(mem, "MEMORY")

	// 3. Preserve the existing output file naming convention.
	memLabel := "mem"
	if prefix == "" {
		memLabel = "memory"
	}

	basePath := KOA_CONFIG.FrontendDataLocation

	writeAnalyticsJSON(
		fmt.Sprintf("%s/%scpu_usage_period_%d.json", basePath, prefix, period),
		cpuUsageExport,
	)
	writeAnalyticsJSON(
		fmt.Sprintf("%s/%s%s_usage_period_%d.json", basePath, prefix, memLabel, period),
		memUsageExport,
	)
	writeAnalyticsJSON(
		fmt.Sprintf("%s/%scpu_requests_period_%d.json", basePath, prefix, period),
		cpuRequestsExport,
	)
	writeAnalyticsJSON(
		fmt.Sprintf("%s/%s%s_requests_period_%d.json", basePath, prefix, memLabel, period),
		memRequestsExport,
	)
}

type histogramMetrics struct {
	usageByDate    map[string]map[string]float64
	requestsByDate map[string]map[string]float64
	totalUsage     map[string]float64
	totalRequests  map[string]float64
}

func newHistogramMetrics() *histogramMetrics {
	return &histogramMetrics{
		usageByDate:    make(map[string]map[string]float64),
		requestsByDate: make(map[string]map[string]float64),
		totalUsage:     make(map[string]float64),
		totalRequests:  make(map[string]float64),
	}
}

func (m *histogramMetrics) add(date, category string, usage, requestFactor float64, excludeFromTotals bool) {
	if usage <= 0 {
		return
	}

	if m.usageByDate[date] == nil {
		m.usageByDate[date] = make(map[string]float64)
		m.requestsByDate[date] = make(map[string]float64)
	}

	requests := 0.0
	if requestFactor > 0 {
		requests = usage / requestFactor
	}

	m.usageByDate[date][category] = usage
	m.requestsByDate[date][category] = requests

	if !excludeFromTotals {
		m.totalUsage[date] += usage
		m.totalRequests[date] += requests
	}
}

func DumpAnalytics(parentCtx context.Context, db *sql.DB, costModelParam string) {
	exportInterval := math.Round(1.5 * float64(KOA_CONFIG.PollingIntervalSec))
	for {
		select {
		case <-parentCtx.Done():
			return
		default:
		}

		ctx, cancel := context.WithCancel(parentCtx)

		func() {
			defer cancel()

			defer func() {
				if r := recover(); r != nil {
					KOA_LOGGER.Error(fmt.Sprintf("panic in CreateMetricsPuller loop: %v", r))
				}
			}()

			categories := GetMetricsCategories(ctx, db)

			nsCategories := []string{}
			rfCategories := []string{}
			gpuCategories := []string{}
			for _, category := range categories {
				if strings.HasSuffix(category, RequestEfficiencyDbFileExtension()) {
					rfCategories = append(rfCategories, category)
				} else if strings.HasSuffix(category, GpuDbFileExtension()) {
					gpuCategories = append(gpuCategories, category)
				} else {
					nsCategories = append(nsCategories, category)
				}
			}

			KOA_LOGGER.Info(fmt.Sprintf("[dump_analytics] nsCategories=%d, rfCategories=%d, gpuCategories=%d", len(nsCategories), len(rfCategories), len(gpuCategories)))
			if len(gpuCategories) > 0 {
				KOA_LOGGER.Info(fmt.Sprintf("[dump_analytics] gpuCategories: %s", strings.Join(gpuCategories, ",")))
			}

			dumpTrendAnalytics(ctx, db, "usage", "", nsCategories)
			dumpTrendAnalytics(ctx, db, "rf", "", rfCategories)
			dumpTrendAnalytics(ctx, db, "usage", "gpu_", gpuCategories)

			costModelSelected := costModelParam
			if costModelParam == "" {
				costModelSelected = KOA_CONFIG.CostModel
			} else {
				if !slices.Contains([]string{"CUMULATIVE", "RATIO", "CHARGE_BACK"}, costModelParam) {
					costModelSelected = "CUMULATIVE"
					KOA_LOGGER.Warn(fmt.Sprintf("Unexpected cost model => %s (using default => CUMULATIVE)", costModelParam))
				}
			}

			dumpHistogramAnalytics(ctx, db, PERIOD_14_DAYS_SEC, costModelSelected, "", nsCategories)
			dumpHistogramAnalytics(ctx, db, PERIOD_YEAR_SEC, costModelSelected, "", nsCategories)
			dumpHistogramAnalytics(ctx, db, PERIOD_14_DAYS_SEC, costModelSelected, "gpu_", gpuCategories)
			dumpHistogramAnalytics(ctx, db, PERIOD_YEAR_SEC, costModelSelected, "gpu_", gpuCategories)
		}()

		select {
		case <-parentCtx.Done():
			return
		case <-time.After(time.Duration(exportInterval) * time.Second):
		}
	}
}
