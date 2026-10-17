package main

type Node struct {
	Id               string
	Name             string
	State            string
	Message          string
	CpuCapacity      float32
	CpuAllocatable   float32
	CpuUsage         float32
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
	GpuCount    int
	GpuUsage    float32 // GPU compute utilization percentage
	GpuMemUsage float32 // GPU memory usage in bytes
	GpuMemFree  float32 // GPU memory free in bytes
}
