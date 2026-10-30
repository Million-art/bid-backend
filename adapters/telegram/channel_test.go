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
		ID:              uuid.New(),
		Title:           "iPhone 16",
		Description:     "Brand new sealed unit",
		EntranceFee:     domain.NewMoney(80),
		RetailValue:     domain.NewMoney(45000),
		MinParticipants: 10,
		CloseAt:         &closeAt,
	}
	cap := buildCaption(a)
	for _, want := range []string{
		"NEW AUCTION LIVE",
		"iPhone 16",
		"Brand new sealed unit",
		"80 ETB",
		"45000 ETB",
		"How it works",
		"Open the auction",
	} {
		if !strings.Contains(cap, want) {
			t.Fatalf("caption missing %q: %s", want, cap)
		}
	}
	if !strings.Contains(cap, "shot at") && !strings.Contains(cap, "Worth") {
		t.Fatalf("expected dynamic value hook in caption: %s", cap)
	}
	btn := auctionCTAButton(a)
	if !strings.Contains(btn, "Enter") && !strings.Contains(btn, "bid") && !strings.Contains(btn, "Bid") {
		t.Fatalf("unexpected CTA button: %s", btn)
	}
}

func TestMiniAppDeepLinks_MainAndShortName(t *testing.T) {
	mainPoster := NewChannelPoster(&env.Config{
		TelegramBotToken:    "123:ABC",
		TelegramChannelID:   "-1001",
		TelegramBotUsername: "carramarket_bot",
	}, nil)
	if got := mainPoster.miniAppDeepLink(); got != "https://t.me/carramarket_bot?startapp" {
		t.Fatalf("unexpected main mini app link: %s", got)
	}
	if got := mainPoster.auctionDeepLink("abc-123"); got != "https://t.me/carramarket_bot?startapp=auction_abc-123" {
		t.Fatalf("unexpected main auction deep link: %s", got)
	}

	shortNamePoster := NewChannelPoster(&env.Config{
		TelegramBotToken:         "123:ABC",
		TelegramChannelID:        "-1001",
		TelegramBotUsername:      "carramarket_bot",
		TelegramMiniAppShortName: "app",
	}, nil)
	if got := shortNamePoster.miniAppDeepLink(); got != "https://t.me/carramarket_bot/app" {
		t.Fatalf("unexpected short name mini app link: %s", got)
	}
	if got := shortNamePoster.auctionDeepLink("abc-123"); got != "https://t.me/carramarket_bot/app?startapp=auction_abc-123" {
		t.Fatalf("unexpected short name auction deep link: %s", got)
	}
}

