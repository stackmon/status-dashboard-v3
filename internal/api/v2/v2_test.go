package v2

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stackmon/otc-status-dashboard/internal/api/errors"
	"github.com/stackmon/otc-status-dashboard/internal/db"
	"github.com/stackmon/otc-status-dashboard/internal/event"
)

func TestGetIncidentsHandler(t *testing.T) {
	r, m, _ := initTests(t)

	startDate := "2024-09-01T11:45:26.371Z"
	endDate := "2024-09-04T11:45:26.371Z"

	testTime, err := time.Parse(time.RFC3339, startDate)
	require.NoError(t, err)

	prepareIncident(t, m, testTime)

	var response = `{"data":[{"id":1,"title":"Incident title A","description":"Description A","impact":0,"components":[150],"start_date":"%s","end_date":"%s","system":false,"type":"maintenance","updates":[{"id":0,"status":"resolved","text":"Issue solved.","timestamp":"%s"}]},{"id":2,"title":"Incident title B","description":"Description B","impact":3,"components":[151],"start_date":"%s","end_date":"%s","system":false,"type":"incident","updates":[{"id":0,"status":"resolved","text":"Issue solved.","timestamp":"%s"}]}]}`

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
	responseA := fmt.Sprintf(`{"data":[{"id":1,"title":"Incident title A","description":"Description A","impact":0,"components":[150],"start_date":"%s","end_date":"%s","system":false,"type":"maintenance","updates":[{"id":0,"status":"completed","text":"Maintenance completed.","timestamp":"%s"}]}]}`, startDate, endDate, endDate)
	responseB := fmt.Sprintf(`{"data":[{"id":2,"title":"Incident title B","description":"Description B","impact":3,"components":[151],"start_date":"%s","system":true,"type":"incident","updates":[{"id":0,"status":"analysing","text":"Incident analysing.","timestamp":"%s"}]}]}`, startDate, startDate)
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
			r, m, _ := initTests(t)
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
	r, _, _ := initTests(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/anyendpoint", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
	assert.JSONEq(t, `{"errMsg":"page not found"}`, w.Body.String())
}

func TestGetEventsHandler(t *testing.T) { //nolint:funlen,gocognit
	const totalEvents = 60
	const maintenanceCount = 10
	const infoCount = 10

	r, m, _ := initTests(t)

	mockEvents := make([]*db.Incident, 0, totalEvents)
	startTime := time.Now().UTC().Add(-time.Hour * 24 * 30)

	for i := 1; i <= totalEvents; i++ {
		eventTitle := fmt.Sprintf("Event %d", i)
		eventDescription := fmt.Sprintf("Description for Event %d", i)
		eventStartTime := startTime.Add(time.Hour * time.Duration(i))
		eventEndTime := eventStartTime.Add(time.Minute * 5)
		componentID := 150 + (i % 5)
		var eventType string
		var impact int
		var status event.Status
		var statusText string

		switch {
		case i <= maintenanceCount:
			eventType = event.TypeMaintenance
			impact = 0
			status = event.MaintenancePlanned
			statusText = event.MaintenancePlannedStatusText()
		case i <= maintenanceCount+infoCount:
			eventType = event.TypeInformation
			impact = 0
			status = event.InfoPlanned
			statusText = event.InfoPlannedStatusText()
		default:
			eventType = event.TypeIncident
			impact = (i % 3) + 1 // Cycle through 1, 2, 3
			status = event.IncidentDetected
			statusText = event.IncidentDetectedStatusText()
		}

		textPtr := new(string)
		*textPtr = eventTitle
		descPtr := new(string)
		*descPtr = eventDescription
		impactPtr := new(int)
		*impactPtr = impact
		startTimePtr := new(time.Time)
		*startTimePtr = eventStartTime
		endTimePtr := new(time.Time)
		*endTimePtr = eventEndTime

		mockEvent := &db.Incident{
			ID:          uint(i),
			Text:        textPtr,
			Description: descPtr,
			StartDate:   startTimePtr,
			EndDate:     endTimePtr,
			Impact:      impactPtr,
			Type:        eventType,
			System:      false,
			Components: []db.Component{
				{ID: uint(componentID), Name: fmt.Sprintf("Component %d", componentID)},
			},
			Statuses: []db.IncidentStatus{
				{ID: uint(i), IncidentID: uint(i), Status: status, Text: statusText, Timestamp: eventStartTime},
			},
			Status: status,
		}
		mockEvents = append(mockEvents, mockEvent)
	}

	testCases := []struct {
		name           string
		url            string
		limit          int
		page           int
		expectedCount  int
		expectedPages  int
		expectedStatus int
	}{
		{
			name:           "Get first page with limit 20",
			url:            "/v2/events?limit=20&page=1",
			limit:          20,
			page:           1,
			expectedCount:  20,
			expectedPages:  3,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Get second page with limit 20",
			url:            "/v2/events?limit=20&page=2",
			limit:          20,
			page:           2,
			expectedCount:  20,
			expectedPages:  3,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Get last page with limit 20",
			url:            "/v2/events?limit=20&page=3",
			limit:          20,
			page:           3,
			expectedCount:  20,
			expectedPages:  3,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Get page beyond total pages",
			url:            "/v2/events?limit=20&page=4",
			limit:          20,
			page:           4,
			expectedCount:  0,
			expectedPages:  3,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Get events with default limit (50)",
			url:            "/v2/events?page=1",
			limit:          50,
			page:           1,
			expectedCount:  50,
			expectedPages:  2,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Get events with invalid limit (150), returns validation error",
			url:            "/v2/events?limit=150&page=1",
			limit:          150,
			page:           1,
			expectedCount:  0,
			expectedPages:  0,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Get page that is out of bounds but should return empty data",
			url:            "/v2/events?limit=50&page=3",
			limit:          50,
			page:           3,
			expectedCount:  0,
			expectedPages:  2,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reversedEvents := make([]*db.Incident, totalEvents)
			for i := range totalEvents {
				reversedEvents[i] = mockEvents[totalEvents-1-i]
			}

			start := (tc.page - 1) * tc.limit
			end := start + tc.limit
			if start > totalEvents {
				start = totalEvents
			}
			if end > totalEvents {
				end = totalEvents
			}
			paginatedMock := reversedEvents[start:end]

			if tc.expectedStatus == http.StatusOK {
				prepareMockForEvents(t, m, paginatedMock, totalEvents)
			}

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, tc.url, nil)
			r.ServeHTTP(w, req)

			t.Logf("Response Body: %s", w.Body.String())

			assert.Equal(t, tc.expectedStatus, w.Code)

			if tc.name == "Get events with invalid limit (150), returns validation error" {
				assert.JSONEq(t, fmt.Sprintf(`{"errMsg":"%s"}`, errors.ErrIncidentFQueryInvalidFormat), w.Body.String())
			}

			assert.Equal(t, tc.expectedStatus, w.Code)

			if tc.expectedStatus == http.StatusOK {
				var response struct {
					Data       []Incident `json:"data"`
					Pagination struct {
						PageIndex      int   `json:"pageIndex"`
						RecordsPerPage int   `json:"recordsPerPage"`
						TotalRecords   int64 `json:"totalRecords"`
						TotalPages     int   `json:"totalPages"`
					} `json:"pagination"`
				}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Len(t, response.Data, tc.expectedCount)
				assert.Equal(t, tc.page, response.Pagination.PageIndex)
				assert.Equal(t, tc.limit, response.Pagination.RecordsPerPage)
				assert.Equal(t, int64(totalEvents), response.Pagination.TotalRecords)
				assert.Equal(t, tc.expectedPages, response.Pagination.TotalPages)
			}

			assert.NoError(t, m.ExpectationsWereMet())
		})
	}
}

func TestGetComponentsAvailabilityHandler(t *testing.T) {
	r, m, _ := initTests(t)
	// Mocking data for testing

	currentTime := time.Now().UTC()
	year, month, _ := currentTime.Date()

	firstDayOfLastMonth := time.Date(year, month-1, 1, 0, 0, 0, 0, time.UTC)
	testTime := firstDayOfLastMonth
	prepareAvailability(t, m, testTime)
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

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v2/availability", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, response, w.Body.String())
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

	compForPeriod := comp
	stDate := time.Date(2025, 6, 21, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2025, 7, 2, 20, 0, 0, 0, time.UTC)
	compForPeriod.Incidents = append(compForPeriod.Incidents, &db.Incident{
		ID:        1,
		StartDate: &stDate,
		EndDate:   &endDate,
		Impact:    &impact,
	})

	testCases := []testCase{
		{
			testDescription: "Test case: June (66.66667%)- July (94.08602%)",
			Component:       &compForPeriod,
			Result: func() []*MonthlyAvailability {
				results := make([]*MonthlyAvailability, 12)

				for i := range [12]int{} {
					year, month := getYearAndMonth(time.Now().Year(), int(time.Now().Month()), 12-i-1)
					results[i] = &MonthlyAvailability{
						Year:       year,
						Month:      month,
						Percentage: 100,
					}
					if month == 6 {
						results[i] = &MonthlyAvailability{
							Month:      month,
							Percentage: 66.66667,
						}
					}
					if month == 7 {
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

func TestPatchEventUpdateHandler(t *testing.T) {
	startDate := "2025-08-01T11:45:26.371Z"
	endDate := "2025-08-04T11:45:26.371Z"
	testTime, err := time.Parse(time.RFC3339, startDate)
	require.NoError(t, err)
	testEndTime, err := time.Parse(time.RFC3339, endDate)
	require.NoError(t, err)

	eventID := 111
	impact2 := 2 // Incident
	systemTrue := true
	updateID1 := 87
	updateID2 := 88
	updateIndex1 := 0
	updateIndex2 := 1

	// Mock data setup
	incidentA := db.Incident{
		ID:          uint(eventID),
		Text:        &[]string{"Incident title A"}[0],
		Description: &[]string{"Description A"}[0],
		StartDate:   &testTime,
		EndDate:     &testEndTime,
		Impact:      &impact2,
		Type:        event.TypeIncident,
		System:      systemTrue,
		Components: []db.Component{
			{
				ID:   151,
				Name: "Component A",
				Attrs: []db.ComponentAttr{
					{ID: 462, ComponentID: 151, Name: "category", Value: "A"},
					{ID: 463, ComponentID: 151, Name: "region", Value: "A"},
					{ID: 464, ComponentID: 151, Name: "type", Value: "a"},
				},
			},
		},
		Statuses: []db.IncidentStatus{
			{ID: uint(updateID1), IncidentID: 111, Status: "analysing", Text: "Incident analysing.", Timestamp: testTime},
			{ID: uint(updateID2), IncidentID: 111, Status: "resolved", Text: "Incident resolved.", Timestamp: testEndTime},
		},
	}

	responseAfterFirst := fmt.Sprintf(
		`{"id":%d,"status":"analysing","text":"Updated: analysing","timestamp":"%s"}`,
		updateIndex1, startDate,
	)
	responseAfterSecond := fmt.Sprintf(
		`{"id":%d,"status":"resolved","text":"Updated: resolved","timestamp":"%s"}`,
		updateIndex2, endDate,
	)

	testCases := []struct {
		name           string
		url            string
		body           string
		mockSetup      func(m sqlmock.Sqlmock)
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "Update incident update id=0",
			url:  fmt.Sprintf("incidents/111/updates/%d", updateIndex1),
			body: `{"text": "Updated: analysing"}`,
			mockSetup: func(m sqlmock.Sqlmock) {
				prepareMockForPatchEventUpdate(
					t, m, &incidentA,
					uint(updateID1),
					"Updated: analysing",
					updateIndex1,
				)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   responseAfterFirst,
		},
		{
			name: "Update incident update id=1",
			url:  fmt.Sprintf("incidents/111/updates/%d", updateIndex2),
			body: `{"text": "Updated: resolved"}`,
			mockSetup: func(m sqlmock.Sqlmock) {
				prepareMockForPatchEventUpdate(
					t, m, &incidentA,
					uint(updateID2),
					"Updated: resolved",
					updateIndex2,
				)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   responseAfterSecond,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r, m, _ := initTests(t)
			tc.mockSetup(m)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodPatch, fmt.Sprintf("/v2/%s", tc.url), strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")

			r.ServeHTTP(w, req)

			t.Logf("Test Case: %s - Expected Response: %s", tc.name, tc.expectedBody)
			t.Logf("Test Case: %s - Actual Response: %s", tc.name, w.Body.String())

			assert.Equal(t, tc.expectedStatus, w.Code)
			assert.JSONEq(t, tc.expectedBody, w.Body.String())
			assert.NoError(t, m.ExpectationsWereMet())
		})
	}
}

func TestModifyEventUpdate(t *testing.T) {
	_, m, d := initTests(t)
	startDate := "2025-08-01T11:45:26.371Z"
	testTime, err := time.Parse(time.RFC3339, startDate)
	require.NoError(t, err)

	eventID := 111
	updateID := 87
	updatedText := "Updated: analysing"

	status := db.IncidentStatus{
		ID:         uint(updateID),
		IncidentID: uint(eventID),
		Status:     "analysing",
		Text:       "Incident analysing.",
		Timestamp:  testTime,
	}
	prepareMockForModifyEventUpdate(t, m, status, updatedText)

	returningRows := sqlmock.NewRows([]string{"id", "incident_id", "text", "status", "timestamp"}).
		AddRow(status.ID, status.IncidentID, updatedText, status.Status, status.Timestamp)
	m.ExpectQuery(`^SELECT \* FROM "incident_status" WHERE id = \$1 AND incident_id = \$2`).
		WithArgs(status.ID, status.IncidentID).
		WillReturnRows(returningRows)

	status.Text = updatedText

	updated, err := d.ModifyEventUpdate(status)
	require.NoError(t, err)
	require.NotZero(t, updated.ID)
	require.Equal(t, updatedText, updated.Text)
}
