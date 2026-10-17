package main

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
