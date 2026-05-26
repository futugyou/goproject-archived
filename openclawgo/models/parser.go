package models

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	slugInvalidCharsRegex = regexp.MustCompile(`[^a-z0-9\s-]`)
	slugWhitespaceRegex   = regexp.MustCompile(`[\s]+`)
	slugMultiDashRegex    = regexp.MustCompile(`-{2,}`)
)

type AgentProfileMarkdownParser struct{}

func (p *AgentProfileMarkdownParser) Parse(markdown string, fallbackName *string) AgentProfile {
	profile := AgentProfile{
		Name: "",
	}

	var body string

	if p.hasFrontMatter(markdown) {
		frontMatter, rest := p.extractFrontMatter(markdown)
		p.applyFrontMatter(&profile, frontMatter)
		body = rest
	} else {
		body = markdown
	}

	profile.Instructions = strings.TrimSpace(body)

	// Derive name from heading if not set via front-matter
	if strings.TrimSpace(profile.Name) == "" {
		heading := p.extractFirstHeading(body)

		if strings.TrimSpace(heading) != "" {
			profile.Name = p.slugify(heading)
		} else if fallbackName != nil && strings.TrimSpace(*fallbackName) != "" {
			profile.Name = *fallbackName
		} else {
			profile.Name = "imported-" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
		}
	}

	return profile
}

func (p *AgentProfileMarkdownParser) hasFrontMatter(markdown string) bool {
	if !strings.HasPrefix(markdown, "---") {
		return false
	}

	return strings.Contains(markdown[3:], "---")
}

func (p *AgentProfileMarkdownParser) extractFrontMatter(markdown string) (frontMatter string, body string) {
	// Skip opening "---" line
	startIndex := strings.Index(markdown, "\n")
	if startIndex < 0 {
		return "", markdown
	}

	startIndex++

	endIndex := strings.Index(markdown[startIndex:], "\n---")
	if endIndex < 0 {
		return "", markdown
	}

	endIndex += startIndex

	frontMatter = markdown[startIndex:endIndex]

	bodyStart := strings.Index(markdown[endIndex+1:], "\n")
	if bodyStart >= 0 {
		bodyStart += endIndex + 1
		body = markdown[bodyStart+1:]
	} else {
		body = ""
	}

	return frontMatter, body
}

func (p *AgentProfileMarkdownParser) applyFrontMatter(profile *AgentProfile, frontMatter string) {
	lines := strings.Split(frontMatter, "\n")

	for _, rawLine := range lines {
		line := strings.Trim(rawLine, "\r ")

		if line == "" {
			continue
		}

		colonIndex := strings.Index(line, ":")
		if colonIndex <= 0 {
			continue
		}

		key := strings.TrimSpace(line[:colonIndex])
		value := strings.TrimSpace(line[colonIndex+1:])

		switch key {
		case "name":
			profile.Name = value

		case "displayName":
			profile.DisplayName = value

		case "provider":
			profile.Provider = value

		case "model":
			// Legacy field ignored intentionally

		case "tools":
			profile.EnabledTools = p.parseToolsList(value)

		case "temperature":
			if temp, err := strconv.ParseFloat(value, 32); err == nil {
				profile.Temperature = float32(temp)
			}

		case "maxTokens":
			if tokens, err := strconv.Atoi(value); err == nil {
				profile.MaxTokens = tokens
			}

		case "kind":
			profile.Kind = StringToProfileKind(value)

		case "retrievalLevel", "retrieval":
			profile.RetrievalLevel = StringToRetrievalLevel(value)
		}
	}
}

func (p *AgentProfileMarkdownParser) parseToolsList(value string) string {
	trimmed := strings.TrimSpace(value)

	// Handle [tool1, tool2]
	if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		trimmed = trimmed[1 : len(trimmed)-1]
	}

	parts := strings.Split(trimmed, ",")

	tools := make([]string, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)

		if part == "" {
			continue
		}

		tools = append(tools, part)
	}

	return strings.Join(tools, ",")
}

func (p *AgentProfileMarkdownParser) extractFirstHeading(body string) string {
	lines := strings.Split(body, "\n")

	for _, rawLine := range lines {
		line := strings.TrimLeft(rawLine, " \t")

		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(line[2:])
		}
	}

	return ""
}

func (p *AgentProfileMarkdownParser) slugify(text string) string {
	slug := strings.ToLower(strings.TrimSpace(text))

	slug = slugInvalidCharsRegex.ReplaceAllString(slug, "")
	slug = slugWhitespaceRegex.ReplaceAllString(slug, "-")
	slug = slugMultiDashRegex.ReplaceAllString(slug, "-")

	return strings.Trim(slug, "-")
}
