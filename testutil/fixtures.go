//go:build integration

package testutil

import (
	"fmt"
	"testing"
	"time"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/google/uuid"
)

func SeedParticipant(t testing.TB, pg *PG, availableBalance domain.Money) uuid.UUID {
	t.Helper()
	id := uuid.New()
	user := &domain.ParticipantUser{
		ID:               id.String(),
		Email:            fmt.Sprintf("user-%s@test.local", id.String()[:8]),
		FullName:         "Test Participant",
		Phone:            "+251922000000",
		AvailableBalance: availableBalance,
		PendingBalance:   domain.Zero(),
		AuthProvider:     "password",
		Status:           "active",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := pg.DB.Create(user).Error; err != nil {
		t.Fatalf("SeedParticipant: %v", err)
	}
	return id
}

type ProductOpts struct {
	Status      domain.ProductStatus
	EntranceFee domain.Money
	MinBidValue domain.Money
	MaxBidValue *domain.Money
	StartInPast bool
}

func SeedLUBProduct(t testing.TB, pg *PG, opts ProductOpts) *domain.Product {
	t.Helper()
	if opts.Status == "" {
		opts.Status = domain.ProductStatusDraft
	}
	if opts.EntranceFee.IsZero() {
		opts.EntranceFee = domain.NewMoney(10)
	}
	if opts.MinBidValue.IsZero() {
		opts.MinBidValue = domain.NewMoney(0.01)
	}
	var start *time.Time
	closeAt := time.Now().UTC().Add(24 * time.Hour)
	if opts.StartInPast {
		t0 := time.Now().UTC().Add(-time.Hour)
		start = &t0
	}
	r := &domain.Product{
		ID:          uuid.New(),
		Name:        "Test Auction " + uuid.New().String()[:6],
		Description: "Integration test auction",
		Status:      opts.Status,
		EntranceFee: opts.EntranceFee,
		MinBidValue: opts.MinBidValue,
		MaxBidValue: opts.MaxBidValue,
		StartAt:     start,
		CloseAt:     &closeAt,
		CreatedBy:   uuid.New(),
	}
	if err := pg.DB.Create(r).Error; err != nil {
		t.Fatalf("SeedLUBProduct: %v", err)
	}
	return r
}

func SeedLUBParticipation(t testing.TB, pg *PG, productID, userID uuid.UUID) *domain.LUBParticipation {
	t.Helper()
	p := &domain.LUBParticipation{
		ID:            uuid.New(),
		ProductID:     productID,
		ParticipantID: userID.String(),
		Status:        domain.LUBParticipationStatusActive,
		TransactionID: "TX-" + uuid.New().String(),
		FeePaid:       domain.NewMoney(10),
	}
	if err := pg.DB.Create(p).Error; err != nil {
		t.Fatalf("SeedLUBParticipation: %v", err)
	}
	return p
}

func SeedLUBBidEntry(t testing.TB, pg *PG, productID uuid.UUID, participantID string, amount domain.Money) *domain.BidEntry {
	t.Helper()
	b := &domain.BidEntry{
		ID:            uuid.New(),
		ProductID:     productID,
		ParticipantID: participantID,
		BidValue:      amount,
	}
	if err := pg.DB.Create(b).Error; err != nil {
		t.Fatalf("SeedLUBBidEntry: %v", err)
	}
	return b
}

func SeedAdminUser(t testing.TB, pg *PG) *domain.User {
	t.Helper()
	user := &domain.User{
		ID:        uuid.New(),
		FullName:  "Test Admin",
		Email:     fmt.Sprintf("admin-%s@test.local", uuid.New().String()[:8]),
		Password:  "hashed-placeholder",
		Phone:     "+251911000000",
		Role:      domain.RoleAdmin,
		Status:    domain.StatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := pg.DB.Create(user).Error; err != nil {
		t.Fatalf("SeedAdminUser: %v", err)
	}
	return user
}
