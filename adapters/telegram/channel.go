package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/platform/env"
)

const (
	telegramAPIBase   = "https://api.telegram.org"
	maxCaptionRunes   = 1024
	maxDescRunes      = 280
	miniAppShortName  = "startapp"
	httpClientTimeout = 15 * time.Second
)

// ChannelPoster posts newly published auctions to a Telegram channel.
type ChannelPoster struct {
	token    string
	channel  string
	botUser  string
	adminID  string
	client   *http.Client
	log      *slog.Logger
	enabled  bool
}

func NewChannelPoster(cfg *env.Config, log *slog.Logger) *ChannelPoster {
	if log == nil {
		log = slog.Default()
	}
	token := ""
	channel := ""
	botUser := ""
	adminID := ""
	if cfg != nil {
		token = strings.TrimSpace(cfg.TelegramBotToken)
		channel = strings.TrimSpace(cfg.TelegramChannelID)
		if channel != "" && !strings.HasPrefix(channel, "@") && !strings.HasPrefix(channel, "-") {
			if _, err := strconv.ParseInt(channel, 10, 64); err != nil {
				channel = "@" + channel
			}
		}
		botUser = strings.TrimPrefix(strings.TrimSpace(cfg.TelegramBotUsername), "@")
		adminID = strings.TrimSpace(cfg.TelegramAdminID)
	}
	enabled := token != "" && channel != "" && botUser != ""
	return &ChannelPoster{
		token:   token,
		channel: channel,
		botUser: botUser,
		adminID: adminID,
		client:  &http.Client{Timeout: httpClientTimeout},
		log:     log,
		enabled: enabled,
	}
}

func (p *ChannelPoster) Enabled() bool { return p != nil && p.enabled }

// PostAuctionPublished sends a channel photo/message with a Join & bid mini-app CTA.
func (p *ChannelPoster) PostAuctionPublished(ctx context.Context, a *domain.Auction) error {
	if !p.Enabled() {
		p.log.Debug("telegram_channel_post_skipped", "reason", "not_configured")
		return nil
	}
	if a == nil {
		return nil
	}

	deepLink := fmt.Sprintf(
		"https://t.me/%s/%s?startapp=auction_%s",
		p.botUser,
		miniAppShortName,
		a.ID.String(),
	)
	caption := buildCaption(a)
	keyboard := map[string]any{
		"inline_keyboard": [][]map[string]string{
			{{"text": "Join & bid", "url": deepLink}},
		},
	}

	photoURL := strings.TrimSpace(a.ImageURL)
	if photoURL == "" && len(a.ImageURLs) > 0 {
		photoURL = strings.TrimSpace(a.ImageURLs[0])
	}

	var err error
	if isPublicHTTPURL(photoURL) {
		err = p.apiCall(ctx, "sendPhoto", map[string]any{
			"chat_id":      p.channel,
			"photo":        photoURL,
			"caption":      caption,
			"parse_mode":   "HTML",
			"reply_markup": keyboard,
		})
	} else {
		err = p.apiCall(ctx, "sendMessage", map[string]any{
			"chat_id":                  p.channel,
			"text":                     caption,
			"parse_mode":               "HTML",
			"disable_web_page_preview": false,
			"reply_markup":             keyboard,
		})
	}

	if err != nil {
		p.log.Error("telegram_channel_post_failed",
			"auction_id", a.ID.String(),
			"title", a.Title,
			"channel", p.channel,
			"error", err,
		)
		p.notifyAdmin(ctx, fmt.Sprintf(
			"Failed to post auction to channel (Channel: %s).\nAuction: %s (%s)\nError: %s",
			p.channel, a.Title, a.ID.String(), err.Error(),
		))
		return err
	}

	p.log.Info("telegram_channel_post_ok", "auction_id", a.ID.String(), "title", a.Title)
	return nil
}

func (p *ChannelPoster) miniAppDeepLink() string {
	botUser := strings.TrimPrefix(strings.TrimSpace(p.botUser), "@")
	if botUser == "" {
		return ""
	}
	return fmt.Sprintf("https://t.me/%s/%s", botUser, miniAppShortName)
}

func (p *ChannelPoster) sendPrivateWithMiniAppButton(ctx context.Context, chatID int64, text, buttonLabel string) error {
	if !p.BroadcastConfigured() || chatID == 0 {
		return nil
	}
	payload := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}
	if link := p.miniAppDeepLink(); link != "" {
		payload["reply_markup"] = map[string]any{
			"inline_keyboard": [][]map[string]string{
				{{"text": buttonLabel, "url": link}},
			},
		}
	}
	return p.apiCall(ctx, "sendMessage", payload)
}

// SendStartWelcome DMs an Amharic welcome + Mini App open button after /start.
func (p *ChannelPoster) SendStartWelcome(ctx context.Context, chatID int64) error {
	if p.miniAppDeepLink() == "" {
		p.log.Debug("telegram_start_welcome_skipped", "reason", "bot_username_missing")
		return nil
	}
	return p.sendPrivateWithMiniAppButton(
		ctx,
		chatID,
		"እንኳን ደህና መጡ! ጨረታዎችን ለማየት ከታች ይጫኑ።",
		"ጨረታ ክፈት",
	)
}

// SendContactThanks acknowledges a shared phone contact in Amharic.
func (p *ChannelPoster) SendContactThanks(ctx context.Context, chatID int64) error {
	return p.sendPrivateWithMiniAppButton(
		ctx,
		chatID,
		"አመሰግናለን! ስልክ ቁጥርዎ ተቀብለናል። ጨረታ ለመቀጠል ከታች ይጫኑ።",
		"ጨረታ ክፈት",
	)
}

func (p *ChannelPoster) notifyAdmin(ctx context.Context, text string) {
	if p.adminID == "" {
		return
	}
	if err := p.apiCall(ctx, "sendMessage", map[string]any{
		"chat_id": p.adminID,
		"text":    truncateRunes(text, 3500),
	}); err != nil {
		p.log.Warn("telegram_admin_notify_failed", "error", err)
	}
}

func (p *ChannelPoster) apiCall(ctx context.Context, method string, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/bot%s/%s", telegramAPIBase, p.token, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))

	var parsed struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("telegram %s: status %d: %s", method, res.StatusCode, string(raw))
	}
	if !parsed.OK {
		msg := parsed.Description
		if msg == "" {
			msg = string(raw)
		}
		return fmt.Errorf("telegram %s: %s", method, msg)
	}
	return nil
}

func buildCaption(a *domain.Auction) string {
	var b strings.Builder
	b.WriteString("<b>")
	b.WriteString(html.EscapeString(a.Title))
	b.WriteString("</b>\n\n")

	desc := strings.TrimSpace(a.Description)
	if desc != "" {
		b.WriteString(html.EscapeString(truncateRunes(desc, maxDescRunes)))
		b.WriteString("\n\n")
	}

	b.WriteString("Entry: <b>")
	b.WriteString(html.EscapeString(formatETB(a.EntranceFee)))
	b.WriteString("</b>")
	if a.RetailValue.IsPositive() {
		b.WriteString("\nWorth: <b>")
		b.WriteString(html.EscapeString(formatETB(a.RetailValue)))
		b.WriteString("</b>")
	}
	if a.CloseAt != nil {
		b.WriteString("\nCloses: ")
		b.WriteString(html.EscapeString(a.CloseAt.UTC().Format("02 Jan 2006 15:04 UTC")))
	}
	b.WriteString("\n\nTap below to open and place your bid.")

	return truncateRunes(b.String(), maxCaptionRunes)
}

func formatETB(m domain.Money) string {
	s := m.String()
	s = strings.TrimSuffix(s, ".00")
	return s + " ETB"
}

func isPublicHTTPURL(u string) bool {
	return strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://")
}

func truncateRunes(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	if max < 1 {
		return ""
	}
	return string(runes[:max-1]) + "…"
}
