package main

type Node struct {
	Id               string  `json:"id"`
	Name             string  `json:"name"`
	State            string  `json:"state"`
	Message          string  `json:"message"`
	CpuCapacity      float32 `json:"cpuCapacity"`
	CpuAllocatable   float32 `json:"cpuAllocatable"`
	CpuUsage         float32 `json:"cpuUsage"`
	CpuRequest       float32 `json:"cpuRequest"`
	CpuUsagePercent  float32 `json:"cpuUsagePercent"`
	MemUsagePercent  float32 `json:"memUsagePercent"`
	MemRequest       float32 `json:"memRequest"`
	MemCapacity      float32 `json:"memCapacity"`
	MemAllocatable   float32 `json:"memAllocatable"`
	MemUsage         float32 `json:"memUsage"`
	ContainerRuntime string  `json:"containerRuntime"`
	PodsRunning      []Pod   `json:"podsRunning"`
	PodsNotRunning   []Pod   `json:"podsNotRunning"`
	Region           string  `json:"region"`
	Os               string  `json:"os"`
	InstanceType     string  `json:"instanceType"`
	HourlyPrice      float32 `json:"hourlyPrice"`
	// GPU metrics aggregated from pods running on this node
	GpuCount       int     `json:"gpuCount"`
	GpuUsage       float32 `json:"gpuUsage"`    // GPU compute utilization percentage
	GpuMemUsage    float32 `json:"gpuMemUsage"` // GPU memory usage in bytes
	GpuMemFree     float32 `json:"gpuMemFree"`  // GPU memory free in bytes
	GpuCapacity    float32 `json:"gpuCapacity"`
	GpuAllocatable float32 `json:"gpuAllocatable"`
}

type Pod struct {
	Name       string  `json:"name"`
	Namespace  string  `json:"namespace"`
	Id         string  `json:"id"`
	NodeName   string  `json:"nodeName"`
	Phase      string  `json:"phase"`
	State      string  `json:"state"` // "PodNotScheduled"
	CpuUsage   float32 `json:"cpuUsage"`
	MemUsage   float32 `json:"memUsage"`
	CpuRequest float32 `json:"cpuRequest"`
	MemRequest float32 `json:"memRequest"`
	// GPU metrics (populated from gpuMetricsByPod)
	GpuUsage    float32 `json:"gpuUsage"`
	GpuMemUsage float32 `json:"gpuMemUsage"`
	GpuCount    int     `json:"gpuCount"`
}

// Data class to store NVIDIA GPU metrics collected from DCGM exporter.
// Metrics are aggregated per pod (identified by namespace + pod name).
// Multiple GPUs on the same pod will have their metrics summed.
type GpuMetrics struct {
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	// GPU compute utilization percentage (from DCGM_FI_DEV_GPU_UTIL)
	GpuCpuUsage float32 `json:"gpuCpuUsage"`
	// GPU memory bandwidth utilization percentage (from DCGM_FI_DEV_MEM_COPY_UTIL)
	GpuMemBandwidth float32 `json:"gpuMemBandwidth"`
	// GPU framebuffer memory used in MiB (from DCGM_FI_DEV_FB_USED)
	GpuMemUsage float32 `json:"gpuMemUsage"`
	// GPU framebuffer memory free in MiB (from DCGM_FI_DEV_FB_FREE)
	GpuMemFree float32 `json:"gpuMemFree"`
	// Number of GPUs contributing to this aggregated metric
	GpuCount int `json:"gpuCount"`
}

type ResourceCapacities struct {
	Cpu float32 `json:"cpu"`
	Mem float32 `json:"mem"`
}
