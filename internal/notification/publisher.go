package notification

import (
	"context"

	"gorm.io/gorm"

	"github.com/stackmon/otc-status-dashboard/internal/db"
)

// Publisher records notification intent: it turns a maintenance Change into outbox
// rows and enqueues them within the caller's transaction, so the email tasks commit
// together with the business change (architecture §3).
type Publisher struct {
	enabled  bool
	resolver *Resolver
	db       *db.DB
}

// NewPublisher builds a Publisher from the parsed config. When cfg.Enabled is false
// the publisher is inert and PublishTx is a no-op.
func NewPublisher(cfg Config, database *db.DB) *Publisher {
	return &Publisher{
		enabled:  cfg.Enabled,
		resolver: NewResolver(cfg),
		db:       database,
	}
}

// Enabled reports whether notifications should be published. It is nil-safe so
// handlers can hold a nil *Publisher when the feature is off.
func (p *Publisher) Enabled() bool {
	return p != nil && p.enabled
}

// PublishTx enqueues one outbox row per recipient for the change, using tx so the
// rows share the business transaction. It is a no-op when disabled or when the
// change resolves to no recipients.
func (p *Publisher) PublishTx(ctx context.Context, tx *gorm.DB, ch Change) error {
	if !p.Enabled() {
		return nil
	}
	rows := p.resolver.BuildRows(ch)
	for i := range rows {
		if err := p.db.Enqueue(ctx, tx, rows[i]); err != nil {
			return err
		}
	}
	return nil
}
