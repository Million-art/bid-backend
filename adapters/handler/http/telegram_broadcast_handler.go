package http

import (
	"strings"

	"github.com/bid-hub/admin-backend/adapters/handler/response"
	telegramadapter "github.com/bid-hub/admin-backend/adapters/telegram"
	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/core/ports"
	"github.com/bid-hub/admin-backend/platform/validate"
	"github.com/gofiber/fiber/v3"
)

type TelegramBroadcastHandler struct {
	poster      *telegramadapter.ChannelPoster
	users       ports.UserRepository
	subscribers ports.TelegramSubscriberRepository
}

func NewTelegramBroadcastHandler(
	poster *telegramadapter.ChannelPoster,
	users ports.UserRepository,
	subscribers ports.TelegramSubscriberRepository,
) *TelegramBroadcastHandler {
	return &TelegramBroadcastHandler{
		poster:      poster,
		users:       users,
		subscribers: subscribers,
	}
}

type broadcastRequest struct {
	Target     string `json:"target" validate:"required,oneof=channel bot"`
	Text       string `json:"text"`
	MediaType  string `json:"mediaType" validate:"omitempty,oneof=none photo video animation"`
	FileID     string `json:"fileId"`
	ButtonText string `json:"buttonText"`
	ButtonURL  string `json:"buttonUrl"`
}

func (h *TelegramBroadcastHandler) requireStaffMutator(c fiber.Ctx) error {
	role := GetUserRole(c)
	if role != domain.RoleSuperAdmin && role != domain.RoleAdmin {
		return response.Error(c, fiber.StatusForbidden, "Forbidden: Admin access required", nil)
	}
	return nil
}

func (h *TelegramBroadcastHandler) Stats(c fiber.Ctx) error {
	if err := h.requireStaffMutator(c); err != nil {
		return err
	}
	userIDs, err := h.users.ListTelegramIDs()
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to count telegram users", err)
	}
	subIDs, err := h.subscribers.ListTelegramIDs()
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to count telegram subscribers", err)
	}
	merged := telegramadapter.MergeTelegramIDs(userIDs, subIDs)
	return response.Success(c, fiber.StatusOK, "Telegram broadcast stats", fiber.Map{
		"botAudience":       len(merged),
		"fromUsers":         len(userIDs),
		"fromSubscribers":   len(subIDs),
		"channelConfigured": h.poster != nil && h.poster.ChannelConfigured(),
		"botConfigured":     h.poster != nil && h.poster.BroadcastConfigured(),
	})
}

func (h *TelegramBroadcastHandler) Broadcast(c fiber.Ctx) error {
	if err := h.requireStaffMutator(c); err != nil {
		return err
	}
	if h.poster == nil || !h.poster.BroadcastConfigured() {
		return response.Error(c, fiber.StatusServiceUnavailable, "Telegram bot is not configured", nil)
	}

	var req broadcastRequest
	if err := validate.BindAndValidate(c, &req); err != nil {
		return err
	}

	in := telegramadapter.BroadcastInput{
		Target:     req.Target,
		Text:       req.Text,
		MediaType:  req.MediaType,
		FileID:     req.FileID,
		ButtonText: req.ButtonText,
		ButtonURL:  req.ButtonURL,
	}

	var chatIDs []int64
	if strings.EqualFold(req.Target, telegramadapter.TargetBot) {
		userIDs, err := h.users.ListTelegramIDs()
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to load telegram users", err)
		}
		subIDs, err := h.subscribers.ListTelegramIDs()
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to load telegram subscribers", err)
		}
		chatIDs = telegramadapter.MergeTelegramIDs(userIDs, subIDs)
	}

	result, err := h.poster.Broadcast(c.Context(), in, chatIDs)
	if err != nil && (result == nil || result.Sent == 0) {
		return response.Error(c, fiber.StatusBadRequest, err.Error(), err)
	}
	msg := "Broadcast completed"
	if err != nil {
		msg = "Broadcast finished with errors"
	}
	return response.Success(c, fiber.StatusOK, msg, result)
}

// TelegramWebhook records /start (and any private message) so staff can broadcast to bot users.
func (h *TelegramBroadcastHandler) Webhook(c fiber.Ctx) error {
	var update struct {
		Message *struct {
			Text string `json:"text"`
			Chat *struct {
				ID        int64  `json:"id"`
				Username  string `json:"username"`
				FirstName string `json:"first_name"`
				LastName  string `json:"last_name"`
				Type      string `json:"type"`
			} `json:"chat"`
			From *struct {
				ID        int64  `json:"id"`
				Username  string `json:"username"`
				FirstName string `json:"first_name"`
				LastName  string `json:"last_name"`
			} `json:"from"`
		} `json:"message"`
	}
	if err := c.Bind().Body(&update); err != nil {
		return c.SendStatus(fiber.StatusOK)
	}
	msg := update.Message
	if msg == nil || msg.From == nil || msg.From.ID == 0 {
		return c.SendStatus(fiber.StatusOK)
	}
	// Only track private chats (bot DMs / Start).
	if msg.Chat != nil && msg.Chat.Type != "" && msg.Chat.Type != "private" {
		return c.SendStatus(fiber.StatusOK)
	}

	name := strings.TrimSpace(msg.From.FirstName + " " + msg.From.LastName)
	_ = h.subscribers.Upsert(&domain.TelegramSubscriber{
		TelegramID:       msg.From.ID,
		TelegramUsername: msg.From.Username,
		FullName:         name,
	})
	return c.SendStatus(fiber.StatusOK)
}
