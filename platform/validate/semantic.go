package validate

import (
	"strings"

	"github.com/gofiber/fiber/v3"
)

// FailReferralBountyWhenEnabled enforces referralBountyPct >= 0.01 when the
// bid accepts referral rewards.
func FailReferralBountyWhenEnabled(c fiber.Ctx, acceptsReferral bool, bountyPct float64) error {
	if !acceptsReferral {
		return nil
	}
	if bountyPct >= 0.01 {
		return nil
	}
	return Fail(c, "Referral bounty must be at least 0.01 when referral rewards are enabled",
		FieldError{
			Field:   "referralBountyPct",
			Rule:    "referral_bounty_min",
			Param:   "0.01",
			Message: "referralBountyPct must be >= 0.01 when acceptsReferralRewards is true",
		})
}

// FailStaffAgentRequired requires agentId when staff creates a bid on behalf
// of an agent.
func FailStaffAgentRequired(c fiber.Ctx, isStaff bool, agentID string) error {
	if !isStaff || strings.TrimSpace(agentID) != "" {
		return nil
	}
	return Fail(c, "agentId is required for staff-created bids",
		FieldError{
			Field:   "agentId",
			Rule:    "required_if_staff",
			Message: "agentId is required when the caller is staff",
		})
}

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

// FailPurchaseQuantity requires either ticketNumbers (non-empty) or quantity >= 1.
func FailPurchaseQuantity(c fiber.Ctx, quantity int, ticketNumbers []int) error {
	if len(ticketNumbers) > 0 || quantity >= 1 {
		return nil
	}
	return Fail(c, "quantity (min 1) or ticketNumbers required",
		FieldError{
			Field:   "quantity",
			Rule:    "one_of_with",
			Param:   "ticketNumbers",
			Message: "either quantity or ticketNumbers must be provided",
		})
}
