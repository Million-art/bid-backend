package notifications

import (
	"fmt"
	"strings"

	"github.com/bid-hub/admin-backend/core/domain"
)

func productStatusNotificationCopy(
	productName string,
	fromStatus, toStatus domain.ProductStatus,
	detail string,
) (title string, message string, priority string) {
	d := strings.TrimSpace(detail)
	switch toStatus {
	case domain.ProductStatusActive:
		return "Auction started",
			fmt.Sprintf("The auction '%s' is now live and accepting bids!", productName),
			domain.NotificationPriorityHigh
	case domain.ProductStatusPostponed:
		msg := fmt.Sprintf("The auction '%s' has been postponed.", productName)
		if d != "" {
			msg += " " + d
		}
		return "Auction postponed", msg, domain.NotificationPriorityHigh
	case domain.ProductStatusClosed:
		return "Auction closed",
			fmt.Sprintf("The auction '%s' has closed. The draw will be executed shortly.", productName),
			domain.NotificationPriorityHigh
	case domain.ProductStatusExecuted:
		return "Auction completed",
			fmt.Sprintf("The auction '%s' has been drawn and a winner has been determined.", productName),
			domain.NotificationPriorityHigh
	case domain.ProductStatusCancelled:
		msg := fmt.Sprintf("The auction '%s' was cancelled.", productName)
		if d != "" {
			msg += " " + d
		}
		return "Auction cancelled", msg, domain.NotificationPriorityHigh
	default:
		return "Auction updated",
			fmt.Sprintf("The auction '%s' changed from %s to %s.", productName, fromStatus, toStatus),
			domain.NotificationPriorityNormal
	}
}

func participationNotificationCopy(productName string, feePaid domain.Money) (title string, message string, priority string) {
	return "Participation confirmed",
		fmt.Sprintf("Your entrance fee of %s for auction '%s' has been confirmed. You can now place bids.", feePaid.String(), productName),
		domain.NotificationPriorityNormal
}

func bidNotificationCopy(productName string, bidValue domain.Money) (title string, message string, priority string) {
	return "Bid placed",
		fmt.Sprintf("Your bid of %s for auction '%s' has been recorded.", bidValue.String(), productName),
		domain.NotificationPriorityNormal
}

func bidUpdateNotificationCopy(productName string, oldValue, newValue domain.Money) (title string, message string, priority string) {
	return "Bid updated",
		fmt.Sprintf("Your bid for auction '%s' has been updated from %s to %s.", productName, oldValue.String(), newValue.String()),
		domain.NotificationPriorityNormal
}

func winnerNotificationCopy(productName string, winningBid domain.Money) (title string, message string, priority string) {
	return "Congratulations! You won",
		fmt.Sprintf("Your bid of %s was the lowest unique bid for auction '%s'. You won!", winningBid.String(), productName),
		domain.NotificationPriorityUrgent
}

func noWinnerNotificationCopy(productName string) (title string, message string, priority string) {
	return "Auction ended  no unique winner",
		fmt.Sprintf("The auction '%s' ended but no unique bid was found. All bid values were placed by multiple participants.", productName),
		domain.NotificationPriorityNormal
}