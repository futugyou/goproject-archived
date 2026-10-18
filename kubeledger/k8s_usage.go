package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

type ResourceCapacities struct {
	Cpu float32
	Mem float32
}

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

type InitUsageDTO struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
	} `json:"items"`
}

type InitUsageItem struct {
}

func (k *K8sUsage) ExtractNamespacesAndInitializeUsage(data string) {
	// exit if not valid data
	data = strings.TrimSpace(data)
	if data == "" {
		return
	}

	var jsondata InitUsageDTO
	if err := json.Unmarshal([]byte(data), &jsondata); err != nil {
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
