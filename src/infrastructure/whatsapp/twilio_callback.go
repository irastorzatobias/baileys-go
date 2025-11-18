package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainChatStorage "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/chatstorage"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// TwilioMessage mirrors the TypeScript Twilio payload expected by the external API.
type TwilioMessage struct {
	WaID             string `json:"WaId"`
	ProfileName      string `json:"ProfileName"`
	SmsMessageSID    string `json:"SmsMessageSid"`
	NumMedia         string `json:"NumMedia"`
	SmsSID           string `json:"SmsSid"`
	SmsStatus        string `json:"SmsStatus"`
	Body             string `json:"Body"`
	To               string `json:"To"`
	NumSegments      string `json:"NumSegments"`
	MessageSID       string `json:"MessageSid"`
	AccountSID       string `json:"AccountSid"`
	From             string `json:"From"`
	GroupName        string `json:"GroupName,omitempty"`
	FromGroup        bool   `json:"FromGroup,omitempty"`
	Latitude         string `json:"Latitude,omitempty"`
	Longitude        string `json:"Longitude,omitempty"`
	Address          string `json:"Address,omitempty"`
	MediaContentType string `json:"MediaContentType,omitempty"`
}

func forwardTwilioCallback(ctx context.Context, evt *events.Message, chatStorageRepo domainChatStorage.IChatStorageRepository) error {
	endpoint := strings.TrimSpace(config.WhatsappAPIEndpoint)
	if endpoint == "" || evt == nil || evt.Message == nil || evt.Info.IsFromMe || evt.Info.IsGroup {
		return nil
	}

	payload := buildTwilioPayload(evt, chatStorageRepo)
	if payload == nil {
		return nil
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal twilio payload: %w", err)
	}

	callbackURL := strings.TrimRight(endpoint, "/") + "/api/callback/twilio/"

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, callbackURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create twilio callback request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send twilio callback: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("twilio callback returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return nil
}

func buildTwilioPayload(evt *events.Message, chatStorageRepo domainChatStorage.IChatStorageRepository) *TwilioMessage {
	senderNumber := sanitizeJIDToNumber(evt.Info.Sender)
	if senderNumber == "" {
		senderNumber = sanitizeJIDToNumber(evt.Info.Chat)
	}
	if senderNumber == "" {
		return nil
	}

	ownNumber := ""
	if cli != nil && cli.Store != nil && cli.Store.ID != nil {
		ownNumber = sanitizeJIDToNumber(*cli.Store.ID)
	}

	body := utils.ExtractMessageTextFromEvent(evt)
	if body == "" {
		body = utils.ExtractMessageTextFromProto(evt.Message)
	}

	payload := &TwilioMessage{
		WaID:          senderNumber,
		ProfileName:   evt.Info.PushName,
		SmsMessageSID: string(evt.Info.ID),
		NumMedia:      "0",
		SmsSID:        string(evt.Info.ID),
		SmsStatus:     "received",
		Body:          body,
		To:            formatWhatsAppAddress(ownNumber),
		NumSegments:   "1",
		MessageSID:    string(evt.Info.ID),
		AccountSID:    config.TwilioAccountSID,
		From:          formatWhatsAppAddress(senderNumber),
	}

	if evt.Info.IsGroup {
		payload.FromGroup = true
		payload.GroupName = resolveGroupName(evt, chatStorageRepo)
	}

	if loc := evt.Message.GetLocationMessage(); loc != nil {
		payload.Latitude = fmt.Sprintf("%f", loc.GetDegreesLatitude())
		payload.Longitude = fmt.Sprintf("%f", loc.GetDegreesLongitude())
		if loc.GetName() != "" {
			payload.Address = loc.GetName()
		} else if loc.GetAddress() != "" {
			payload.Address = loc.GetAddress()
		}
	}

	mediaCount, mime := countMedia(evt.Message)
	if mediaCount > 0 {
		payload.NumMedia = strconv.Itoa(mediaCount)
		if mime != "" {
			payload.MediaContentType = mime
		}
	}

	return payload
}

func sanitizeJIDToNumber(jid types.JID) string {
	user := strings.TrimSpace(jid.User)
	return strings.TrimPrefix(user, "+")
}

func formatWhatsAppAddress(number string) string {
	number = strings.TrimSpace(number)
	if number == "" {
		return ""
	}
	number = strings.TrimPrefix(number, "+")
	return "whatsapp:+" + number
}

func resolveGroupName(evt *events.Message, chatStorageRepo domainChatStorage.IChatStorageRepository) string {
	if chatStorageRepo != nil {
		if chat, err := chatStorageRepo.GetChat(evt.Info.Chat.String()); err == nil && chat != nil && chat.Name != "" {
			return chat.Name
		}
	}

	// Fall back to readable JID if no stored name is available
	if evt.Info.Chat.User != "" {
		return evt.Info.Chat.User
	}
	return evt.Info.Chat.String()
}

func countMedia(msg *waE2E.Message) (int, string) {
	if msg == nil {
		return 0, ""
	}

	if img := msg.GetImageMessage(); img != nil {
		return 1, img.GetMimetype()
	}
	if vid := msg.GetVideoMessage(); vid != nil {
		return 1, vid.GetMimetype()
	}
	if doc := msg.GetDocumentMessage(); doc != nil {
		return 1, doc.GetMimetype()
	}
	if audio := msg.GetAudioMessage(); audio != nil {
		return 1, audio.GetMimetype()
	}
	if sticker := msg.GetStickerMessage(); sticker != nil {
		return 1, sticker.GetMimetype()
	}

	return 0, ""
}
