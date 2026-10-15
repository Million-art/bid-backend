package postgres

import (
	"github.com/bid-hub/admin-backend/core/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type LegalDocumentRepo struct {
	db *gorm.DB
}

func NewLegalDocumentRepository(db *gorm.DB) *LegalDocumentRepo {
	return &LegalDocumentRepo{db: db}
}

func (r *LegalDocumentRepo) GetByDocType(docType string) (*domain.PlatformLegalDocument, error) {
	var doc domain.PlatformLegalDocument
	err := r.db.Where("doc_type = ?", docType).First(&doc).Error
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (r *LegalDocumentRepo) Upsert(doc *domain.PlatformLegalDocument) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "doc_type"}},
		DoUpdates: clause.AssignmentColumns([]string{"content_html", "updated_at", "updated_by"}),
	}).Create(doc).Error
}
