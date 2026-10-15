package domain

import (
	"time"

	"github.com/google/uuid"
)

// PlatformLegalDocument stores published Privacy Policy and Terms HTML (managed by staff).
type PlatformLegalDocument struct {
	DocType     string     `gorm:"primaryKey;size:32;column:doc_type" json:"docType"`
	ContentHTML string     `gorm:"type:text;not null;column:content_html" json:"contentHtml"`
	UpdatedAt   time.Time  `gorm:"column:updated_at" json:"updatedAt"`
	UpdatedBy   *uuid.UUID `gorm:"type:uuid;column:updated_by" json:"updatedBy,omitempty"`
}

func (PlatformLegalDocument) TableName() string {
	return "platform_legal_documents"
}
