package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"text/template"
	"time"
)

type PageData struct {
	KoaFrontendDataLocation string
	KoaVersion              string
	KoaDcgmEnabled          bool
}

func renderHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := PageData{
		KoaFrontendDataLocation: KOA_CONFIG.FrontendDataLocation,
		KoaVersion:              KOA_CONFIG.Version,
		KoaDcgmEnabled:          KOA_CONFIG.NvidiaDcgmEndpoint != "",
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("Error rendering template: %v", err)
	}
}

func getNodeHeatmapHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	nodesFile := filepath.Join(KOA_CONFIG.FrontendDataLocation, "nodes.json")
	if _, err := os.Stat(nodesFile); os.IsNotExist(err) {
		json.NewEncoder(w).Encode(map[string]any{
			"error": "Nodes data not available",
			"nodes": []any{},
		})
		return
	}

	fileBytes, err := os.ReadFile(nodesFile)
	if err != nil {
		log.Printf("Error reading nodes file: %v", err)
		json.NewEncoder(w).Encode(map[string]any{
			"error": "Failed to generate heatmap data",
			"nodes": []any{},
		})
		return
	}

	var nodesData map[string]map[string]any
	if err := json.Unmarshal(fileBytes, &nodesData); err != nil {
		log.Printf("Error parsing nodes json: %v", err)
		json.NewEncoder(w).Encode(map[string]any{
			"error": "Failed to generate heatmap data",
			"nodes": []any{},
		})
		return
	}

	var heatmapData []map[string]any

	for nodeName, nodeInfo := range nodesData {
		getFloat := func(key string) float64 {
			if val, ok := nodeInfo[key]; ok {
				switch v := val.(type) {
				case float64:
					return v
				case int:
					return float64(v)
				}
			}
			return 0
		}

		cpuAllocatable := getFloat("cpuAllocatable")
		cpuUsage := getFloat("cpuUsage")
		cpuUsagePct := 0.0
		if cpuAllocatable > 0 {
			cpuUsagePct = (cpuUsage / cpuAllocatable) * 100
		}

		memAllocatable := getFloat("memAllocatable")
		memUsage := getFloat("memUsage")
		memoryUsagePct := 0.0
		if memAllocatable > 0 {
			memoryUsagePct = (memUsage / memAllocatable) * 100
		}

		cpuCapacity := getFloat("cpuCapacity")
		if cpuCapacity == 0 {
			cpuCapacity = 1
		}
		baseSize := 50.0
		sizeMultiplier := max(1.0, cpuCapacity/2)
		rectSize := min(baseSize*sizeMultiplier, 200.0)

		gpuUsagePct := getFloat("gpuUsage")
		gpuCount := getFloat("gpuCount")
		gpuMemUsage := getFloat("gpuMemUsage")
		gpuMemFree := getFloat("gpuMemFree")
		gpuMemTotal := gpuMemUsage + gpuMemFree
		gpuMemUsagePct := 0.0
		if gpuMemTotal > 0 {
			gpuMemUsagePct = (gpuMemUsage / gpuMemTotal) * 100
		}

		state := "Unknown"
		if s, ok := nodeInfo["state"].(string); ok {
			state = s
		}

		podsRunningCount := 0
		if pods, ok := nodeInfo["podsRunning"].([]any); ok {
			podsRunningCount = len(pods)
		}

		nodeHeatmapInfo := map[string]any{
			"name":                   nodeName,
			"cpuUsagePercent":        round(cpuUsagePct, 2),
			"memoryUsagePercent":     round(memoryUsagePct, 2),
			"gpuComputeUsagePercent": round(gpuUsagePct, 2),
			"gpuMemoryUsagePercent":  round(gpuMemUsagePct, 2),
			"cpuCapacity":            nodeInfo["cpuCapacity"],
			"memoryCapacity":         nodeInfo["memCapacity"],
			"cpuAllocatable":         cpuAllocatable,
			"memoryAllocatable":      memAllocatable,
			"cpuUsage":               cpuUsage,
			"memoryUsage":            memUsage,
			"gpuCount":               gpuCount,
			"gpuComputeUsage":        gpuUsagePct,
			"gpuMemoryUsage":         gpuMemUsage,
			"gpuMemoryFree":          gpuMemFree,
			"gpuMemoryTotal":         gpuMemTotal,
			"state":                  state,
			"podsRunning":            podsRunningCount,
			"rectSize":               round(rectSize, 0),
		}
		heatmapData = append(heatmapData, nodeHeatmapInfo)
	}

	response := map[string]any{
		"nodes":       heatmapData,
		"timestamp":   time.Now().Unix(),
		"total_nodes": len(heatmapData),
	}

	json.NewEncoder(w).Encode(response)
}
