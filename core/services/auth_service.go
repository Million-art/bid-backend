package services

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/bid-hub/admin-backend/core/ports"
	"github.com/bid-hub/admin-backend/platform/env"
	"github.com/bid-hub/admin-backend/platform/logger"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	userRepo        ports.UserRepository
	participantRepo ports.ParticipantUserRepository
	audit           ports.AuditLogService
	email           ports.EmailService
	cache           ports.CacheRepository
	sms             ports.SmsService
	config          *env.Config
}

func NewAuthService(userRepo ports.UserRepository, participantRepo ports.ParticipantUserRepository, audit ports.AuditLogService, email ports.EmailService, cache ports.CacheRepository, sms ports.SmsService, config *env.Config) *AuthService {
	return &AuthService{
		userRepo:        userRepo,
		participantRepo: participantRepo,
		audit:           audit,
		email:           email,
		cache:           cache,
		sms:             sms,
		config:          config,
	}
}

func NormalizePhone(phone string) (normalized string, digits string) {
	clean := strings.TrimSpace(phone)
	digits = strings.ReplaceAll(strings.ReplaceAll(clean, "+", ""), " ", "")
	if strings.HasPrefix(digits, "0") {
		digits = "251" + digits[1:]
	}
	if digits == "" {
		return "", ""
	}
	return "+" + digits, digits
}

func (s *AuthService) SendSignupOTP(ctx context.Context, phone string) error {
	log := logger.FromContext(ctx)
	normalizedPhone, digits := NormalizePhone(phone)
	if normalizedPhone == "" || len(digits) < 8 {
		return domain.NewAppError(400, "Valid phone number is required", nil)
	}

	if existing, err := s.userRepo.GetByPhone(normalizedPhone); err == nil && existing != nil {
		return domain.ErrConflict
	}
	mockEmail := fmt.Sprintf("%s@carramarket.com", digits)
	if existing, err := s.userRepo.GetByEmail(mockEmail); err == nil && existing != nil {
		return domain.ErrConflict
	}

	otp := generateNumericOTP(6)
	cacheKey := fmt.Sprintf("signup_otp:%s", normalizedPhone)
	if s.cache != nil {
		if err := s.cache.SetOTP(ctx, cacheKey, otp, 10*time.Minute); err != nil {
			return err
		}
	}

	log.Info("auth.signup.otp_generated", "phone", normalizedPhone, "otp", otp)

	if s.sms != nil {
		msg := fmt.Sprintf("Your CarraMarket verification code is: %s. Valid for 10 minutes.", otp)
		if err := s.sms.SendSms(normalizedPhone, msg); err != nil {
			log.Warn("auth.signup.sms_failed", "phone", normalizedPhone, "error", err)
		}
	}

	return nil
}

func (s *AuthService) VerifySignupOTP(ctx context.Context, phone, code string) (string, error) {
	normalizedPhone, digits := NormalizePhone(phone)
	code = strings.TrimSpace(code)
	if normalizedPhone == "" || len(digits) < 8 || len(code) != 6 {
		return "", domain.ErrUnauthorized
	}

	cacheKey := fmt.Sprintf("signup_otp:%s", normalizedPhone)
	if s.cache == nil {
		return "", domain.ErrUnauthorized
	}
	storedOTP, err := s.cache.GetOTP(ctx, cacheKey)
	if err != nil || storedOTP == "" || storedOTP != code {
		return "", domain.ErrUnauthorized
	}

	_ = s.cache.DeleteOTP(ctx, cacheKey)

	return s.generatePhoneToken(normalizedPhone, 15*time.Minute, "phone_verify")
}

func (s *AuthService) VerifyPhoneToken(tokenStr, expectedPurpose string) (string, error) {
	if strings.TrimSpace(tokenStr) == "" {
		return "", domain.ErrUnauthorized
	}
	claims, err := s.VerifyToken(tokenStr, s.config.JWTAccessSecret)
	if err != nil {
		return "", domain.ErrUnauthorized
	}
	purpose, _ := claims["purpose"].(string)
	if purpose != expectedPurpose {
		return "", domain.ErrUnauthorized
	}
	sub, _ := claims["sub"].(string)
	if strings.TrimSpace(sub) == "" {
		return "", domain.ErrUnauthorized
	}
	return sub, nil
}

func (s *AuthService) Signup(ctx context.Context, phone, fullName, password string, auditCtx *domain.AuditContext) (string, string, *domain.User, error) {
	normalizedPhone, digits := NormalizePhone(phone)
	fullName = strings.TrimSpace(fullName)
	if normalizedPhone == "" || len(digits) < 8 {
		return "", "", nil, domain.NewAppError(400, "Valid phone number is required", nil)
	}
	if len(fullName) < 2 {
		return "", "", nil, domain.NewAppError(400, "Full name is required", nil)
	}
	if len(password) < 4 {
		return "", "", nil, domain.NewAppError(400, "Password must be at least 4 characters", nil)
	}

	if existing, err := s.userRepo.GetByPhone(normalizedPhone); err == nil && existing != nil {
		return "", "", nil, domain.ErrConflict
	}
	mockEmail := fmt.Sprintf("%s@carramarket.com", digits)
	if existing, err := s.userRepo.GetByEmailIncludeDeleted(mockEmail); err == nil && existing != nil {
		return "", "", nil, domain.ErrConflict
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", "", nil, err
	}

	user := &domain.User{
		ID:        uuid.New(),
		FullName:  fullName,
		Email:     mockEmail,
		Password:  string(hashed),
		Phone:     normalizedPhone,
		JoinedVia: "web",
		Role:      domain.RoleParticipant,
		Status:    domain.StatusActive,
	}

	if err := s.userRepo.Create(user); err != nil {
		return "", "", nil, err
	}

	if s.participantRepo != nil {
		_ = s.participantRepo.UpsertByPhone(&domain.ParticipantUser{
			ID:        user.ID.String(),
			Email:     user.Email,
			FullName:  user.FullName,
			Phone:     user.Phone,
			Status:    "active",
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		})
	}

	accessToken, err := s.generateToken(user, s.config.JWTAccessSecret, 24*time.Hour, "access")
	if err != nil {
		return "", "", nil, err
	}

	refreshToken, err := s.generateToken(user, s.config.JWTRefreshSecret, 30*24*time.Hour, "refresh")
	if err != nil {
		return "", "", nil, err
	}

	_ = s.audit.Log(&domain.AuditLog{
		UserID:   user.ID,
		Action:   "USER_SIGNUP",
		Entity:   "user",
		EntityID: user.ID.String(),
		Metadata: map[string]interface{}{
			"phone": user.Phone,
			"role":  user.Role,
		},
	}, auditCtx)

	return accessToken, refreshToken, user, nil
}

func (s *AuthService) SendForgotPasswordOTP(ctx context.Context, phone string) error {
	log := logger.FromContext(ctx)
	normalizedPhone, digits := NormalizePhone(phone)
	if normalizedPhone == "" || len(digits) < 8 {
		return domain.NewAppError(400, "Valid phone number is required", nil)
	}

	_, err := s.userRepo.GetByPhone(normalizedPhone)
	if err != nil && errors.Is(err, domain.ErrNotFound) {
		mockEmail := fmt.Sprintf("%s@carramarket.com", digits)
		_, err = s.userRepo.GetByEmail(mockEmail)
	}
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.NewAppError(404, "No account found with this phone number", nil)
		}
		return err
	}

	otp := generateNumericOTP(6)
	cacheKey := fmt.Sprintf("forgot_otp:%s", normalizedPhone)
	if s.cache != nil {
		if err := s.cache.SetOTP(ctx, cacheKey, otp, 10*time.Minute); err != nil {
			return err
		}
	}

	log.Info("auth.forgot_password.otp_generated", "phone", normalizedPhone, "otp", otp)

	if s.sms != nil {
		msg := fmt.Sprintf("Your CarraMarket password reset code is: %s. Valid for 10 minutes.", otp)
		if err := s.sms.SendSms(normalizedPhone, msg); err != nil {
			log.Warn("auth.forgot_password.sms_failed", "phone", normalizedPhone, "error", err)
		}
	}

	return nil
}

func (s *AuthService) VerifyForgotPasswordOTP(ctx context.Context, phone, code string) (string, error) {
	normalizedPhone, digits := NormalizePhone(phone)
	code = strings.TrimSpace(code)
	if normalizedPhone == "" || len(digits) < 8 || len(code) != 6 {
		return "", domain.ErrUnauthorized
	}

	cacheKey := fmt.Sprintf("forgot_otp:%s", normalizedPhone)
	if s.cache == nil {
		return "", domain.ErrUnauthorized
	}
	storedOTP, err := s.cache.GetOTP(ctx, cacheKey)
	if err != nil || storedOTP == "" || storedOTP != code {
		return "", domain.ErrUnauthorized
	}

	_ = s.cache.DeleteOTP(ctx, cacheKey)

	return s.generatePhoneToken(normalizedPhone, 15*time.Minute, "password_reset")
}

func (s *AuthService) ResetPasswordWithToken(ctx context.Context, phone, resetToken, newPassword string) error {
	tokenPhone, err := s.VerifyPhoneToken(resetToken, "password_reset")
	if err != nil {
		return domain.ErrUnauthorized
	}
	normalizedPhone, digits := NormalizePhone(phone)
	if tokenPhone != normalizedPhone && tokenPhone != strings.TrimSpace(phone) {
		return domain.ErrUnauthorized
	}
	if len(newPassword) < 4 {
		return domain.NewAppError(400, "Password must be at least 4 characters", nil)
	}

	user, err := s.userRepo.GetByPhone(normalizedPhone)
	if err != nil && errors.Is(err, domain.ErrNotFound) {
		mockEmail := fmt.Sprintf("%s@carramarket.com", digits)
		user, err = s.userRepo.GetByEmail(mockEmail)
	}
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.NewAppError(404, "User not found", nil)
		}
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.Password = string(hash)
	if err := s.userRepo.Update(user); err != nil {
		return err
	}

	return s.audit.Log(&domain.AuditLog{
		UserID:   user.ID,
		Action:   "PASSWORD_RESET",
		Entity:   "user",
		EntityID: user.ID.String(),
	}, nil)
}

func (s *AuthService) Login(ctx context.Context, loginIdentifier, password string, auditCtx *domain.AuditContext) (string, string, *domain.User, error) {
	log := logger.FromContext(ctx)
	identifier := strings.TrimSpace(loginIdentifier)
	maskedIdentifier := maskEmail(identifier)
	log.Debug("auth.login.lookup", "identifier", maskedIdentifier)

	var user *domain.User
	var err error

	// 1. Try lookup by exact email
	user, err = s.userRepo.GetByEmail(identifier)

	// Normalize phone for subsequent checks
	sanitizedPhone := strings.ReplaceAll(strings.ReplaceAll(identifier, "+", ""), " ", "")
	if strings.HasPrefix(sanitizedPhone, "0") {
		sanitizedPhone = "251" + sanitizedPhone[1:]
	}
	normalizedPhone := "+" + sanitizedPhone

	// 2. If not found by email, try lookup by phone (raw and normalized)
	if err != nil && errors.Is(err, domain.ErrNotFound) {
		user, err = s.userRepo.GetByPhone(identifier)
		if err != nil && errors.Is(err, domain.ErrNotFound) {
			user, err = s.userRepo.GetByPhone(normalizedPhone)
		}
	}

	// 3. If not found by phone directly, check if mock email derived from phone exists
	if err != nil && errors.Is(err, domain.ErrNotFound) {
		mockEmail := fmt.Sprintf("%s@carramarket.com", sanitizedPhone)
		user, err = s.userRepo.GetByEmail(mockEmail)
	}

	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Info("auth.login.user_not_found", "identifier", maskedIdentifier)
			return "", "", nil, domain.ErrUnauthorized
		}
		log.Error("auth.login.db_error", "identifier", maskedIdentifier, "error", err)
		return "", "", nil, err
	}

	log.Debug("auth.login.user_found", "email", maskEmail(user.Email), "user_id", user.ID, "role", user.Role)

	// Check Password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		log.Info("auth.login.password_mismatch", "identifier", maskedIdentifier)
		return "", "", nil, domain.ErrUnauthorized
	}

	log.Debug("auth.login.password_correct", "identifier", maskedIdentifier)

	// Generate Tokens
	accessToken, err := s.generateToken(user, s.config.JWTAccessSecret, 24*time.Hour, "access")
	if err != nil {
		return "", "", nil, err
	}

	refreshToken, err := s.generateToken(user, s.config.JWTRefreshSecret, 30*24*time.Hour, "refresh")
	if err != nil {
		return "", "", nil, err
	}

	// Log Action
	s.audit.Log(&domain.AuditLog{
		UserID:   user.ID,
		Action:   "USER_LOGIN",
		Entity:   "user",
		EntityID: user.ID.String(),
		Metadata: map[string]interface{}{
			"email": user.Email,
			"role":  user.Role,
		},
	}, auditCtx)

	return accessToken, refreshToken, user, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (string, string, error) {
	claims, err := s.VerifyToken(refreshToken, s.config.JWTRefreshSecret)
	if err != nil {
		return "", "", domain.ErrUnauthorized
	}

	userID, ok := claims["sub"].(string)
	if !ok || userID == "" {
		return "", "", domain.ErrUnauthorized
	}

	uid, err := uuid.Parse(userID)
	if err != nil {
		return "", "", domain.ErrUnauthorized
	}

	user, err := s.userRepo.GetByID(uid)
	if err != nil {
		return "", "", err
	}
	if user == nil || user.Status != domain.StatusActive {
		return "", "", domain.ErrUnauthorized
	}

	accessToken, err := s.generateToken(user, s.config.JWTAccessSecret, 24*time.Hour, "access")
	if err != nil {
		return "", "", err
	}

	newRefreshToken, err := s.generateToken(user, s.config.JWTRefreshSecret, 30*24*time.Hour, "refresh")
	if err != nil {
		return "", "", err
	}

	return accessToken, newRefreshToken, nil
}

func (s *AuthService) VerifyToken(tokenStr, secret string) (jwt.MapClaims, error) {
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, domain.ErrUnauthorized
	}
	return claims, nil
}

func (s *AuthService) RevokeTokenByString(ctx context.Context, tokenStr, secret string, defaultTTL time.Duration) {
	if s.cache == nil {
		return
	}
	claims, err := s.VerifyToken(tokenStr, secret)
	if err != nil {
		return
	}
	jti, _ := claims["jti"].(string)
	if jti == "" {
		return
	}
	ttl := defaultTTL
	if exp, ok := claims["exp"].(float64); ok {
		rem := time.Until(time.Unix(int64(exp), 0))
		if rem > 0 {
			ttl = rem
		}
	}
	_ = s.cache.RevokeToken(ctx, jti, ttl)
}

func (s *AuthService) Logout(ctx context.Context, userID uuid.UUID) error {
	return s.audit.Log(&domain.AuditLog{
		UserID:   userID,
		Action:   "USER_LOGOUT",
		Entity:   "user",
		EntityID: userID.String(),
	}, nil)
}

func (s *AuthService) SetPassword(ctx context.Context, tokenStr, password string, auditCtx *domain.AuditContext) error {
	if len(password) < 4 {
		return domain.NewAppError(400, "password must be at least 4 characters", nil)
	}
	claims, err := s.VerifyToken(tokenStr, s.config.JWTAccessSecret)
	if err != nil {
		return domain.ErrUnauthorized
	}
	userIDStr, _ := claims["sub"].(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return domain.ErrUnauthorized
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return err
	}
	user.Password = string(hash)
	if user.Status == domain.StatusPending {
		user.Status = domain.StatusActive
	}
	if err := s.userRepo.Update(user); err != nil {
		return err
	}
	return s.audit.Log(&domain.AuditLog{
		UserID:   userID,
		Action:   "PASSWORD_SET",
		Entity:   "user",
		EntityID: userID.String(),
	}, auditCtx)
}

func (s *AuthService) ForgotPassword(ctx context.Context, identifier string) (string, error) {
	log := logger.FromContext(ctx)
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return "", domain.NewAppError(400, "email or phone number is required", nil)
	}

	var user *domain.User
	user, err := s.userRepo.GetByEmail(identifier)
	if err != nil && errors.Is(err, domain.ErrNotFound) {
		sanitizedPhone := strings.ReplaceAll(strings.ReplaceAll(identifier, "+", ""), " ", "")
		if strings.HasPrefix(sanitizedPhone, "0") {
			sanitizedPhone = "251" + sanitizedPhone[1:]
		}
		normalizedPhone := "+" + sanitizedPhone
		user, err = s.userRepo.GetByPhone(normalizedPhone)
		if err != nil && errors.Is(err, domain.ErrNotFound) {
			user, err = s.userRepo.GetByPhone(identifier)
		}
	}
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			log.Info("auth.forgot_password.user_not_found", "identifier", maskEmail(identifier))
			return "email", nil
		}
		return "", err
	}

	// Generate 6-digit OTP
	otp := generateNumericOTP(6)

	// Store OTP in cache with 10-minute expiry
	cacheKey := fmt.Sprintf("reset_otp:%s", user.ID.String())
	if s.cache != nil {
		if err := s.cache.SetOTP(ctx, cacheKey, otp, 10*time.Minute); err != nil {
			return "", err
		}
	}

	method := "email"
	// Send OTP via SMS
	if s.sms != nil && user.Phone != "" {
		msg := fmt.Sprintf("Your CarraMarket password reset code is: %s. Valid for 10 minutes.", otp)
		if err := s.sms.SendSms(user.Phone, msg); err != nil {
			log.Error("auth.forgot_password.sms_failed", "user_id", user.ID, "error", err)
		} else {
			method = "sms"
		}
	}

	// Send OTP via Email if available
	if s.email != nil && user.Email != "" && !strings.HasSuffix(user.Email, "@carramarket.com") {
		htmlContent := fmt.Sprintf(`
			<p>Your CarraMarket password reset code is: <strong>%s</strong></p>
			<p>This code is valid for 10 minutes.</p>
		`, otp)
		if err := s.email.SendEmail([]string{user.Email}, "Password Reset Code", htmlContent); err != nil {
			log.Error("auth.forgot_password.email_failed", "user_id", user.ID, "error", err)
		}
	}

	return method, nil
}

func (s *AuthService) ResetPasswordWithOTP(ctx context.Context, identifier, otp, newPassword string) error {
	identifier = strings.TrimSpace(identifier)
	otp = strings.TrimSpace(otp)

	var user *domain.User
	user, err := s.userRepo.GetByEmail(identifier)
	if err != nil && errors.Is(err, domain.ErrNotFound) {
		sanitizedPhone := strings.ReplaceAll(strings.ReplaceAll(identifier, "+", ""), " ", "")
		if strings.HasPrefix(sanitizedPhone, "0") {
			sanitizedPhone = "251" + sanitizedPhone[1:]
		}
		normalizedPhone := "+" + sanitizedPhone
		user, err = s.userRepo.GetByPhone(normalizedPhone)
		if err != nil && errors.Is(err, domain.ErrNotFound) {
			user, err = s.userRepo.GetByPhone(identifier)
		}
	}
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.NewAppError(404, "user not found", nil)
		}
		return err
	}

	cacheKey := fmt.Sprintf("reset_otp:%s", user.ID.String())
	if s.cache == nil {
		return domain.NewAppError(400, "invalid or expired reset code", nil)
	}
	storedOTP, err := s.cache.GetOTP(ctx, cacheKey)
	if err != nil || storedOTP != otp {
		return domain.NewAppError(400, "invalid or expired reset code", nil)
	}

	if len(newPassword) < 8 {
		return domain.NewAppError(400, "password must be at least 8 characters", nil)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.Password = string(hash)
	if err := s.userRepo.Update(user); err != nil {
		return err
	}

	// Delete used OTP
	_ = s.cache.DeleteOTP(ctx, cacheKey)

	return s.audit.Log(&domain.AuditLog{
		UserID:   user.ID,
		Action:   "PASSWORD_RESET",
		Entity:   "user",
		EntityID: user.ID.String(),
	}, nil)
}

func (s *AuthService) GenerateSetPasswordToken(user *domain.User) (string, error) {
	return s.generateToken(user, s.config.JWTAccessSecret, 7*24*time.Hour, "set_password")
}

func (s *AuthService) generateToken(user *domain.User, secret string, expiry time.Duration, purpose string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":     user.ID.String(),
		"email":   user.Email,
		"role":    user.Role,
		"purpose": purpose,
		"jti":     uuid.New().String(),
		"iat":     now.Unix(),
		"exp":     now.Add(expiry).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func (s *AuthService) generatePhoneToken(phone string, expiry time.Duration, purpose string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":     phone,
		"purpose": purpose,
		"jti":     uuid.New().String(),
		"iat":     now.Unix(),
		"exp":     now.Add(expiry).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWTAccessSecret))
}

func maskEmail(email string) string {
	if email == "" {
		return ""
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "****"
	}
	local := parts[0]
	domain := parts[1]
	if len(local) <= 2 {
		return "**@" + domain
	}
	masked := local[:2] + strings.Repeat("*", len(local)-2)
	return masked + "@" + domain
}

func generateNumericOTP(length int) string {
	const digits = "0123456789"
	var result strings.Builder
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			result.WriteByte(digits[time.Now().UnixNano()%10])
		} else {
			result.WriteByte(digits[n.Int64()])
		}
	}
	return result.String()
}
