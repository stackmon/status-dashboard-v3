package v2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/stackmon/otc-status-dashboard/internal/db"
)

func TestMapEventUpdates_FiltersInternalStatuses(t *testing.T) {
	testTime := time.Now().UTC()

	statuses := []db.IncidentStatus{
		{ID: 1, Status: "pending_review", Text: "Pending", Timestamp: testTime},
		{ID: 2, Status: "reviewed", Text: "Reviewed", Timestamp: testTime},
		{ID: 3, Status: "planned", Text: "Planned", Timestamp: testTime},
		{ID: 4, Status: "in_progress", Text: "In progress", Timestamp: testTime},
	}

	t.Run("authenticated sees all statuses", func(t *testing.T) {
		updates := mapEventUpdates(statuses, true)
		assert.Len(t, updates, 4)
		assert.Equal(t, "pending_review", string(updates[0].Status))
		assert.Equal(t, "reviewed", string(updates[1].Status))
	})

	t.Run("unauthenticated sees only public statuses", func(t *testing.T) {
		updates := mapEventUpdates(statuses, false)
		assert.Len(t, updates, 2)
		assert.Equal(t, "planned", string(updates[0].Status))
		assert.Equal(t, "in_progress", string(updates[1].Status))
	})

	t.Run("IDs are sequential after filtering", func(t *testing.T) {
		updates := mapEventUpdates(statuses, false)
		for i, u := range updates {
			assert.Equal(t, i, u.ID)
		}
	})
}

func TestIsCancelledWithoutPublicStatus(t *testing.T) {
	testTime := time.Now().UTC()

	tests := []struct {
		name     string
		incident *db.Incident
		want     bool
	}{
		{
			name: "maintenance cancelled without public status",
			incident: &db.Incident{
				Type:   "maintenance",
				Status: "cancelled",
				Statuses: []db.IncidentStatus{
					{Status: "pending_review", Timestamp: testTime},
					{Status: "cancelled", Timestamp: testTime},
				},
			},
			want: true,
		},
		{
			name: "maintenance cancelled after planned",
			incident: &db.Incident{
				Type:   "maintenance",
				Status: "cancelled",
				Statuses: []db.IncidentStatus{
					{Status: "pending_review", Timestamp: testTime},
					{Status: "planned", Timestamp: testTime},
					{Status: "cancelled", Timestamp: testTime},
				},
			},
			want: false,
		},
		{
			name: "maintenance cancelled after in_progress",
			incident: &db.Incident{
				Type:   "maintenance",
				Status: "cancelled",
				Statuses: []db.IncidentStatus{
					{Status: "planned", Timestamp: testTime},
					{Status: "in_progress", Timestamp: testTime},
					{Status: "cancelled", Timestamp: testTime},
				},
			},
			want: false,
		},
		{
			name: "info cancelled without active status",
			incident: &db.Incident{
				Type:   "info",
				Status: "cancelled",
				Statuses: []db.IncidentStatus{
					{Status: "planned", Timestamp: testTime},
					{Status: "cancelled", Timestamp: testTime},
				},
			},
			want: true,
		},
		{
			name: "info cancelled after active",
			incident: &db.Incident{
				Type:   "info",
				Status: "cancelled",
				Statuses: []db.IncidentStatus{
					{Status: "planned", Timestamp: testTime},
					{Status: "active", Timestamp: testTime},
					{Status: "cancelled", Timestamp: testTime},
				},
			},
			want: false,
		},
		{
			name: "incident type is never hidden",
			incident: &db.Incident{
				Type:   "incident",
				Status: "resolved",
				Statuses: []db.IncidentStatus{
					{Status: "detected", Timestamp: testTime},
					{Status: "resolved", Timestamp: testTime},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isCancelledWithoutPublicStatus(tt.incident)
			assert.Equal(t, tt.want, got)
		})
	}
}
