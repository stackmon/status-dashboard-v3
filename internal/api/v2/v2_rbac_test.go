package v2

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

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
			logger := zap.NewNop()

			// Set up user context for creator tests
			userID := "test-user-123"
			c.Set(UsernameContextKey, userID)

			stored := &db.Incident{
				Status:    tc.storedStatus,
				CreatedBy: &userID,
			}
			incoming := &PatchIncidentData{
				Status: tc.incomingStatus,
			}

			result := allowMaintenancePatch(c, logger, tc.role, stored, incoming)

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
			logger := zap.NewNop()

			stored := &db.Incident{Status: tc.storedStatus}
			incoming := &PatchIncidentData{Status: tc.incomingStatus}

			result := allowMaintenancePatchAsOperator(c, logger, stored, incoming)

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
			logger := zap.NewNop()

			// Set up user context
			userID := "test-user-123"
			c.Set(UsernameContextKey, userID)

			stored := &db.Incident{
				Status:    tc.storedStatus,
				CreatedBy: &userID,
			}
			incoming := &PatchIncidentData{Status: tc.incomingStatus}

			result := allowMaintenancePatchAsCreator(c, logger, stored, incoming)

			assert.Equal(t, tc.expectAllow, result)
			if !tc.expectAllow {
				assert.Equal(t, 403, w.Code)
			}
		})
	}
}

func TestGetRoleFromContext(t *testing.T) {
	tests := []struct {
		name         string
		setRole      bool
		roleVal      interface{}
		expectRole   rbac.Role
		expectOk     bool
		expectStatus int
	}{
		{
			name:         "Valid role in context",
			setRole:      true,
			roleVal:      rbac.Creator,
			expectRole:   rbac.Creator,
			expectOk:     true,
			expectStatus: 200,
		},
		{
			name:         "Missing role in context",
			setRole:      false,
			expectRole:   rbac.NoRole,
			expectOk:     false,
			expectStatus: 403,
		},
		{
			name:         "Wrong type in context",
			setRole:      true,
			roleVal:      "not-a-role",
			expectRole:   rbac.NoRole,
			expectOk:     false,
			expectStatus: 403,
		},
		{
			name:         "Integer in context instead of rbac.Role",
			setRole:      true,
			roleVal:      42,
			expectRole:   rbac.NoRole,
			expectOk:     false,
			expectStatus: 403,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			logger := zap.NewNop()

			if tc.setRole {
				c.Set("role", tc.roleVal)
			}

			role, ok := getRoleFromContext(c, logger)

			assert.Equal(t, tc.expectOk, ok)
			assert.Equal(t, tc.expectRole, role)
			if !tc.expectOk {
				assert.Equal(t, tc.expectStatus, w.Code)
			}
		})
	}
}

func TestGetUserIDFromContext(t *testing.T) {
	tests := []struct {
		name      string
		setValue  bool
		value     interface{}
		expectNil bool
		expectUID string
	}{
		{
			name:      "Valid userID",
			setValue:  true,
			value:     "test-user",
			expectNil: false,
			expectUID: "test-user",
		},
		{
			name:      "Missing userID key",
			setValue:  false,
			expectNil: true,
		},
		{
			name:      "Empty string userID",
			setValue:  true,
			value:     "",
			expectNil: true,
		},
		{
			name:      "Non-string type",
			setValue:  true,
			value:     12345,
			expectNil: true,
		},
		{
			name:      "Nil value",
			setValue:  true,
			value:     nil,
			expectNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			if tc.setValue {
				c.Set(UsernameContextKey, tc.value)
			}

			result := getUserIDFromContext(c)

			if tc.expectNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tc.expectUID, *result)
			}
		})
	}
}

func TestAllowMaintenancePatchAsCreatorOwnership(t *testing.T) {
	otherUser := "other-user"

	tests := []struct {
		name        string
		setUser     bool
		userID      string
		createdBy   *string
		expectAllow bool
	}{
		{
			name:        "No userID in context",
			setUser:     false,
			createdBy:   &otherUser,
			expectAllow: false,
		},
		{
			name:        "CreatedBy is nil",
			setUser:     true,
			userID:      "user-a",
			createdBy:   nil,
			expectAllow: false,
		},
		{
			name:        "Both nil: no userID and nil CreatedBy",
			setUser:     false,
			createdBy:   nil,
			expectAllow: false,
		},
		{
			name:        "Mismatched users",
			setUser:     true,
			userID:      "user-a",
			createdBy:   &otherUser,
			expectAllow: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			logger := zap.NewNop()

			if tc.setUser {
				c.Set(UsernameContextKey, tc.userID)
			}

			stored := &db.Incident{
				Status:    event.MaintenancePendingReview,
				CreatedBy: tc.createdBy,
			}
			incoming := &PatchIncidentData{Status: event.MaintenancePendingReview}

			result := allowMaintenancePatchAsCreator(c, logger, stored, incoming)

			assert.Equal(t, tc.expectAllow, result)
			assert.Equal(t, 403, w.Code)
		})
	}
}

func TestPrepareIncidentCreateNonMaintenance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	logger := zap.NewNop()

	impact := 1
	startDate := time.Now().Add(-time.Hour).UTC()
	incData := &IncidentData{
		Title:       "Test incident",
		Description: "desc",
		Impact:      &impact,
		Components:  []int{1},
		StartDate:   startDate,
		Type:        event.TypeIncident,
	}

	result := prepareIncidentCreate(c, logger, incData)

	assert.True(t, result, "non-maintenance should pass without RBAC check")
	assert.Empty(t, incData.Status, "status should not be set for non-maintenance")
}

func TestPrepareIncidentPatchNonMaintenance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	logger := zap.NewNop()

	impact := 2
	stored := &db.Incident{
		Type:   event.TypeIncident,
		Status: event.IncidentAnalysing,
		Impact: &impact,
	}
	version := 1
	incoming := &PatchIncidentData{
		Message:    "update",
		Status:     event.IncidentResolved,
		UpdateDate: time.Now().UTC(),
		Version:    &version,
	}

	result := prepareIncidentPatch(c, logger, stored, incoming)

	assert.True(t, result, "non-maintenance incident patch should pass without RBAC check")
}
