package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
)

var capacityQuantities = map[string]float32{
	"Ki":   1024,
	"Mi":   1048576,
	"Gi":   1073741824,
	"Ti":   1099511627776,
	"Pi":   1125899906842624,
	"Ei":   1152921504606847000,
	"k":    1e3,
	"K":    1e3,
	"M":    1e6,
	"G":    1e9,
	"T":    1e12,
	"P":    1e15,
	"E":    1e18,
	"m":    1e-3,
	"u":    1e-6,
	"n":    1e-9,
	"None": 1,
}

type K8sUsage struct {
	nodes              map[string]Node
	pods               map[string]Pod
	usageByNamespace   map[string]ResourceCapacities
	requestByNamespace map[string]ResourceCapacities
	popupContent       string
	nodeHtmlList       string
	cpuUsageAllPods    float32
	memUsageAllPods    float32
	cpuCapacity        float32
	memCapacity        float32
	cpuAllocatable     float32
	memAllocatable     float32
	gpuMetricsByPod    map[string]GpuMetrics
}

func hasSuffixAny(s string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(s, suffix) {
			return true
		}
	}
	return false
}

func (k *K8sUsage) DecodeCapacity(cap_input string) float32 {
	if cap_input == "" {
		cap_input = "0"
	}

	data_length := len(cap_input)
	cap_unit := "None"
	cap_value := cap_input
	suffixes := []string{"Ki", "Mi", "Gi", "Ti", "Pi", "Ei"}
	suffixes2 := []string{"n", "u", "m", "k", "K", "M", "G", "T", "P", "E"}

	if hasSuffixAny(cap_input, suffixes) {
		cap_unit = cap_input[data_length-2:]
		cap_value = cap_input[0 : data_length-2]
	} else if hasSuffixAny(cap_input, suffixes2) {
		cap_unit = cap_input[data_length-1:]
		cap_value = cap_input[0 : data_length-1]
	}

	cap_val, _ := strconv.ParseFloat(cap_value, 32)
	KOA_LOGGER.Debug(cap_value)
	return capacityQuantities[cap_unit] * float32(cap_val)
}

func (k *K8sUsage) ExtractNamespacesAndInitializeUsage(data []byte) {
	// exit if not valid data
	if len(data) == 0 {
		return
	}

	var jsondata NamespacesResponse
	if err := json.Unmarshal(data, &jsondata); err != nil {
		return
	}

	for _, item := range jsondata.Items {
		name := item.Metadata.Name
		if !KOA_CONFIG.AllowNamespace(name) {
			continue
		}

		k.usageByNamespace[name] = ResourceCapacities{}
		k.requestByNamespace[name] = ResourceCapacities{}
	}

	KOA_LOGGER.Debug(fmt.Sprintf("Found namespaces: %s", strings.Join(slices.Collect(maps.Keys(k.usageByNamespace)), ",")))
}

func determineNodeState(conditions []Condition) (state, message string) {
	conditionMap := make(map[string]Condition, len(conditions))
	for _, cond := range conditions {
		conditionMap[cond.Type] = cond
	}

	ready, exists := conditionMap["Ready"]
	if !exists {
		return "Unknown", "Ready condition is missing"
	}

	switch ready.Status {
	case "False":
		return "NotReady", ready.Message
	case "Unknown":
		return "Unknown", ready.Message
	case "True":
		// Ready=True，继续检查其他健康条件。
	default:
		return "Unknown", ready.Message
	}

	// 按优先级检查异常条件。
	for _, conditionType := range []string{
		"KernelDeadlock",
		"NetworkUnavailable",
		"MemoryPressure",
		"DiskPressure",
		"PIDPressure",
		"OutOfDisk",
	} {
		cond, exists := conditionMap[conditionType]
		if !exists {
			continue
		}

		switch cond.Status {
		case "True":
			return conditionType, cond.Message
		case "Unknown":
			return "Unknown", cond.Message
		}
	}

	return "Ready", ready.Message
}

func (k *K8sUsage) ExtractNodes(data []byte) {
	//exit if not valid data
	if len(data) == 0 {
		return
	}

	// process likely valid data
	var jsondata NodesResponse
	if err := json.Unmarshal(data, &jsondata); err != nil {
		return
	}

	for _, item := range jsondata.Items {
		node := Node{}
		node.PodsRunning = []Pod{}
		node.PodsNotRunning = []Pod{}

		metadata := item.Metadata
		if metadata != nil {
			node.Id = metadata.UID
			node.Name = metadata.Name
		}

		status := item.Status
		if status != nil {
			node.ContainerRuntime = status.NodeInfo.ContainerRuntimeVersion
			node.CpuCapacity = k.DecodeCapacity(status.Capacity["cpu"])
			node.CpuAllocatable = k.DecodeCapacity(status.Allocatable["cpu"])
			node.MemCapacity = k.DecodeCapacity(status.Capacity["memory"])
			node.MemAllocatable = k.DecodeCapacity(status.Allocatable["memory"])
			node.GpuCapacity = k.DecodeCapacity(status.Capacity["nvidia.com/gpu"])
			node.GpuAllocatable = k.DecodeCapacity(status.Allocatable["nvidia.com/gpu"])

			state, message := determineNodeState(status.Conditions)
			node.State = state
			node.Message = message
		}

		k.nodes[node.Name] = node
	}
}

func (k *K8sUsage) ExtractNodeMetrics(data []byte) {
	//exit if not valid data
	if len(data) == 0 {
		return
	}

	// process likely valid data
	var jsondata MetricsNodesResponse
	if err := json.Unmarshal(data, &jsondata); err != nil {
		return
	}

	for _, item := range jsondata.Items {
		node, ok := k.nodes[item.Metadata.Name]
		if !ok {
			continue
		}

		node.CpuUsage = k.DecodeCapacity(item.Usage.Cpu)
		node.MemUsage = k.DecodeCapacity(item.Usage.Memory)

		k.nodes[node.Name] = node
	}
}

func (k *K8sUsage) ExtractPods(data []byte) {
	//exit if not valid data
	if len(data) == 0 {
		return
	}

	// process likely valid data
	var jsondata PodsResponse
	if err := json.Unmarshal(data, &jsondata); err != nil {
		return
	}

	for _, item := range jsondata.Items {
		if !KOA_CONFIG.AllowNamespace(item.Metadata.Namespace) {
			continue
		}
		pod := Pod{}
		pod.Namespace = item.Metadata.Namespace
		pod.Name = fmt.Sprintf("%s.%s", item.Metadata.Name, pod.Namespace)
		pod.Id = item.Metadata.Uid
		pod.Phase = item.Status.Phase
		pod.State = "PodNotScheduled"

		if len(item.Status.Conditions) == 0 {
			KOA_LOGGER.Debug(fmt.Sprintf("[puller] phase of pod %s in namespace %s is %s", pod.Name, pod.Namespace, pod.Phase))
		}

		for _, cond := range item.Status.Conditions {
			if cond.Type == "Ready" && cond.Status == "True" {
				pod.State = "Ready"
				break
			}
			if cond.Type == "ContainersReady" && cond.Status == "True" {
				pod.State = "ContainersReady"
				break
			}
			if cond.Type == "PodScheduled" && cond.Status == "True" {
				pod.State = "PodScheduled"
				break
			}
			if cond.Type == "Initialized" && cond.Status == "True" {
				pod.State = "Initialized"
				break
			}
		}

		if pod.State != "PodNotScheduled" {
			pod.NodeName = item.Spec.NodeName
			for _, cond := range item.Spec.Containers {
				if cond.Resources == nil || cond.Resources.Request == nil {
					continue
				}

				pod.CpuRequest += k.DecodeCapacity(cond.Resources.Request.Cpu)
				pod.MemRequest += k.DecodeCapacity(cond.Resources.Request.Memory)
			}
		}

		k.pods[pod.Name] = pod
	}
}

func (k *K8sUsage) ExtractPodMetrics(data []byte) {
	//exit if not valid data
	if len(data) == 0 {
		return
	}

	// process likely valid data
	var jsondata MetricsPodsResponse
	if err := json.Unmarshal(data, &jsondata); err != nil {
		return
	}

	for _, item := range jsondata.Items {
		if !KOA_CONFIG.AllowNamespace(item.Metadata.Namespace) {
			continue
		}

		pod_name := fmt.Sprintf("%s.%s", item.Metadata.Name, item.Metadata.Namespace)

		pod, ok := k.pods[pod_name]
		if !ok {
			continue
		}

		pod.CpuUsage = 0.0
		pod.MemUsage = 0.0

		for _, cond := range item.Containers {
			pod.CpuRequest += k.DecodeCapacity(cond.Usage.Cpu)
			pod.MemRequest += k.DecodeCapacity(cond.Usage.Memory)
		}

		k.pods[pod_name] = pod
	}
}

var metric_mapping = map[string]string{
	"DCGM_FI_DEV_GPU_UTIL":      "gpuCpuUsage",
	"DCGM_FI_DEV_MEM_COPY_UTIL": "gpuMemBandwidth",
	"DCGM_FI_DEV_FB_USED":       "gpuMemUsage",
	"DCGM_FI_DEV_FB_FREE":       "gpuMemFree",
}

// Extract GPU metrics from DCGM exporter JSON data.
//
// Processes the following DCGM metrics:
//   - DCGM_FI_DEV_GPU_UTIL: GPU compute utilization percentage -> gpuCpuUsage
//   - DCGM_FI_DEV_MEM_COPY_UTIL: Memory bandwidth utilization percentage -> gpuMemBandwidth
//   - DCGM_FI_DEV_FB_USED: Framebuffer memory used (MiB) -> gpuMemUsage
//   - DCGM_FI_DEV_FB_FREE: Framebuffer memory free (MiB) -> gpuMemFree
//
// Metrics are aggregated per pod using namespace and pod labels.
//
// Multiple GPUs on the same pod will have their metrics summed.
//
// :param data: Raw JSON string from DCGM exporter endpoint
func (k *K8sUsage) ExtractGpuMetrics(data []byte) {
	//exit if not valid data
	if len(data) == 0 {
		return
	}

	// process likely valid data
	var jsondata map[string][]MetricData
	if err := json.Unmarshal(data, &jsondata); err != nil {
		return
	}

	gpus_per_pod := make(map[string]map[string]struct{})
	for metric_name := range metric_mapping {
		metric_data, ok := jsondata[metric_name]
		if !ok {
			continue
		}

		for _, entry := range metric_data {
			namespace := entry.Labels.Namespace
			pod := entry.Labels.Pod
			gpu_uuid := entry.Labels.Uuid
			value := entry.Value

			if namespace == "" || pod == "" {
				KOA_LOGGER.Debug(fmt.Sprintf("Skipping DCGM metric %s entry without namespace/pod labels", metric_name))
				continue
			}

			if !KOA_CONFIG.AllowNamespace(namespace) {
				continue
			}

			pod_key := fmt.Sprintf("%s.%s", pod, namespace)

			if _, ok := k.gpuMetricsByPod[pod_key]; !ok {
				gpu_metrics := GpuMetrics{}
				gpu_metrics.Namespace = namespace
				gpu_metrics.Pod = pod
				k.gpuMetricsByPod[pod_key] = gpu_metrics
				gpus_per_pod[pod_key] = make(map[string]struct{})
			}

			if _, ok := gpus_per_pod[pod_key]; ok && gpu_uuid != "" {
				gpus_per_pod[pod_key][gpu_uuid] = struct{}{}
			}

			gpu_metrics := k.gpuMetricsByPod[pod_key]

			switch metric_name {
			case "DCGM_FI_DEV_GPU_UTIL":
				gpu_metrics.GpuCpuUsage += value
			case "DCGM_FI_DEV_MEM_COPY_UTIL":
				gpu_metrics.GpuMemBandwidth += value
			case "DCGM_FI_DEV_FB_USED":
				gpu_metrics.GpuMemUsage += value
			case "DCGM_FI_DEV_FB_FREE":
				gpu_metrics.GpuMemFree += value
			}
			k.gpuMetricsByPod[pod_key] = gpu_metrics
		}
	}

	for pod_key, gpu_uuids := range gpus_per_pod {
		if m, ok := k.gpuMetricsByPod[pod_key]; ok {
			m.GpuCount = len(gpu_uuids)
			k.gpuMetricsByPod[pod_key] = m
		}
	}

	KOA_LOGGER.Debug(fmt.Sprintf("[puller] Extracted GPU metrics for %d pods", len(k.gpuMetricsByPod)))
}

// Dump GPU metrics to a JSON file for frontend consumption.
func (k *K8sUsage) DumpGpuMetrics() error {
	if len(k.gpuMetricsByPod) == 0 {
		return nil
	}

	data, err := json.Marshal(k.gpuMetricsByPod)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("%s/gpu_metrics.json", KOA_CONFIG.FrontendDataLocation)

	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
