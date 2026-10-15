package worker

import (
	"log/slog"
	"sync"

	"github.com/getsentry/sentry-go"
)

// BackgroundWorker provides a safe way to run goroutines
// ensuring that panic recovery occurs and that the server
// can wait for running tasks to finish upon graceful shutdown.
type BackgroundWorker struct {
	wg sync.WaitGroup
}

// NewBackgroundWorker creates a new instance of BackgroundWorker.
func NewBackgroundWorker() *BackgroundWorker {
	return &BackgroundWorker{}
}

// Run executes the provided function in a managed goroutine.
func (w *BackgroundWorker) Run(f func()) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("Panic recovered in background worker", "panic", r)
				// Report panic to Sentry with full context.
				sentry.CurrentHub().Clone().Recover(r)
				sentry.Flush(2 * sentry.DefaultFlushTimeout)
			}
		}()
		f()
	}()
}

// Wait blocks until all functions executed by Run have finished.
func (w *BackgroundWorker) Wait() {
	slog.Info("Waiting for background workers to finish...")
	w.wg.Wait()
	slog.Info("All background workers finished.")
}
