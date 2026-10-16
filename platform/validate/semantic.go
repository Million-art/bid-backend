package validate

import (
	"strings"

	"github.com/gofiber/fiber/v3"
)

// FailPhoneVerifyToken ensures the phone-verify JWT subject matches the signup phone.
func FailPhoneVerifyToken(c fiber.Ctx, tokenPhone, normalizedPhone, rawPhone string) error {
	tokenPhone = strings.TrimSpace(tokenPhone)
	if tokenPhone == normalizedPhone || tokenPhone == rawPhone {
		return nil
	}
	return Fail(c, "Phone mismatch",
		FieldError{
			Field:   "phone",
			Rule:    "phone_verify_token",
			Message: "Verification was for a different phone number",
		})
}

// FailVerificationToken is returned when the phone-verify JWT is missing or invalid.
func FailVerificationToken(c fiber.Ctx) error {
	return FailWithStatus(c, fiber.StatusUnauthorized, "Invalid or expired verification", "UNAUTHORIZED",
		FieldError{
			Field:   "verificationToken",
			Rule:    "invalid",
			Message: "Please request a new code and verify your phone again",
		})
}
