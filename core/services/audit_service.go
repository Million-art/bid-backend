package services

import (
	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/core/ports"
	"github.com/google/uuid"
)

type AuditLogService struct {
	repo ports.AuditLogRepository
}

func NewAuditLogService(repo ports.AuditLogRepository) *AuditLogService {
	return &AuditLogService{repo: repo}
}

func (s *AuditLogService) Log(log *domain.AuditLog, ctx *domain.AuditContext) error {
	if ctx != nil {
		log.IPAddress = ctx.IPAddress
		log.UserAgent = ctx.UserAgent
		if log.ActorRole == "" {
			log.ActorRole = ctx.ActorRole
		}
		if log.ActorEmail == "" {
			log.ActorEmail = ctx.ActorEmail
		}
	}
	if log.UserID == uuid.Nil {
		return nil
	}
	return s.repo.Create(log)
}
