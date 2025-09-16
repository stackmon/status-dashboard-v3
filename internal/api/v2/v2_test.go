package v2

import (
	"database/sql/driver"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/stackmon/otc-status-dashboard/internal/api/errors"
	"github.com/stackmon/otc-status-dashboard/internal/db"
	"github.com/stackmon/otc-status-dashboard/internal/event"
)

func TestGetIncidentsHandler(t *testing.T) {
	r, m := initTests(t)

	startDate := "2024-09-01T11:45:26.371Z"
	endDate := "2024-09-04T11:45:26.371Z"

	testTime, err := time.Parse(time.RFC3339, startDate)
	require.NoError(t, err)

	prepareIncident(t, m, testTime)

	var response = `{"data":[{"id":1,"title":"Incident title A","description":"Description A","impact":0,"components":[150],"start_date":"%s","end_date":"%s","system":false,"type":"maintenance","updates":[{"status":"resolved","text":"Issue solved.","timestamp":"%s"}]},{"id":2,"title":"Incident title B","description":"Description B","impact":3,"components":[151],"start_date":"%s","end_date":"%s","system":false,"type":"incident","updates":[{"status":"resolved","text":"Issue solved.","timestamp":"%s"}]}]}`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v2/incidents", nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, fmt.Sprintf(response, startDate, endDate, endDate, startDate, endDate, endDate), w.Body.String())
}

func TestGetIncidentsHandlerFilters(t *testing.T) {
	startDate := "2025-03-01T11:45:26.371Z"
	endDate := "2025-03-04T11:45:26.371Z"
	testTime, err := time.Parse(time.RFC3339, startDate)
	require.NoError(t, err)
	testEndTime, err := time.Parse(time.RFC3339, endDate)
	require.NoError(t, err)

	impact0 := 0
	impact3 := 3
	systemFalse := false
	systemTrue := true

	// Mock data setup
	incidentA := db.Incident{
		ID:          1,
		Text:        &[]string{"Incident title A"}[0],
		Description: &[]string{"Description A"}[0],
		StartDate:   &testTime,
		EndDate:     &testEndTime,
		Impact:      &impact0, // Maintenance
		Type:        event.TypeMaintenance,
		System:      systemFalse,
		Components: []db.Component{
			{
				ID:   150,
				Name: "Component A",
				Attrs: []db.ComponentAttr{
					{ID: 859, ComponentID: 150, Name: "category", Value: "A"},
					{ID: 860, ComponentID: 150, Name: "region", Value: "A"},
					{ID: 861, ComponentID: 150, Name: "type", Value: "b"},
				},
			},
		},
		Statuses: []db.IncidentStatus{{ID: 1, IncidentID: 1, Timestamp: testEndTime, Text: "Maintenance completed.", Status: "completed"}},
	}
	incidentB := db.Incident{
		ID:          2,
		Text:        &[]string{"Incident title B"}[0],
		Description: &[]string{"Description B"}[0],
		StartDate:   &testTime,
		EndDate:     nil,      // IsActive
		Impact:      &impact3, // Incident
		Type:        event.TypeIncident,
		System:      systemTrue,
		Components: []db.Component{
			{
				ID:   151,
				Name: "Component B",
				Attrs: []db.ComponentAttr{
					{ID: 862, ComponentID: 151, Name: "category", Value: "B"},
					{ID: 863, ComponentID: 151, Name: "region", Value: "B"},
					{ID: 864, ComponentID: 151, Name: "type", Value: "a"},
				},
			},
		},
		Statuses: []db.IncidentStatus{{ID: 2, IncidentID: 2, Timestamp: testTime, Text: "Incident analysing.", Status: "analysing"}},
	}

	// Expected JSON responses (simplified for brevity)
	responseA := fmt.Sprintf(`{"data":[{"id":1,"title":"Incident title A","description":"Description A","impact":0,"components":[150],"start_date":"%s","end_date":"%s","system":false,"type":"maintenance","updates":[{"status":"completed","text":"Maintenance completed.","timestamp":"%s"}]}]}`, startDate, endDate, endDate)
	responseB := fmt.Sprintf(`{"data":[{"id":2,"title":"Incident title B","description":"Description B","impact":3,"components":[151],"start_date":"%s","system":true,"type":"incident","updates":[{"status":"analysing","text":"Incident analysing.","timestamp":"%s"}]}]}`, startDate, startDate)
	responseEmpty := `{"data":[]}`
	isActiveTrue := true

	testCases := []struct {
		name           string
		url            string
		mockSetup      func(m sqlmock.Sqlmock, params *db.IncidentsParams)
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "Filter by type=maintenance",
			url:  "/v2/incidents?type=maintenance",
			mockSetup: func(m sqlmock.Sqlmock, params *db.IncidentsParams) {
				params.Types = []string{event.TypeMaintenance}
				prepareMockForIncidents(t, m, []*db.Incident{&incidentA})
			},
			expectedStatus: http.StatusOK,
			expectedBody:   responseA,
		},
		{
			name: "Filter by type=incident",
			url:  "/v2/incidents?type=incident",
			mockSetup: func(m sqlmock.Sqlmock, params *db.IncidentsParams) {
				params.Types = []string{event.TypeIncident}
				prepareMockForIncidents(t, m, []*db.Incident{&incidentB})
			},
			expectedStatus: http.StatusOK,
			expectedBody:   responseB,
		},
		{
			name: "Filter by opened=true",
			url:  "/v2/incidents?opened=true",
			mockSetup: func(m sqlmock.Sqlmock, params *db.IncidentsParams) {
				params.IsActive = &isActiveTrue
				prepareMockForIncidents(t, m, []*db.Incident{&incidentB})
			},
			expectedStatus: http.StatusOK,
			expectedBody:   responseB,
		},
		{
			name:           "Filter by active=false",
			url:            "/v2/incidents?active=false",
			mockSetup:      func(_ sqlmock.Sqlmock, _ *db.IncidentsParams) {}, // No DB call expected
			expectedStatus: http.StatusBadRequest,
			expectedBody:   fmt.Sprintf(`{"errMsg":"%s"}`, errors.ErrIncidentFQueryInvalidFormat),
		},
		{
			name: "Filter by impact=3",
			url:  "/v2/incidents?impact=3",
			mockSetup: func(m sqlmock.Sqlmock, params *db.IncidentsParams) {
				params.Impact = &impact3
				prepareMockForIncidents(t, m, []*db.Incident{&incidentB})
			},
			expectedStatus: http.StatusOK,
			expectedBody:   responseB,
		},
		{
			name: "Filter by system=true",
			url:  "/v2/incidents?system=true",
			mockSetup: func(m sqlmock.Sqlmock, params *db.IncidentsParams) {
				params.IsSystem = &systemTrue
				prepareMockForIncidents(t, m, []*db.Incident{&incidentB})
			},
			expectedStatus: http.StatusOK,
			expectedBody:   responseB,
		},
		{
			name: "Filter by components=151",
			url:  "/v2/incidents?components=151",
			mockSetup: func(m sqlmock.Sqlmock, params *db.IncidentsParams) {
				params.ComponentIDs = []int{151}
				prepareMockForIncidents(t, m, []*db.Incident{&incidentB})
			},
			expectedStatus: http.StatusOK,
			expectedBody:   responseB,
		},
		{
			name: "Filter by status=analysing",
			url:  "/v2/incidents?status=analysing",
			mockSetup: func(m sqlmock.Sqlmock, params *db.IncidentsParams) {
				params.Status = &incidentB.Statuses[0].Status
				prepareMockForIncidents(t, m, []*db.Incident{&incidentB})
			},
			expectedStatus: http.StatusOK,
			expectedBody:   responseB,
		},
		{
			name: "Filter by status=analysing",
			url:  "/v2/incidents?status=analysing",
			mockSetup: func(m sqlmock.Sqlmock, params *db.IncidentsParams) {
				params.Status = &incidentB.Statuses[0].Status
				prepareMockForIncidents(t, m, []*db.Incident{&incidentB})
			},
			expectedStatus: http.StatusOK,
			expectedBody:   responseB,
		},
		{
			name: "Filter combination: type=incident&active=true",
			url:  "/v2/incidents?type=incident&active=true",
			mockSetup: func(m sqlmock.Sqlmock, params *db.IncidentsParams) {
				params.Types = []string{event.TypeIncident}
				params.IsActive = &isActiveTrue
				prepareMockForIncidents(t, m, []*db.Incident{&incidentB})
			},
			expectedStatus: http.StatusOK,
			expectedBody:   responseB,
		},
		{
			name: "Filter combination: status=analysing&impact=3",
			url:  "/v2/incidents?status=analysing&impact=3",
			mockSetup: func(m sqlmock.Sqlmock, params *db.IncidentsParams) {
				params.Status = &incidentB.Statuses[0].Status
				prepareMockForIncidents(t, m, []*db.Incident{&incidentB})
			},
			expectedStatus: http.StatusOK,
			expectedBody:   responseB,
		},
		{
			name: "Filter wrong status paramter: status=resurrected",
			url:  "/v2/incidents?status=resurrected",
			mockSetup: func(m sqlmock.Sqlmock, params *db.IncidentsParams) {
				params.Status = &incidentB.Statuses[0].Status
				prepareMockForIncidents(t, m, []*db.Incident{})
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   fmt.Sprintf(`{"errMsg":"%s"}`, errors.ErrIncidentFQueryInvalidFormat),
		},
		{
			name: "Filter combination: no results",
			url:  "/v2/incidents?impact=1",
			mockSetup: func(m sqlmock.Sqlmock, params *db.IncidentsParams) {
				impact1 := 1
				params.Impact = &impact1
				prepareMockForIncidents(t, m, []*db.Incident{}) // Empty slice
			},
			expectedStatus: http.StatusOK,
			expectedBody:   responseEmpty,
		},
		{
			name:           "Invalid filter: type=invalid",
			url:            "/v2/incidents?type=invalid",
			mockSetup:      func(_ sqlmock.Sqlmock, _ *db.IncidentsParams) {}, // No DB call expected
			expectedStatus: http.StatusBadRequest,
			expectedBody:   fmt.Sprintf(`{"errMsg":"%s"}`, errors.ErrIncidentFQueryInvalidFormat),
		},
		{
			name:           "Invalid filter: active=maybe",
			url:            "/v2/incidents?active=maybe",
			mockSetup:      func(_ sqlmock.Sqlmock, _ *db.IncidentsParams) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   fmt.Sprintf(`{"errMsg":"%s"}`, errors.ErrIncidentFQueryInvalidFormat),
		},
		{
			name:           "Invalid filter: active=false",
			url:            "/v2/incidents?active=false",
			mockSetup:      func(_ sqlmock.Sqlmock, _ *db.IncidentsParams) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   fmt.Sprintf(`{"errMsg":"%s"}`, errors.ErrIncidentFQueryInvalidFormat),
		},
		{
			name:           "Invalid filter: impact=abc",
			url:            "/v2/incidents?impact=abc",
			mockSetup:      func(_ sqlmock.Sqlmock, _ *db.IncidentsParams) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   fmt.Sprintf(`{"errMsg":"%s"}`, errors.ErrIncidentFQueryInvalidFormat),
		},
		{
			name:           "Invalid filter: impact=5",
			url:            "/v2/incidents?impact=5",
			mockSetup:      func(_ sqlmock.Sqlmock, _ *db.IncidentsParams) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   fmt.Sprintf(`{"errMsg":"%s"}`, errors.ErrIncidentFQueryInvalidFormat),
		},
		{
			name:           "Invalid filter: components=abc",
			url:            "/v2/incidents?components=abc",
			mockSetup:      func(_ sqlmock.Sqlmock, _ *db.IncidentsParams) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   fmt.Sprintf(`{"errMsg":"%s"}`, errors.ErrIncidentFQueryInvalidFormat),
		},
		{
			name:           "Invalid filter: components=2147483649",
			url:            "/v2/incidents?components=2147483649",
			mockSetup:      func(_ sqlmock.Sqlmock, _ *db.IncidentsParams) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   fmt.Sprintf(`{"errMsg":"%s"}`, errors.ErrIncidentFQueryInvalidFormat),
		},
		{
			name:           "Invalid filter: start_date after end_date",
			url:            fmt.Sprintf("/v2/incidents?start_date=%s&end_date=%s", endDate, startDate), // Swapped start and end dates
			mockSetup:      func(_ sqlmock.Sqlmock, _ *db.IncidentsParams) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   fmt.Sprintf(`{"errMsg":"%s"}`, errors.ErrIncidentFQueryInvalidFormat),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r, m := initTests(t)
			if tc.expectedStatus == http.StatusOK {
				params := &db.IncidentsParams{} // Expected params for the mock
				tc.mockSetup(m, params)
			}
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, tc.url, nil)
			r.ServeHTTP(w, req)
			t.Logf("Test Case: %s - Expected Response: %s", tc.name, tc.expectedBody)
			t.Logf("Test Case: %s - Actual Response: %s", tc.name, w.Body.String())
			assert.Equal(t, tc.expectedStatus, w.Code)
			assert.JSONEq(t, tc.expectedBody, w.Body.String())
			assert.NoError(t, m.ExpectationsWereMet())
		})
	}
}

func TestReturn404Handler(t *testing.T) {
	r, _ := initTests(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/anyendpoint", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
	assert.JSONEq(t, `{"errMsg":"page not found"}`, w.Body.String())
}

func TestGetComponentsAvailabilityHandler(t *testing.T) {
	r, m := initTests(t)
	// Mocking data for testing

	currentTime := time.Now().UTC()
	year, month, _ := currentTime.Date()

	firstDayOfLastMonth := time.Date(year, month-1, 1, 0, 0, 0, 0, time.UTC)
	testTime := firstDayOfLastMonth
	prepareAvailability(t, m, testTime)

	getYearAndMonth := func(year, month, offset int) (int, int) {
		newMonth := month - offset
		for newMonth <= 0 {
			year--
			newMonth += 12
		}
		return year, newMonth
	}

	expectedAvailability := ""
	for i := range [12]int{} {
		availYear, availMonth := getYearAndMonth(year, int(month), i)
		percentage := 100
		// For the second month (current month in test setup), set percentage to 0
		if i == 1 {
			percentage = 0
		}
		expectedAvailability += fmt.Sprintf(`{"year":%d,"month":%d,"percentage":%d},`, availYear, availMonth, percentage)
	}
	// Remove trailing comma
	expectedAvailability = expectedAvailability[:len(expectedAvailability)-1]

	response := fmt.Sprintf(`{"data":[{"id":151,"name":"Component B","availability":[%s],"region":"B"}]}`, expectedAvailability)

	// Sending GET request to get availability of components
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v2/availability", nil)
	r.ServeHTTP(w, req)
	// Checking status code of response and format
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, response, w.Body.String())
	// unmarshal data to golang struct
}

func TestCalculateAvailability(t *testing.T) {
	type testCase struct {
		testDescription string
		Component       *db.Component
		Result          []*MonthlyAvailability
	}

	impact := 3

	comp := db.Component{
		ID:        150,
		Name:      "DataArts",
		Incidents: []*db.Incident{},
	}

	compForSept := comp
	stDate := time.Date(2024, 9, 21, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 10, 2, 20, 0, 0, 0, time.UTC)
	compForSept.Incidents = append(compForSept.Incidents, &db.Incident{
		ID:        1,
		StartDate: &stDate,
		EndDate:   &endDate,
		Impact:    &impact,
	})

	testCases := []testCase{
		{
			testDescription: "Test case: September (66.66667%)- October (94.08602%)",
			Component:       &compForSept,
			Result: func() []*MonthlyAvailability {
				results := make([]*MonthlyAvailability, 12)

				for i := range [12]int{} {
					year, month := getYearAndMonth(time.Now().Year(), int(time.Now().Month()), 12-i-1)
					results[i] = &MonthlyAvailability{
						Year:       year,
						Month:      month,
						Percentage: 100,
					}
					if month == 9 {
						results[i] = &MonthlyAvailability{
							Month:      month,
							Percentage: 66.66667,
						}
					}
					if month == 10 {
						results[i] = &MonthlyAvailability{
							Month:      month,
							Percentage: 94.08602,
						}
					}
				}
				return results
			}(),
		},
	}

	for _, tc := range testCases {
		result, err := calculateAvailability(tc.Component)
		require.NoError(t, err)

		t.Logf("Test '%s': Calculated availability: %+v", tc.testDescription, result)

		assert.Len(t, result, 12)
		for i, r := range result {
			assert.InEpsilon(t, tc.Result[i].Percentage, r.Percentage, 0.0001)
		}
	}
}

func TestValidateStatusesPatches(t *testing.T) {
	// Create test incidents for different types
	infoEvent := &db.Incident{
		Type: event.TypeInformation,
	}

	maintenance := &db.Incident{
		Type: event.TypeMaintenance,
	}

	incident := &db.Incident{
		Type: event.TypeIncident,
	}

	testCases := []struct {
		name        string
		incoming    *PatchIncidentData
		stored      *db.Incident
		expectError bool
		expectedErr error
	}{
		// Information event status tests - all information statuses
		{
			name: "Valid InfoPlanned status for info incident",
			incoming: &PatchIncidentData{
				Status: event.InfoPlanned,
			},
			stored:      infoEvent,
			expectError: false,
		},
		{
			name: "Valid InfoCompleted status for info incident",
			incoming: &PatchIncidentData{
				Status: event.InfoCompleted,
			},
			stored:      infoEvent,
			expectError: false,
		},
		{
			name: "Valid InfoCancelled status for info incident",
			incoming: &PatchIncidentData{
				Status: event.InfoCancelled,
			},
			stored:      infoEvent,
			expectError: false,
		},
		{
			name: "Valid MaintenancePlanned status for info incident, both have same status - planned",
			incoming: &PatchIncidentData{
				Status: event.MaintenancePlanned,
			},
			stored:      infoEvent,
			expectError: false,
		},

		// Maintenance event status tests - all maintenance statuses
		{
			name: "Valid MaintenancePlanned status for maintenance incident",
			incoming: &PatchIncidentData{
				Status: event.MaintenancePlanned,
			},
			stored:      maintenance,
			expectError: false,
		},
		{
			name: "Valid MaintenanceInProgress status for maintenance incident",
			incoming: &PatchIncidentData{
				Status: event.MaintenanceInProgress,
			},
			stored:      maintenance,
			expectError: false,
		},
		{
			name: "Valid MaintenanceCompleted status for maintenance incident",
			incoming: &PatchIncidentData{
				Status: event.MaintenanceCompleted,
			},
			stored:      maintenance,
			expectError: false,
		},
		{
			name: "Valid MaintenanceCancelled status for maintenance incident",
			incoming: &PatchIncidentData{
				Status: event.MaintenanceCancelled,
			},
			stored:      maintenance,
			expectError: false,
		},
		{
			name: "Valid InfoPlanned status for maintenance incident, both have same status - planned",
			incoming: &PatchIncidentData{
				Status: event.InfoPlanned,
			},
			stored:      maintenance,
			expectError: false,
		},

		// Incident event status tests - open statuses
		{
			name: "Valid IncidentDetected status for incident",
			incoming: &PatchIncidentData{
				Status: event.IncidentDetected,
			},
			stored:      incident,
			expectError: false,
		},
		{
			name: "Valid IncidentAnalysing status for incident",
			incoming: &PatchIncidentData{
				Status: event.IncidentAnalysing,
			},
			stored:      incident,
			expectError: false,
		},
		{
			name: "Valid IncidentImpactChanged status for incident",
			incoming: &PatchIncidentData{
				Status: event.IncidentImpactChanged,
			},
			stored:      incident,
			expectError: false,
		},
		{
			name: "Valid IncidentReopened status for incident",
			incoming: &PatchIncidentData{
				Status: event.IncidentReopened,
			},
			stored:      incident,
			expectError: false,
		},
		{
			name: "Valid IncidentChanged status for incident",
			incoming: &PatchIncidentData{
				Status: event.IncidentChanged,
			},
			stored:      incident,
			expectError: false,
		},

		// Incident event status tests - closed statuses
		{
			name: "Valid IncidentResolved status for incident",
			incoming: &PatchIncidentData{
				Status: event.IncidentResolved,
			},
			stored:      incident,
			expectError: false,
		},

		// Invalid status combinations - info incident with non-info statuses

		{
			name: "Invalid IncidentDetected status for info incident",
			incoming: &PatchIncidentData{
				Status: event.IncidentDetected,
			},
			stored:      infoEvent,
			expectError: true,
			expectedErr: errors.ErrIncidentPatchInfoStatus,
		},
		{
			name: "Invalid IncidentResolved status for info incident",
			incoming: &PatchIncidentData{
				Status: event.IncidentResolved,
			},
			stored:      infoEvent,
			expectError: true,
			expectedErr: errors.ErrIncidentPatchInfoStatus,
		},

		// Invalid status combinations - maintenance incident with non-maintenance statuses
		{
			name: "Invalid IncidentDetected status for maintenance incident",
			incoming: &PatchIncidentData{
				Status: event.IncidentDetected,
			},
			stored:      maintenance,
			expectError: true,
			expectedErr: errors.ErrIncidentPatchMaintenanceStatus,
		},
		{
			name: "Invalid IncidentResolved status for maintenance incident",
			incoming: &PatchIncidentData{
				Status: event.IncidentResolved,
			},
			stored:      maintenance,
			expectError: true,
			expectedErr: errors.ErrIncidentPatchMaintenanceStatus,
		},

		// Invalid status combinations - incident with non-incident statuses
		{
			name: "Invalid InfoPlanned status for incident",
			incoming: &PatchIncidentData{
				Status: event.InfoPlanned,
			},
			stored:      incident,
			expectError: true,
			expectedErr: errors.ErrIncidentPatchIncidentStatus,
		},
		{
			name: "Invalid InfoCompleted status for incident",
			incoming: &PatchIncidentData{
				Status: event.InfoCompleted,
			},
			stored:      incident,
			expectError: true,
			expectedErr: errors.ErrIncidentPatchIncidentStatus,
		},
		{
			name: "Invalid MaintenancePlanned status for incident",
			incoming: &PatchIncidentData{
				Status: event.MaintenancePlanned,
			},
			stored:      incident,
			expectError: true,
			expectedErr: errors.ErrIncidentPatchIncidentStatus,
		},
		{
			name: "Invalid MaintenanceCompleted status for incident",
			incoming: &PatchIncidentData{
				Status: event.MaintenanceCompleted,
			},
			stored:      incident,
			expectError: true,
			expectedErr: errors.ErrIncidentPatchIncidentStatus,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateStatusesPatch(tc.incoming, tc.stored)

			if tc.expectError {
				require.Error(t, err)
				if tc.expectedErr != nil {
					assert.Equal(t, tc.expectedErr, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func getYearAndMonth(year, month, offset int) (int, int) {
	newMonth := month - offset
	for newMonth <= 0 {
		year--
		newMonth += 12
	}
	return year, newMonth
}

func initTests(t *testing.T) (*gin.Engine, sqlmock.Sqlmock) {
	t.Helper()

	t.Log("start initialisation")
	d, m, err := db.NewWithMock()
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	r := gin.Default()
	r.NoRoute(errors.Return404)

	log, _ := zap.NewDevelopment()
	initRoutes(t, r, d, log)

	return r, m
}

func initRoutes(t *testing.T, c *gin.Engine, dbInst *db.DB, log *zap.Logger) {
	t.Helper()

	v2Api := c.Group("v2")
	{
		v2Api.GET("components", GetComponentsHandler(dbInst, log))
		v2Api.GET("components/:id", GetComponentHandler(dbInst, log))
		v2Api.GET("component_status", GetComponentsHandler(dbInst, log))
		v2Api.POST("component_status", PostComponentHandler(dbInst, log))

		v2Api.GET("incidents", GetEventsHandler(dbInst, log, false))
		v2Api.POST("incidents", PostIncidentHandler(dbInst, log))
		v2Api.GET("incidents/:id", GetIncidentHandler(dbInst, log))
		v2Api.PATCH("incidents/:id", PatchIncidentHandler(dbInst, log))

		v2Api.GET("availability", GetComponentsAvailabilityHandler(dbInst, log))
	}
}

func prepareIncident(t *testing.T, mock sqlmock.Sqlmock, testTime time.Time) {
	t.Helper()

	rowsInc := sqlmock.NewRows([]string{"id", "text", "description", "start_date", "end_date", "impact", "system", "type"}).
		AddRow(1, "Incident title A", "Description A", testTime, testTime.Add(time.Hour*72), 0, false, "maintenance").
		AddRow(2, "Incident title B", "Description B", testTime, testTime.Add(time.Hour*72), 3, false, "incident")
	mock.ExpectQuery("^SELECT (.+) FROM \"incident\" ORDER BY incident.start_date DESC$").WillReturnRows(rowsInc)

	rowsIncComp := sqlmock.NewRows([]string{"incident_id", "component_id"}).
		AddRow(1, 150).
		AddRow(2, 151)
	mock.ExpectQuery("^SELECT (.+) FROM \"incident_component_relation\"(.+)").WillReturnRows(rowsIncComp)

	rowsComp := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(150, "Component A").
		AddRow(151, "Component B")
	mock.ExpectQuery("^SELECT (.+) FROM \"component\"(.+)").WillReturnRows(rowsComp)

	rowsCompAttr := sqlmock.NewRows([]string{"id", "component_id", "name", "value"}).
		AddRows([][]driver.Value{
			{859, 150, "category", "A"},
			{860, 150, "region", "A"},
			{861, 150, "type", "b"},
			{862, 151, "category", "B"},
			{863, 151, "region", "B"},
			{864, 151, "type", "a"},
		}...)
	mock.ExpectQuery("^SELECT (.+) FROM \"component_attribute\"").WillReturnRows(rowsCompAttr)

	rowsStatus := sqlmock.NewRows([]string{"id", "incident_id", "timestamp", "text", "status"}).
		AddRow(1, 1, testTime.Add(time.Hour*72), "Issue solved.", "resolved").
		AddRow(2, 2, testTime.Add(time.Hour*72), "Issue solved.", "resolved")
	mock.ExpectQuery("^SELECT (.+) FROM \"incident_status\"").WillReturnRows(rowsStatus)

	mock.NewRowsWithColumnDefinition()
}

func prepareIncidentRows(result []*db.Incident) (*sqlmock.Rows, []driver.Value, []driver.Value) {
	incidentIDs := make([]driver.Value, len(result))
	componentIDs := make([]driver.Value, 0)
	rowsInc := sqlmock.NewRows([]string{"id", "text", "description", "start_date", "end_date", "impact", "system", "type"})

	for i, inc := range result {
		incidentIDs[i] = inc.ID
		var descriptionVal interface{}
		if inc.Description != nil {
			descriptionVal = *inc.Description
		}
		rowsInc.AddRow(inc.ID, *inc.Text, descriptionVal, *inc.StartDate, inc.EndDate, *inc.Impact, inc.System, inc.Type)
		for _, comp := range inc.Components {
			componentIDs = append(componentIDs, comp.ID)
		}
	}
	return rowsInc, incidentIDs, componentIDs
}

func prepareRelatedRows(result []*db.Incident) (*sqlmock.Rows, *sqlmock.Rows, *sqlmock.Rows, *sqlmock.Rows) {
	rowsIncComp := sqlmock.NewRows([]string{"incident_id", "component_id"})
	rowsComp := sqlmock.NewRows([]string{"id", "name"})
	rowsCompAttr := sqlmock.NewRows([]string{"id", "component_id", "name", "value"})
	rowsStatus := sqlmock.NewRows([]string{"id", "incident_id", "timestamp", "text", "status"})

	for _, inc := range result {
		for _, comp := range inc.Components {
			rowsIncComp.AddRow(inc.ID, comp.ID)
			rowsComp.AddRow(comp.ID, comp.Name)
			for _, attr := range comp.Attrs {
				rowsCompAttr.AddRow(attr.ID, attr.ComponentID, attr.Name, attr.Value)
			}
		}
		for _, status := range inc.Statuses {
			rowsStatus.AddRow(status.ID, status.IncidentID, status.Timestamp, status.Text, status.Status)
		}
	}
	return rowsIncComp, rowsComp, rowsCompAttr, rowsStatus
}

func prepareMockForIncidents(t *testing.T, mock sqlmock.Sqlmock, result []*db.Incident) {
	t.Helper()

	if len(result) == 0 {
		mock.ExpectQuery(`^SELECT (.+) FROM "incident"`).WillReturnRows(sqlmock.NewRows([]string{"id", "text", "description", "start_date", "end_date", "impact", "system", "type"}))
		return
	}

	rowsInc, incidentIDs, componentIDs := prepareIncidentRows(result)
	mock.ExpectQuery(`^SELECT (.+) FROM "incident"`).WillReturnRows(rowsInc)

	rowsIncComp, rowsComp, rowsCompAttr, rowsStatus := prepareRelatedRows(result)

	mock.ExpectQuery(`^SELECT (.+) FROM "incident_component_relation"`).WithArgs(incidentIDs...).WillReturnRows(rowsIncComp)
	mock.ExpectQuery(`^SELECT (.+) FROM "component"`).WithArgs(componentIDs...).WillReturnRows(rowsComp)
	mock.ExpectQuery("^SELECT (.+) FROM \"component_attribute\"").WillReturnRows(rowsCompAttr)
	mock.ExpectQuery(`^SELECT (.+) FROM "incident_status"`).WithArgs(incidentIDs...).WillReturnRows(rowsStatus)
}

func prepareAvailability(t *testing.T, mock sqlmock.Sqlmock, testTime time.Time) {
	t.Helper()

	rowsComp := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(151, "Component B")
	mock.ExpectQuery("^SELECT (.+) FROM \"component\"$").WillReturnRows(rowsComp)

	rowsCompAttr := sqlmock.NewRows([]string{"id", "component_id", "name", "value"}).
		AddRows([][]driver.Value{
			{862, 151, "category", "B"},
			{863, 151, "region", "B"},
			{864, 151, "type", "a"},
		}...)
	mock.ExpectQuery("^SELECT (.+) FROM \"component_attribute\"").WillReturnRows(rowsCompAttr)

	rowsIncComp := sqlmock.NewRows([]string{"incident_id", "component_id"}).
		AddRow(2, 151)
	mock.ExpectQuery("^SELECT (.+) FROM \"incident_component_relation\"(.+)").WillReturnRows(rowsIncComp)

	startOfMonth := time.Date(testTime.Year(), testTime.Month(), 1, 0, 0, 0, 0, time.UTC)
	startOfNextMonth := startOfMonth.AddDate(0, 1, 0)

	rowsInc := sqlmock.NewRows([]string{"id", "text", "description", "start_date", "end_date", "impact", "system", "type"}).
		AddRow(2, "Incident title B", "Description B for Availability", startOfMonth, startOfNextMonth, 3, false, "incident")
	mock.ExpectQuery("^SELECT (.+) FROM \"incident\" WHERE \"incident\".\"id\" = \\$1$").WillReturnRows(rowsInc)

	rowsStatus := sqlmock.NewRows([]string{"id", "incident_id", "timestamp", "text", "status"}).
		AddRow(2, 2, testTime.Add(time.Hour*72), "Issue solved.", "resolved")
	mock.ExpectQuery("^SELECT (.+) FROM \"incident_status\"").WillReturnRows(rowsStatus)

	mock.NewRowsWithColumnDefinition()
}
