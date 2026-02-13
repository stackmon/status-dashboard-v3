package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/stackmon/otc-status-dashboard/internal/api"
	"github.com/stackmon/otc-status-dashboard/internal/api/auth"
	apiErrors "github.com/stackmon/otc-status-dashboard/internal/api/errors"
	"github.com/stackmon/otc-status-dashboard/internal/api/rbac"
	v2 "github.com/stackmon/otc-status-dashboard/internal/api/v2"
	"github.com/stackmon/otc-status-dashboard/internal/conf"
	"github.com/stackmon/otc-status-dashboard/internal/db"
	"github.com/stackmon/otc-status-dashboard/internal/event"
)

const testHMACSecret = "test-secret-key-for-rbac-tests"

func generateTestToken(userID string, groups []string) string {
	ifaceGroups := make([]interface{}, len(groups))
	for i, g := range groups {
		ifaceGroups[i] = g
	}

	claims := jwt.MapClaims{
		"preferred_username": userID,
		"groups":             ifaceGroups,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testHMACSecret))
	if err != nil {
		panic(fmt.Sprintf("failed to sign test token: %v", err))
	}
	return signed
}

func initTestsWithHMAC(t *testing.T) *gin.Engine {
	t.Helper()

	d, err := db.New(&conf.Config{DB: databaseURL})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	r := gin.Default()
	r.NoRoute(apiErrors.Return404)
	r.Use(api.ErrorHandle())

	logger, _ := zap.NewDevelopment()
	prov := &auth.Provider{}
	rbacSvc := rbac.New("sd_creators", "sd_operators", "sd_admins")

	v2Api := r.Group("v2")

	v2Api.GET("events",
		api.SetJWTClaims(prov, logger, testHMACSecret),
		v2.GetEventsHandler(d, logger, rbacSvc))
	v2Api.POST("events",
		api.AuthenticationMW(prov, logger, testHMACSecret),
		api.RBACAuthorizationMW(rbacSvc, logger),
		api.ValidateComponentsMW(d, logger),
		v2.PostIncidentHandler(d, logger))
	v2Api.GET("events/:eventID",
		api.SetJWTClaims(prov, logger, testHMACSecret),
		api.CheckEventExistenceMW(d, logger),
		v2.GetIncidentHandler(d, logger, rbacSvc))
	v2Api.PATCH("events/:eventID",
		api.AuthenticationMW(prov, logger, testHMACSecret),
		api.RBACAuthorizationMW(rbacSvc, logger),
		api.CheckEventExistenceMW(d, logger),
		v2.PatchIncidentHandler(d, logger))

	return r
}

func rbacCreateEvent(t *testing.T, r *gin.Engine, inc *v2.IncidentData, token string) *v2.PostIncidentResp {
	t.Helper()

	data, err := json.Marshal(inc)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v2/events", bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Logf("rbacCreateEvent error: %s", w.Body.String())
		return nil
	}

	resp := &v2.PostIncidentResp{}
	err = json.Unmarshal(w.Body.Bytes(), resp)
	require.NoError(t, err)
	return resp
}

func rbacGetEvent(t *testing.T, r *gin.Engine, id int, token string) *v2.Incident {
	t.Helper()

	url := fmt.Sprintf("/v2/events/%d", id)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var incident v2.Incident
	err := json.Unmarshal(w.Body.Bytes(), &incident)
	require.NoError(t, err)
	return &incident
}

func rbacPatchEvent(t *testing.T, r *gin.Engine, id int, patch *v2.PatchIncidentData, token string) *httptest.ResponseRecorder {
	t.Helper()

	data, err := json.Marshal(patch)
	require.NoError(t, err)

	url := fmt.Sprintf("/v2/events/%d", id)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	return w
}

func TestV2PatchEventCreatorOwnership(t *testing.T) {
	truncateIncidents(t)

	r := initTestsWithHMAC(t)

	creatorTokenA := generateTestToken("user-a", []string{"sd_creators"})
	creatorTokenB := generateTestToken("user-b", []string{"sd_creators"})
	operatorToken := generateTestToken("operator-user", []string{"sd_operators"})
	adminToken := generateTestToken("admin-user", []string{"sd_admins"})

	components := []int{1, 2}
	impact := 0
	system := false
	startDate := time.Now().Add(time.Hour).UTC()
	endDate := time.Now().Add(2 * time.Hour).UTC()

	makeMaintenanceData := func() v2.IncidentData {
		return v2.IncidentData{
			Title:        "Test maintenance",
			Description:  "Test description",
			ContactEmail: "test@example.com",
			Impact:       &impact,
			Components:   components,
			StartDate:    startDate,
			EndDate:      &endDate,
			System:       &system,
			Type:         event.TypeMaintenance,
		}
	}

	makePatch := func(status event.Status, version int) *v2.PatchIncidentData {
		return &v2.PatchIncidentData{
			Message:    "test update",
			Status:     status,
			UpdateDate: time.Now().UTC(),
			Version:    &version,
		}
	}

	getVersion := func(inc *v2.Incident) int {
		if inc.Version != nil {
			return *inc.Version
		}
		return 1
	}

	t.Run("creator can patch own event in pending review", func(t *testing.T) {
		truncateIncidents(t)

		incData := makeMaintenanceData()
		result := rbacCreateEvent(t, r, &incData, creatorTokenA)
		require.NotNil(t, result)

		inc := rbacGetEvent(t, r, result.Result[0].IncidentID, creatorTokenA)
		assert.Equal(t, event.MaintenancePendingReview, inc.Updates[0].Status)

		w := rbacPatchEvent(t, r, inc.ID, makePatch(event.MaintenancePendingReview, getVersion(inc)), creatorTokenA)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("creator cannot patch another creators event", func(t *testing.T) {
		truncateIncidents(t)

		incData := makeMaintenanceData()
		result := rbacCreateEvent(t, r, &incData, creatorTokenA)
		require.NotNil(t, result)

		inc := rbacGetEvent(t, r, result.Result[0].IncidentID, creatorTokenA)

		w := rbacPatchEvent(t, r, inc.ID, makePatch(event.MaintenancePendingReview, getVersion(inc)), creatorTokenB)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("admin can patch any creators event", func(t *testing.T) {
		truncateIncidents(t)

		incData := makeMaintenanceData()
		result := rbacCreateEvent(t, r, &incData, creatorTokenA)
		require.NotNil(t, result)

		inc := rbacGetEvent(t, r, result.Result[0].IncidentID, creatorTokenA)

		w := rbacPatchEvent(t, r, inc.ID, makePatch(event.MaintenanceReviewed, getVersion(inc)), adminToken)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("operator can patch any creators event", func(t *testing.T) {
		truncateIncidents(t)

		incData := makeMaintenanceData()
		result := rbacCreateEvent(t, r, &incData, creatorTokenA)
		require.NotNil(t, result)

		inc := rbacGetEvent(t, r, result.Result[0].IncidentID, creatorTokenA)

		w := rbacPatchEvent(t, r, inc.ID, makePatch(event.MaintenancePendingReview, getVersion(inc)), operatorToken)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("operator can approve any creators event", func(t *testing.T) {
		truncateIncidents(t)

		incData := makeMaintenanceData()
		result := rbacCreateEvent(t, r, &incData, creatorTokenA)
		require.NotNil(t, result)

		inc := rbacGetEvent(t, r, result.Result[0].IncidentID, creatorTokenA)

		w := rbacPatchEvent(t, r, inc.ID, makePatch(event.MaintenanceReviewed, getVersion(inc)), operatorToken)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("creator can cancel own event in pending review", func(t *testing.T) {
		truncateIncidents(t)

		incData := makeMaintenanceData()
		result := rbacCreateEvent(t, r, &incData, creatorTokenA)
		require.NotNil(t, result)

		inc := rbacGetEvent(t, r, result.Result[0].IncidentID, creatorTokenA)

		w := rbacPatchEvent(t, r, inc.ID, makePatch(event.MaintenanceCancelled, getVersion(inc)), creatorTokenA)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("unauthenticated request is rejected", func(t *testing.T) {
		truncateIncidents(t)

		incData := makeMaintenanceData()
		result := rbacCreateEvent(t, r, &incData, creatorTokenA)
		require.NotNil(t, result)

		inc := rbacGetEvent(t, r, result.Result[0].IncidentID, creatorTokenA)

		url := fmt.Sprintf("/v2/events/%d", inc.ID)
		data, _ := json.Marshal(makePatch(event.MaintenancePendingReview, getVersion(inc)))
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestV2MaintenanceVisibility(t *testing.T) {
	truncateIncidents(t)
	r := initTestsWithHMAC(t)

	creatorToken := generateTestToken("user-a", []string{"sd_creators"})

	// Create maintenance (pending review)
	components := []int{1, 2}
	impact := 0
	system := false
	startDate := time.Now().Add(time.Hour).UTC()
	endDate := time.Now().Add(2 * time.Hour).UTC()

	incData := v2.IncidentData{
		Title: "Visibility test", Description: "desc",
		ContactEmail: "test@example.com", Impact: &impact,
		Components: components, StartDate: startDate,
		EndDate: &endDate, System: &system, Type: event.TypeMaintenance,
	}
	result := rbacCreateEvent(t, r, &incData, creatorToken)
	require.NotNil(t, result)
	eventID := result.Result[0].IncidentID

	t.Run("unauth GET list hides pending review maintenance", func(t *testing.T) {
		// GET /v2/events without token
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/v2/events?limit=50&page=1", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Parse response and check no pending review events
		var resp struct {
			Data []v2.Incident `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		for _, ev := range resp.Data {
			assert.NotEqual(t, event.MaintenancePendingReview, ev.Status,
				"pending review event should not be visible to unauthenticated users")
		}
	})

	t.Run("unauth GET by ID returns 404 for pending review", func(t *testing.T) {
		url := fmt.Sprintf("/v2/events/%d", eventID)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("auth GET by ID shows contact_email and creator", func(t *testing.T) {
		inc := rbacGetEvent(t, r, eventID, creatorToken)
		assert.Equal(t, "test@example.com", inc.ContactEmail)
		assert.Equal(t, "user-a", inc.CreatedBy)
	})

	t.Run("unauth GET by ID hides contact_email and creator for non-pending event", func(t *testing.T) {
		// First approve the event as admin so it's visible to unauth
		adminToken := generateTestToken("admin", []string{"sd_admins"})
		inc := rbacGetEvent(t, r, eventID, creatorToken)
		version := 1
		if inc.Version != nil {
			version = *inc.Version
		}
		// Move to reviewed then planned via admin
		patch := &v2.PatchIncidentData{
			Message: "approve", Status: event.MaintenanceReviewed,
			UpdateDate: time.Now().UTC(), Version: &version,
		}
		w := rbacPatchEvent(t, r, eventID, patch, adminToken)
		require.Equal(t, http.StatusOK, w.Code)

		// Get updated version
		inc = rbacGetEvent(t, r, eventID, adminToken)
		version = *inc.Version
		patch = &v2.PatchIncidentData{
			Message: "plan", Status: event.MaintenancePlanned,
			UpdateDate: time.Now().UTC(), Version: &version,
		}
		w = rbacPatchEvent(t, r, eventID, patch, adminToken)
		require.Equal(t, http.StatusOK, w.Code)

		// Now GET without token
		url := fmt.Sprintf("/v2/events/%d", eventID)
		w2 := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		r.ServeHTTP(w2, req)
		assert.Equal(t, http.StatusOK, w2.Code)

		var unauthInc v2.Incident
		err := json.Unmarshal(w2.Body.Bytes(), &unauthInc)
		require.NoError(t, err)
		assert.Empty(t, unauthInc.ContactEmail, "contact_email should be hidden from unauth")
		assert.Empty(t, unauthInc.CreatedBy, "creator should be hidden from unauth")
	})
}

func TestV2MaintenanceVersionConflict(t *testing.T) {
	truncateIncidents(t)
	r := initTestsWithHMAC(t)

	creatorToken := generateTestToken("user-a", []string{"sd_creators"})

	components := []int{1, 2}
	impact := 0
	system := false
	startDate := time.Now().Add(time.Hour).UTC()
	endDate := time.Now().Add(2 * time.Hour).UTC()

	incData := v2.IncidentData{
		Title: "Version test", Description: "desc",
		ContactEmail: "test@example.com", Impact: &impact,
		Components: components, StartDate: startDate,
		EndDate: &endDate, System: &system, Type: event.TypeMaintenance,
	}
	result := rbacCreateEvent(t, r, &incData, creatorToken)
	require.NotNil(t, result)

	inc := rbacGetEvent(t, r, result.Result[0].IncidentID, creatorToken)
	version := 1
	if inc.Version != nil {
		version = *inc.Version
	}

	// First PATCH succeeds
	patch := &v2.PatchIncidentData{
		Message: "first update", Status: event.MaintenancePendingReview,
		UpdateDate: time.Now().UTC(), Version: &version,
	}
	w := rbacPatchEvent(t, r, inc.ID, patch, creatorToken)
	assert.Equal(t, http.StatusOK, w.Code)

	// Second PATCH with same (now stale) version → 409
	patch2 := &v2.PatchIncidentData{
		Message: "second update", Status: event.MaintenancePendingReview,
		UpdateDate: time.Now().UTC(), Version: &version,
	}
	w2 := rbacPatchEvent(t, r, inc.ID, patch2, creatorToken)
	assert.Equal(t, http.StatusConflict, w2.Code)
}

func TestV2MaintenanceValidation(t *testing.T) {
	truncateIncidents(t)
	r := initTestsWithHMAC(t)

	creatorToken := generateTestToken("user-a", []string{"sd_creators"})

	components := []int{1, 2}
	impact := 0
	system := false
	startDate := time.Now().Add(time.Hour).UTC()
	endDate := time.Now().Add(2 * time.Hour).UTC()

	t.Run("invalid email rejected", func(t *testing.T) {
		incData := v2.IncidentData{
			Title: "Test", Description: "desc",
			ContactEmail: "not-an-email", Impact: &impact,
			Components: components, StartDate: startDate,
			EndDate: &endDate, System: &system, Type: event.TypeMaintenance,
		}
		data, _ := json.Marshal(incData)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/v2/events", bytes.NewReader(data))
		req.Header.Set("Authorization", "Bearer "+creatorToken)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("past start_date rejected", func(t *testing.T) {
		pastDate := time.Now().Add(-time.Hour).UTC()
		incData := v2.IncidentData{
			Title: "Test", Description: "desc",
			ContactEmail: "test@example.com", Impact: &impact,
			Components: components, StartDate: pastDate,
			EndDate: &endDate, System: &system, Type: event.TypeMaintenance,
		}
		data, _ := json.Marshal(incData)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/v2/events", bytes.NewReader(data))
		req.Header.Set("Authorization", "Bearer "+creatorToken)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("empty description rejected", func(t *testing.T) {
		incData := v2.IncidentData{
			Title: "Test", Description: "",
			ContactEmail: "test@example.com", Impact: &impact,
			Components: components, StartDate: startDate,
			EndDate: &endDate, System: &system, Type: event.TypeMaintenance,
		}
		data, _ := json.Marshal(incData)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/v2/events", bytes.NewReader(data))
		req.Header.Set("Authorization", "Bearer "+creatorToken)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing contact_email rejected", func(t *testing.T) {
		incData := v2.IncidentData{
			Title: "Test", Description: "desc",
			ContactEmail: "", Impact: &impact,
			Components: components, StartDate: startDate,
			EndDate: &endDate, System: &system, Type: event.TypeMaintenance,
		}
		data, _ := json.Marshal(incData)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/v2/events", bytes.NewReader(data))
		req.Header.Set("Authorization", "Bearer "+creatorToken)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestV2CreatorPatchReviewedEvent(t *testing.T) {
	truncateIncidents(t)
	r := initTestsWithHMAC(t)

	creatorToken := generateTestToken("user-a", []string{"sd_creators"})
	adminToken := generateTestToken("admin", []string{"sd_admins"})

	components := []int{1, 2}
	impact := 0
	system := false
	startDate := time.Now().Add(time.Hour).UTC()
	endDate := time.Now().Add(2 * time.Hour).UTC()

	incData := v2.IncidentData{
		Title: "Review test", Description: "desc",
		ContactEmail: "test@example.com", Impact: &impact,
		Components: components, StartDate: startDate,
		EndDate: &endDate, System: &system, Type: event.TypeMaintenance,
	}
	result := rbacCreateEvent(t, r, &incData, creatorToken)
	require.NotNil(t, result)

	inc := rbacGetEvent(t, r, result.Result[0].IncidentID, creatorToken)
	version := *inc.Version

	// Admin approves → reviewed
	patch := &v2.PatchIncidentData{
		Message: "approved", Status: event.MaintenanceReviewed,
		UpdateDate: time.Now().UTC(), Version: &version,
	}
	w := rbacPatchEvent(t, r, inc.ID, patch, adminToken)
	require.Equal(t, http.StatusOK, w.Code)

	// Get updated version
	inc = rbacGetEvent(t, r, inc.ID, adminToken)
	version = *inc.Version

	// Creator tries to patch reviewed event → 403
	patch2 := &v2.PatchIncidentData{
		Message: "creator tries", Status: event.MaintenanceCancelled,
		UpdateDate: time.Now().UTC(), Version: &version,
	}
	w2 := rbacPatchEvent(t, r, inc.ID, patch2, creatorToken)
	assert.Equal(t, http.StatusForbidden, w2.Code)
}

func TestV2OperatorCreatesMaintenance(t *testing.T) {
	truncateIncidents(t)
	r := initTestsWithHMAC(t)

	operatorToken := generateTestToken("operator-user", []string{"sd_operators"})

	components := []int{1, 2}
	impact := 0
	system := false
	startDate := time.Now().Add(time.Hour).UTC()
	endDate := time.Now().Add(2 * time.Hour).UTC()

	incData := v2.IncidentData{
		Title: "Operator maintenance", Description: "desc",
		ContactEmail: "ops@example.com", Impact: &impact,
		Components: components, StartDate: startDate,
		EndDate: &endDate, System: &system, Type: event.TypeMaintenance,
	}
	result := rbacCreateEvent(t, r, &incData, operatorToken)
	require.NotNil(t, result)

	inc := rbacGetEvent(t, r, result.Result[0].IncidentID, operatorToken)
	assert.Equal(t, event.MaintenancePlanned, inc.Updates[0].Status,
		"operator-created maintenance should have planned status")
}

func TestV2MaintenanceFullWorkflow(t *testing.T) {
	truncateIncidents(t)
	r := initTestsWithHMAC(t)

	creatorToken := generateTestToken("creator-user", []string{"sd_creators"})
	operatorToken := generateTestToken("operator-user", []string{"sd_operators"})
	adminToken := generateTestToken("admin-user", []string{"sd_admins"})

	components := []int{1, 2}
	impact := 0
	system := false
	startDate := time.Now().Add(time.Hour).UTC()
	endDate := time.Now().Add(2 * time.Hour).UTC()

	incData := v2.IncidentData{
		Title: "Workflow test", Description: "full workflow",
		ContactEmail: "workflow@example.com", Impact: &impact,
		Components: components, StartDate: startDate,
		EndDate: &endDate, System: &system, Type: event.TypeMaintenance,
	}

	// Step 1: Creator creates → pending review
	result := rbacCreateEvent(t, r, &incData, creatorToken)
	require.NotNil(t, result)
	eventID := result.Result[0].IncidentID

	inc := rbacGetEvent(t, r, eventID, creatorToken)
	assert.Equal(t, event.MaintenancePendingReview, inc.Updates[0].Status)
	version := *inc.Version

	// Step 2: Operator approves → reviewed
	patch := &v2.PatchIncidentData{
		Message: "approved", Status: event.MaintenanceReviewed,
		UpdateDate: time.Now().UTC(), Version: &version,
	}
	w := rbacPatchEvent(t, r, eventID, patch, operatorToken)
	require.Equal(t, http.StatusOK, w.Code)

	inc = rbacGetEvent(t, r, eventID, operatorToken)
	assert.Equal(t, event.MaintenanceReviewed, inc.Updates[len(inc.Updates)-1].Status)
	version = *inc.Version

	// Step 3: Admin transitions reviewed → planned (operator can only act on pending review)
	patch = &v2.PatchIncidentData{
		Message: "scheduled", Status: event.MaintenancePlanned,
		UpdateDate: time.Now().UTC(), Version: &version,
	}
	w = rbacPatchEvent(t, r, eventID, patch, adminToken)
	require.Equal(t, http.StatusOK, w.Code)

	inc = rbacGetEvent(t, r, eventID, adminToken)
	assert.Equal(t, event.MaintenancePlanned, inc.Updates[len(inc.Updates)-1].Status)
}

func TestV2AdminCreatesMaintenance(t *testing.T) {
	truncateIncidents(t)
	r := initTestsWithHMAC(t)

	adminToken := generateTestToken("admin-user", []string{"sd_admins"})

	components := []int{1, 2}
	impact := 0
	system := false
	startDate := time.Now().Add(time.Hour).UTC()
	endDate := time.Now().Add(2 * time.Hour).UTC()

	incData := v2.IncidentData{
		Title: "Admin maintenance", Description: "admin creates directly",
		ContactEmail: "admin@example.com", Impact: &impact,
		Components: components, StartDate: startDate,
		EndDate: &endDate, System: &system, Type: event.TypeMaintenance,
	}

	result := rbacCreateEvent(t, r, &incData, adminToken)
	require.NotNil(t, result)

	inc := rbacGetEvent(t, r, result.Result[0].IncidentID, adminToken)
	assert.Equal(t, event.MaintenancePlanned, inc.Updates[0].Status,
		"admin-created maintenance should have planned status directly")
}

func TestV2HasExtendedViewBehavior(t *testing.T) {
	truncateIncidents(t)
	r := initTestsWithHMAC(t)

	adminToken := generateTestToken("admin-user", []string{"sd_admins"})
	creatorToken := generateTestToken("creator-user", []string{"sd_creators"})

	components := []int{1, 2}
	impact := 0
	system := false
	startDate := time.Now().Add(time.Hour).UTC()
	endDate := time.Now().Add(2 * time.Hour).UTC()

	// Create one planned (admin) and one pending review (creator)
	plannedData := v2.IncidentData{
		Title: "Planned event", Description: "visible to all",
		ContactEmail: "admin@example.com", Impact: &impact,
		Components: components, StartDate: startDate,
		EndDate: &endDate, System: &system, Type: event.TypeMaintenance,
	}
	plannedResult := rbacCreateEvent(t, r, &plannedData, adminToken)
	require.NotNil(t, plannedResult)

	pendingData := v2.IncidentData{
		Title: "Pending event", Description: "hidden from unauth",
		ContactEmail: "creator@example.com", Impact: &impact,
		Components: components, StartDate: startDate,
		EndDate: &endDate, System: &system, Type: event.TypeMaintenance,
	}
	pendingResult := rbacCreateEvent(t, r, &pendingData, creatorToken)
	require.NotNil(t, pendingResult)

	t.Run("auth user sees both events", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/v2/events?limit=50&page=1", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Data []v2.Incident `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(resp.Data), 2)
	})

	t.Run("unauth user sees only planned event", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/v2/events?limit=50&page=1", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Data []v2.Incident `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		for _, ev := range resp.Data {
			assert.NotEqual(t, event.MaintenancePendingReview, ev.Status,
				"pending review events should be hidden from unauth")
		}
		assert.GreaterOrEqual(t, len(resp.Data), 1, "should see at least the planned event")
	})
}
