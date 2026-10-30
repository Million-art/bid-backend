package services

import (
	"context"
	"time"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/core/ports"
)

type DashboardStats struct {
	TotalAuctions         int64            `json:"totalAuctions"`
	TotalParticipants     int64            `json:"totalParticipants"`
	TotalRevenue          float64          `json:"totalRevenue"`
	PendingRevenue        float64          `json:"pendingRevenue"`
	StatusBreakdown       map[string]int64 `json:"statusBreakdown"`
	PendingAuctionsCount  int64            `json:"pendingAuctionsCount"`
	ActiveAuctionsCount   int64            `json:"activeAuctionsCount"`
	ExecutedAuctionsCount int64            `json:"executedAuctionsCount"`

	// Transaction monitoring snapshot
	PaymentsSuccess        int64 `json:"paymentsSuccess"`
	PaymentsFailed         int64 `json:"paymentsFailed"`
	PaymentsPending        int64 `json:"paymentsPending"`
	PaymentsRefunded       int64 `json:"paymentsRefunded"`
	PaymentsUnprocessed    int64 `json:"paymentsUnprocessed"`
	ParticipationsTotal    int64 `json:"participationsTotal"`
}

type DashboardService struct {
	userRepo            ports.UserRepository
	auctionRepo         ports.AuctionRepository
	participantUserRepo ports.ParticipantUserRepository
	auditRepo           ports.AuditLogRepository
	paymentRepo         ports.PaymentLedgerRepository
	participationRepo   ports.AuctionParticipationRepository
	bidRepo             ports.BidRepository
}

func NewDashboardService(
	u ports.UserRepository,
	a ports.AuctionRepository,
	pu ports.ParticipantUserRepository,
	au ports.AuditLogRepository,
	paymentRepo ports.PaymentLedgerRepository,
	participationRepo ports.AuctionParticipationRepository,
	bidRepo ports.BidRepository,
) *DashboardService {
	return &DashboardService{
		userRepo:            u,
		auctionRepo:         a,
		participantUserRepo: pu,
		auditRepo:           au,
		paymentRepo:         paymentRepo,
		participationRepo:   participationRepo,
		bidRepo:             bidRepo,
	}
}

func (s *DashboardService) GetStats() (*DashboardStats, error) {
	ctx := context.Background()
	var totalAuctions int64
	statusBreakdown := make(map[string]int64)
	var pendingAuctionsCount, activeAuctionsCount, executedAuctionsCount int64

	statuses := []domain.AuctionStatus{
		domain.AuctionStatusDraft,
		domain.AuctionStatusActive,
		domain.AuctionStatusPostponed,
		domain.AuctionStatusClosed,
		domain.AuctionStatusExecuted,
		domain.AuctionStatusCancelled,
	}

	if s.auctionRepo != nil {
		_, total, err := s.auctionRepo.List("", "", 1, 0)
		if err != nil {
			return nil, err
		}
		totalAuctions = total

		for _, st := range statuses {
			_, count, _ := s.auctionRepo.List(string(st), "", 1, 0)
			statusBreakdown[string(st)] = count
			switch st {
			case domain.AuctionStatusDraft:
				pendingAuctionsCount = count
			case domain.AuctionStatusActive, domain.AuctionStatusPostponed:
				activeAuctionsCount += count
			case domain.AuctionStatusExecuted:
				executedAuctionsCount = count
			}
		}
	}

	var totalParticipants int64
	if s.participantUserRepo != nil {
		var err error
		totalParticipants, err = s.participantUserRepo.Count(map[string]interface{}{})
		if err != nil {
			return nil, err
		}
	}

	stats := &DashboardStats{
		TotalAuctions:         totalAuctions,
		TotalParticipants:     totalParticipants,
		StatusBreakdown:       statusBreakdown,
		PendingAuctionsCount:  pendingAuctionsCount,
		ActiveAuctionsCount:   activeAuctionsCount,
		ExecutedAuctionsCount: executedAuctionsCount,
	}

	if s.paymentRepo != nil {
		if sum, err := s.paymentRepo.SumAmountByStatus(ctx, domain.PaymentStatusSuccess); err == nil {
			stats.TotalRevenue = sum.Float64()
		}
		if sum, err := s.paymentRepo.SumAmountByStatus(ctx, domain.PaymentStatusPending); err == nil {
			stats.PendingRevenue = sum.Float64()
		}
		stats.PaymentsSuccess, _ = s.paymentRepo.CountByStatus(ctx, domain.PaymentStatusSuccess)
		stats.PaymentsFailed, _ = s.paymentRepo.CountByStatus(ctx, domain.PaymentStatusFailed)
		stats.PaymentsPending, _ = s.paymentRepo.CountByStatus(ctx, domain.PaymentStatusPending)
		stats.PaymentsRefunded, _ = s.paymentRepo.CountByStatus(ctx, domain.PaymentStatusRefunded)
		stats.PaymentsUnprocessed, _ = s.paymentRepo.CountSuccessUnprocessed(ctx)
	}

	if s.participationRepo != nil {
		// Approximate total via monthly sum is awkward; count active rows.
		if rows, err := s.participationRepo.GetCountByMonth(120); err == nil {
			var n int64
			for _, r := range rows {
				n += r.Count
			}
			stats.ParticipationsTotal = n
		}
	}

	return stats, nil
}

func (s *DashboardService) ListAuditLogs(filter map[string]interface{}, page, limit int) ([]domain.AuditLog, int64, error) {
	if s.auditRepo == nil {
		return nil, 0, nil
	}
	offset := (page - 1) * limit
	return s.auditRepo.List(filter, limit, offset)
}

func (s *DashboardService) GetPlatformRevenueChart() (*ChartDataResponse, error) {
	ctx := context.Background()
	var rows []domain.MonthlyAmount
	if s.paymentRepo != nil {
		var err error
		rows, err = s.paymentRepo.GetRevenueByMonth(ctx, chartMonths)
		if err != nil {
			return nil, err
		}
	}
	return fillMonthlyChart(rows, chartMonths, func(m domain.MonthlyAmount) float64 { return m.Amount.Float64() }), nil
}

func (s *DashboardService) GetUserGrowthChart() (*ChartDataResponse, error) {
	var rows []domain.MonthlyCount
	if s.participantUserRepo != nil {
		var err error
		rows, err = s.participantUserRepo.GetCountByMonth(chartMonths)
		if err != nil {
			return nil, err
		}
	}
	return fillMonthlyCountChart(rows, chartMonths), nil
}

func (s *DashboardService) GetParticipationsChart() (*ChartDataResponse, error) {
	var rows []domain.MonthlyCount
	if s.participationRepo != nil {
		var err error
		rows, err = s.participationRepo.GetCountByMonth(chartMonths)
		if err != nil {
			return nil, err
		}
	}
	return fillMonthlyCountChart(rows, chartMonths), nil
}

func (s *DashboardService) GetBidsChart() (*ChartDataResponse, error) {
	var rows []domain.MonthlyCount
	if s.bidRepo != nil {
		var err error
		rows, err = s.bidRepo.GetCountByMonth(chartMonths)
		if err != nil {
			return nil, err
		}
	}
	return fillMonthlyCountChart(rows, chartMonths), nil
}

func (s *DashboardService) GetPaymentsChart() (*ChartDataResponse, error) {
	ctx := context.Background()
	var rows []domain.MonthlyCount
	if s.paymentRepo != nil {
		var err error
		rows, err = s.paymentRepo.GetCountByMonth(ctx, domain.PaymentStatusSuccess, chartMonths)
		if err != nil {
			return nil, err
		}
	}
	return fillMonthlyCountChart(rows, chartMonths), nil
}

type ChartDataResponse struct {
	Labels []string  `json:"labels"`
	Values []float64 `json:"values"`
}

const chartMonths = 12

func fillMonthlyChart(rows []domain.MonthlyAmount, months int, valueFn func(domain.MonthlyAmount) float64) *ChartDataResponse {
	byMonth := make(map[string]float64)
	for _, r := range rows {
		byMonth[r.Month] = valueFn(r)
	}
	labels := make([]string, 0, months)
	values := make([]float64, 0, months)
	monthNames := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	now := time.Now()
	for i := months - 1; i >= 0; i-- {
		d := now.AddDate(0, -i, 0)
		ym := d.Format("2006-01")
		labels = append(labels, monthNames[d.Month()-1]+" "+d.Format("2006"))
		values = append(values, byMonth[ym])
	}
	return &ChartDataResponse{Labels: labels, Values: values}
}

func fillMonthlyCountChart(rows []domain.MonthlyCount, months int) *ChartDataResponse {
	byMonth := make(map[string]int64)
	for _, r := range rows {
		byMonth[r.Month] = r.Count
	}
	labels := make([]string, 0, months)
	values := make([]float64, 0, months)
	monthNames := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	now := time.Now()
	for i := months - 1; i >= 0; i-- {
		d := now.AddDate(0, -i, 0)
		ym := d.Format("2006-01")
		labels = append(labels, monthNames[d.Month()-1]+" "+d.Format("2006"))
		values = append(values, float64(byMonth[ym]))
	}
	return &ChartDataResponse{Labels: labels, Values: values}
}
