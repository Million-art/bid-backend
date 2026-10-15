package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/core/ports"
	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"
	"gorm.io/gorm"
)

// LegalDocumentService loads and publishes Privacy / Terms HTML.
type LegalDocumentService struct {
	repo          ports.LegalDocumentRepository
	notifications ports.NotificationService
}

func NewLegalDocumentService(repo ports.LegalDocumentRepository, notifications ports.NotificationService) *LegalDocumentService {
	return &LegalDocumentService{repo: repo, notifications: notifications}
}

func NormalizeLegalDocType(raw string) (string, error) {
	d := strings.ToLower(strings.TrimSpace(raw))
	switch d {
	case "privacy", "terms":
		return d, nil
	default:
		return "", fmt.Errorf("invalid legal document type %q: %w", raw, domain.ErrInvalidInput)
	}
}

// GetPublic returns published document or found=false when not yet stored.
func (s *LegalDocumentService) GetPublic(docTypeRaw string) (*domain.PlatformLegalDocument, bool, error) {
	docType, err := NormalizeLegalDocType(docTypeRaw)
	if err != nil {
		return nil, false, err
	}
	doc, err := s.repo.GetByDocType(docType)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return doc, true, nil
}

// GetStaff returns stored HTML or an empty shell for the editor.
func (s *LegalDocumentService) GetStaff(docTypeRaw string) (*domain.PlatformLegalDocument, error) {
	docType, err := NormalizeLegalDocType(docTypeRaw)
	if err != nil {
		return nil, err
	}
	doc, err := s.repo.GetByDocType(docType)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &domain.PlatformLegalDocument{DocType: docType, ContentHTML: ""}, nil
		}
		return nil, err
	}
	return doc, nil
}

// Update sanitizes HTML, saves, then notifies all agents and participants asynchronously.
func (s *LegalDocumentService) Update(docTypeRaw string, contentHTML string, updatedBy uuid.UUID) (*domain.PlatformLegalDocument, error) {
	docType, err := NormalizeLegalDocType(docTypeRaw)
	if err != nil {
		return nil, err
	}
	p := bluemonday.UGCPolicy()
	safe := p.Sanitize(contentHTML)
	doc := &domain.PlatformLegalDocument{
		DocType:     docType,
		ContentHTML: safe,
		UpdatedAt:   time.Now().UTC(),
		UpdatedBy:   &updatedBy,
	}
	if err := s.repo.Upsert(doc); err != nil {
		return nil, err
	}
	go func() {
		// A panic inside the notification broadcaster (e.g. nil deref in a WS hub)
		// must not take down the API process. Recover and log so the request that
		// triggered this fire-and-forget broadcast still returns normally.
		defer func() {
			if r := recover(); r != nil {
				slog.Error("legal document notification panic", "panic", r, "docType", docType)
			}
		}()
		if err := s.notifications.NotifyLegalDocumentPublished(context.Background(), docType); err != nil {
			slog.Error("legal document notification broadcast failed", "error", err, "docType", docType)
		}
	}()
	return s.repo.GetByDocType(docType)
}
