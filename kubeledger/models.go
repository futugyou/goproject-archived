package main

type Node struct {
	Id               string  `json:"id"`
	Name             string  `json:"name"`
	State            string  `json:"state"`
	Message          string  `json:"message"`
	CpuCapacity      float64 `json:"cpuCapacity"`
	CpuAllocatable   float64 `json:"cpuAllocatable"`
	CpuUsage         float64 `json:"cpuUsage"`
	CpuRequest       float64 `json:"cpuRequest"`
	CpuUsagePercent  float64 `json:"cpuUsagePercent"`
	MemUsagePercent  float64 `json:"memUsagePercent"`
	MemRequest       float64 `json:"memRequest"`
	MemCapacity      float64 `json:"memCapacity"`
	MemAllocatable   float64 `json:"memAllocatable"`
	MemUsage         float64 `json:"memUsage"`
	ContainerRuntime string  `json:"containerRuntime"`
	PodsRunning      []Pod   `json:"podsRunning"`
	PodsNotRunning   []Pod   `json:"podsNotRunning"`
	Region           string  `json:"region"`
	Os               string  `json:"os"`
	InstanceType     string  `json:"instanceType"`
	HourlyPrice      float64 `json:"hourlyPrice"`
	// GPU metrics aggregated from pods running on this node
	GpuCount       int     `json:"gpuCount"`
	GpuUsage       float64 `json:"gpuUsage"`    // GPU compute utilization percentage
	GpuMemUsage    float64 `json:"gpuMemUsage"` // GPU memory usage in bytes
	GpuMemFree     float64 `json:"gpuMemFree"`  // GPU memory free in bytes
	GpuCapacity    float64 `json:"gpuCapacity"`
	GpuAllocatable float64 `json:"gpuAllocatable"`
}

type Pod struct {
	Name       string  `json:"name"`
	Namespace  string  `json:"namespace"`
	Id         string  `json:"id"`
	NodeName   string  `json:"nodeName"`
	Phase      string  `json:"phase"`
	State      string  `json:"state"` // "PodNotScheduled"
	CpuUsage   float64 `json:"cpuUsage"`
	MemUsage   float64 `json:"memUsage"`
	CpuRequest float64 `json:"cpuRequest"`
	MemRequest float64 `json:"memRequest"`
	// GPU metrics (populated from gpuMetricsByPod)
	GpuUsage    float64 `json:"gpuUsage"`
	GpuMemUsage float64 `json:"gpuMemUsage"`
	GpuCount    int     `json:"gpuCount"`
}

// Data class to store NVIDIA GPU metrics collected from DCGM exporter.
// Metrics are aggregated per pod (identified by namespace + pod name).
// Multiple GPUs on the same pod will have their metrics summed.
type GpuMetrics struct {
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	// GPU compute utilization percentage (from DCGM_FI_DEV_GPU_UTIL)
	GpuCpuUsage float64 `json:"gpuCpuUsage"`
	// GPU memory bandwidth utilization percentage (from DCGM_FI_DEV_MEM_COPY_UTIL)
	GpuMemBandwidth float64 `json:"gpuMemBandwidth"`
	// GPU framebuffer memory used in MiB (from DCGM_FI_DEV_FB_USED)
	GpuMemUsage float64 `json:"gpuMemUsage"`
	// GPU framebuffer memory free in MiB (from DCGM_FI_DEV_FB_FREE)
	GpuMemFree float64 `json:"gpuMemFree"`
	// Number of GPUs contributing to this aggregated metric
	GpuCount int `json:"gpuCount"`
}

type ResourceCapacities struct {
	Cpu float64 `json:"cpu"`
	Mem float64 `json:"mem"`
}
