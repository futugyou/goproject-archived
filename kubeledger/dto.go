package main

type NamespacesResponse struct {
	Items []NamespaceItem `json:"items"`
}

type NamespaceItem struct {
	Metadata *NamespaceMetadata `json:"metadata"`
}

type NamespaceMetadata struct {
	Name string `json:"name"`
}

type NodesResponse struct {
	Items []NodeItem `json:"items"`
}

type NodeItem struct {
	Metadata *NodeMetadata `json:"metadata"`
	Status   *NodeStatus   `json:"status"`
}

type NodeMetadata struct {
	UID  string `json:"uid"`
	Name string `json:"name"`
}

type NodeStatus struct {
	NodeInfo    NodeInfo          `json:"nodeInfo"`
	Capacity    map[string]string `json:"capacity"`
	Allocatable map[string]string `json:"allocatable"`
	Conditions  []Condition       `json:"conditions"`
}

type NodeInfo struct {
	ContainerRuntimeVersion string `json:"containerRuntimeVersion"`
}

type Condition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type MetricsNodesResponse struct {
	Items []MetricsNodeItem `json:"items"`
}

type MetricsNodeItem struct {
	Metadata MetricsNodeMetadata `json:"metadata"`
	Usage    *MetricsNodeUsage   `json:"usage"`
}

type MetricsNodeMetadata struct {
	Name string `json:"name"`
}

type MetricsNodeUsage struct {
	Cpu    string `json:"cpu"`
	Memory string `json:"memory"`
}

type PodsResponse struct {
	Items []PodItem `json:"items"`
}

type PodItem struct {
	Metadata PodMetadata `json:"metadata"`
	Status   PodStatus   `json:"status"`
	Spec     PodSpec     `json:"spec"`
}

type PodSpec struct {
	NodeName   string         `json:"nodeName"`
	Containers []PodContainer `json:"containers"`
}

type PodContainer struct {
	Resources *PodResource `json:"resources"`
}

type PodResource struct {
	Request *PodResourceSpec `json:"requests"`
}

type PodResourceSpec struct {
	Cpu    string `json:"cpu"`
	Memory string `json:"memory"`
}

type PodStatus struct {
	Conditions []Condition `json:"conditions"`
	Phase      string      `json:"phase"`
}

type PodMetadata struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Uid       string `json:"uid"`
}

type MetricsPodsResponse struct {
	Items []MetricsPodItem `json:"items"`
}

type MetricsPodItem struct {
	Metadata   MetricsPodMetadata    `json:"metadata"`
	Containers []MetricsPodContainer `json:"containers"`
}

type MetricsPodMetadata struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type MetricsPodContainer struct {
	Usage MetricsPodUsage `json:"usage"`
}

type MetricsPodUsage struct {
	Cpu    string `json:"cpu"`
	Memory string `json:"memory"`
}

type MetricData struct {
	Labels MetricDataLabels `json:"labels"`
	Value  float64          `json:"value"`
}

type MetricDataLabels struct {
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Uuid      string `json:"UUID"`
}
