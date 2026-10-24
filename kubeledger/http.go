package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// Transform Prometheus metrics from DCGM Exporter to JSON.
func jsonifyDcgmMetrics(data string) map[string][]MetricData {
	metrics := make(map[string][]MetricData)
	lines := strings.SplitSeq(data, "\n")

	for line := range lines {
		line = strings.TrimSpace(line)
		// 过滤非 DCGM_ 开头的行（如注释或空行）
		if !strings.HasPrefix(line, "DCGM_") {
			continue
		}

		var name string
		var labelsStr string
		var valueStr string

		// 检查是否包含标签
		if idx := strings.Index(line, "{"); idx != -1 {
			name = line[:idx]
			endIdx := strings.Index(line, "}")
			if endIdx != -1 && endIdx > idx {
				labelsStr = line[idx+1 : endIdx]
				valueStr = strings.TrimSpace(line[endIdx+1:])
			}
		} else {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				name = parts[0]
			}
			if len(parts) > 1 {
				valueStr = parts[1]
			}
		}

		// 解析标签
		var labels MetricDataLabels
		if labelsStr != "" {
			pairs := strings.SplitSeq(labelsStr, ",")
			for pair := range pairs {
				kv := strings.SplitN(pair, "=", 2)
				if len(kv) == 2 {
					k := strings.TrimSpace(kv[0])
					v := strings.Trim(strings.TrimSpace(kv[1]), "\"")

					// 匹配对应的结构体字段（支持大小写兼容）
					switch strings.ToLower(k) {
					case "namespace":
						labels.Namespace = v
					case "pod":
						labels.Pod = v
					case "UUID":
						labels.Uuid = v
					}
				}
			}
		}

		// 转换数值
		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			value = 0.0 // 解析失败时的降级处理
		}

		// 组装数据
		metric := MetricData{
			Labels: labels,
			Value:  value,
		}

		metrics[name] = append(metrics[name], metric)
	}

	return metrics
}

// Fetch GPU metrics from NVIDIA DCGM exporter endpoint.
//
//	The DCGM exporter provides metrics in JSON format when queried at /metrics/json.
//	Returns the raw JSON data or None if the endpoint is not configured or on error.
func pullDcgmMetrics() map[string][]MetricData {
	if KOA_CONFIG.NvidiaDcgmEndpoint == "" {
		return nil
	}
	apiEndpoint := KOA_CONFIG.NvidiaDcgmEndpoint

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !KOA_CONFIG.KoaVerifySslOption.K8sVerifySsl,
		},
	}

	client := &http.Client{
		Transport: tr,
	}

	resp, err := client.Get(apiEndpoint)
	if err != nil {
		KOA_LOGGER.Error(fmt.Sprintf("DCGM endpoint %s returned error: %s", apiEndpoint, err.Error()))
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		KOA_LOGGER.Error(fmt.Sprintf("DCGM endpoint %s readed error: %s", apiEndpoint, err.Error()))
		return nil
	}

	if resp.StatusCode == http.StatusOK {
		return jsonifyDcgmMetrics(string(body))
	} else {
		KOA_LOGGER.Error(fmt.Sprintf("DCGM endpoint %s readed error: %s", apiEndpoint, string(body)))
		return nil
	}
}

func pullK8s(apiContext string) []byte {
	apiEndpoint := KOA_CONFIG.K8sApiEndpoint + apiContext

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !KOA_CONFIG.K8sVerifySsl,
		},
	}

	if fileExists(KOA_CONFIG.K8sSslClientCert) && fileExists(KOA_CONFIG.K8sSslClientCertKey) {
		cert, err := tls.LoadX509KeyPair(KOA_CONFIG.K8sSslClientCert, KOA_CONFIG.K8sSslClientCertKey)
		if err != nil {
			KOA_LOGGER.Error(fmt.Sprintf("failed to load client cert/key: %s", err.Error()))
			return nil
		}
		tr.TLSClientConfig.Certificates = []tls.Certificate{cert}
	}

	client := &http.Client{
		Transport: tr,
	}

	resp, err := client.Get(apiEndpoint)
	if err != nil {
		KOA_LOGGER.Error(fmt.Sprintf("calling HTTP endpoint error, %s (%vs)", apiEndpoint, err.Error()))
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		KOA_LOGGER.Error(fmt.Sprintf("reading HTTP response error, endpoint %s (%vs)", apiEndpoint, err.Error()))
		return nil
	}

	if resp.StatusCode == http.StatusOK {
		return (body)
	} else {
		KOA_LOGGER.Error(fmt.Sprintf("call to %s returned error status: %d, body: %s", apiEndpoint, resp.StatusCode, string(body)))
		return nil
	}
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
