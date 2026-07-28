package mcp

import (
	"encoding/json"

	"github.com/futugyou/extensions_ai/abstractions/chatcompletion"
	"github.com/futugyou/extensions_ai/abstractions/contents"
	"github.com/futugyou/mcp/protocol"
)

func ResourceContentsToAIContent(content protocol.ResourceContents) contents.IAIContent {
	var c contents.IAIContent
	switch content := content.IResourceContents.(type) {
	case *protocol.BlobResourceContents:
		decoded := content.Blob
		mimeType := "application/octet-stream"
		if content.MimeType != nil && len(*content.MimeType) > 0 {
			mimeType = *content.MimeType
		}
		d := contents.NewDataContent(string(decoded), mimeType)
		d.AddAdditionalProperty("uri", content.Uri)
		c = d
	case *protocol.TextResourceContents:
		d := contents.NewTextContent(content.Text)
		d.AddAdditionalProperty("uri", content.Uri)
		c = d
	}
	return c
}

func ContentToAIContent(content protocol.ContentBlock) contents.IAIContent {
	var c contents.IAIContent

	switch block := content.IContentBlock.(type) {
	case *protocol.ImageContentBlock:
		if len(block.GetMeta()) > 0 && len(block.Data) > 0 {
			d := contents.NewDataContent(string(block.Data), block.MimeType)
			d.RawRepresentation = content
			c = d
		}
	case *protocol.AudioContentBlock:
		if len(block.GetMeta()) > 0 && len(block.Data) > 0 {
			d := contents.NewDataContent(string(block.Data), block.MimeType)
			d.RawRepresentation = content
			c = d
		}
	case *protocol.EmbeddedResourceBlock:
		c = ResourceContentsToAIContent(block.Resource)
	case *protocol.TextContentBlock:
		d := contents.NewTextContent(block.Text)
		d.RawRepresentation = content
		c = d
	}

	return c
}

func ChatMessageToPromptMessages(chatMessage chatcompletion.ChatMessage) []protocol.PromptMessage {
	r := protocol.RoleAssistant
	if chatMessage.Role == chatcompletion.RoleUser {
		r = protocol.RoleUser
	}
	messages := []protocol.PromptMessage{}

	for _, content := range chatMessage.Contents {
		if c, ok := content.(contents.TextContent); ok {
			messages = append(messages, protocol.PromptMessage{Role: r, Content: AIContentToContent(c)})
		}
		if c, ok := content.(contents.DataContent); ok {
			messages = append(messages, protocol.PromptMessage{Role: r, Content: AIContentToContent(c)})
		}
	}
	return messages
}

func AIContentToContent(content contents.IAIContent) protocol.ContentBlock {
	switch content := content.(type) {
	case *contents.TextContent:
		return protocol.ContentBlock{
			IContentBlock: &protocol.TextContentBlock{
				Text: content.Text,
			}}
	case *contents.DataContent:
		c := protocol.ContentBlock{}
		if content.MediaTypeStartsWith("image") {
			c.IContentBlock = &protocol.ImageContentBlock{
				MimeType: content.MediaType,
				Data:     content.Data,
			}
		} else if content.MediaTypeStartsWith("audio") {
			c.IContentBlock = &protocol.AudioContentBlock{
				MimeType: content.MediaType,
				Data:     content.Data,
			}
		}
		return c
	default:
		data, err := json.Marshal(content.(*contents.AIContent))
		if err != nil {
			data = []byte{}
		}

		return protocol.ContentBlock{
			IContentBlock: &protocol.TextContentBlock{
				Text: string(data),
			},
		}
	}
}

func ResourceContentsListToAIContents(cont []protocol.ResourceContents) []contents.IAIContent {
	list := []contents.IAIContent{}
	for _, content := range cont {
		list = append(list, ResourceContentsToAIContent(content))
	}
	return list
}

func ListContentToAIContents(cont []protocol.ContentBlock) []contents.IAIContent {
	list := []contents.IAIContent{}
	for _, content := range cont {
		list = append(list, ContentToAIContent(content))
	}
	return list
}

func ToChatMessages(promptResult protocol.GetPromptResult) []chatcompletion.ChatMessage {
	list := []chatcompletion.ChatMessage{}
	for _, v := range promptResult.Messages {
		list = append(list, PromptMessageToChatMessage(v))
	}
	return list
}

func PromptMessageToChatMessage(promptMessage protocol.PromptMessage) chatcompletion.ChatMessage {
	role := chatcompletion.RoleAssistant
	if promptMessage.Role == protocol.RoleUser {
		role = chatcompletion.RoleUser
	}
	return chatcompletion.ChatMessage{
		RawRepresentation: promptMessage,
		Role:              role,
		Contents:          []contents.IAIContent{ContentToAIContent(promptMessage.Content)},
	}
}
