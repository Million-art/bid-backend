package ports

import "github.com/bid-hub/admin-backend/core/domain"

// LegalDocumentRepository persists platform-wide legal HTML documents.
type LegalDocumentRepository interface {
	GetByDocType(docType string) (*domain.PlatformLegalDocument, error)
	Upsert(doc *domain.PlatformLegalDocument) error
}
