package homeassistant

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type HomeAssistantRestClient struct {
	config     core.HomeAssistantConfig
	httpClient *http.Client
	baseUri    string
	token      string
}

func NewHomeAssistantRestClient(config core.HomeAssistantConfig, httpClient *http.Client) *HomeAssistantRestClient {
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	token := core.SecretResolverInstance.Resolve(config.TokenRef)

	return &HomeAssistantRestClient{
		config:     config,
		httpClient: httpClient,
		token:      token,
		baseUri:    strings.TrimRight(config.BaseURL, "/"),
	}
}

func (h *HomeAssistantRestClient) readBodyLimited(_ context.Context, resp *http.Response) (string, error) {
	var maxChars = max(1_000, h.config.MaxOutputChars)
	text, truncated, err := util.ReadLimitedClean(resp.Body, int64(maxChars))
	if err != nil {
		return "", err
	}

	if truncated {
		text += "…"
	}
	return text, nil
}

func (h *HomeAssistantRestClient) getStringLimited(parentCtx context.Context, url string) (string, error) {
	ctx, cancel := context.WithTimeout(parentCtx, time.Duration(max(1, h.config.TimeoutSeconds))*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+h.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		text, err := h.readBodyLimited(ctx, resp)
		if err != nil {
			text = err.Error()
		}
		return "", fmt.Errorf("Home Assistant request failed: %d\n%s", resp.StatusCode, text)
	}

	return h.readBodyLimited(ctx, resp)
}

func (h *HomeAssistantRestClient) CallService(parentCtx context.Context, domain, service, bodyJson string) (string, error) {
	ctx, cancel := context.WithTimeout(parentCtx, time.Duration(max(1, h.config.TimeoutSeconds))*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/api/services/%s/%s", h.baseUri, url.QueryEscape(domain), url.QueryEscape(service))
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer([]byte(bodyJson)))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+h.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return "", errors.New("Home Assistant authorization failed (401/403). Check your token.")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		text, err := h.readBodyLimited(ctx, resp)
		if err != nil {
			text = err.Error()
		}
		return "", fmt.Errorf("Home Assistant request failed: %d\n%s", resp.StatusCode, text)
	}

	return h.readBodyLimited(ctx, resp)
}

func (h *HomeAssistantRestClient) GetStates(ctx context.Context) (string, error) {
	return h.getStringLimited(ctx, fmt.Sprintf("%s/api/states", h.baseUri))
}

func (h *HomeAssistantRestClient) GetState(ctx context.Context, entityId string) (string, error) {
	return h.getStringLimited(ctx, fmt.Sprintf("%s/api/states/%s", h.baseUri, url.QueryEscape(entityId)))
}

func (h *HomeAssistantRestClient) GetServices(ctx context.Context) (string, error) {
	return h.getStringLimited(ctx, fmt.Sprintf("%s/api/services", h.baseUri))
}
