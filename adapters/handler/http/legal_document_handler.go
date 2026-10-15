package http

import (
	"github.com/bid-hub/admin-backend/adapters/handler/response"
	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/core/services"
	"github.com/bid-hub/admin-backend/platform/validate"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type LegalDocumentHandler struct {
	svc *services.LegalDocumentService
}

func NewLegalDocumentHandler(svc *services.LegalDocumentService) *LegalDocumentHandler {
	return &LegalDocumentHandler{svc: svc}
}

func staffCanManageLegalDocuments(role domain.UserRole) bool {
	switch role {
	case domain.RoleSuperAdmin, domain.RoleAdmin, domain.RoleCompliance:
		return true
	default:
		return false
	}
}

// PublicGet returns published legal HTML (no auth).
func (h *LegalDocumentHandler) PublicGet(c fiber.Ctx) error {
	doc, found, err := h.svc.GetPublic(c.Params("docType"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid document type", err)
	}
	if !found || doc == nil {
		return response.Success(c, fiber.StatusOK, "OK", fiber.Map{"contentHtml": "", "updatedAt": nil})
	}
	return response.Success(c, fiber.StatusOK, "OK", fiber.Map{
		"contentHtml": doc.ContentHTML,
		"updatedAt":   doc.UpdatedAt,
	})
}

// StaffGet returns current content for compliance/admin editors.
func (h *LegalDocumentHandler) StaffGet(c fiber.Ctx) error {
	role := GetUserRole(c)
	if !staffCanManageLegalDocuments(role) {
		return response.Error(c, fiber.StatusForbidden, "Forbidden", nil)
	}
	doc, err := h.svc.GetStaff(c.Params("docType"))
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid document type", err)
	}
	return response.Success(c, fiber.StatusOK, "OK", fiber.Map{
		"docType":     doc.DocType,
		"contentHtml": doc.ContentHTML,
		"updatedAt":   doc.UpdatedAt,
		"updatedBy":   doc.UpdatedBy,
	})
}

// StaffPut publishes HTML and triggers in-app notifications to all agents and participants.
func (h *LegalDocumentHandler) StaffPut(c fiber.Ctx) error {
	role := GetUserRole(c)
	if !staffCanManageLegalDocuments(role) {
		return response.Error(c, fiber.StatusForbidden, "Forbidden", nil)
	}
	userIDStr, authErr := GetUserIDOrUnauthorized(c)
	if authErr != nil {
		return response.Error(c, fiber.StatusUnauthorized, "Unauthorized", authErr)
	}
	actorID, err := uuid.Parse(userIDStr)
	if err != nil {
		return response.Error(c, fiber.StatusUnauthorized, "Unauthorized", err)
	}

	var req struct {
		ContentHTML string `json:"contentHtml" validate:"required,min=1"`
	}
	if err := validate.BindAndValidate(c, &req); err != nil {
		return err
	}

	if _, err := services.NormalizeLegalDocType(c.Params("docType")); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid document type", err)
	}

	doc, err := h.svc.Update(c.Params("docType"), req.ContentHTML, actorID)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to save", err)
	}
	return response.Success(c, fiber.StatusOK, "Published", fiber.Map{
		"docType":     doc.DocType,
		"contentHtml": doc.ContentHTML,
		"updatedAt":   doc.UpdatedAt,
		"updatedBy":   doc.UpdatedBy,
	})
}
