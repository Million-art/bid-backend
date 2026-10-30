package ports

import "context"

// AdminNotifier sends operational alerts to the configured Telegram admin chat.
type AdminNotifier interface {
	NotifyAdmin(ctx context.Context, text string)
}

// TelegramUserMessenger DMs a participant via the Telegram bot (by chat / telegram id).
type TelegramUserMessenger interface {
	NotifyUser(ctx context.Context, chatID int64, text, buttonLabel string) error
}
