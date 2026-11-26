package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/futugyou/openclaw/core"
)

type TeamsConversationReference struct {
	ServiceUrl       string
	ConversationId   string
	UserId           string
	TenantId         string
	ConversationType string
}

var _ core.IChannelAdapter = (*TeamsChannel)(nil)

type TeamsChannel struct {
	config                 core.TeamsChannelConfig
	httpClient             *http.Client
	logger                 *slog.Logger
	appId                  string
	appPassword            string
	tenantId               string
	cachedToken            string
	tokenExpiry            time.Time
	conversationReferences map[string]TeamsConversationReference
	onMessageReceived      core.ChannelMessageHandler

	cancelFunc context.CancelFunc
	mu         sync.Mutex
}

func (t *TeamsChannel) ChannelId() string {
	return "teams"
}

func (t *TeamsChannel) ChannelType() string {
	return "teams"
}

func (t *TeamsChannel) Close(ctx context.Context) error {
	if t.cancelFunc != nil {
		t.cancelFunc()
	}

	return nil
}

func (t *TeamsChannel) GetMessageReceivedHandler() core.ChannelMessageHandler {
	return t.onMessageReceived
}

func (t *TeamsChannel) SetMessageReceivedHandler(handler core.ChannelMessageHandler) {
	t.onMessageReceived = handler
}

func NewTeamsChannel(config core.TeamsChannelConfig, httpClient *http.Client, logger *slog.Logger) (*TeamsChannel, error) {
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	if logger == nil {
		logger = slog.Default()
	}

	appId := core.SecretResolverInstance.Resolve(config.AppIdRef)
	if appId == "" {
		appId = config.AppId
	}
	if appId == "" {
		return nil, errors.New("Teams App ID not configured. Set Channels.Teams.AppId or env:TEAMS_APP_ID.")
	}

	appPassword := core.SecretResolverInstance.Resolve(config.AppPasswordRef)
	if appPassword == "" {
		appPassword = config.AppPassword
	}
	if appPassword == "" {
		return nil, errors.New("Teams App Password not configured. Set Channels.Teams.AppPassword or env:TEAMS_APP_PASSWORD.")
	}

	tenantId := core.SecretResolverInstance.Resolve(config.TenantIdRef)
	if tenantId == "" {
		tenantId = config.TenantId
	}
	if tenantId == "" {
		return nil, errors.New("Teams Tenant ID not configured. Set Channels.Teams.TenantId or env:TEAMS_TENANT_ID.")
	}

	return &TeamsChannel{
		config:                 config,
		logger:                 logger,
		httpClient:             httpClient,
		appId:                  appId,
		tenantId:               tenantId,
		appPassword:            appPassword,
		conversationReferences: map[string]TeamsConversationReference{},
	}, nil
}

func (t *TeamsChannel) Start(ctx context.Context) error {
	return nil
}

func (t *TeamsChannel) StoreConversationReference(senderId string, reference TeamsConversationReference) {
	t.conversationReferences[senderId] = reference
	// Also store by conversation ID for group chat routing
	if !strings.EqualFold(senderId, reference.ConversationId) {
		t.conversationReferences[reference.ConversationId] = reference
	}
}

func (t *TeamsChannel) RaiseInboundAsync(ctx context.Context, message *core.InboundMessage) error {
	handler := t.onMessageReceived
	if handler != nil {
		return handler(ctx, message)
	}

	return nil
}

func chunkText(text string, limit int, mode string) []string {
	if limit <= 0 {
		return []string{text}
	}

	runes := []rune(text)
	if len(runes) <= limit {
		return []string{text}
	}

	var chunks []string

	if mode == "newline" {
		var current strings.Builder
		currentLen := 0

		for _, line := range strings.Split(text, "\n") {
			lineLen := len([]rune(line))
			addLen := lineLen
			if currentLen > 0 {
				addLen += 1
			}
			if currentLen+addLen > limit && currentLen > 0 {
				chunks = append(chunks, current.String())
				current.Reset()
				currentLen = 0
			}

			if currentLen > 0 {
				current.WriteByte('\n')
				currentLen++
			}
			current.WriteString(line)
			currentLen += lineLen
		}

		if currentLen > 0 {
			chunks = append(chunks, current.String())
		}
	} else {
		for i := 0; i < len(runes); i += limit {
			end := i + limit
			if end > len(runes) {
				end = len(runes)
			}
			chunks = append(chunks, string(runes[i:end]))
		}
	}

	return chunks
}

type TeamsOutboundActivity struct {
	Type         string                    `json:"type"`
	Text         string                    `json:"text"`
	From         *TeamsAccount             `json:"from,omitempty"`
	Conversation *TeamsConversationAccount `json:"conversation,omitempty"`
	Recipient    *TeamsAccount             `json:"recipient,omitempty"`
	ReplyToId    string                    `json:"replyToId,omitempty"`
}

type TeamsAccount struct {
	Id   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type TeamsConversationAccount struct {
	Id string `json:"id"`
}

type TeamsTokenResponse struct {
	AccessToken string `json:"access_token,omitempty"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type,omitempty"`
}

type TeamsInboundActivity struct {
	Type              string                     `json:"type,omitempty"`
	Id                string                     `json:"id,omitempty"`
	Text              string                     `json:"text,omitempty"`
	From              *TeamsActivityFrom         `json:"from,omitempty"`
	Conversation      *TeamsActivityConversation `json:"conversation,omitempty"`
	ActivityChannelId string                     `json:"channelId,omitempty"`
	ServiceUrl        string                     `json:"serviceUrl,omitempty"`
	Entities          []TeamsActivityEntity      `json:"entities,omitempty"`
	ChannelData       json.RawMessage            `json:"channelData,omitempty"`
	ReplyToId         string                     `json:"replyToId,omitempty"`
}

type TeamsActivityFrom struct {
	Id          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	AadObjectId string `json:"aadObjectId,omitempty"`
}

type TeamsActivityConversation struct {
	Id               string `json:"id,omitempty"`
	ConversationType string `json:"conversationType,omitempty"`
	TenantId         string `json:"tenantId,omitempty"`
	IsGroup          *bool  `json:"isGroup,omitempty"`
}

type TeamsActivityEntity struct {
	Type      string                  `json:"type,omitempty"`
	Mentioned *TeamsActivityMentioned `json:"mentioned,omitempty"`
	Text      string                  `json:"text,omitempty"`
}

type TeamsActivityMentioned struct {
	Id   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

func (t *TeamsChannel) getToken(ctx context.Context) (string, error) {
	if t.cachedToken != "" && time.Now().Before(t.tokenExpiry.Add(-5*time.Minute)) {
		return t.cachedToken, nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cachedToken != "" && time.Now().Before(t.tokenExpiry.Add(-5*time.Minute)) {
		return t.cachedToken, nil
	}

	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", url.PathEscape(t.tenantId))

	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {t.appId},
		"client_secret": {t.appPassword},
		"scope":         {"https://api.botframework.com/.default"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("unexpected status code from token endpoint: %d", resp.StatusCode)
	}

	var tokenResp TeamsTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	if strings.TrimSpace(tokenResp.AccessToken) == "" {
		return "", errors.New("failed to acquire Teams OAuth token — empty response")
	}

	t.cachedToken = tokenResp.AccessToken
	t.tokenExpiry = time.Now().UTC().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	t.logger.Debug("Acquired Teams OAuth token", "expires_in_sec", tokenResp.ExpiresIn)

	return t.cachedToken, nil
}

func (t *TeamsChannel) Send(ctx context.Context, outbound *core.OutboundMessage) error {
	if outbound == nil || outbound.Text == "" {
		return errors.New("Teams message is empty.")
	}

	reference, ok := t.conversationReferences[outbound.RecipientId]
	if !ok {
		t.logger.Warn(fmt.Sprintf("Teams SendAsync: no conversation reference for '%s'. Bot must receive a message first.", outbound.RecipientId))
		return errors.New("Teams conversation reference is missing.")
	}

	token, err := t.getToken(ctx)
	if err != nil {
		return err
	}

	teamsAccountid := reference.UserId
	if teamsAccountid == "" {
		teamsAccountid = outbound.RecipientId
	}
	chunks := chunkText(outbound.Text, t.config.TextChunkLimit, t.config.ChunkMode)
	for _, chunk := range chunks {
		activity := TeamsOutboundActivity{
			Type:         "message",
			Text:         chunk,
			From:         &TeamsAccount{Id: t.appId},
			Conversation: &TeamsConversationAccount{Id: reference.ConversationId},
			Recipient:    &TeamsAccount{Id: teamsAccountid},
		}

		// Thread reply support
		if t.config.ReplyStyle == "thread" && outbound.ReplyToMessageId != "" {
			activity.ReplyToId = outbound.ReplyToMessageId
		}

		url := fmt.Sprintf("%s/v3/conversations/%s/activities", strings.TrimRight(reference.ServiceUrl, "/"), reference.ConversationId)
		bodyBytes, _ := json.Marshal(activity)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := t.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			return fmt.Errorf("unexpected status code from token endpoint: %d", resp.StatusCode)
		}
	}

	return nil
}
