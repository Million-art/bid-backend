//go:build integration

package integration

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/testutil"
)

func TestRecordParticipation_ConcurrentSameParticipant_IsIdempotent(t *testing.T) {
	s := stack(t)

	auction := testutil.SeedAuction(t, s.PG, testutil.AuctionOpts{
		Status:      domain.AuctionStatusActive,
		EntranceFee: domain.NewMoney(10),
		StartInPast: true,
	})
	participantID := uuid.New().String()

	const concurrentBuyers = 20
	var okCount int32
	var wg sync.WaitGroup
	wg.Add(concurrentBuyers)
	for i := 0; i < concurrentBuyers; i++ {
		i := i
		go func() {
			defer wg.Done()
			txID := fmt.Sprintf("TX-RACE-%d-%s", i, uuid.New().String())
			_, err := s.LUB.RecordParticipation(context.Background(), auction.ID, participantID, txID, domain.NewMoney(10))
			if err == nil {
				atomic.AddInt32(&okCount, 1)
			}
		}()
	}
	wg.Wait()

	var partCount int64
	if err := s.PG.DB.Table("auction_participations").Where("auction_id = ?", auction.ID).Count(&partCount).Error; err != nil {
		t.Fatalf("count participations: %v", err)
	}
	if partCount != 1 {
		t.Fatalf("participation rows: want 1, got %d", partCount)
	}
}

func TestRecordParticipation_DuplicateTxRef_IsIdempotent(t *testing.T) {
	s := stack(t)

	auction := testutil.SeedAuction(t, s.PG, testutil.AuctionOpts{
		Status:      domain.AuctionStatusActive,
		EntranceFee: domain.NewMoney(10),
		StartInPast: true,
	})
	participantID := uuid.New().String()
	txRef := "TXREF-DUPE-" + uuid.New().String()[:8]

	if _, err := s.LUB.RecordParticipation(context.Background(), auction.ID, participantID, txRef, domain.NewMoney(10)); err != nil {
		t.Fatalf("first call: %v", err)
	}

	if _, err := s.LUB.RecordParticipation(context.Background(), auction.ID, participantID, txRef, domain.NewMoney(10)); err != nil {
		t.Fatalf("second call should be idempotent, got: %v", err)
	}

	var count int64
	if err := s.PG.DB.Table("auction_participations").Where("auction_id = ? AND participant_id = ?", auction.ID, participantID).Count(&count).Error; err != nil {
		t.Fatalf("count participations: %v", err)
	}
	if count != 1 {
		t.Errorf("duplicate delivery produced %d participations (want 1)", count)
	}
}

func TestRecordParticipation_AuctionClosed_Rejects(t *testing.T) {
	s := stack(t)

	auction := testutil.SeedAuction(t, s.PG, testutil.AuctionOpts{
		Status:      domain.AuctionStatusClosed,
		EntranceFee: domain.NewMoney(10),
		StartInPast: true,
	})
	participantID := uuid.New().String()
	txRef := "TXREF-CLOSED-" + uuid.New().String()[:8]

	_, err := s.LUB.RecordParticipation(context.Background(), auction.ID, participantID, txRef, domain.NewMoney(10))
	if err == nil {
		t.Fatalf("expected participation on closed auction to be rejected")
	}

	var c int64
	_ = s.PG.DB.Table("auction_participations").Where("auction_id = ?", auction.ID).Count(&c).Error
	if c != 0 {
		t.Errorf("closed auction accepted a participation (got %d)", c)
	}
}
