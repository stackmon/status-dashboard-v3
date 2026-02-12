package checker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/stackmon/otc-status-dashboard/internal/db"
	"github.com/stackmon/otc-status-dashboard/internal/event"
)

func TestCalculateCurrentMntStatus(t *testing.T) {
	future := time.Now().UTC().Add(24 * time.Hour)
	farFuture := future.Add(48 * time.Hour)
	past := time.Now().UTC().Add(-24 * time.Hour)
	recentPast := time.Now().UTC().Add(-1 * time.Hour)

	tests := []struct {
		name           string
		status         event.Status
		startDate      time.Time
		endDate        time.Time
		history        *MntStatusHistory
		expectedStatus event.Status
	}{
		{
			name:           "Reviewed transitions to planned",
			status:         event.MaintenanceReviewed,
			startDate:      future,
			endDate:        farFuture,
			history:        &MntStatusHistory{hasReviewed: true},
			expectedStatus: event.MaintenancePlanned,
		},
		{
			name:           "Pending review stays pending review",
			status:         event.MaintenancePendingReview,
			startDate:      future,
			endDate:        farFuture,
			history:        &MntStatusHistory{},
			expectedStatus: event.MaintenancePendingReview,
		},
		{
			name:           "Cancelled overrides reviewed",
			status:         event.MaintenanceReviewed,
			startDate:      future,
			endDate:        farFuture,
			history:        &MntStatusHistory{hasReviewed: true, hasCancelled: true},
			expectedStatus: event.MaintenanceCancelled,
		},
		{
			name:           "Planned with future start stays planned",
			status:         event.MaintenancePlanned,
			startDate:      future,
			endDate:        farFuture,
			history:        &MntStatusHistory{hasPlanned: true},
			expectedStatus: event.MaintenancePlanned,
		},
		{
			name:           "Planned with past start becomes in progress",
			status:         event.MaintenancePlanned,
			startDate:      past,
			endDate:        future,
			history:        &MntStatusHistory{hasPlanned: true},
			expectedStatus: event.MaintenanceInProgress,
		},
		{
			name:           "Planned with past end becomes completed",
			status:         event.MaintenancePlanned,
			startDate:      past,
			endDate:        recentPast,
			history:        &MntStatusHistory{hasPlanned: true},
			expectedStatus: event.MaintenanceCompleted,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mn := &db.Incident{
				Status:    tc.status,
				StartDate: &tc.startDate,
				EndDate:   &tc.endDate,
			}

			result := calculateCurrentMntStatus(tc.history, mn)
			assert.Equal(t, tc.expectedStatus, result)
		})
	}
}
