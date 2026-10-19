package main

type NodeListResponse struct {
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
