//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/testutil"
	"github.com/google/uuid"
)

func TestLUB_Lifecycle_HappyPath(t *testing.T) {
	s := stack(t)
	ctx := context.Background()

	product := testutil.SeedLUBProduct(t, s.PG, testutil.ProductOpts{
		Status:      domain.ProductStatusDraft,
		EntranceFee: domain.NewMoney(100),
		StartInPast: true,
	})

	err := s.LUB.ActivateProduct(ctx, product.ID, uuid.New())
	if err != nil {
		t.Fatalf("ActivateProduct: %v", err)
	}

	product, err = s.LUB.GetProductByID(product.ID)
	if err != nil {
		t.Fatalf("GetProductByID: %v", err)
	}
	if product.Status != domain.ProductStatusActive {
		t.Errorf("expected product status Active, got %s", product.Status)
	}

	participantID := testutil.SeedParticipant(t, s.PG, domain.Zero()).String()
	txRef := "TX-ENTRANCE-" + uuid.New().String()[:8]

	participation, err := s.LUB.RecordParticipation(ctx, product.ID, participantID, "Buyer", "buyer@test.local", "+251911000000", txRef, domain.NewMoney(100))
	if err != nil {
		t.Fatalf("RecordParticipation: %v", err)
	}
	if participation.ID == uuid.Nil {
		t.Fatal("participation ID should be set")
	}

	bidEntry, err := s.LUB.PlaceBid(ctx, product.ID, participantID, domain.NewMoney(50))
	if err != nil {
		t.Fatalf("PlaceBid: %v", err)
	}
	if bidEntry.ID == uuid.Nil {
		t.Fatal("bid entry ID should be set")
	}

	drawResult, err := s.LUB.ExecuteDraw(ctx, product.ID, "manual", uuid.New())
	if err != nil {
		t.Fatalf("ExecuteDraw: %v", err)
	}
	if !drawResult.HasWinner {
		t.Fatal("expected draw to have a winner")
	}
	if drawResult.WinnerParticipant != participantID {
		t.Errorf("expected winner to be our participant, got %s", drawResult.WinnerParticipant)
	}

	product, err = s.LUB.GetProductByID(product.ID)
	if err != nil {
		t.Fatalf("GetProductByID after draw: %v", err)
	}
	if product.Status != domain.ProductStatusExecuted {
		t.Errorf("expected product status Executed, got %s", product.Status)
	}
	if product.WinnerID == nil || *product.WinnerID != participantID {
		t.Errorf("expected winner ID to match participant")
	}
	if product.WinnerBidValue == nil || product.WinnerBidValue.String() != "50.00" {
		t.Errorf("expected winner bid value 50.00, got %v", product.WinnerBidValue)
	}
}

func TestLUB_Lifecycle_NoUniqueBid(t *testing.T) {
	s := stack(t)
	ctx := context.Background()

	product := testutil.SeedLUBProduct(t, s.PG, testutil.ProductOpts{
		Status:      domain.ProductStatusDraft,
		EntranceFee: domain.NewMoney(100),
		StartInPast: true,
	})

	err := s.LUB.ActivateProduct(ctx, product.ID, uuid.New())
	if err != nil {
		t.Fatalf("ActivateProduct: %v", err)
	}

	participantID1 := testutil.SeedParticipant(t, s.PG, domain.Zero()).String()
	participantID2 := testutil.SeedParticipant(t, s.PG, domain.Zero()).String()

	txRef1 := "TX-ENTRANCE-1-" + uuid.New().String()[:8]
	txRef2 := "TX-ENTRANCE-2-" + uuid.New().String()[:8]

	_, err = s.LUB.RecordParticipation(ctx, product.ID, participantID1, "Buyer1", "buyer1@test.local", "+251911000001", txRef1, domain.NewMoney(100))
	if err != nil {
		t.Fatalf("RecordParticipation 1: %v", err)
	}
	_, err = s.LUB.RecordParticipation(ctx, product.ID, participantID2, "Buyer2", "buyer2@test.local", "+251911000002", txRef2, domain.NewMoney(100))
	if err != nil {
		t.Fatalf("RecordParticipation 2: %v", err)
	}

	_, err = s.LUB.PlaceBid(ctx, product.ID, participantID1, domain.NewMoney(50))
	if err != nil {
		t.Fatalf("PlaceBid 1: %v", err)
	}
	_, err = s.LUB.PlaceBid(ctx, product.ID, participantID2, domain.NewMoney(50))
	if err != nil {
		t.Fatalf("PlaceBid 2: %v", err)
	}

	drawResult, err := s.LUB.ExecuteDraw(ctx, product.ID, "manual", uuid.New())
	if err != nil {
		t.Fatalf("ExecuteDraw: %v", err)
	}
	if drawResult.HasWinner {
		t.Fatal("expected no winner when all bids are duplicated")
	}

	product, err = s.LUB.GetProductByID(product.ID)
	if err != nil {
		t.Fatalf("GetProductByID after draw: %v", err)
	}
	if product.Status != domain.ProductStatusClosed {
		t.Errorf("expected product status Closed (no unique bid), got %s", product.Status)
	}
	if product.WinnerID != nil {
		t.Errorf("expected no winner ID when no unique bid")
	}
}

func TestLUB_BidUpdate(t *testing.T) {
	s := stack(t)
	ctx := context.Background()

	product := testutil.SeedLUBProduct(t, s.PG, testutil.ProductOpts{
		Status:      domain.ProductStatusActive,
		EntranceFee: domain.NewMoney(100),
		StartInPast: true,
	})

	participantID := testutil.SeedParticipant(t, s.PG, domain.Zero()).String()
	txRef := "TX-ENTRANCE-" + uuid.New().String()[:8]

	_, err := s.LUB.RecordParticipation(ctx, product.ID, participantID, "Buyer", "buyer@test.local", "+251911000000", txRef, domain.NewMoney(100))
	if err != nil {
		t.Fatalf("RecordParticipation: %v", err)
	}

	bidEntry, err := s.LUB.PlaceBid(ctx, product.ID, participantID, domain.NewMoney(30))
	if err != nil {
		t.Fatalf("PlaceBid: %v", err)
	}

	updatedEntry, err := s.LUB.UpdateBid(ctx, bidEntry.ID, participantID, domain.NewMoney(40))
	if err != nil {
		t.Fatalf("UpdateBid: %v", err)
	}
	if updatedEntry.BidValue.String() != "40.00" {
		t.Errorf("expected updated bid value 40.00, got %s", updatedEntry.BidValue.String())
	}

	entries, err := s.LUB.ListMyBids(product.ID, participantID)
	if err != nil {
		t.Fatalf("ListMyBids: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 bid entry after update, got %d", len(entries))
	}
	if entries[0].BidValue.String() != "40.00" {
		t.Errorf("expected updated bid value 40.00 in list, got %s", entries[0].BidValue.String())
	}
}

func TestLUB_ParticipationIdempotency(t *testing.T) {
	s := stack(t)
	ctx := context.Background()

	product := testutil.SeedLUBProduct(t, s.PG, testutil.ProductOpts{
		Status:      domain.ProductStatusActive,
		EntranceFee: domain.NewMoney(100),
		StartInPast: true,
	})

	participantID := testutil.SeedParticipant(t, s.PG, domain.Zero()).String()
	txRef := "TX-IDEMPOTENT-" + uuid.New().String()[:8]

	part1, err := s.LUB.RecordParticipation(ctx, product.ID, participantID, "Buyer", "buyer@test.local", "+251911000000", txRef, domain.NewMoney(100))
	if err != nil {
		t.Fatalf("RecordParticipation 1: %v", err)
	}

	part2, err := s.LUB.RecordParticipation(ctx, product.ID, participantID, "Buyer", "buyer@test.local", "+251911000000", txRef, domain.NewMoney(100))
	if err != nil {
		t.Fatalf("RecordParticipation 2: %v", err)
	}

	if part1.ID != part2.ID {
		t.Errorf("expected same participation ID for duplicate txRef, got %s and %s", part1.ID, part2.ID)
	}

	var count int64
	if err := s.PG.DB.Table("lub_participations").Where("product_id = ? AND participant_id = ?", product.ID, participantID).Count(&count).Error; err != nil {
		t.Fatalf("count participations: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 participation record, got %d", count)
	}
}
