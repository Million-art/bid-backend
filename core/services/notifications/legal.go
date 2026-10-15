package notifications

import (
	"context"
	"fmt"
	"strings"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/google/uuid"
)

// NotifyLegalDocumentPublished creates in-app notifications for all agents and participants.
func (s *Service) NotifyLegalDocumentPublished(ctx context.Context, docType string) error {
	docType = strings.ToLower(strings.TrimSpace(docType))
	var title, msg, path string
	switch docType {
	case "privacy":
		title = "Privacy Policy updated"
		msg = "We've published an updated Privacy Policy. Please review the latest version."
		path = "/privacy"
	case "terms":
		title = "Terms & Conditions updated"
		msg = "We've published updated Terms & Conditions. Please review the latest version."
		path = "/terms"
	default:
		return fmt.Errorf("unknown document type: %s", docType)
	}

	for page := 1; ; page++ {
		users, total, err := s.userRepo.ListByRoles([]domain.UserRole{domain.RoleAgent}, page, legalNotifyBatchSize)
		if err != nil {
			return fmt.Errorf("list agents: %w", err)
		}
		if len(users) == 0 {
			break
		}
		batch := make([]*domain.Notification, 0, len(users))
		for i := range users {
			u := users[i]
			batch = append(batch, &domain.Notification{
				UserID:      u.ID,
				UserType:    domain.UserTypeAgent,
				Type:        domain.NotificationTypeLegalDocumentUpdated,
				Title:       title,
				Message:     msg,
				ActionURL:   path,
				ActionLabel: "View",
				Priority:    domain.NotificationPriorityNormal,
			})
		}
		if err := s.flushLegalNotificationBatch(ctx, batch); err != nil {
			return err
		}
		if int64(page*legalNotifyBatchSize) >= total {
			break
		}
	}

	off := 0
	for {
		ids, total, err := s.participantUserRepo.ListAllIDs(legalNotifyBatchSize, off)
		if err != nil {
			return fmt.Errorf("list participants: %w", err)
		}
		if len(ids) == 0 {
			break
		}
		batch := make([]*domain.Notification, 0, len(ids))
		for _, idStr := range ids {
			pid, err := uuid.Parse(idStr)
			if err != nil {
				continue
			}
			batch = append(batch, &domain.Notification{
				UserID:      pid,
				UserType:    domain.UserTypeParticipant,
				Type:        domain.NotificationTypeLegalDocumentUpdated,
				Title:       title,
				Message:     msg,
				ActionURL:   path,
				ActionLabel: "View",
				Priority:    domain.NotificationPriorityNormal,
			})
		}
		if err := s.flushLegalNotificationBatch(ctx, batch); err != nil {
			return err
		}
		off += len(ids)
		if int64(off) >= total {
			break
		}
	}

	return nil
}

func (s *Service) flushLegalNotificationBatch(ctx context.Context, batch []*domain.Notification) error {
	if len(batch) == 0 {
		return nil
	}
	if err := s.notificationRepo.CreateBatch(batch); err != nil {
		return fmt.Errorf("create notifications batch: %w", err)
	}
	for _, n := range batch {
		if s.cacheRepo != nil {
			_ = s.cacheRepo.InvalidateUnreadCount(ctx, n.UserID)
		}
		_ = s.BroadcastToUser(n.UserID, n.UserType, n)
	}
	return nil
}
