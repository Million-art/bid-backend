package http

import (
	"strings"

	"github.com/bid-hub/admin-backend/adapters/handler/response"
	telegramadapter "github.com/bid-hub/admin-backend/adapters/telegram"
	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/core/ports"
	"github.com/bid-hub/admin-backend/core/services"
	"github.com/bid-hub/admin-backend/platform/validate"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type TelegramBroadcastHandler struct {
	poster *telegramadapter.ChannelPoster
	users  ports.UserRepository
}

func NewTelegramBroadcastHandler(
	poster *telegramadapter.ChannelPoster,
	users ports.UserRepository,
) *TelegramBroadcastHandler {
	return &TelegramBroadcastHandler{
		poster: poster,
		users:  users,
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
	return response.Success(c, fiber.StatusOK, "Telegram broadcast stats", fiber.Map{
		"botAudience":       len(userIDs),
		"fromUsers":         len(userIDs),
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
		ids, err := h.users.ListTelegramIDs()
		if err != nil {
			return response.Error(c, fiber.StatusInternalServerError, "Failed to load telegram users", err)
		}
		chatIDs = ids
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

type shareContactRequest struct {
	TelegramID int64  `json:"telegramId" validate:"required,gt=0"`
	Phone      string `json:"phone" validate:"required"`
	FullName   string `json:"fullName"`
	Username   string `json:"username"`
}

// ShareContact saves a phone shared from the Telegram Mini App onto users.
func (h *TelegramBroadcastHandler) ShareContact(c fiber.Ctx) error {
	var req shareContactRequest
	if err := validate.BindAndValidate(c, &req); err != nil {
		return err
	}
	normalized, _ := services.NormalizePhone(req.Phone)
	if normalized == "" {
		normalized = strings.TrimSpace(req.Phone)
	}

	user, err := h.users.UpsertTelegramProfile(
		req.TelegramID,
		req.Username,
		req.FullName,
		normalized,
	)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to save contact", err)
	}

	// If a different logged-in account is open, also attach phone/telegram when blank.
	if uid := strings.TrimSpace(GetUserID(c)); uid != "" {
		if userID, err := uuid.Parse(uid); err == nil && user != nil && user.ID != userID {
			if loggedIn, err := h.users.GetByID(userID); err == nil && loggedIn != nil {
				changed := false
				if strings.TrimSpace(loggedIn.Phone) == "" {
					loggedIn.Phone = normalized
					changed = true
				}
				if loggedIn.TelegramID == nil {
					tg := req.TelegramID
					loggedIn.TelegramID = &tg
					changed = true
				}
				if changed {
					_ = h.users.Update(loggedIn)
				}
			}
		}
	}

	return response.Success(c, fiber.StatusOK, "Contact saved", fiber.Map{
		"telegramId": req.TelegramID,
		"phone":      normalized,
		"userId":     user.ID,
	})
}

// TelegramWebhook records /start and shared contacts onto the users table.
func (h *TelegramBroadcastHandler) Webhook(c fiber.Ctx) error {
	var update struct {
		Message *struct {
			Text    string `json:"text"`
			Contact *struct {
				PhoneNumber string `json:"phone_number"`
				FirstName   string `json:"first_name"`
				LastName    string `json:"last_name"`
				UserID      int64  `json:"user_id"`
			} `json:"contact"`
			Chat *struct {
				Type string `json:"type"`
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
	if msg.Chat != nil && msg.Chat.Type != "" && msg.Chat.Type != "private" {
		return c.SendStatus(fiber.StatusOK)
	}

	name := strings.TrimSpace(msg.From.FirstName + " " + msg.From.LastName)
	phone := ""
	if msg.Contact != nil && strings.TrimSpace(msg.Contact.PhoneNumber) != "" {
		phone, _ = services.NormalizePhone(msg.Contact.PhoneNumber)
		if phone == "" {
			phone = strings.TrimSpace(msg.Contact.PhoneNumber)
		}
		if cn := strings.TrimSpace(msg.Contact.FirstName + " " + msg.Contact.LastName); cn != "" {
			name = cn
		}
	}

	_, _ = h.users.UpsertTelegramProfile(msg.From.ID, msg.From.Username, name, phone)
	return c.SendStatus(fiber.StatusOK)
}
