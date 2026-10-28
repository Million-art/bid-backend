package domain

import "time"

// TelegramSubscriber is a chat that pressed Start (or opened the mini app) on the bot.
type TelegramSubscriber struct {
	TelegramID       int64     `gorm:"primaryKey" json:"telegramId"`
	TelegramUsername string    `gorm:"type:varchar(100);index" json:"telegramUsername,omitempty"`
	FullName         string    `gorm:"type:varchar(255)" json:"fullName,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

func (TelegramSubscriber) TableName() string { return "telegram_subscribers" }
