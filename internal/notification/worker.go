package notification

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"

	"github.com/stackmon/otc-status-dashboard/internal/db"
)

const (
	defaultBatchSize  = 50
	defaultSweepEvery = 5 * time.Minute
)

// Worker delivers queued outbox rows. On the happy path it is woken by Notify right
// after a change commits; a low-frequency ticker sweeps for retries and rows orphaned
// by a crashed pod. Sending happens outside any DB transaction (architecture §5).
type Worker struct {
	db       *db.DB
	renderer *Renderer
	sender   Sender
	log      *zap.Logger

	leaseOwner   string
	leaseTimeout time.Duration
	maxAttempts  int
	smtpTimeout  time.Duration
	backoff      func(attempts int) time.Time

	batchSize  int
	sweepEvery time.Duration

	signal chan struct{}
}

// NewWorker builds a delivery worker from the parsed config and a sender.
func NewWorker(cfg Config, database *db.DB, sender Sender, log *zap.Logger) (*Worker, error) {
	renderer, err := NewRenderer()
	if err != nil {
		return nil, err
	}
	return &Worker{
		db:           database,
		renderer:     renderer,
		sender:       sender,
		log:          log,
		leaseOwner:   leaseOwner(),
		leaseTimeout: cfg.LeaseTimeout,
		maxAttempts:  cfg.MaxAttempts,
		smtpTimeout:  cfg.Timeout,
		backoff:      Backoff(cfg.BackoffBase),
		batchSize:    defaultBatchSize,
		sweepEvery:   defaultSweepEvery,
		signal:       make(chan struct{}, 1),
	}, nil
}

// Notify wakes the worker after a commit. It never blocks: a pending signal already
// covers the next drain.
func (w *Worker) Notify() {
	select {
	case w.signal <- struct{}{}:
	default:
	}
}

// Run processes due rows on every signal and on a periodic safety sweep until the
// context is cancelled. In-flight sends finish before Run returns.
func (w *Worker) Run(ctx context.Context) {
	w.log.Info("notification worker started", zap.String("lease_owner", w.leaseOwner))
	ticker := time.NewTicker(w.sweepEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.log.Info("notification worker stopped")
			return
		case <-w.signal:
			w.drainQuietly(ctx)
		case <-ticker.C:
			w.drainQuietly(ctx)
		}
	}
}

func (w *Worker) drainQuietly(ctx context.Context) {
	if err := w.Drain(ctx); err != nil && ctx.Err() == nil {
		w.log.Error("notification drain failed", zap.Error(err))
	}
}

// Drain recovers stale rows, then claims and sends batches until none remain due.
// It is exported so it can be driven deterministically in tests.
func (w *Worker) Drain(ctx context.Context) error {
	if _, err := w.db.RecoverStaleProcessing(ctx, nil, w.leaseTimeout, w.maxAttempts); err != nil {
		return err
	}

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		rows, err := w.db.ClaimPending(ctx, nil, w.batchSize, w.leaseOwner, w.leaseTimeout)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}

		for i := range rows {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			w.deliver(ctx, rows[i])
		}
	}
}

// deliver sends one claimed row and records the outcome. Failures (including panics)
// are isolated per row so one bad email cannot stop the batch.
func (w *Worker) deliver(ctx context.Context, row db.NotificationOutbox) {
	if err := w.sendGuarded(ctx, row); err != nil {
		w.log.Warn("notification send failed",
			zap.Uint("outbox_id", row.ID), zap.Uint("incident_id", row.IncidentID),
			zap.String("recipient", row.Recipient), zap.Int("attempts", row.Attempts),
			zap.Error(err))
		if merr := w.db.MarkFailed(ctx, nil, row.ID, err.Error(), w.maxAttempts, w.backoff); merr != nil {
			w.log.Error("mark failed", zap.Uint("outbox_id", row.ID), zap.Error(merr))
		}
		return
	}

	if err := w.db.MarkSent(ctx, nil, row.ID); err != nil {
		w.log.Error("mark sent", zap.Uint("outbox_id", row.ID), zap.Error(err))
	}
}

// sendGuarded renders and sends one row inside a recover() guard and an SMTP timeout.
func (w *Worker) sendGuarded(ctx context.Context, row db.NotificationOutbox) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic sending notification: %v", r)
		}
	}()

	email, err := w.renderer.Render(row)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}

	sendCtx, cancel := context.WithTimeout(ctx, w.smtpTimeout)
	defer cancel()
	return w.sender.Send(sendCtx, row.Recipient, email)
}

// leaseOwner identifies this pod for outbox lease bookkeeping.
func leaseOwner() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "pod"
	}
	return fmt.Sprintf("%s-%d", host, os.Getpid())
}
