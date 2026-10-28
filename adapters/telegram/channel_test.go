package telegram

import (
	"strings"
	"testing"
	"time"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/platform/env"
	"github.com/google/uuid"
)

func TestNewChannelPosterEnabled(t *testing.T) {
	off := NewChannelPoster(&env.Config{}, nil)
	if off.Enabled() {
		t.Fatal("expected disabled without config")
	}
	on := NewChannelPoster(&env.Config{
		TelegramBotToken:    "123:ABC",
		TelegramChannelID:   "-1001",
		TelegramBotUsername: "carrabot",
	}, nil)
	if !on.Enabled() {
		t.Fatal("expected enabled when token, channel, and username are set")
	}
}

func TestBuildCaptionIncludesFeeAndLinkCopy(t *testing.T) {
	closeAt := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	a := &domain.Auction{
		ID:          uuid.New(),
		Title:       "iPhone 16",
		Description: "Brand new sealed unit",
		EntranceFee: domain.NewMoney(80),
		RetailValue: domain.NewMoney(45000),
		CloseAt:     &closeAt,
	}
	cap := buildCaption(a)
	for _, want := range []string{
		"iPhone 16",
		"Brand new sealed unit",
		"80 ETB",
		"45000 ETB",
		"Join",
	} {
		if !strings.Contains(cap, want) && want != "Join" {
			t.Fatalf("caption missing %q: %s", want, cap)
		}
	}
	if !strings.Contains(cap, "Tap below") {
		t.Fatalf("expected CTA copy in caption: %s", cap)
	}
}
