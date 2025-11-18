package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"go.mau.fi/whatsmeow/types/events"
)

type outboundDevicePayload struct {
	Did        string `json:"did"`
	Number     string `json:"number"`
	Name       string `json:"name"`
	Text       string `json:"text"`
	CompanyNID int    `json:"companyNid,omitempty"`
}

func forwardOutboundDeviceMessage(ctx context.Context, evt *events.Message, chatStorageRepo domainChatStorage.IChatStorageRepository) error {
	// Only process outbound messages sent from this device.
	if evt == nil || evt.Message == nil || !evt.Info.IsFromMe {
		return nil
	}

	// Skip protocol messages (edits, revokes, etc.) for this callback.
	if evt.Message.GetProtocolMessage() != nil {
		return nil
	}

	endpoint := strings.TrimSpace(config.WhatsappAPIEndpoint)
	if endpoint == "" {
		return nil
	}

	did := ""
	if cli != nil && cli.Store != nil && cli.Store.ID != nil {
		did = strings.TrimSpace(cli.Store.ID.User)
	}
	if did == "" {
		return fmt.Errorf("cannot determine device ID for outbound callback")
	}

	chatJID := evt.Info.Chat.String()
	recipientNumber := strings.TrimPrefix(strings.TrimSpace(evt.Info.Chat.User), "+")
	if recipientNumber == "" {
		// Fallback: use full JID user part
		recipientNumber = strings.Split(chatJID, "@")[0]
	}

	messageBody := utils.ExtractMessageTextFromEvent(evt)
	if messageBody == "" {
		messageBody = utils.ExtractMessageTextFromProto(evt.Message)
	}

	name := evt.Info.PushName
	if evt.Info.IsGroup {
		if groupName := resolveGroupName(evt, chatStorageRepo); groupName != "" {
			name = groupName
		}
		if evt.Info.PushName != "" {
			messageBody = fmt.Sprintf("*%s*\n\n%s", evt.Info.PushName, messageBody)
		}
	}

	payload := outboundDevicePayload{
		Did:        did,
		Number:     recipientNumber,
		Name:       name,
		Text:       messageBody,
		CompanyNID: 6,
	}
	if config.CompanyNID > 0 {
		payload.CompanyNID = config.CompanyNID
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal outbound payload: %w", err)
	}

	url := strings.TrimRight(endpoint, "/") + "/api/callback/qr/message/send"
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create outbound request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send outbound request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("outbound callback returned %d", resp.StatusCode)
	}

	return nil
}
