package core

import (
	"net"
	"net/http"
	"time"
)

const defaultIdleConnTimeout = 2 * time.Minute

func NewClient(allowAutoRedirect bool, idleConnTimeout *time.Duration) *http.Client {
	timeout := defaultIdleConnTimeout
	if idleConnTimeout != nil {
		timeout = *idleConnTimeout
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,

		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,     // 最大空闲连接数
		MaxIdleConnsPerHost:   100,     // 每个 Host 的最大空闲连接数（高并发时非常重要！）
		IdleConnTimeout:       timeout, // 核心：连接空闲多久后销毁重连（防止DNS过时）
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second, // 单次请求的整体超时时间（包含读取 Body）
	}

	// 2. 处理是否允许自动重定向
	if !allowAutoRedirect {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			// 返回这个特殊的错误，Go 就会停止重定向，并返回当前（最后一次）的响应
			return http.ErrUseLastResponse
		}
	}

	return client
}
