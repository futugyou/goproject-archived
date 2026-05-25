package models

import (
	"context"

	"github.com/futugyou/extensions_ai/abstractions/chatcompletion"
)

type IAgentProvider interface {
	GetProviderName() string
	CreateChatClient(profile *AgentProfile) (chatcompletion.IChatClient, error)

	IsAvailable(ctx context.Context) (bool, error)
}

type IChannel interface {
	GetChannelName() string
	IsEnabled() bool
	SendMessage(ctx context.Context, conversationId, message string) error
	IsAvailable(ctx context.Context) (bool, error)
}

type IChannelRegistry interface {
	Register(channel IChannel) error
	GetChannel(name string) (IChannel, error)
	GetAllChannels() ([]IChannel, error)
}

type IModelClient interface {
	GetProviderName() string
	Complete(ctx context.Context, request *ChatRequest) (*ChatResponse, error)
	Stream(ctx context.Context, request *ChatRequest) (<-chan ChatResponseChunk, error)
	IsAvailable(ctx context.Context) (bool, error)
}
