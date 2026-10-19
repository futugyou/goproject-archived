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
	Conditions  []NodeCondition   `json:"conditions"`
}

type NodeInfo struct {
	ContainerRuntimeVersion string `json:"containerRuntimeVersion"`
}

type NodeCondition struct {
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
