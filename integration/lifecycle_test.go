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

	auction := testutil.SeedAuction(t, s.PG, testutil.AuctionOpts{
		Status:      domain.AuctionStatusDraft,
		EntranceFee: domain.NewMoney(100),
		StartInPast: true,
	})

	err := s.LUB.ActivateAuction(ctx, auction.ID, uuid.New())
	if err != nil {
		t.Fatalf("ActivateAuction: %v", err)
	}

	auction, err = s.LUB.GetAuctionByID(auction.ID)
	if err != nil {
		t.Fatalf("GetAuctionByID: %v", err)
	}
	if auction.Status != domain.AuctionStatusActive {
		t.Errorf("expected auction status Active, got %s", auction.Status)
	}

	participantID := testutil.SeedParticipant(t, s.PG, domain.Zero()).String()
	txRef := "TX-ENTRANCE-" + uuid.New().String()[:8]

	participation, err := s.LUB.RecordParticipation(ctx, auction.ID, participantID, txRef, domain.NewMoney(100))
	if err != nil {
		t.Fatalf("RecordParticipation: %v", err)
	}
	if participation.ID == uuid.Nil {
		t.Fatal("participation ID should be set")
	}

	bid, err := s.LUB.PlaceBid(ctx, auction.ID, participantID, domain.NewMoney(50))
	if err != nil {
		t.Fatalf("PlaceBid: %v", err)
	}
	if bid.ID == uuid.Nil {
		t.Fatal("bid ID should be set")
	}

	drawResult, err := s.LUB.ExecuteDraw(ctx, auction.ID, "manual", uuid.New())
	if err != nil {
		t.Fatalf("ExecuteDraw: %v", err)
	}
	if !drawResult.HasWinner {
		t.Fatal("expected draw to have a winner")
	}
	if drawResult.WinnerParticipant != participantID {
		t.Errorf("expected winner to be our participant, got %s", drawResult.WinnerParticipant)
	}

	auction, err = s.LUB.GetAuctionByID(auction.ID)
	if err != nil {
		t.Fatalf("GetAuctionByID after draw: %v", err)
	}
	if auction.Status != domain.AuctionStatusExecuted {
		t.Errorf("expected auction status Executed, got %s", auction.Status)
	}
	if auction.WinnerParticipantID == nil || *auction.WinnerParticipantID != participantID {
		t.Errorf("expected winner ID to match participant")
	}
	if auction.WinnerBidValue == nil || auction.WinnerBidValue.String() != "50.00" {
		t.Errorf("expected winner bid value 50.00, got %v", auction.WinnerBidValue)
	}
}

func TestLUB_Lifecycle_NoUniqueBid(t *testing.T) {
	s := stack(t)
	ctx := context.Background()

	auction := testutil.SeedAuction(t, s.PG, testutil.AuctionOpts{
		Status:      domain.AuctionStatusDraft,
		EntranceFee: domain.NewMoney(100),
		StartInPast: true,
	})

	err := s.LUB.ActivateAuction(ctx, auction.ID, uuid.New())
	if err != nil {
		t.Fatalf("ActivateAuction: %v", err)
	}

	participantID1 := testutil.SeedParticipant(t, s.PG, domain.Zero()).String()
	participantID2 := testutil.SeedParticipant(t, s.PG, domain.Zero()).String()

	txRef1 := "TX-ENTRANCE-1-" + uuid.New().String()[:8]
	txRef2 := "TX-ENTRANCE-2-" + uuid.New().String()[:8]

	_, err = s.LUB.RecordParticipation(ctx, auction.ID, participantID1, txRef1, domain.NewMoney(100))
	if err != nil {
		t.Fatalf("RecordParticipation 1: %v", err)
	}
	_, err = s.LUB.RecordParticipation(ctx, auction.ID, participantID2, txRef2, domain.NewMoney(100))
	if err != nil {
		t.Fatalf("RecordParticipation 2: %v", err)
	}

	_, err = s.LUB.PlaceBid(ctx, auction.ID, participantID1, domain.NewMoney(50))
	if err != nil {
		t.Fatalf("PlaceBid 1: %v", err)
	}
	_, err = s.LUB.PlaceBid(ctx, auction.ID, participantID2, domain.NewMoney(50))
	if err != nil {
		t.Fatalf("PlaceBid 2: %v", err)
	}

	drawResult, err := s.LUB.ExecuteDraw(ctx, auction.ID, "manual", uuid.New())
	if err != nil {
		t.Fatalf("ExecuteDraw: %v", err)
	}
	if drawResult.HasWinner {
		t.Fatal("expected no winner when all bids are duplicated")
	}

	auction, err = s.LUB.GetAuctionByID(auction.ID)
	if err != nil {
		t.Fatalf("GetAuctionByID after draw: %v", err)
	}
	if auction.Status != domain.AuctionStatusClosed {
		t.Errorf("expected auction status Closed (no unique bid), got %s", auction.Status)
	}
	if auction.WinnerParticipantID != nil {
		t.Errorf("expected no winner ID when no unique bid")
	}
}

func TestLUB_BidUpdate(t *testing.T) {
	s := stack(t)
	ctx := context.Background()

	auction := testutil.SeedAuction(t, s.PG, testutil.AuctionOpts{
		Status:      domain.AuctionStatusActive,
		EntranceFee: domain.NewMoney(100),
		StartInPast: true,
	})

	participantID := testutil.SeedParticipant(t, s.PG, domain.Zero()).String()
	txRef := "TX-ENTRANCE-" + uuid.New().String()[:8]

	_, err := s.LUB.RecordParticipation(ctx, auction.ID, participantID, txRef, domain.NewMoney(100))
	if err != nil {
		t.Fatalf("RecordParticipation: %v", err)
	}

	bid, err := s.LUB.PlaceBid(ctx, auction.ID, participantID, domain.NewMoney(30))
	if err != nil {
		t.Fatalf("PlaceBid: %v", err)
	}

	updatedBid, err := s.LUB.UpdateBid(ctx, bid.ID, participantID, domain.NewMoney(40))
	if err != nil {
		t.Fatalf("UpdateBid: %v", err)
	}
	if updatedBid.BidValue.String() != "40.00" {
		t.Errorf("expected updated bid value 40.00, got %s", updatedBid.BidValue.String())
	}

	bids, err := s.LUB.ListMyBids(auction.ID, participantID)
	if err != nil {
		t.Fatalf("ListMyBids: %v", err)
	}
	if len(bids) != 1 {
		t.Errorf("expected 1 bid after update, got %d", len(bids))
	}
	if bids[0].BidValue.String() != "40.00" {
		t.Errorf("expected updated bid value 40.00 in list, got %s", bids[0].BidValue.String())
	}
}

func TestLUB_ParticipationIdempotency(t *testing.T) {
	s := stack(t)
	ctx := context.Background()

	auction := testutil.SeedAuction(t, s.PG, testutil.AuctionOpts{
		Status:      domain.AuctionStatusActive,
		EntranceFee: domain.NewMoney(100),
		StartInPast: true,
	})

	participantID := testutil.SeedParticipant(t, s.PG, domain.Zero()).String()
	txRef := "TX-IDEMPOTENT-" + uuid.New().String()[:8]

	part1, err := s.LUB.RecordParticipation(ctx, auction.ID, participantID, txRef, domain.NewMoney(100))
	if err != nil {
		t.Fatalf("RecordParticipation 1: %v", err)
	}

	part2, err := s.LUB.RecordParticipation(ctx, auction.ID, participantID, txRef, domain.NewMoney(100))
	if err != nil {
		t.Fatalf("RecordParticipation 2: %v", err)
	}

	if part1.ID != part2.ID {
		t.Errorf("expected same participation ID for duplicate txRef, got %s and %s", part1.ID, part2.ID)
	}

	var count int64
	if err := s.PG.DB.Table("auction_participations").Where("auction_id = ? AND participant_id = ?", auction.ID, participantID).Count(&count).Error; err != nil {
		t.Fatalf("count participations: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 participation record, got %d", count)
	}
}
