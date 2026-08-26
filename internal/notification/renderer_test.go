package notification

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stackmon/otc-status-dashboard/internal/db"
)

func TestRender_SubjectAndBodyFromPayload(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)

	row := db.NotificationOutbox{
		Payload: map[string]any{
			"incident_id": "42",
			"title":       "DB upgrade",
			"old_status":  "pending_review",
			"new_status":  "reviewed",
			"actor":       "admin-user",
			"changed_at":  "2026-08-26T10:00:00Z",
			"link":        "https://status.example.com/incidents/42",
		},
	}

	email, err := r.Render(row)
	require.NoError(t, err)

	assert.Equal(t, "[Maintenance] DB upgrade — reviewed", email.Subject)
	assert.Contains(t, email.Body, "event #42")
	assert.Contains(t, email.Body, "pending_review -> reviewed")
	assert.Contains(t, email.Body, "admin-user")
	assert.Contains(t, email.Body, "https://status.example.com/incidents/42")
}

func TestRender_OmitsArrowWhenNoOldStatus(t *testing.T) {
	r, err := NewRenderer()
	require.NoError(t, err)

	row := db.NotificationOutbox{
		Payload: map[string]any{
			"title":      "New maintenance",
			"new_status": "pending_review",
		},
	}

	email, err := r.Render(row)
	require.NoError(t, err)
	assert.Contains(t, email.Body, "Status: pending_review")
	assert.NotContains(t, email.Body, "->", "no arrow without old status")
}
