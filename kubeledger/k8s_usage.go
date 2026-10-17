package main

type ResourceCapacities struct {
	Cpu float32
	Mem float32
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
}
