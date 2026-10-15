package http

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/bid-hub/admin-backend/adapters/handler/response"
	r2storage "github.com/bid-hub/admin-backend/adapters/storage/r2"
	"github.com/bid-hub/admin-backend/platform/validate"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type MediaPresignHandler struct {
	presign         *r2storage.PresignService
	maxUploadSizeMB int
}

func NewMediaPresignHandler(presign *r2storage.PresignService, maxUploadSizeMB int) *MediaPresignHandler {
	return &MediaPresignHandler{presign: presign, maxUploadSizeMB: maxUploadSizeMB}
}

// UploadMedia handles direct file uploads via the backend to bypass browser CORS issues with R2.
func (h *MediaPresignHandler) UploadMedia(c fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, "No file provided", nil)
	}

	maxSize := int64(h.maxUploadSizeMB * 1024 * 1024)
	if file.Size > maxSize {
		return response.Error(c, fiber.StatusBadRequest, fmt.Sprintf("File size exceeds maximum allowed size of %dMB", h.maxUploadSizeMB), nil)
	}

	// Validate content type
	contentType := file.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") && !strings.HasPrefix(contentType, "video/") {
		return response.Error(c, fiber.StatusBadRequest, "Invalid file type. Only images and videos are allowed.", nil)
	}

	// Read file data
	f, err := file.Open()
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to read file", nil)
	}
	defer f.Close()

	fileData := make([]byte, file.Size)
	if _, err := f.Read(fileData); err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to read file data", nil)
	}

	// Generate secure filename
	ext := getExtensionFromMime(contentType)
	if ext == "" {
		parts := strings.Split(file.Filename, ".")
		if len(parts) > 1 {
			ext = parts[len(parts)-1]
		}
	}
	if ext == "" {
		ext = "bin"
	}
	secureName := fmt.Sprintf("%s.%s", uuid.New().String(), ext)

	// Upload to R2 via the backend proxy
	publicURL, err := h.presign.Upload(secureName, contentType, fileData)
	if err != nil {
		return response.Error(c, fiber.StatusInternalServerError, "Failed to upload file to storage", err)
	}

	return response.Success(c, fiber.StatusOK, "File uploaded successfully", fiber.Map{
		"url": publicURL,
	})
}

type presignFileRequest struct {
	Name string `json:"name" validate:"required,min=1,max=255"`
	Type string `json:"type" validate:"omitempty,max=128"`
	Size int64  `json:"size" validate:"required,gt=0"`
}

type presignBatchRequest struct {
	Files []presignFileRequest `json:"files" validate:"required,min=1,max=50,dive"`
}

type presignViewRequest struct {
	Keys []string `json:"keys" validate:"required,min=1,max=200,dive,required"`
}

func (h *MediaPresignHandler) getMaxBytes() int64 {
	maxMB := h.maxUploadSizeMB
	if maxMB <= 0 {
		maxMB = 5
	}
	return int64(maxMB) * 1024 * 1024
}

func (h *MediaPresignHandler) Presign(c fiber.Ctx) error {
	if h.presign == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "R2 upload is not configured", nil)
	}

	var req presignFileRequest
	if err := validate.BindAndValidate(c, &req); err != nil {
		return err
	}

	item, err := h.presignOne(req)
	if err != nil {
		return response.Error(c, fiber.StatusBadRequest, err.Error(), nil)
	}

	return response.Success(c, fiber.StatusOK, "Presigned URL created", item)
}

func (h *MediaPresignHandler) PresignBatch(c fiber.Ctx) error {
	if h.presign == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "R2 upload is not configured", nil)
	}

	var req presignBatchRequest
	if err := validate.BindAndValidate(c, &req); err != nil {
		return err
	}

	maxBytes := h.getMaxBytes()
	var total int64
	for _, f := range req.Files {
		total += f.Size
	}
	if total > maxBytes {
		return response.Error(c, fiber.StatusBadRequest,
			fmt.Sprintf("Total file size (%dMB) exceeds limit (%dMB)", total/(1024*1024), h.maxUploadSizeMB), nil)
	}

	var items []fiber.Map
	for _, f := range req.Files {
		item, err := h.presignOne(f)
		if err != nil {
			return response.Error(c, fiber.StatusBadRequest, err.Error(), nil)
		}
		items = append(items, item)
	}

	return response.Success(c, fiber.StatusOK, "Presigned URLs created", fiber.Map{"items": items})
}

// PresignViewBatch returns URLs for existing object keys on the custom domain.
func (h *MediaPresignHandler) PresignViewBatch(c fiber.Ctx) error {
	if h.presign == nil {
		return response.Error(c, fiber.StatusServiceUnavailable, "R2 upload is not configured", nil)
	}

	var req presignViewRequest
	if err := validate.BindAndValidate(c, &req); err != nil {
		return err
	}

	var items []fiber.Map
	for _, k := range req.Keys {
		p, err := h.presign.PresignGet(k, 10*time.Minute)
		if err != nil {
			return response.Error(c, fiber.StatusBadRequest, "Failed to create view URL", nil)
		}
		items = append(items, fiber.Map{
			"public_id":  p.Key,
			"signed_url": p.SignedURL, // This is now a direct public URL for R2
		})
	}

	return response.Success(c, fiber.StatusOK, "View URLs created", fiber.Map{"items": items})
}

var safeNameRe = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func sanitizeFilename(name string) string {
	n := strings.TrimSpace(name)
	if n == "" {
		return "file"
	}
	n = strings.ReplaceAll(n, " ", "-")
	n = safeNameRe.ReplaceAllString(n, "-")
	n = strings.Trim(n, "-")
	if n == "" {
		return "file"
	}
	if len(n) > 120 {
		n = n[:120]
	}
	return n
}

func (h *MediaPresignHandler) presignOne(f presignFileRequest) (fiber.Map, error) {
	name := sanitizeFilename(f.Name)
	contentType := strings.TrimSpace(f.Type)

	if f.Size <= 0 {
		return nil, fmt.Errorf("Invalid file size")
	}
	if f.Size > h.getMaxBytes() {
		return nil, fmt.Errorf("File too large (max %dMB)", h.maxUploadSizeMB)
	}
	if !isAllowedByTypeOrName(name, contentType) {
		return nil, fmt.Errorf("Unsupported file type")
	}

	mediaType := "image"
	if strings.HasPrefix(strings.ToLower(contentType), "video/") ||
		strings.HasSuffix(strings.ToLower(name), ".mp4") ||
		strings.HasSuffix(strings.ToLower(name), ".webm") ||
		strings.HasSuffix(strings.ToLower(name), ".mov") {
		mediaType = "video"
	}

	objectName := fmt.Sprintf("%s-%s", uuid.NewString(), name)

	p, err := h.presign.PresignPut(objectName, contentType, 10*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("Failed to create presigned URL")
	}
	if p.PublicURL == "" {
		return nil, fmt.Errorf("R2 public domain is not configured")
	}

	view, err := h.presign.PresignGet(p.Key, 10*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("Failed to create view URL")
	}

	return fiber.Map{
		"url":        p.PublicURL,
		"public_id":  p.Key,
		"signed_url": p.SignedURL,
		"view_url":   view.SignedURL,
		"type":       mediaType,
		"name":       f.Name,
		"size":       f.Size,
	}, nil
}

func isAllowedByTypeOrName(filename, contentType string) bool {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	lower := strings.ToLower(filename)
	if strings.HasPrefix(ct, "image/") || strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") ||
		strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".gif") || strings.HasSuffix(lower, ".webp") {
		return true
	}
	if strings.HasPrefix(ct, "video/") || strings.HasSuffix(lower, ".mp4") || strings.HasSuffix(lower, ".webm") ||
		strings.HasSuffix(lower, ".mov") {
		return true
	}
	return false
}

func getExtensionFromMime(mimeType string) string {
	parts := strings.Split(mimeType, "/")
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}
