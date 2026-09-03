package util

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

func GetRetryAfter(resp *http.Response, defaultDuration time.Duration) time.Duration {
	if resp == nil {
		return defaultDuration
	}

	retryHeader := resp.Header.Get("Retry-After")
	if retryHeader == "" {
		return defaultDuration
	}

	// 1. 尝试按秒数解析 (如 "120")
	if seconds, err := strconv.Atoi(retryHeader); err == nil {
		return time.Duration(seconds) * time.Second
	}

	// 2. 尝试按 HTTP 日期格式 (RFC1123) 解析 (如 "Fri, 31 Dec 2026 23:59:59 GMT")
	if targetTime, err := http.ParseTime(retryHeader); err == nil {
		duration := time.Until(targetTime)
		if duration > 0 {
			return duration
		}
		// 如果指定的时间已经过去了，返回 0 或者默认值
		return 0
	}

	// 3. 解析失败，使用默认值
	return defaultDuration
}

func CreateBasicAuth(accountSid, authToken string) string {
	raw := fmt.Sprintf("%s:%s", accountSid, authToken)
	b64 := base64.StdEncoding.EncodeToString([]byte(raw))
	return fmt.Sprintf("Basic %s", b64)
}
