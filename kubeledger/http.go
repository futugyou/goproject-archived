package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Fetch GPU metrics from NVIDIA DCGM exporter endpoint.
//
//	The DCGM exporter provides metrics in JSON format when queried at /metrics/json.
//	Returns the raw JSON data or None if the endpoint is not configured or on error.
func PullDcgmMetrics() []byte {
	data := []byte{}

	if KOA_CONFIG.NvidiaDcgmEndpoint == "" {
		return data
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
		return data
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		KOA_LOGGER.Error(fmt.Sprintf("DCGM endpoint %s readed error: %s", apiEndpoint, err.Error()))
		return data
	}

	if resp.StatusCode == http.StatusOK {
		return body
	} else {
		KOA_LOGGER.Error(fmt.Sprintf("DCGM endpoint %s readed error: %s", apiEndpoint, string(body)))
		return data
	}
}

func PullK8s(apiContext string) []byte {
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
