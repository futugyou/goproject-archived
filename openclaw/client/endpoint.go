package client

import (
	"net/url"
	"strings"
)

func TryResolveHttpBaseUrl(serverUrl string) (string, bool) {
	parsedUrl, err := url.Parse(serverUrl)
	if err != nil || !parsedUrl.IsAbs() {
		return "", false
	}

	scheme := strings.ToLower(parsedUrl.Scheme)
	host := parsedUrl.Host

	switch scheme {
	case "ws":
		parsedUrl.Scheme = "http"
	case "wss":
		parsedUrl.Scheme = "https"
	case "http", "https":
		parsedUrl.Scheme = scheme
	default:
	}

	if parsedUrl.Scheme == "http" && strings.HasSuffix(host, ":80") {
		parsedUrl.Host = strings.TrimSuffix(host, ":80")
	} else if parsedUrl.Scheme == "https" && strings.HasSuffix(host, ":443") {
		parsedUrl.Host = strings.TrimSuffix(host, ":443")
	}

	// 处理 Path 路径
	path := parsedUrl.Path
	if strings.HasSuffix(strings.ToLower(path), "/ws") {
		path = path[:len(path)-3]
	}

	if strings.TrimSpace(path) == "" {
		path = "/"
	}

	// 清空 Query 和 Fragment
	parsedUrl.RawQuery = ""
	parsedUrl.Fragment = ""
	parsedUrl.Path = path

	// 拼接 Base URL (移除结尾多余的 '/')
	baseUrl := parsedUrl.Scheme + "://" + parsedUrl.Host + strings.TrimRight(parsedUrl.Path, "/")
	return baseUrl, true
}
