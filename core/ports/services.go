package ports

type EmailService interface {
	SendEmail(to []string, subject string, htmlContent string) error
	SendTemplateEmail(to []string, subject string, templateName string, data interface{}) error
	SendWelcomeEmail(to string, name string, link string) error
}

type SmsService interface {
	SendSms(to string, message string) error
}

type BroadcastProvider interface {
	BroadcastAuctionUpdate(productID string, data map[string]interface{})
	BroadcastParticipation(productID string, data map[string]interface{})
	BroadcastBidPlaced(productID string, data map[string]interface{})
	BroadcastBidUpdated(productID string, data map[string]interface{})
	BroadcastAuctionClosed(productID string, data map[string]interface{})
	BroadcastAuctionExecuted(productID string, data map[string]interface{})
	BroadcastAuctionCountdown(productID string, countdown int)
	BroadcastDrawStarted(productID string, data map[string]interface{})
}

type TaskRunner interface {
	Run(func())
	Wait()
}

type PaymentProvider interface {
	InitializeTransaction(req ChapaInitializeRequest) (*ChapaInitializeResponse, error)
	VerifyTransaction(txRef string) (*ChapaTransactionSnapshot, error)
	InitiateRefund(txRef string, amount float64, reason string, internalRef string) (string, error)
	VerifyRefund(refID string) (string, error)
}
