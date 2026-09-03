package channels

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/futugyou/openclaw/core"
)

type TwilioSmsClient struct {
	httpClient *http.Client
	config     core.TwilioSmsConfig
	authToken  string
}

func NewTwilioSmsClient(httpClient *http.Client,
	config core.TwilioSmsConfig,
	authToken string) *TwilioSmsClient {
	return &TwilioSmsClient{
		config:     config,
		authToken:  authToken,
		httpClient: httpClient,
	}
}

func (t *TwilioSmsClient) Send(ctx context.Context, toE164, body string) (ok bool, message string) {
	if t.config.AccountSid == "" {
		message = "Twilio AccountSid is not configured."
		return
	}

	if t.config.MessagingServiceSid == "" && t.config.FromNumber == "" {
		message = "Twilio MessagingServiceSid or FromNumber must be configured."
		return

	}

	var requesturl = fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", t.config.AccountSid)

	data := url.Values{}
	data.Set("To", toE164)
	data.Set("Body", body)
	if t.config.MessagingServiceSid != "" {
		data.Set("MessagingServiceSid", t.config.MessagingServiceSid)
	} else {
		data.Set("From", t.config.FromNumber)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", requesturl, strings.NewReader(data.Encode()))
	if err != nil {
		message = err.Error()
		return
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(t.config.AccountSid, t.authToken)
	resp, err := t.httpClient.Do(req)
	if err != nil {
		message = err.Error()
		return
	}

	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		message = err.Error()
		return
	}

	bodyString := string(bodyBytes)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message = fmt.Sprintf("Twilio send failed (%d): %s", resp.StatusCode, bodyString)
		return
	}

	ok = true
	message = "ok"
	return
}
