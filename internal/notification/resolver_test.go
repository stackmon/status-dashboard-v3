package notification

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stackmon/otc-status-dashboard/internal/db"
	"github.com/stackmon/otc-status-dashboard/internal/event"
)

func testResolver() *Resolver {
	return NewResolver(Config{
		ReviewSMOD:      "support@com.com",
		ReviewOperators: []string{"ops@com.com"},
		ReviewAdmins:    []string{"admin@com.com"},
		BaseURL:         "https://status.example.com",
	})
}

func TestRecipients_ReviewStatusesIncludeAudienceAndCreator(t *testing.T) {
	r := testResolver()

	for _, status := range []event.Status{event.MaintenancePendingReview, event.MaintenanceReviewed} {
		got := r.Recipients(status, "creator@com.com")
		assert.ElementsMatch(t,
			[]string{"support@com.com", "ops@com.com", "admin@com.com", "creator@com.com"},
			got, "status %s", status)
	}
}

func TestRecipients_LifecycleStatusesCreatorOnly(t *testing.T) {
	r := testResolver()

	for _, status := range []event.Status{
		event.MaintenancePlanned, event.MaintenanceInProgress,
		event.MaintenanceCompleted, event.MaintenanceCancelled,
	} {
		got := r.Recipients(status, "creator@com.com")
		assert.Equal(t, []string{"creator@com.com"}, got, "status %s", status)
	}
}

func TestRecipients_NormalizesAndDeduplicates(t *testing.T) {
	r := NewResolver(Config{
		ReviewSMOD:      "Support@Com.com",
		ReviewOperators: []string{" ops@com.com "},
		ReviewAdmins:    []string{"support@com.com"}, // duplicate of SMOD after normalize
	})

	// Creator equals the operator address (different case) -> must appear once.
	got := r.Recipients(event.MaintenancePendingReview, "OPS@com.com")
	assert.Equal(t, []string{"support@com.com", "ops@com.com"}, got)
}

func TestRecipients_EmptyContactEmailForLifecycleYieldsNone(t *testing.T) {
	r := testResolver()
	assert.Empty(t, r.Recipients(event.MaintenancePlanned, ""))
}

func TestDedupKey(t *testing.T) {
	assert.Equal(t, "abc:pending_review:creator@com.com",
		DedupKey("abc", db.NotificationKindPendingReview, "creator@com.com"))
}

func TestBuildRows_OneRowPerRecipientSharedChangeID(t *testing.T) {
	r := testResolver()
	ch := Change{
		IncidentID:   42,
		Title:        "DB upgrade",
		OldStatus:    "",
		NewStatus:    event.MaintenancePendingReview,
		ContactEmail: "creator@com.com",
		Actor:        "admin-user",
	}

	rows := r.BuildRows(ch)
	require.Len(t, rows, 4)

	changeID := rows[0].ChangeID
	require.NotEmpty(t, changeID)
	seenRecipients := make(map[string]struct{})
	for _, row := range rows {
		assert.Equal(t, changeID, row.ChangeID, "all rows share one change_id")
		assert.Equal(t, db.NotificationKindPendingReview, row.Kind)
		assert.Equal(t, uint(42), row.IncidentID)
		assert.Equal(t, db.NotificationStatusPending, row.Status)
		assert.Equal(t, DedupKey(changeID, row.Kind, row.Recipient), row.DedupKey)
		assert.Equal(t, "42", row.Payload["incident_id"])
		assert.Equal(t, "DB upgrade", row.Payload["title"])
		assert.Equal(t, "https://status.example.com/incidents/42", row.Payload["link"])
		seenRecipients[row.Recipient] = struct{}{}
	}
	assert.Len(t, seenRecipients, 4, "recipients are unique")
}

func TestBuildRows_NoRecipientsReturnsNil(t *testing.T) {
	r := testResolver()
	rows := r.BuildRows(Change{
		IncidentID: 7,
		NewStatus:  event.MaintenancePlanned, // lifecycle -> creator only
		// no contact email
	})
	assert.Nil(t, rows)
}
