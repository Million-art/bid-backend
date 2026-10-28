package lub

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/google/uuid"
)

func TestComputeCardTagPriority(t *testing.T) {
	now := time.Now().UTC()
	a := domain.Auction{
		ID:               uuid.New(),
		Status:           domain.AuctionStatusActive,
		CreatedAt:        now.Add(-1 * time.Hour),
		ParticipantCount: 9,
		MinParticipants:  10,
	}

	if got := computeCardTag(a, 5, 5, 10, false, now); got != cardTagHot {
		t.Fatalf("expected hot, got %q", got)
	}
	if got := computeCardTag(a, 1, 5, 10, false, now); got != cardTagAlmostUnlocked {
		t.Fatalf("expected almost_unlocked, got %q", got)
	}

	a.ParticipantCount = 0
	if got := computeCardTag(a, 0, 0, 10, false, now); got != cardTagNew {
		t.Fatalf("expected new, got %q", got)
	}

	a.CreatedAt = now.Add(-72 * time.Hour)
	if got := computeCardTag(a, 0, 0, 10, false, now); got != "" {
		t.Fatalf("expected empty tag, got %q", got)
	}
}

func TestToPublicAuctionCardHidesBidFieldsAndUnlocks(t *testing.T) {
	now := time.Now().UTC()
	closeAt := now.Add(2 * time.Hour)
	a := domain.Auction{
		ID:               uuid.New(),
		Title:            "Phone",
		Status:           domain.AuctionStatusActive,
		EntranceFee:      domain.NewMoney(80),
		RetailValue:      domain.NewMoney(45000),
		MinParticipants:  10,
		ParticipantCount: 10,
		CloseAt:          &closeAt,
		CreatedAt:        now.Add(-72 * time.Hour),
		MinBidValue:      domain.NewMoney(1),
		WinnerBidValue:   ptrMoney(1),
	}

	card := toPublicAuctionCard(a, 0, 0, now)
	if !card.PrizeUnlocked {
		t.Fatal("expected prize unlocked at 100%")
	}
	if card.EndsAt == nil || card.EndsAt.Unix() != closeAt.Unix() {
		t.Fatalf("endsAt mismatch: %#v", card.EndsAt)
	}
	if card.ParticipantCount != 10 || card.MinParticipants != 10 {
		t.Fatalf("unexpected counts: %+v", card)
	}
}

func TestPublicAuctionCardJSONOmitsBidFields(t *testing.T) {
	now := time.Now().UTC()
	closeAt := now.Add(72 * time.Hour)
	a := domain.Auction{
		ID:               uuid.New(),
		Title:            "TV",
		Status:           domain.AuctionStatusActive,
		EntranceFee:      domain.NewMoney(80),
		RetailValue:      domain.NewMoney(12000),
		MinParticipants:  10,
		ParticipantCount: 3,
		CloseAt:          &closeAt,
		CreatedAt:        now.Add(-72 * time.Hour),
		MinBidValue:      domain.NewMoney(1),
		WinnerBidValue:   ptrMoney(1.25),
	}
	card := toPublicAuctionCard(a, 0, 0, now)
	raw, err := json.Marshal(card)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, banned := range []string{
		"minBidValue",
		"maxBidValue",
		"winnerBidValue",
		"winnerBidId",
		"bidValue",
		"totalBids",
	} {
		if strings.Contains(s, banned) {
			t.Fatalf("list card JSON must not contain %q: %s", banned, s)
		}
	}
	for _, required := range []string{
		"participantCount",
		"minParticipants",
		"retailValue",
		"entranceFee",
		"endsAt",
		"createdAt",
		"prizeUnlocked",
	} {
		if !strings.Contains(s, required) {
			t.Fatalf("list card JSON missing %q: %s", required, s)
		}
	}
}

func ptrMoney(v float64) *domain.Money {
	m := domain.NewMoney(v)
	return &m
}
