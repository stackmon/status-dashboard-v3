package notification

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/stackmon/otc-status-dashboard/internal/db"
	"github.com/stackmon/otc-status-dashboard/internal/event"
)

// Change describes a single committed maintenance change to notify about.
type Change struct {
	IncidentID uint
	Title      string
	OldStatus  event.Status
	NewStatus  event.Status
	// ContactEmail is the maintenance creator address (incident.contact_email).
	ContactEmail string
	// Actor is the preferred_username for API changes, or ActorChecker.
	Actor string
	// ChangedAt is the change time; defaults to now (UTC) when zero.
	ChangedAt time.Time
}

// Resolver turns a maintenance Change into outbox rows using the recipient rules
// (architecture §1). Review-audience addresses come from configuration; the
// creator address comes from the change.
type Resolver struct {
	smod      string
	operators []string
	admins    []string
	baseURL   string
}

// NewResolver builds a Resolver from the parsed notification config.
func NewResolver(cfg Config) *Resolver {
	return &Resolver{
		smod:      cfg.ReviewSMOD,
		operators: cfg.ReviewOperators,
		admins:    cfg.ReviewAdmins,
		baseURL:   cfg.BaseURL,
	}
}

// Recipients returns the normalized, deduplicated recipient list for the resulting
// status: review audience (SMOD + operators + admins) for review states, plus the
// creator for every state.
func (r *Resolver) Recipients(status event.Status, contactEmail string) []string {
	var ordered []string
	seen := make(map[string]struct{})

	add := func(raw string) {
		e := normalizeEmail(raw)
		if e == "" {
			return
		}
		if _, ok := seen[e]; ok {
			return
		}
		seen[e] = struct{}{}
		ordered = append(ordered, e)
	}

	if isReviewStatus(status) {
		add(r.smod)
		for _, e := range r.operators {
			add(e)
		}
		for _, e := range r.admins {
			add(e)
		}
	}
	add(contactEmail)

	return ordered
}

// BuildRows produces one pending outbox row per recipient for the change, sharing a
// single generated change_id. It returns nil when there are no recipients.
func (r *Resolver) BuildRows(ch Change) []db.NotificationOutbox {
	recipients := r.Recipients(ch.NewStatus, ch.ContactEmail)
	if len(recipients) == 0 {
		return nil
	}

	kind := KindForStatus(ch.NewStatus)
	changeID := uuid.NewString()
	changedAt := ch.ChangedAt
	if changedAt.IsZero() {
		changedAt = time.Now().UTC()
	}
	payload := buildPayload(ch, changedAt, r.link(ch.IncidentID))

	rows := make([]db.NotificationOutbox, 0, len(recipients))
	for _, rcpt := range recipients {
		rows = append(rows, db.NotificationOutbox{
			Kind:       kind,
			IncidentID: ch.IncidentID,
			Recipient:  rcpt,
			Payload:    payload,
			ChangeID:   changeID,
			DedupKey:   DedupKey(changeID, kind, rcpt),
			Status:     db.NotificationStatusPending,
		})
	}
	return rows
}

// DedupKey builds the unique key change_id : kind : recipient (architecture §4).
func DedupKey(changeID, kind, recipient string) string {
	return fmt.Sprintf("%s:%s:%s", changeID, kind, recipient)
}

// link builds the maintenance deep link from the configured web origin.
func (r *Resolver) link(incidentID uint) string {
	return fmt.Sprintf("%s/incidents/%d", r.baseURL, incidentID)
}

// buildPayload snapshots everything the renderer needs, as strings so a JSONB
// round-trip never changes types (map[string]any with json serializer).
func buildPayload(ch Change, changedAt time.Time, link string) map[string]any {
	return map[string]any{
		"incident_id": fmt.Sprint(ch.IncidentID),
		"title":       ch.Title,
		"old_status":  string(ch.OldStatus),
		"new_status":  string(ch.NewStatus),
		"actor":       ch.Actor,
		"changed_at":  changedAt.UTC().Format(time.RFC3339),
		"link":        link,
	}
}
