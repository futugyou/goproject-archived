package calendar

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type CalendarTool struct {
	config      *core.CalendarConfig
	httpClient  *http.Client
	accessToken string
	tokenExpiry time.Time
}

func New(config *core.CalendarConfig, httpClient *http.Client) *CalendarTool {
	if config == nil {
		config = &core.CalendarConfig{}
	}

	if httpClient == nil {
		httpClient = &http.Client{}
	}

	return &CalendarTool{config: config}
}

func (a *CalendarTool) Name() string {
	return "calendar	"
}

func (a *CalendarTool) Description() string {
	return "Manage calendar events. Supports listing upcoming events, creating, updating, and deleting events."
}

const calendarApiBase string = "https://www.googleapis.com/calendar/v3"

func (a *CalendarTool) ParameterSchema() string {
	return `
	{
          "type": "object",
          "properties": {
            "action": {
              "type": "string",
              "description": "Action to perform",
              "enum": ["list", "create", "update", "delete", "search"]
            },
            "query": {
              "type": "string",
              "description": "Search query (for search action) or event title filter (for list)"
            },
            "title": {
              "type": "string",
              "description": "Event title (for create/update)"
            },
            "start": {
              "type": "string",
              "description": "Start datetime in ISO 8601 format (e.g., 2026-02-20T10:00:00-05:00)"
            },
            "end": {
              "type": "string",
              "description": "End datetime in ISO 8601 format"
            },
            "description": {
              "type": "string",
              "description": "Event description/notes"
            },
            "location": {
              "type": "string",
              "description": "Event location"
            },
            "event_id": {
              "type": "string",
              "description": "Event ID (for update/delete)"
            },
            "days_ahead": {
              "type": "integer",
              "description": "Number of days ahead to list events (default: 7)",
              "default": 7
            }
          },
          "required": ["action"]
        }
	`
}

type AuthClaims struct {
	Issuer    string `json:"iss,omitempty"`
	Scope     string `json:"scope,omitempty"`
	Audience  string `json:"aud,omitempty"`
	IssuedAt  int64  `json:"iat,omitempty"`
	ExpiresAt int64  `json:"exp,omitempty"`
}

type EventTime struct {
	DateTime string `json:"dateTime,omitempty"`
	Date     string `json:"date,omitempty"`
}

type CalendarEvent struct {
	Id          string     `json:"id,omitempty"`
	Summary     string     `json:"summary,omitempty"`
	Start       *EventTime `json:"start,omitempty"`
	End         *EventTime `json:"end,omitempty"`
	Description string     `json:"description,omitempty"`
	Location    string     `json:"location,omitempty"`
}

type CalendarResponse struct {
	Items []CalendarEvent `json:"items,omitempty"`
}

func base64UrlEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func buildEventJsonContent(title, start, end, description, location string) []byte {
	data := CalendarEvent{
		Summary: title,
		Start: &EventTime{
			DateTime: start,
		},
		End: &EventTime{
			DateTime: end,
		},
		Description: description,
		Location:    location,
	}

	result, _ := json.Marshal(data)
	return result
}

func buildJwtClaims(clientEmail, tokenUri string, iat, exp int64) string {
	auth := AuthClaims{
		Issuer:    clientEmail,
		Scope:     "https://www.googleapis.com/auth/calendar",
		Audience:  tokenUri,
		IssuedAt:  iat,
		ExpiresAt: exp,
	}

	data, err := json.Marshal(auth)
	if err != nil {
		return ""
	}

	return string(data)
}

type GoogleCred struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenUri    string `json:"token_uri"`
}

type AuthResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

func (a *CalendarTool) ensureAccessToken(ctx context.Context) error {
	if a.accessToken != "" && time.Now().UTC().Compare(a.tokenExpiry) < 0 {
		return nil
	}

	cred, err := util.LoadOneFile[GoogleCred](ctx, a.config.CredentialsPath)
	if err != nil {
		return err
	}

	if cred.TokenUri == "" {
		cred.TokenUri = "https://oauth2.googleapis.com/token"
	}

	// Build JWT
	now := time.Now().UTC()
	exp := now.Add(time.Hour)

	var headerJson = `{"alg":"RS256","typ":"JWT"}`
	var claimsJson = buildJwtClaims(cred.ClientEmail, cred.TokenUri, now.Unix(), exp.Unix())

	var headerB64 = base64UrlEncode([]byte(headerJson))
	var claimsB64 = base64UrlEncode([]byte(claimsJson))
	var signingInput = fmt.Sprintf("%s.%s", headerB64, claimsB64)

	// Sign with RSA
	signature, err := util.SignData(signingInput, cred.PrivateKey)
	if err != nil {
		return err
	}

	var jwt = fmt.Sprintf("%s.%s", signingInput, base64UrlEncode(signature))

	// Exchange JWT for access token
	data := url.Values{}
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	data.Set("assertion", jwt)
	req, err := http.NewRequestWithContext(ctx, "POST", cred.TokenUri, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http response code: %d", resp.StatusCode)
	}

	var doc AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return err
	}

	a.accessToken = doc.AccessToken
	expiresIn := doc.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}

	a.tokenExpiry = time.Now().UTC().Add(time.Second * (time.Duration(expiresIn) - 60))

	return nil
}

func formatEventList(root CalendarResponse) string {
	if len(root.Items) == 0 {
		return "No events found."
	}

	var sb = strings.Builder{}
	var count = 0

	for _, ev := range root.Items {
		count++
		var summary = ev.Summary
		if summary == "" {
			summary = "(no title)"
		}
		var eventId = ev.Id
		var location = ev.Location

		startStr := ""
		if ev.Start != nil {
			startStr = ev.Start.DateTime
			if startStr == "" {
				startStr = ev.Start.Date
			}
		}
		endStr := ""
		if ev.End != nil {
			endStr = ev.End.DateTime
			if endStr == "" {
				endStr = ev.End.Date
			}
		}

		sb.WriteString(fmt.Sprintf("[%d] %s\n", count, summary))
		sb.WriteString(fmt.Sprintf("    ID: %s\n", eventId))
		if startStr != "" {
			sb.WriteString(fmt.Sprintf("    Start: %ss\n", startStr))
		}
		if endStr != "" {
			sb.WriteString(fmt.Sprintf("    End: %ss\n", endStr))
		}
		if location != "" {
			sb.WriteString(fmt.Sprintf("    Location: %ss\n", location))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (a *CalendarTool) deleteEvent(ctx context.Context, args map[string]any) string {
	var eventId = util.GetString(args, "event_id")
	if eventId == nil || strings.TrimSpace(*eventId) == "" {
		return "Error: 'event_id' is required to delete an event."
	}

	var url = fmt.Sprintf("%s/calendars/%s/events/%s", calendarApiBase, url.QueryEscape(a.config.CalendarId), url.QueryEscape(*eventId))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err.Error()
	}

	req.Header.Set("Authorization", "Bearer "+a.accessToken)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err.Error()
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Sprintf("Error: Failed to delete event (HTTP %d)", resp.StatusCode)
	}

	return fmt.Sprintf("Event '%s' deleted successfully.", *eventId)
}

func getStringOrDefault(ptr *string, defaultValue string) string {
	if ptr != nil {
		return *ptr
	}
	return defaultValue
}

func (a *CalendarTool) updateEvent(ctx context.Context, args map[string]any) string {
	var eventId = util.GetString(args, "event_id")
	if eventId == nil || strings.TrimSpace(*eventId) == "" {
		return "Error: 'event_id' is required to update an event."
	}

	var title = getStringOrDefault(util.GetString(args, "title"), "")
	var start = getStringOrDefault(util.GetString(args, "start"), "")
	var end = getStringOrDefault(util.GetString(args, "end"), "")
	var description = getStringOrDefault(util.GetString(args, "description"), "")
	var location = getStringOrDefault(util.GetString(args, "location"), "")

	content := buildEventJsonContent(title, start, end, description, location)

	var url = fmt.Sprintf("%s/calendars/%s/events/%s", calendarApiBase, url.QueryEscape(a.config.CalendarId), url.QueryEscape(*eventId))
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewBuffer(content))
	if err != nil {
		return err.Error()
	}

	req.Header.Set("Authorization", "Bearer "+a.accessToken)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err.Error()
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Sprintf("Error: Failed to update event (HTTP %d)", resp.StatusCode)
	}

	return fmt.Sprintf("Event '%s' updated successfully.", *eventId)
}

func (a *CalendarTool) createEvent(ctx context.Context, args map[string]any) string {
	var title = getStringOrDefault(util.GetString(args, "title"), "")
	var start = getStringOrDefault(util.GetString(args, "start"), "")
	var end = getStringOrDefault(util.GetString(args, "end"), "")
	var desc = getStringOrDefault(util.GetString(args, "description"), "")
	var loc = getStringOrDefault(util.GetString(args, "location"), "")

	if title == "" || start == "" {
		return "Error: 'title' and 'start' are required to create an event."
	}

	// Default end = start + 1 hour
	if end == "" {
		t, err := time.Parse(time.RFC3339Nano, start)
		if err == nil {
			end = t.Add(time.Hour).Format(time.RFC3339Nano)
		}
	}

	content := buildEventJsonContent(title, start, end, desc, loc)

	var url = fmt.Sprintf("%s/calendars/%s/events", calendarApiBase, url.QueryEscape(a.config.CalendarId))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(content))
	if err != nil {
		return err.Error()
	}

	req.Header.Set("Authorization", "Bearer "+a.accessToken)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err.Error()
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Sprintf("Error: Failed to create event (HTTP %d)", resp.StatusCode)
	}

	var doc CalendarCreateEventResp
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return err.Error()
	}

	eventId := doc.Id
	if eventId == "" {
		eventId = "unknown"
	}
	htmlLink := doc.HtmlLink

	return fmt.Sprintf("Event created successfully.\nID: %s\nTitle: %s\nStart: %s\nEnd: %s\nLink: %s", eventId, title, start, end, htmlLink)
}

type CalendarCreateEventResp struct {
	Id       string `json:"id"`
	HtmlLink string `json:"htmlLink"`
}

func (a *CalendarTool) searchEvents(ctx context.Context, args map[string]any) string {
	var query = getStringOrDefault(util.GetString(args, "query"), "")
	if query == "" {
		return "Error: 'query' parameter is required for search."
	}

	var url = fmt.Sprintf("%s/calendars/%s/events?q=%s&maxResults=%d&singleEvents=true&orderBy=startTime", calendarApiBase, url.QueryEscape(a.config.CalendarId), url.QueryEscape(query), a.config.MaxEvents)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err.Error()
	}

	req.Header.Set("Authorization", "Bearer "+a.accessToken)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err.Error()
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Sprintf("Error: Failed to search (HTTP %d)", resp.StatusCode)
	}

	var root CalendarResponse
	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil {
		return err.Error()
	}

	return formatEventList(root)
}

var defaultDaysAhead int = 7

func (a *CalendarTool) listEvents(ctx context.Context, args map[string]any) string {
	var daysAhead = util.GetInt(args, "days_ahead")
	if daysAhead == nil {
		daysAhead = &defaultDaysAhead
	}
	var now = time.Now().UTC()
	var until = now.Add(time.Hour * 24 * time.Duration(*daysAhead))

	var geturl = fmt.Sprintf("%s/calendars/%s/events?timeMin=%s&timeMax=%s&maxResults=%d&singleEvents=true&orderBy=startTime",
		calendarApiBase,
		url.QueryEscape(a.config.CalendarId),
		url.QueryEscape(now.Format(time.RFC3339Nano)),
		url.QueryEscape(until.Format(time.RFC3339Nano)),
		a.config.MaxEvents,
	)

	var query = util.GetString(args, "query")

	if query != nil && strings.TrimSpace(*query) != "" {
		geturl += fmt.Sprintf("&q=%s", url.QueryEscape(*query))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, geturl, nil)
	if err != nil {
		return err.Error()
	}

	req.Header.Set("Authorization", "Bearer "+a.accessToken)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err.Error()
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Sprintf("Error: Failed to search (HTTP %d)", resp.StatusCode)
	}

	var root CalendarResponse
	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil {
		return err.Error()
	}

	return formatEventList(root)
}

func (a *CalendarTool) Execute(ctx context.Context, argumentsJson string) string {
	if a.config.CredentialsPath == "" || !util.FileExists(a.config.CredentialsPath) {
		return "Error: Calendar credentials not configured. Set Calendar.CredentialsPath to a valid service account JSON key file."
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(argumentsJson), &args); err != nil {
		return err.Error()
	}

	action, ok := args["action"].(string)
	if !ok {
		return "Error: Unsupported calendar action. Use: list, search, create, update, delete"
	}
	action = strings.ToLower(action)

	if err := a.ensureAccessToken(ctx); err != nil {
		return fmt.Sprintf("Error: Failed to authenticate with Google Calendar — %s", err.Error())
	}

	switch action {
	case "list":
		return a.listEvents(ctx, args)
	case "search":
		return a.searchEvents(ctx, args)
	case "create":
		return a.createEvent(ctx, args)
	case "update":
		return a.updateEvent(ctx, args)
	case "delete":
		return a.deleteEvent(ctx, args)
	default:
		return fmt.Sprintf("Error: Unsupported calendar action '%s'. Use: list, search, create, update, delete.", action)
	}
}
