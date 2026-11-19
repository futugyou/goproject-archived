package whatsapp

import (
	"context"
	"time"

	"github.com/futugyou/openclaw/core"
)

type UnavailableBaileysWorkerEngine struct {
	config            core.WhatsAppFirstPartyWorkerConfig
	AuthEventReceived AuthEventReceived
}

func (u *UnavailableBaileysWorkerEngine) DebugEmitAuthEvent(ctx context.Context, payload any) error {
	return nil
}

func (u *UnavailableBaileysWorkerEngine) DebugGetState() any {
	data := map[string]any{}
	data["river"] = u.config.Driver
	return data
}

func (u *UnavailableBaileysWorkerEngine) DebugSimulateInbound(ctx context.Context, payload any) error {
	return nil
}

func (u *UnavailableBaileysWorkerEngine) Send(ctx context.Context, request core.BridgeChannelSendRequest) error {
	return nil
}

func (u *UnavailableBaileysWorkerEngine) SendReaction(ctx context.Context, request core.BridgeChannelReactionRequest) error {
	return nil
}

func (u *UnavailableBaileysWorkerEngine) SendReadReceipt(ctx context.Context, request core.BridgeChannelReceiptRequest) error {
	return nil
}

func (u *UnavailableBaileysWorkerEngine) SendTyping(ctx context.Context, request core.BridgeChannelTypingRequest) error {
	return nil
}

func (u *UnavailableBaileysWorkerEngine) Start(ctx context.Context) (*WhatsAppWorkerStartResult, error) {
	if u.AuthEventReceived != nil {
		accounts := u.config.Accounts
		if len(accounts) == 0 {
			accounts = []core.WhatsAppWorkerAccountConfig{{
				AccountId: "default", SessionPath: "./session/default",
			}}
		}
		for _, account := range accounts {
			if err := u.AuthEventReceived(core.BridgeChannelAuthEvent{
				ChannelId:    "whatsapp",
				AccountId:    account.AccountId,
				State:        "error",
				Data:         "BaileysCSharp transport adapter is not bundled in this build. Switch driver to 'simulated' for tests or supply a supported transport implementation.",
				UpdatedAtUtc: time.Now(),
			}); err != nil {
				return nil, err
			}
		}
	}

	return &WhatsAppWorkerStartResult{}, nil
}

func (u *UnavailableBaileysWorkerEngine) Stop(ctx context.Context) error {
	return nil
}

func NewUnavailableBaileysWorkerEngine(config core.WhatsAppFirstPartyWorkerConfig) *UnavailableBaileysWorkerEngine {
	return &UnavailableBaileysWorkerEngine{config: config}
}
