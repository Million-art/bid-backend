package http

import (
	"log/slog"
	"strings"
	"time"

	"github.com/bid-hub/admin-backend/adapters/notification/sms"
	"github.com/bid-hub/admin-backend/adapters/payment"
	"github.com/bid-hub/admin-backend/adapters/repository/postgres"
	"github.com/bid-hub/admin-backend/adapters/repository/redis"
	"github.com/bid-hub/admin-backend/adapters/service"
	r2storage "github.com/bid-hub/admin-backend/adapters/storage/r2"
	"github.com/bid-hub/admin-backend/core/services"
	"github.com/bid-hub/admin-backend/core/services/chapapayment"
	"github.com/bid-hub/admin-backend/core/services/lub"
	"github.com/bid-hub/admin-backend/core/services/notifications"
	"github.com/bid-hub/admin-backend/core/services/paymentledger"
	"github.com/bid-hub/admin-backend/platform/cache"
	"github.com/bid-hub/admin-backend/platform/env"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func RegisterRoutes(app *fiber.App, db *gorm.DB, redisClient *goredis.Client, cfg *env.Config) error {
	api := app.Group("/api", CSRF(cfg))

	// 0. Global Rate Limiter (shared via Redis if available, in-memory fallback)
	var limiterStorage fiber.Storage
	limiterConfig := limiter.Config{
		Max:        1000,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c fiber.Ctx) string {
			return "limit:global:" + c.IP()
		},
	}
	if redisClient != nil {
		limiterStorage = cache.NewRedisStorage(redisClient)
		limiterConfig.Storage = limiterStorage
	}
	app.Use(limiter.New(limiterConfig))

	api.Get("/csrf-token", func(c fiber.Ctx) error { return HandleGetCSRFToken(c, cfg) })

	// Repositories
	userRepo := postgres.NewUserRepository(db)
	participantUserRepo := postgres.NewParticipantUserRepository(db)
	notificationRepo := postgres.NewNotificationRepository(db)
	cacheRepo := redis.NewCacheRepository(redisClient)

	// Services
	auditRepo := postgres.NewAuditLogRepository(db)
	auditService := services.NewAuditLogService(auditRepo)
	emailService, err := service.NewResendEmailService(cfg)
	if err != nil {
		return err
	}
	metricsProvider := NewPrometheusMetrics()
	// Prefer CHAPA_API_URL when provided (matches client-backend env naming)
	baseURL := cfg.ChapaBaseURL
	if strings.TrimSpace(cfg.ChapaAPIURL) != "" {
		baseURL = strings.TrimSpace(cfg.ChapaAPIURL)
	}
	chapaClient := payment.NewChapaClient(cfg.ChapaSecretKey, baseURL)
	paymentLedgerRepo := postgres.NewPaymentLedgerRepository(db)
	refundRepo := postgres.NewRefundRecordRepository(db)
	legalDocRepo := postgres.NewLegalDocumentRepository(db)

	// Use Prometheus middleware for HTTP metrics
	app.Use(metricsProvider.RequestMetrics())
	app.Get("/metrics", PrometheusHandler)

	// 1. Initialize Core Services
	smsService := sms.NewAfroMessageService(cfg)
	notificationService := notifications.NewService(notificationRepo, userRepo, participantUserRepo, cacheRepo, nil, emailService, smsService)

	// LUB (Lowest Unique Bid) setup
	lubProductRepo := postgres.NewLUBProductRepository(db)
	lubParticipationRepo := postgres.NewLUBParticipationRepository(db)
	lubBidEntryRepo := postgres.NewLUBBidEntryRepository(db)
	lubService := lub.NewService(lubProductRepo, lubParticipationRepo, lubBidEntryRepo, auditService, notificationService)
	lubProductHandler := NewLUBProductHandler(lubService)
	lubHandler := NewLUBHandler(lubService)

	authService := services.NewAuthService(userRepo, auditService, emailService, cacheRepo, smsService, cfg)
	userService := services.NewUserService(userRepo, auditService, emailService, authService, cfg)
	_ = chapapayment.NewService(paymentLedgerRepo, lubService, participantUserRepo, notificationService, slog.Default())
	paymentLedgerSvc := paymentledger.NewService(paymentLedgerRepo, refundRepo, chapaClient, lubService, auditService)
	dashboardService := services.NewDashboardService(userRepo, lubProductRepo, participantUserRepo, auditRepo)
	legalDocService := services.NewLegalDocumentService(legalDocRepo, notificationService)

	// Handlers
	authHandler := NewAuthHandler(authService, cfg)
	userHandler := NewUserHandler(userService)
	participantHandler := NewParticipantHandler(participantUserRepo)
	dashboardHandler := NewDashboardHandler(dashboardService)
	notificationHandler := NewNotificationHandler(notificationService)
	legalDocHandler := NewLegalDocumentHandler(legalDocService)
	paymentHandler := NewPaymentHandler(paymentLedgerSvc)

	r2PresignService, err := r2storage.NewPresignService(cfg)
	if err != nil {
		slog.Warn("R2 presign service not configured; media presign endpoints will return 503", "error", err)
		r2PresignService = nil
	}
	mediaPresignHandler := NewMediaPresignHandler(r2PresignService, cfg.MaxUploadSizeMB)

	// Register Categorized Routing Modules
	registerPublicRoutes(api, legalDocHandler, limiterStorage, cfg)
	registerAuthRoutes(api, authHandler, limiterStorage)

	protected := api.Group("/", AuthMiddlewareWithCache(authService, cfg.JWTAccessSecret, cacheRepo))

	registerUserRoutes(protected, userHandler, participantHandler)
	registerPaymentRoutes(protected, paymentHandler)
	registerLUBRoutes(api, protected, lubProductHandler, lubHandler, cfg)
	registerMiscRoutes(protected, dashboardHandler, mediaPresignHandler, notificationHandler, legalDocHandler)

	return nil
}

func registerPublicRoutes(api fiber.Router, legalDocHandler *LegalDocumentHandler, limiterStorage fiber.Storage, cfg *env.Config) {
	api.Get("/public/legal/:docType", legalDocHandler.PublicGet)
}

func registerAuthRoutes(api fiber.Router, authHandler *AuthHandler, limiterStorage fiber.Storage) {
	auth := api.Group("/auth")
	auth.Use(AuthRateLimit(limiterStorage))
	auth.Post("/login", authHandler.Login)
	auth.Post("/refresh", authHandler.Refresh)
	auth.Post("/logout", authHandler.Logout)
	auth.Post("/set-password", authHandler.SetPassword)
	auth.Post("/forgot-password", authHandler.ForgotPassword)
	auth.Post("/reset-password", authHandler.SetPassword)          // Email link reset (token-based)
	auth.Post("/reset-password-otp", authHandler.ResetPasswordOTP) // Phone reset (OTP-based)
}

func registerUserRoutes(protected fiber.Router, userHandler *UserHandler, participantHandler *ParticipantHandler) {
	protected.Get("/me", userHandler.GetMe)
	protected.Patch("/me", userHandler.UpdateMe)

	protected.Get("/participants", participantHandler.List)
	protected.Patch("/participants/:id/status", participantHandler.UpdateStatus)
	protected.Delete("/participants/:id", participantHandler.Delete)
}

func registerPaymentRoutes(protected fiber.Router, paymentHandler *PaymentHandler) {
	payments := protected.Group("/payments")
	payments.Get("/", paymentHandler.List)
	payments.Post("/:txRef/refund", paymentHandler.Refund)
}

func registerMiscRoutes(protected fiber.Router, dashboardHandler *DashboardHandler, mediaPresignHandler *MediaPresignHandler, notificationHandler *NotificationHandler, legalDocHandler *LegalDocumentHandler) {
	audit := protected.Group("/audit-logs")
	audit.Get("/", dashboardHandler.ListAuditLogs)

	protected.Get("/stats/charts", dashboardHandler.GetCharts)
	protected.Get("/stats", dashboardHandler.GetStats)

	protected.Get("/legal-documents/:docType", legalDocHandler.StaffGet)
	protected.Put("/legal-documents/:docType", legalDocHandler.StaffPut)

	protected.Post("/media/presign", mediaPresignHandler.Presign)
	protected.Post("/media/presign-batch", mediaPresignHandler.PresignBatch)
	protected.Post("/media/presign-view-batch", mediaPresignHandler.PresignViewBatch)
	protected.Post("/media/upload", mediaPresignHandler.UploadMedia)

	notifications := protected.Group("/notifications")
	notifications.Get("/", notificationHandler.GetUserNotifications)
	notifications.Get("/unread-count", notificationHandler.GetUnreadCount)
	notifications.Patch("/:id/read", notificationHandler.MarkAsRead)
	notifications.Patch("/read-all", notificationHandler.MarkAllAsRead)
}

// registerLUBRoutes wires up all LUB (Lowest Unique Bid) endpoints.
//
// Public (no auth):
//   GET  /api/public/lub-products            — list active products
//   GET  /api/public/lub-products/:id        — get one product
//
// Client-backend service-to-service (X-Client-Secret header):
//   POST /api/public/lub-products/:productId/participate  — record entrance fee payment
//   POST /api/public/lub-bids                             — place a bid entry
//   PATCH /api/public/lub-bids/:id                        — update a bid entry's value
//   GET  /api/public/lub-products/:productId/my-bids      — list participant's bids
//   GET  /api/public/lub-products/:productId/my-participation — check participation
//
// Admin (JWT auth, admin/super_admin role):
//   GET    /api/lub-products
//   POST   /api/lub-products
//   GET    /api/lub-products/:id
//   PATCH  /api/lub-products/:id
//   POST   /api/lub-products/:id/activate
//   POST   /api/lub-products/:id/postpone
//   POST   /api/lub-products/:id/close
//   POST   /api/lub-products/:id/cancel
//   POST   /api/lub-products/:id/execute-draw
//   GET    /api/lub-products/:id/participants
//   GET    /api/lub-products/:id/bid-entries
func registerLUBRoutes(api fiber.Router, protected fiber.Router, productHandler *LUBProductHandler, lubHandler *LUBHandler, cfg *env.Config) {
	// --- Public routes (no auth) ---
	api.Get("/public/lub-products", lubHandler.ListPublicProducts)
	api.Get("/public/lub-products/:id", lubHandler.GetPublicProduct)

	// --- Client-backend service-to-service routes (CSRF-exempt, protected by shared secret) ---
	clientSecret := ClientSecretMiddleware(cfg.ClientSecret)

	api.Post("/public/lub-products/:productId/participate",
		clientSecret,
		lubHandler.RecordParticipation,
	)
	api.Post("/public/lub-bids",
		clientSecret,
		lubHandler.PlaceBid,
	)
	api.Patch("/public/lub-bids/:id",
		clientSecret,
		lubHandler.UpdateBid,
	)
	api.Get("/public/lub-products/:productId/my-bids",
		clientSecret,
		lubHandler.GetMyBids,
	)
	api.Get("/public/lub-products/:productId/my-participation",
		clientSecret,
		lubHandler.GetMyParticipation,
	)

	// --- Admin routes (JWT authenticated) ---
	protected.Get("/lub-products", productHandler.List)
	lubProducts := protected.Group("/lub-products")
	lubProducts.Post("/", productHandler.Create)
	lubProducts.Get("/:id", productHandler.GetByID)
	lubProducts.Patch("/:id", productHandler.Update)
	lubProducts.Post("/:id/activate", productHandler.Activate)
	lubProducts.Post("/:id/postpone", productHandler.Postpone)
	lubProducts.Post("/:id/close", productHandler.Close)
	lubProducts.Post("/:id/cancel", productHandler.Cancel)
	lubProducts.Post("/:id/execute-draw", productHandler.ExecuteDraw)
	lubProducts.Get("/:id/participants", productHandler.ListParticipants)
	lubProducts.Get("/:id/bid-entries", productHandler.ListBidEntries)
}