package whatsapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/futugyou/openclaw/core"
)

var _ IWhatsAppWorkerEngine = (*SimulatedWhatsAppWorkerEngine)(nil)

type SimulatedWhatsAppWorkerEngine struct {
	config            core.WhatsAppFirstPartyWorkerConfig
	sends             []core.BridgeChannelSendRequest
	typings           []core.BridgeChannelTypingRequest
	receipts          []core.BridgeChannelReceiptRequest
	reactions         []core.BridgeChannelReactionRequest
	startCount        int
	stopCount         int
	MessageReceived   MessageReceived
	AuthEventReceived AuthEventReceived
}

func NewSimulatedWhatsAppWorkerEngine(config core.WhatsAppFirstPartyWorkerConfig) *SimulatedWhatsAppWorkerEngine {
	return &SimulatedWhatsAppWorkerEngine{
		config:    config,
		sends:     []core.BridgeChannelSendRequest{},
		typings:   []core.BridgeChannelTypingRequest{},
		receipts:  []core.BridgeChannelReceiptRequest{},
		reactions: []core.BridgeChannelReactionRequest{},
	}
}

func (s *SimulatedWhatsAppWorkerEngine) DebugEmitAuthEvent(ctx context.Context, payload []byte) error {
	type Event struct {
		AccountId string `json:"accountId"`
		State     string `json:"state"`
		Data      string `json:"data"`
	}

	var data Event
	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}

	if s.AuthEventReceived != nil {
		return s.AuthEventReceived(core.BridgeChannelAuthEvent{
			ChannelId:    "whatsapp",
			AccountId:    data.AccountId,
			State:        data.State,
			Data:         data.Data,
			UpdatedAtUtc: time.Now(),
		})
	}

	return nil
}

func (s *SimulatedWhatsAppWorkerEngine) DebugGetState() any {
	data := map[string]any{}
	data["river"] = s.config.Driver
	data["startCount"] = s.startCount
	data["stopCount"] = s.stopCount
	data["sends"] = s.sends
	data["typings"] = s.typings
	data["receipts"] = s.receipts
	data["reactions"] = s.reactions
	return data
}

func (s *SimulatedWhatsAppWorkerEngine) DebugSimulateInbound(ctx context.Context, payload []byte) error {
	var data WhatsAppWorkerInboundMessage
	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}

	if data.SenderId == "" {
		return errors.New("Debug inbound payload requires senderId")
	}

	if s.MessageReceived != nil {
		return s.MessageReceived(data)
	}

	return nil
}

func (s *SimulatedWhatsAppWorkerEngine) Send(ctx context.Context, request core.BridgeChannelSendRequest) error {
	s.sends = append(s.sends, request)
	return nil
}

func (s *SimulatedWhatsAppWorkerEngine) SendReaction(ctx context.Context, request core.BridgeChannelReactionRequest) error {
	s.reactions = append(s.reactions, request)
	return nil
}

func (s *SimulatedWhatsAppWorkerEngine) SendReadReceipt(ctx context.Context, request core.BridgeChannelReceiptRequest) error {
	s.receipts = append(s.receipts, request)
	return nil
}

func (s *SimulatedWhatsAppWorkerEngine) SendTyping(ctx context.Context, request core.BridgeChannelTypingRequest) error {
	s.typings = append(s.typings, request)
	return nil
}

func (s *SimulatedWhatsAppWorkerEngine) Start(ctx context.Context) (*WhatsAppWorkerStartResult, error) {
	s.startCount++

	selfIds := []string{}
	for _, account := range s.config.Accounts {
		selfId := fmt.Sprintf("%s@s.whatsapp.net", account.AccountId)
		selfIds = append(selfIds, selfId)
		state := "qr_code"
		if account.PairingMode == "pairing_code" {
			state = "pairing_code"
		}

		data := fmt.Sprintf("SIMULATED:%s", account.AccountId)
		if state == "pairing_code" {
			if account.PhoneNumber == "" {
				data = fmt.Sprintf("PAIR-%s-unknown", account.AccountId)
			} else {
				data = fmt.Sprintf("PAIR-%s-%s", account.AccountId, account.PhoneNumber)
			}
		}
		if s.AuthEventReceived != nil {
			s.AuthEventReceived(core.BridgeChannelAuthEvent{
				ChannelId:    "whatsapp",
				AccountId:    account.AccountId,
				State:        state,
				Data:         data,
				UpdatedAtUtc: time.Now(),
			})
		}
	}

	return &WhatsAppWorkerStartResult{SelfIds: selfIds}, nil
}

func (s *SimulatedWhatsAppWorkerEngine) Stop(ctx context.Context) error {
	s.stopCount++
	return nil
}
func (s *SimulatedWhatsAppWorkerEngine) SetMessageReceived(messageReceived MessageReceived) {
	s.MessageReceived = messageReceived
}

func (s *SimulatedWhatsAppWorkerEngine) SetAuthEventReceived(authEventReceived AuthEventReceived) {
	s.AuthEventReceived = authEventReceived
}
