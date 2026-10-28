package notifications

import (
	"fmt"

	"github.com/bid-hub/admin-backend/core/domain"
	"github.com/google/uuid"
)

func participationNotificationCopy(productName string, feePaid domain.Money) (title string, message string, priority string) {
	return "Participation confirmed",
		fmt.Sprintf("Your entrance fee of %s for auction '%s' has been confirmed. You can now place bids.", feePaid.String(), productName),
		domain.NotificationPriorityNormal
}

func bidPlacedCopy(productName string, bidValue domain.Money) (title, message, priority string) {
	return "Bid placed",
		fmt.Sprintf("Your bid of %s on '%s' was placed successfully.", bidValue.String(), productName),
		domain.NotificationPriorityNormal
}

func bidUpdatedCopy(productName string, oldValue, newValue domain.Money) (title, message, priority string) {
	return "Bid updated",
		fmt.Sprintf("Your bid on '%s' was updated from %s to %s.", productName, oldValue.String(), newValue.String()),
		domain.NotificationPriorityNormal
}

func winnerCopy(productName string, winningBid domain.Money) (title, message, priority string) {
	return "You won!",
		fmt.Sprintf("Congratulations! You won '%s' with the lowest unique bid of %s.", productName, winningBid.String()),
		domain.NotificationPriorityHigh
}

func noWinnerCopy(productName string) (title, message, priority string) {
	return "Auction ended  no winner",
		fmt.Sprintf("The draw for '%s' found no unique lowest bid. Stay tuned for the next auction.", productName),
		domain.NotificationPriorityNormal
}

func auctionClosedCopy(productName string) (title, message, priority string) {
	return "Auction closed",
		fmt.Sprintf("'%s' is now closed. The draw will determine the winner shortly.", productName),
		domain.NotificationPriorityNormal
}

func auctionPostponedCopy(productName, newCloseAt string) (title, message, priority string) {
	return "Auction postponed",
		fmt.Sprintf("'%s' has been postponed. New close time: %s.", productName, newCloseAt),
		domain.NotificationPriorityHigh
}

func auctionCancelledCopy(productName, reason string) (title, message, priority string) {
	msg := fmt.Sprintf("'%s' has been cancelled.", productName)
	if reason != "" {
		msg = fmt.Sprintf("%s Reason: %s", msg, reason)
	}
	return "Auction cancelled", msg, domain.NotificationPriorityHigh
}

func auctionActivatedCopy(productName string) (title, message, priority string) {
	return "New auction live",
		fmt.Sprintf("'%s' is now open for bidding. Join and place your lowest unique bid!", productName),
		domain.NotificationPriorityNormal
}

func paymentSuccessCopy(amount domain.Money, txRef string) (title, message, priority string) {
	return "Payment successful",
		fmt.Sprintf("Your payment of %s was successful (ref: %s).", amount.String(), txRef),
		domain.NotificationPriorityNormal
}

func paymentFailedCopy(txRef, reason string) (title, message, priority string) {
	msg := fmt.Sprintf("Payment failed (ref: %s).", txRef)
	if reason != "" {
		msg = fmt.Sprintf("%s %s", msg, reason)
	}
	return "Payment failed", msg, domain.NotificationPriorityHigh
}

func refundCopy(productName string, amount domain.Money) (title, message, priority string) {
	return "Refund processed",
		fmt.Sprintf("Your refund of %s for '%s' has been processed.", amount.String(), productName),
		domain.NotificationPriorityNormal
}

func auctionActionURL(auctionID uuid.UUID) string {
	return fmt.Sprintf("/auctions/%s", auctionID.String())
}
