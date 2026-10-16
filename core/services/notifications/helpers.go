package notifications

import (
	"fmt"

	"github.com/bid-hub/admin-backend/core/domain"
)

func participationNotificationCopy(productName string, feePaid domain.Money) (title string, message string, priority string) {
	return "Participation confirmed",
		fmt.Sprintf("Your entrance fee of %s for auction '%s' has been confirmed. You can now place bids.", feePaid.String(), productName),
		domain.NotificationPriorityNormal
}