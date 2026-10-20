package main

type Node struct {
	Id               string
	Name             string
	State            string
	Message          string
	CpuCapacity      float32
	CpuAllocatable   float32
	CpuUsage         float32
	CpuRequest       float32
	CpuUsagePercent  float32
	MemUsagePercent  float32
	MemRequest       float32
	MemCapacity      float32
	MemAllocatable   float32
	MemUsage         float32
	ContainerRuntime string
	PodsRunning      []Pod
	PodsNotRunning   []Pod
	Region           string
	Os               string
	InstanceType     string
	HourlyPrice      float32
	// GPU metrics aggregated from pods running on this node
	GpuCount       int
	GpuUsage       float32 // GPU compute utilization percentage
	GpuMemUsage    float32 // GPU memory usage in bytes
	GpuMemFree     float32 // GPU memory free in bytes
	GpuCapacity    float32
	GpuAllocatable float32
}

type Pod struct {
	Name       string
	Namespace  string
	Id         string
	NodeName   string
	Phase      string
	State      string // "PodNotScheduled"
	CpuUsage   float32
	MemUsage   float32
	CpuRequest float32
	MemRequest float32
	// GPU metrics (populated from gpuMetricsByPod)
	GpuUsage    float32
	GpuMemUsage float32
	GpuCount    int
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
	Cpu float32
	Mem float32
}
