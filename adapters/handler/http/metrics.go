package http

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

type PrometheusMetrics struct {
	requestTotal   *prometheus.CounterVec
	requestLatency *prometheus.HistogramVec

	auctionCreated      prometheus.Counter
	auctionActivated    prometheus.Counter
	auctionPostponed    prometheus.Counter
	auctionClosed       prometheus.Counter
	auctionCancelled    prometheus.Counter
	auctionExecuted     prometheus.Counter
	participationCount  prometheus.Counter
	participationAmount prometheus.Counter
	bidPlaced           prometheus.Counter
	bidUpdated          prometheus.Counter
	drawDuration        prometheus.Histogram

	paymentSuccess prometheus.Counter
	paymentFailure prometheus.Counter
	paymentAmount  prometheus.Counter

	refundProcessed  prometheus.Counter
	refundAmount     prometheus.Counter
}

func NewPrometheusMetrics() *PrometheusMetrics {
	return &PrometheusMetrics{
		requestTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		}, []string{"path", "method", "status"}),
		requestLatency: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"path", "method"}),

		auctionCreated: promauto.NewCounter(prometheus.CounterOpts{
			Name: "auction_created_total",
			Help: "Total number of auctions created.",
		}),
		auctionActivated: promauto.NewCounter(prometheus.CounterOpts{
			Name: "auction_activated_total",
			Help: "Total number of auctions activated.",
		}),
		auctionPostponed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "auction_postponed_total",
			Help: "Total number of auctions postponed.",
		}),
		auctionClosed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "auction_closed_total",
			Help: "Total number of auctions closed.",
		}),
		auctionCancelled: promauto.NewCounter(prometheus.CounterOpts{
			Name: "auction_cancelled_total",
			Help: "Total number of auctions cancelled.",
		}),
		auctionExecuted: promauto.NewCounter(prometheus.CounterOpts{
			Name: "auction_executed_total",
			Help: "Total number of auctions executed (draw completed).",
		}),
		participationCount: promauto.NewCounter(prometheus.CounterOpts{
			Name: "auction_participation_total",
			Help: "Total number of auction participations (entrance fees paid).",
		}),
		participationAmount: promauto.NewCounter(prometheus.CounterOpts{
			Name: "auction_participation_amount_total",
			Help: "Total entrance fee amount collected.",
		}),
		bidPlaced: promauto.NewCounter(prometheus.CounterOpts{
			Name: "auction_bid_placed_total",
			Help: "Total number of bids placed.",
		}),
		bidUpdated: promauto.NewCounter(prometheus.CounterOpts{
			Name: "auction_bid_updated_total",
			Help: "Total number of bids updated.",
		}),
		drawDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "auction_draw_duration_seconds",
			Help:    "Duration of an auction draw execution.",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10},
		}),

		paymentSuccess: promauto.NewCounter(prometheus.CounterOpts{
			Name: "payment_success_total",
			Help: "Total number of successful payments.",
		}),
		paymentFailure: promauto.NewCounter(prometheus.CounterOpts{
			Name: "payment_failure_total",
			Help: "Total number of failed payments.",
		}),
		paymentAmount: promauto.NewCounter(prometheus.CounterOpts{
			Name: "payment_amount_total",
			Help: "Total amount processed in payments.",
		}),

		refundProcessed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "refund_processed_total",
			Help: "Total number of refunds processed.",
		}),
		refundAmount: promauto.NewCounter(prometheus.CounterOpts{
			Name: "refund_amount_total",
			Help: "Total amount refunded.",
		}),
	}
}

func (m *PrometheusMetrics) RecordAuctionCreated() {
	m.auctionCreated.Inc()
}

func (m *PrometheusMetrics) RecordAuctionActivated() {
	m.auctionActivated.Inc()
}

func (m *PrometheusMetrics) RecordAuctionPostponed() {
	m.auctionPostponed.Inc()
}

func (m *PrometheusMetrics) RecordAuctionClosed() {
	m.auctionClosed.Inc()
}

func (m *PrometheusMetrics) RecordAuctionCancelled() {
	m.auctionCancelled.Inc()
}

func (m *PrometheusMetrics) RecordAuctionExecuted() {
	m.auctionExecuted.Inc()
}

func (m *PrometheusMetrics) RecordParticipation(entranceFee float64) {
	m.participationCount.Inc()
	m.participationAmount.Add(entranceFee)
}

func (m *PrometheusMetrics) RecordBidPlaced() {
	m.bidPlaced.Inc()
}

func (m *PrometheusMetrics) RecordBidUpdated() {
	m.bidUpdated.Inc()
}

func (m *PrometheusMetrics) RecordDrawDuration(duration time.Duration) {
	m.drawDuration.Observe(duration.Seconds())
}

func (m *PrometheusMetrics) RecordPaymentSuccess(amount float64) {
	m.paymentSuccess.Inc()
	m.paymentAmount.Add(amount)
}

func (m *PrometheusMetrics) RecordPaymentFailure() {
	m.paymentFailure.Inc()
}

func (m *PrometheusMetrics) RecordRefundProcessed(amount float64) {
	m.refundProcessed.Inc()
	m.refundAmount.Add(amount)
}

func (m *PrometheusMetrics) RequestMetrics() fiber.Handler {
	return func(c fiber.Ctx) error {
		path := string(c.Path())
		method := string(c.Method())
		start := time.Now()

		err := c.Next()

		m.requestTotal.WithLabelValues(path, method, fmt.Sprintf("%d", c.Response().StatusCode())).Inc()
		m.requestLatency.WithLabelValues(path, method).Observe(time.Since(start).Seconds())

		return err
	}
}

func PrometheusHandler(c fiber.Ctx) error {
	handler := promhttp.Handler()
	fasthttpHandler := fasthttpadaptor.NewFastHTTPHandler(handler)
	fasthttpHandler(c.Context().(*fasthttp.RequestCtx))
	return nil
}