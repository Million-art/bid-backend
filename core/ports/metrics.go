package ports

import "time"

type MetricsProvider interface {
	RecordAuctionCreated()
	RecordAuctionActivated()
	RecordAuctionPostponed()
	RecordAuctionClosed()
	RecordAuctionCancelled()
	RecordAuctionExecuted()
	RecordParticipation(entranceFee float64)
	RecordBidPlaced()
	RecordBidUpdated()
	RecordPaymentSuccess(amount float64)
	RecordPaymentFailure()
	RecordRefundProcessed(amount float64)
	RecordDrawDuration(duration time.Duration)
}