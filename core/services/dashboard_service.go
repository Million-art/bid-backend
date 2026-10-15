package services

import (
	"time"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/core/ports"
)

type DashboardStats struct {
	TotalProducts         int64            `json:"totalProducts"`
	TotalParticipants     int64            `json:"totalParticipants"`
	TotalRevenue          float64          `json:"totalRevenue"`
	PendingRevenue        float64          `json:"pendingRevenue"`
	StatusBreakdown       map[string]int64 `json:"statusBreakdown"`
	PendingProductsCount  int64            `json:"pendingProductsCount"`
	ActiveProductsCount   int64            `json:"activeProductsCount"`
	ExecutedProductsCount int64            `json:"executedProductsCount"`
}

type DashboardService struct {
	userRepo            ports.UserRepository
	productRepo         ports.LUBProductRepository
	participantUserRepo ports.ParticipantUserRepository
	auditRepo           ports.AuditLogRepository
}

func NewDashboardService(
	u ports.UserRepository,
	p ports.LUBProductRepository,
	pu ports.ParticipantUserRepository,
	a ports.AuditLogRepository,
) *DashboardService {
	return &DashboardService{
		userRepo:            u,
		productRepo:         p,
		participantUserRepo: pu,
		auditRepo:           a,
	}
}

func (s *DashboardService) GetStats() (*DashboardStats, error) {
	var totalProducts int64
	statusBreakdown := make(map[string]int64)
	var pendingProductsCount, activeProductsCount, executedProductsCount int64

	statuses := []domain.ProductStatus{
		domain.ProductStatusDraft,
		domain.ProductStatusActive,
		domain.ProductStatusPostponed,
		domain.ProductStatusClosed,
		domain.ProductStatusExecuted,
		domain.ProductStatusCancelled,
	}

	if s.productRepo != nil {
		_, total, err := s.productRepo.List("", "", 1, 0)
		if err != nil {
			return nil, err
		}
		totalProducts = total

		for _, st := range statuses {
			_, count, _ := s.productRepo.List(string(st), "", 1, 0)
			statusBreakdown[string(st)] = count
			switch st {
			case domain.ProductStatusDraft:
				pendingProductsCount = count
			case domain.ProductStatusActive, domain.ProductStatusPostponed:
				activeProductsCount += count
			case domain.ProductStatusExecuted:
				executedProductsCount = count
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

	return &DashboardStats{
		TotalProducts:         totalProducts,
		TotalParticipants:     totalParticipants,
		TotalRevenue:          0,
		PendingRevenue:        0,
		StatusBreakdown:       statusBreakdown,
		PendingProductsCount:  pendingProductsCount,
		ActiveProductsCount:   activeProductsCount,
		ExecutedProductsCount: executedProductsCount,
	}, nil
}

func (s *DashboardService) ListAuditLogs(filter map[string]interface{}, page, limit int) ([]domain.AuditLog, int64, error) {
	if s.auditRepo == nil {
		return nil, 0, nil
	}
	offset := (page - 1) * limit
	return s.auditRepo.List(filter, limit, offset)
}

func (s *DashboardService) GetPlatformRevenueChart() (*ChartDataResponse, error) {
	return fillMonthlyChart(nil, chartMonths, func(m domain.MonthlyAmount) float64 { return m.Amount.Float64() }), nil
}

func (s *DashboardService) GetUserGrowthChart() (*ChartDataResponse, error) {
	return fillMonthlyCountChart(nil, chartMonths), nil
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
