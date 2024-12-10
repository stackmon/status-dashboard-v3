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
)

func TestGetIncidentsHandler(t *testing.T) {
	r, m := initTests(t)

	startDate := "2024-09-01T11:45:26.371Z"
	endDate := "2024-09-04T11:45:26.371Z"

	testTime, err := time.Parse(time.RFC3339, startDate)
	require.NoError(t, err)

	prepareIncident(t, m, testTime)

	var response = `{"data":[{"id":1,"title":"Incident title","impact":0,"components":[150],"start_date":"%s","system":false,"updates":[{"id":1,"status":"resolved","text":"Issue solved.","timestamp":"%s"}]}]}`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v2/incidents", nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, fmt.Sprintf(response, startDate, endDate, endDate, startDate, endDate, endDate), w.Body.String())
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
		description string
		Component   *db.Component
		Result      []*MonthlyAvailability
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
			description: "Test case: September (66.66667%)- October (94.08602%)",
			Component:   &compForSept,
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

		t.Logf("Test '%s': Calculated availability: %+v", tc.description, result)

		assert.Len(t, result, 12)
		for i, r := range result {
			assert.InEpsilon(t, tc.Result[i].Percentage, r.Percentage, 0.0001)
		}
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

		v2Api.GET("incidents", GetIncidentsHandler(dbInst, log))
		v2Api.POST("incidents", PostIncidentHandler(dbInst, log))
		v2Api.GET("incidents/:id", GetIncidentHandler(dbInst, log))
		v2Api.PATCH("incidents/:id", PatchIncidentHandler(dbInst, log))

		v2Api.GET("availability", GetComponentsAvailabilityHandler(dbInst, log))
	}
}

func prepareIncident(t *testing.T, mock sqlmock.Sqlmock, testTime time.Time) {
	t.Helper()

	rowsInc := sqlmock.NewRows([]string{"id", "text", "start_date", "end_date", "impact", "system"}).
		AddRow(1, "Incident title A", testTime, testTime.Add(time.Hour*72), 0, false).
		AddRow(2, "Incident title B", testTime, testTime.Add(time.Hour*72), 3, false)
	mock.ExpectQuery("^SELECT (.+) FROM \"incident\"$").WillReturnRows(rowsInc)

	rowsIncComp := sqlmock.NewRows([]string{"incident_id", "component_id"}).
		AddRow(1, 150).
		AddRow(2, 151)
	mock.ExpectQuery("^SELECT (.+) FROM \"incident_component_relation\"(.+)").WillReturnRows(rowsIncComp)

	rowsComp := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(150, "Component A").
		AddRow(151, "Component B")
	mock.ExpectQuery("^SELECT (.+) FROM \"component\"(.+)").WillReturnRows(rowsComp)

	rowsStatus := sqlmock.NewRows([]string{"id", "incident_id", "timestamp", "text", "status"}).
		AddRow(1, 1, testTime.Add(time.Hour*72), "Issue solved.", "resolved").
		AddRow(2, 2, testTime.Add(time.Hour*72), "Issue solved.", "resolved")
	mock.ExpectQuery("^SELECT (.+) FROM \"incident_status\"").WillReturnRows(rowsStatus)

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

	mock.NewRowsWithColumnDefinition()
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

	rowsInc := sqlmock.NewRows([]string{"id", "text", "start_date", "end_date", "impact", "system"}).
		AddRow(2, "Incident title B", startOfMonth, startOfNextMonth, 3, false)
	mock.ExpectQuery("^SELECT (.+) FROM \"incident\" WHERE \"incident\".\"id\" = \\$1$").WillReturnRows(rowsInc)

	rowsStatus := sqlmock.NewRows([]string{"id", "incident_id", "timestamp", "text", "status"}).
		AddRow(2, 2, testTime.Add(time.Hour*72), "Issue solved.", "resolved")
	mock.ExpectQuery("^SELECT (.+) FROM \"incident_status\"").WillReturnRows(rowsStatus)

	mock.NewRowsWithColumnDefinition()
}
