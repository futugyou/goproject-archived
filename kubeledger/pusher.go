package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func computeUsagePercentRatio(value, total float64) float64 {
	return round((100.0*value)/total, KOA_CONFIG.DbRoundDecimals)
}

func CreateMetricsPuller(parentCtx context.Context, db *sql.DB) {
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

			KOA_LOGGER.Debug("[puller] collecting new samples")
			KOA_CONFIG.LoadRbacAuthToken()

			k8sUsage := K8sUsage{}
			k8sUsage.ExtractNamespacesAndInitializeUsage(pullK8s("/api/v1/namespaces"))
			k8sUsage.ExtractNodes(pullK8s("/api/v1/nodes"))
			k8sUsage.ExtractNodeMetrics(pullK8s("/apis/metrics.k8s.io/v1beta1/nodes"))
			k8sUsage.ExtractPods(pullK8s("/api/v1/pods"))
			k8sUsage.ExtractPodMetrics(pullK8s("/apis/metrics.k8s.io/v1beta1/pods"))
			// Collect GPU metrics from DCGM exporter if configured
			k8sUsage.ExtractGpuMetrics(pullDcgmMetrics())
			k8sUsage.ConsolidateNsUsage()
			k8sUsage.CalculateNodeUsage()
			k8sUsage.DumpNodes()
			k8sUsage.DumpGpuMetrics()

			if k8sUsage.cpuCapacity > 0.0 && k8sUsage.memCapacity > 0.0 {
				nowEpoch := time.Now().Unix()

				//  handle non-allocatable cpu
				cpuNonAllocatable := computeUsagePercentRatio(k8sUsage.cpuCapacity-k8sUsage.cpuAllocatable, k8sUsage.cpuCapacity)
				//  handle non-allocatable memory
				memNonAllocatable := computeUsagePercentRatio(k8sUsage.memCapacity-k8sUsage.memAllocatable, k8sUsage.memCapacity)

				rrd, err := NewPartman(db, KOA_CONFIG.DbNonAllocatable, PERIOD_1_HOUR_SEC)
				if err != nil {
					KOA_LOGGER.Error(fmt.Sprintf("create rrd error: %s", err.Error()))
					return
				}

				rrd.AddSample(ctx, nowEpoch, cpuNonAllocatable, memNonAllocatable)

				var hourlyRate float64 = -1
				if KOA_CONFIG.BillingHourlyRate > 0 {
					hourlyRate = KOA_CONFIG.BillingHourlyRate
				}

				rrd, err = NewPartman(db, KOA_CONFIG.DbBillingHourlyRate, PERIOD_1_HOUR_SEC)
				if err != nil {
					KOA_LOGGER.Error(fmt.Sprintf("create rrd error: %s", err.Error()))
					return
				}

				rrd.AddSample(ctx, nowEpoch, hourlyRate, hourlyRate)

				//  handle resource request and usage by pods
				for ns, nsUsage := range k8sUsage.usageByNamespace {
					rrd, err = NewPartman(db, ns, PERIOD_1_HOUR_SEC)
					if err != nil {
						KOA_LOGGER.Error(fmt.Sprintf("create rrd error: %s", err.Error()))
						continue
					}

					cpuUsage := computeUsagePercentRatio(nsUsage.Cpu, k8sUsage.cpuCapacity)
					memUsage := computeUsagePercentRatio(nsUsage.Mem, k8sUsage.memCapacity)
					rrd.AddSample(ctx, nowEpoch, cpuUsage, memUsage)

					cpuEfficiency := 1.0
					memEfficiency := 1.0
					requestCapacities, ok := k8sUsage.requestByNamespace[ns]
					if ok {
						if requestCapacities.Cpu > 0.0 {
							cpuEfficiency = round(nsUsage.Cpu/requestCapacities.Cpu, 2)
						}
						if requestCapacities.Mem > 0.0 {
							memEfficiency = round(nsUsage.Mem/requestCapacities.Mem, 2)
						}
					}

					if cpuEfficiency > 0.0 || memEfficiency > 0.0 {
						rrd, err = NewPartman(db, UsageEfficiencyDb(ns), PERIOD_1_HOUR_SEC)
						if err != nil {
							KOA_LOGGER.Error(fmt.Sprintf("create rrd error: %s", err.Error()))
							continue
						}
						rrd.AddSample(ctx, nowEpoch, cpuEfficiency, memEfficiency)
					}
				}

				//  handle GPU metrics by namespace (outside the namespace loop)
				if len(k8sUsage.gpuMetricsByPod) > 0 {
					//  Aggregate GPU metrics by namespace
					gpuByNamespace := map[string]GpuMetrics{}
					for _, gpuMetrics := range k8sUsage.gpuMetricsByPod {
						gpuNs := gpuMetrics.Namespace
						gpuMetrics, ok := gpuByNamespace[gpuNs]
						if !ok {
							gpuMetrics = GpuMetrics{}
						}
						gpuMetrics.GpuCpuUsage += gpuMetrics.GpuCpuUsage
						gpuMetrics.GpuMemUsage += gpuMetrics.GpuMemUsage
						gpuMetrics.GpuMemFree += gpuMetrics.GpuMemFree
						gpuMetrics.GpuCount += gpuMetrics.GpuCount
						gpuByNamespace[gpuNs] = gpuMetrics
					}

					//  Store GPU metrics in RRD databases
					for gpuNs, gpuMetrics := range gpuByNamespace {
						//  Calculate average GPU compute utilization across all GPUs in namespace
						gpuCount := gpuMetrics.GpuCount
						var gpuCpuUsage float64 = 0
						if gpuCount > 0 {
							gpuCpuUsage = gpuMetrics.GpuCpuUsage / float64(gpuCount)
						}

						//  Calculate GPU memory utilization percentage
						totalGpuMem := gpuMetrics.GpuMemUsage + gpuMetrics.GpuMemFree
						var gpuMemUsage float64 = 0
						if totalGpuMem > 0 {
							gpuMemUsage = gpuMetrics.GpuMemUsage / totalGpuMem
						}

						rrd, err = NewPartman(db, GpuMetricsDb(gpuNs), PERIOD_1_HOUR_SEC)
						if err != nil {
							KOA_LOGGER.Error(fmt.Sprintf("create rrd error: %s", err.Error()))
							continue
						}

						rrd.AddSample(ctx, nowEpoch, gpuCpuUsage, gpuMemUsage)

						KOA_LOGGER.Debug(fmt.Sprintf("[puller] GPU metrics for namespace %s: compute=%.2f%%, memory=%.2f%%", gpuNs, gpuCpuUsage, gpuMemUsage))
					}
				}
			}
		}()

		select {
		case <-parentCtx.Done():
			return
		case <-time.After(time.Duration(KOA_CONFIG.PollingIntervalSec) * time.Second):
		}
	}
}
