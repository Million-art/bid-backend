package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MediaNone      = "none"
	MediaPhoto     = "photo"
	MediaVideo     = "video"
	MediaAnimation = "animation" // GIF

	TargetChannel = "channel"
	TargetBot     = "bot"

	maxMessageRunes = 4096
	botSendDelay    = 40 * time.Millisecond
)

// BroadcastInput is a staff-authored Telegram broadcast payload.
type BroadcastInput struct {
	Target     string // channel | bot
	Text       string // Telegram HTML
	MediaType  string // none | photo | video | animation
	FileID     string // Telegram file_id from @file_detector style bots
	ButtonText string
	ButtonURL  string
}

// BroadcastResult summarizes delivery.
type BroadcastResult struct {
	Target   string `json:"target"`
	Sent     int    `json:"sent"`
	Failed   int    `json:"failed"`
	Skipped  int    `json:"skipped"`
	Audience int    `json:"audience"`
}

func (p *ChannelPoster) BroadcastConfigured() bool {
	return p != nil && strings.TrimSpace(p.token) != ""
}

func (p *ChannelPoster) ChannelConfigured() bool {
	return p.BroadcastConfigured() && strings.TrimSpace(p.channel) != ""
}

// Broadcast sends one message to the channel or many chats (bot DMs).
func (p *ChannelPoster) Broadcast(ctx context.Context, in BroadcastInput, chatIDs []int64) (*BroadcastResult, error) {
	if !p.BroadcastConfigured() {
		return nil, fmt.Errorf("telegram bot token is not configured")
	}

	target := strings.ToLower(strings.TrimSpace(in.Target))
	mediaType := strings.ToLower(strings.TrimSpace(in.MediaType))
	if mediaType == "" {
		mediaType = MediaNone
	}
	text := strings.TrimSpace(in.Text)
	fileID := strings.TrimSpace(in.FileID)
	btnText := strings.TrimSpace(in.ButtonText)
	btnURL := strings.TrimSpace(in.ButtonURL)

	if text == "" && fileID == "" {
		return nil, fmt.Errorf("message text or file_id is required")
	}
	if mediaType != MediaNone && fileID == "" {
		return nil, fmt.Errorf("file_id is required for media type %s", mediaType)
	}
	if fileID != "" && mediaType == MediaNone {
		mediaType = MediaPhoto
	}
	if (btnText == "") != (btnURL == "") {
		return nil, fmt.Errorf("button text and button URL must both be set or both empty")
	}
	if btnURL != "" && !(strings.HasPrefix(btnURL, "https://") || strings.HasPrefix(btnURL, "http://") || strings.HasPrefix(btnURL, "tg://")) {
		return nil, fmt.Errorf("button URL must start with http(s):// or tg://")
	}

	switch mediaType {
	case MediaNone, MediaPhoto, MediaVideo, MediaAnimation:
	default:
		return nil, fmt.Errorf("unsupported media type %q", mediaType)
	}

	caption := text
	if mediaType != MediaNone {
		caption = truncateRunes(text, maxCaptionRunes)
	} else {
		caption = truncateRunes(text, maxMessageRunes)
	}

	var keyboard any
	if btnText != "" && btnURL != "" {
		keyboard = map[string]any{
			"inline_keyboard": [][]map[string]string{
				{{"text": btnText, "url": btnURL}},
			},
		}
	}

	result := &BroadcastResult{Target: target}

	switch target {
	case TargetChannel:
		if !p.ChannelConfigured() {
			return nil, fmt.Errorf("CHANNEL_ID is not configured")
		}
		result.Audience = 1
		if err := p.sendToChat(ctx, p.channel, mediaType, fileID, caption, keyboard); err != nil {
			result.Failed = 1
			return result, err
		}
		result.Sent = 1
		return result, nil

	case TargetBot:
		result.Audience = len(chatIDs)
		if len(chatIDs) == 0 {
			return result, nil
		}
		for _, id := range chatIDs {
			if id == 0 {
				result.Skipped++
				continue
			}
			chat := fmt.Sprintf("%d", id)
			if err := p.sendToChat(ctx, chat, mediaType, fileID, caption, keyboard); err != nil {
				result.Failed++
				p.log.Warn("telegram_broadcast_dm_failed", "chat_id", id, "error", err)
			} else {
				result.Sent++
			}
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			case <-time.After(botSendDelay):
			}
		}
		return result, nil

	default:
		return nil, fmt.Errorf("target must be %q or %q", TargetChannel, TargetBot)
	}
}

func (p *ChannelPoster) sendToChat(ctx context.Context, chatID, mediaType, fileID, text string, keyboard any) error {
	switch mediaType {
	case MediaPhoto:
		payload := map[string]any{
			"chat_id":    chatID,
			"photo":      fileID,
			"parse_mode": "HTML",
		}
		if text != "" {
			payload["caption"] = text
		}
		if keyboard != nil {
			payload["reply_markup"] = keyboard
		}
		return p.apiCall(ctx, "sendPhoto", payload)

	case MediaVideo:
		payload := map[string]any{
			"chat_id":    chatID,
			"video":      fileID,
			"parse_mode": "HTML",
		}
		if text != "" {
			payload["caption"] = text
		}
		if keyboard != nil {
			payload["reply_markup"] = keyboard
		}
		return p.apiCall(ctx, "sendVideo", payload)

	case MediaAnimation:
		payload := map[string]any{
			"chat_id":    chatID,
			"animation":  fileID,
			"parse_mode": "HTML",
		}
		if text != "" {
			payload["caption"] = text
		}
		if keyboard != nil {
			payload["reply_markup"] = keyboard
		}
		return p.apiCall(ctx, "sendAnimation", payload)

	default:
		if strings.TrimSpace(text) == "" {
			return fmt.Errorf("text is required when media type is none")
		}
		payload := map[string]any{
			"chat_id":                  chatID,
			"text":                     text,
			"parse_mode":               "HTML",
			"disable_web_page_preview": false,
		}
		if keyboard != nil {
			payload["reply_markup"] = keyboard
		}
		return p.apiCall(ctx, "sendMessage", payload)
	}
}

// MergeTelegramIDs returns a unique sorted-ish list of chat ids.
func MergeTelegramIDs(sets ...[]int64) []int64 {
	seen := make(map[int64]struct{})
	out := make([]int64, 0)
	for _, set := range sets {
		for _, id := range set {
			if id == 0 {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}

func SanitizeHTMLPreview(s string) string {
	if utf8.RuneCountInString(s) <= 120 {
		return s
	}
	return truncateRunes(s, 120)
}
