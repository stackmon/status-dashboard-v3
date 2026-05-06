package tests

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stackmon/otc-status-dashboard/internal/event"
)

// TestVisibility_PendingReviewHiddenFromUnauth verifies that maintenance
// events in pending_review status are not visible to unauthenticated users.
func TestVisibility_PendingReviewHiddenFromUnauth(t *testing.T) {
	r := initTestsWithHMAC(t)
	truncateIncidents(t)

	// Create a pending_review event (creator).
	resp := createEventOK(t, r, maintenanceData(), creatorTokenA)
	eventID := resp.Result[0].IncidentID

	t.Run("GET list hides pending_review", func(t *testing.T) {
		events := listEvents(t, r, "")
		for _, ev := range events {
			assert.NotEqual(t, event.MaintenancePendingReview, ev.Status,
				"pending_review events must not be visible to unauthenticated users")
		}
	})

	t.Run("GET by ID returns 404 for pending_review", func(t *testing.T) {
		w, _ := getEvent(t, r, eventID, "")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// TestVisibility_PendingReviewVisibleToAuth verifies that authenticated
// users can see pending_review events.
func TestVisibility_PendingReviewVisibleToAuth(t *testing.T) {
	r := initTestsWithHMAC(t)
	truncateIncidents(t)

	resp := createEventOK(t, r, maintenanceData(), creatorTokenA)
	eventID := resp.Result[0].IncidentID

	roles := []struct {
		name  string
		token string
	}{
		{"creator", creatorTokenA},
		{"operator", operatorToken},
		{"admin", adminToken},
	}

	for _, role := range roles {
		t.Run(role.name+"_can_see_pending_review_by_id", func(t *testing.T) {
			w, inc := getEvent(t, r, eventID, role.token)
			assert.Equal(t, http.StatusOK, w.Code)
			require.NotNil(t, inc)
		})
	}

	t.Run("auth user sees pending_review in list", func(t *testing.T) {
		events := listEvents(t, r, adminToken)
		found := false
		for _, ev := range events {
			if ev.ID == eventID {
				found = true
				break
			}
		}
		assert.True(t, found, "authenticated user should see pending_review event in list")
	})
}

// TestVisibility_ContactEmailAndCreator verifies that contact_email and
// creator fields are visible to authenticated users but hidden from
// unauthenticated users.
func TestVisibility_ContactEmailAndCreator(t *testing.T) {
	r := initTestsWithHMAC(t)
	truncateIncidents(t)

	// Create a maintenance event (visible to unauth since it starts as planned).
	resp := createEventOK(t, r, maintenanceData(), adminToken)
	eventID := resp.Result[0].IncidentID

	t.Run("auth user sees contact_email and creator", func(t *testing.T) {
		inc := getEventOK(t, r, eventID, adminToken)
		assert.NotEmpty(t, inc.ContactEmail, "contact_email should be visible to authenticated user")
		assert.NotEmpty(t, inc.CreatedBy, "creator should be visible to authenticated user")
	})

	t.Run("unauth user does not see contact_email and creator", func(t *testing.T) {
		_, inc := getEvent(t, r, eventID, "")
		require.NotNil(t, inc)
		assert.Empty(t, inc.ContactEmail, "contact_email must be hidden from unauthenticated users")
		assert.Empty(t, inc.CreatedBy, "creator must be hidden from unauthenticated users")
	})
}

// TestVisibility_AuthVsUnauthEventList verifies that authenticated users
// see more events than unauthenticated users (pending_review included).
func TestVisibility_AuthVsUnauthEventList(t *testing.T) {
	r := initTestsWithHMAC(t)
	truncateIncidents(t)

	// Create one planned (visible) and one pending_review (hidden from unauth).
	createEventOK(t, r, maintenanceData(), adminToken)    // planned
	createEventOK(t, r, maintenanceData(), creatorTokenA) // pending_review

	authEvents := listEvents(t, r, adminToken)
	unauthEvents := listEvents(t, r, "")

	assert.GreaterOrEqual(t, len(authEvents), 2, "auth user should see both events")
	assert.Less(t, len(unauthEvents), len(authEvents),
		"unauth user should see fewer events (pending_review hidden)")

	// Verify no pending_review in unauth list.
	for _, ev := range unauthEvents {
		assert.NotEqual(t, event.MaintenancePendingReview, ev.Status)
	}
}
