package v2

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/stackmon/otc-status-dashboard/internal/api/rbac"
	"github.com/stackmon/otc-status-dashboard/internal/db"
	"github.com/stackmon/otc-status-dashboard/internal/event"
)

func TestResolveMaintenanceCreateStatus(t *testing.T) {
	tests := []struct {
		name           string
		role           rbac.Role
		expectedStatus event.Status
		expectForbid   bool
	}{
		{
			name:           "Admin creates maintenance with planned status",
			role:           rbac.Admin,
			expectedStatus: event.MaintenancePlanned,
			expectForbid:   false,
		},
		{
			name:           "Operator creates maintenance with planned status",
			role:           rbac.Operator,
			expectedStatus: event.MaintenancePlanned,
			expectForbid:   false,
		},
		{
			name:           "Creator creates maintenance with pending review status",
			role:           rbac.Creator,
			expectedStatus: event.MaintenancePendingReview,
			expectForbid:   false,
		},
		{
			name:           "NoRole is forbidden",
			role:           rbac.NoRole,
			expectedStatus: "",
			expectForbid:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			status := resolveMaintenanceCreateStatus(c, tc.role)

			assert.Equal(t, tc.expectedStatus, status)
			if tc.expectForbid {
				assert.Equal(t, 403, w.Code)
			}
		})
	}
}

func TestAllowMaintenancePatch(t *testing.T) {
	tests := []struct {
		name           string
		role           rbac.Role
		storedStatus   event.Status
		incomingStatus event.Status
		expectAllow    bool
	}{
		// Admin tests - always allowed
		{
			name:           "Admin can patch pending review",
			role:           rbac.Admin,
			storedStatus:   event.MaintenancePendingReview,
			incomingStatus: event.MaintenanceReviewed,
			expectAllow:    true,
		},
		{
			name:           "Admin can patch reviewed",
			role:           rbac.Admin,
			storedStatus:   event.MaintenanceReviewed,
			incomingStatus: event.MaintenancePlanned,
			expectAllow:    true,
		},
		{
			name:           "Admin can patch planned",
			role:           rbac.Admin,
			storedStatus:   event.MaintenancePlanned,
			incomingStatus: event.MaintenanceCancelled,
			expectAllow:    true,
		},

		// Operator tests
		{
			name:           "Operator can approve pending review to reviewed",
			role:           rbac.Operator,
			storedStatus:   event.MaintenancePendingReview,
			incomingStatus: event.MaintenanceReviewed,
			expectAllow:    true,
		},
		{
			name:           "Operator can cancel pending review",
			role:           rbac.Operator,
			storedStatus:   event.MaintenancePendingReview,
			incomingStatus: event.MaintenanceCancelled,
			expectAllow:    true,
		},
		{
			name:           "Operator can update pending review",
			role:           rbac.Operator,
			storedStatus:   event.MaintenancePendingReview,
			incomingStatus: event.MaintenancePendingReview,
			expectAllow:    true,
		},
		{
			name:           "Operator cannot patch reviewed status",
			role:           rbac.Operator,
			storedStatus:   event.MaintenanceReviewed,
			incomingStatus: event.MaintenancePlanned,
			expectAllow:    false,
		},
		{
			name:           "Operator cannot patch planned status",
			role:           rbac.Operator,
			storedStatus:   event.MaintenancePlanned,
			incomingStatus: event.MaintenanceCancelled,
			expectAllow:    false,
		},

		// Creator tests
		{
			name:           "Creator can update pending review",
			role:           rbac.Creator,
			storedStatus:   event.MaintenancePendingReview,
			incomingStatus: event.MaintenancePendingReview,
			expectAllow:    true,
		},
		{
			name:           "Creator can cancel pending review",
			role:           rbac.Creator,
			storedStatus:   event.MaintenancePendingReview,
			incomingStatus: event.MaintenanceCancelled,
			expectAllow:    true,
		},
		{
			name:           "Creator cannot approve to reviewed",
			role:           rbac.Creator,
			storedStatus:   event.MaintenancePendingReview,
			incomingStatus: event.MaintenanceReviewed,
			expectAllow:    false,
		},
		{
			name:           "Creator cannot patch reviewed status",
			role:           rbac.Creator,
			storedStatus:   event.MaintenanceReviewed,
			incomingStatus: event.MaintenancePendingReview,
			expectAllow:    false,
		},
		{
			name:           "Creator cannot patch planned status",
			role:           rbac.Creator,
			storedStatus:   event.MaintenancePlanned,
			incomingStatus: event.MaintenanceCancelled,
			expectAllow:    false,
		},

		// NoRole tests
		{
			name:           "NoRole is always forbidden",
			role:           rbac.NoRole,
			storedStatus:   event.MaintenancePendingReview,
			incomingStatus: event.MaintenancePendingReview,
			expectAllow:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			stored := &db.Incident{
				Status: tc.storedStatus,
			}
			incoming := &PatchIncidentData{
				Status: tc.incomingStatus,
			}

			result := allowMaintenancePatch(c, tc.role, stored, incoming)

			assert.Equal(t, tc.expectAllow, result)
			if !tc.expectAllow {
				assert.Equal(t, 403, w.Code)
			}
		})
	}
}

func TestAllowMaintenancePatchAsOperator(t *testing.T) {
	tests := []struct {
		name           string
		storedStatus   event.Status
		incomingStatus event.Status
		expectAllow    bool
	}{
		{
			name:           "Approve: pending review to reviewed",
			storedStatus:   event.MaintenancePendingReview,
			incomingStatus: event.MaintenanceReviewed,
			expectAllow:    true,
		},
		{
			name:           "Cancel: pending review to cancelled",
			storedStatus:   event.MaintenancePendingReview,
			incomingStatus: event.MaintenanceCancelled,
			expectAllow:    true,
		},
		{
			name:           "Update: pending review stays pending review",
			storedStatus:   event.MaintenancePendingReview,
			incomingStatus: event.MaintenancePendingReview,
			expectAllow:    true,
		},
		{
			name:           "Forbidden: pending review to planned directly",
			storedStatus:   event.MaintenancePendingReview,
			incomingStatus: event.MaintenancePlanned,
			expectAllow:    false,
		},
		{
			name:           "Forbidden: reviewed status",
			storedStatus:   event.MaintenanceReviewed,
			incomingStatus: event.MaintenancePlanned,
			expectAllow:    false,
		},
		{
			name:           "Forbidden: planned status",
			storedStatus:   event.MaintenancePlanned,
			incomingStatus: event.MaintenanceInProgress,
			expectAllow:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			stored := &db.Incident{Status: tc.storedStatus}
			incoming := &PatchIncidentData{Status: tc.incomingStatus}

			result := allowMaintenancePatchAsOperator(c, stored, incoming)

			assert.Equal(t, tc.expectAllow, result)
			if !tc.expectAllow {
				assert.Equal(t, 403, w.Code)
			}
		})
	}
}

func TestAllowMaintenancePatchAsCreator(t *testing.T) {
	tests := []struct {
		name           string
		storedStatus   event.Status
		incomingStatus event.Status
		expectAllow    bool
	}{
		{
			name:           "Update: pending review stays pending review",
			storedStatus:   event.MaintenancePendingReview,
			incomingStatus: event.MaintenancePendingReview,
			expectAllow:    true,
		},
		{
			name:           "Cancel: pending review to cancelled",
			storedStatus:   event.MaintenancePendingReview,
			incomingStatus: event.MaintenanceCancelled,
			expectAllow:    true,
		},
		{
			name:           "Forbidden: approve to reviewed",
			storedStatus:   event.MaintenancePendingReview,
			incomingStatus: event.MaintenanceReviewed,
			expectAllow:    false,
		},
		{
			name:           "Forbidden: change to planned",
			storedStatus:   event.MaintenancePendingReview,
			incomingStatus: event.MaintenancePlanned,
			expectAllow:    false,
		},
		{
			name:           "Forbidden: reviewed status",
			storedStatus:   event.MaintenanceReviewed,
			incomingStatus: event.MaintenanceCancelled,
			expectAllow:    false,
		},
		{
			name:           "Forbidden: planned status",
			storedStatus:   event.MaintenancePlanned,
			incomingStatus: event.MaintenanceCancelled,
			expectAllow:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			stored := &db.Incident{Status: tc.storedStatus}
			incoming := &PatchIncidentData{Status: tc.incomingStatus}

			result := allowMaintenancePatchAsCreator(c, stored, incoming)

			assert.Equal(t, tc.expectAllow, result)
			if !tc.expectAllow {
				assert.Equal(t, 403, w.Code)
			}
		})
	}
}
