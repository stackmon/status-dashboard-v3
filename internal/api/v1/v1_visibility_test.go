package v1

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stackmon/otc-status-dashboard/internal/db"
	"github.com/stackmon/otc-status-dashboard/internal/event"
)

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
			name: "info type is not affected",
			incident: &db.Incident{
				Type:   "info",
				Status: "cancelled",
				Statuses: []db.IncidentStatus{
					{Status: "planned", Timestamp: testTime},
					{Status: "cancelled", Timestamp: testTime},
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

func TestIsPubliclyVisible(t *testing.T) {
	testTime := time.Now().UTC()

	tests := []struct {
		name     string
		incident *db.Incident
		want     bool
	}{
		{
			name: "pending_review is hidden",
			incident: &db.Incident{
				Type:   "maintenance",
				Status: "pending_review",
			},
			want: false,
		},
		{
			name: "reviewed is hidden",
			incident: &db.Incident{
				Type:   "maintenance",
				Status: "reviewed",
			},
			want: false,
		},
		{
			name: "planned is visible",
			incident: &db.Incident{
				Type:   "maintenance",
				Status: "planned",
			},
			want: true,
		},
		{
			name: "cancelled without public status is hidden",
			incident: &db.Incident{
				Type:   "maintenance",
				Status: "cancelled",
				Statuses: []db.IncidentStatus{
					{Status: "pending_review", Timestamp: testTime},
					{Status: "cancelled", Timestamp: testTime},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPubliclyVisible(tt.incident)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestToPublicIncidents_FiltersUpdates(t *testing.T) {
	testTime := time.Now().UTC()
	startDate := testTime
	dbIncidents := []*db.Incident{
		{
			ID:        1,
			Type:      "maintenance",
			Status:    "planned",
			Text:      stringPtr("Planned maintenance"),
			StartDate: &startDate,
			Statuses: []db.IncidentStatus{
				{Status: "pending_review", Timestamp: testTime},
				{Status: "reviewed", Timestamp: testTime},
				{Status: "planned", Timestamp: testTime},
			},
		},
	}

	incidents := toPublicIncidents(dbIncidents)
	require.Len(t, incidents, 1)
	require.Len(t, incidents[0].Updates, 1)
	assert.Equal(t, event.MaintenancePlanned, incidents[0].Updates[0].Status)
}

func stringPtr(s string) *string {
	return &s
}
