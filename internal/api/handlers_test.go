package api

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

	"github.com/stackmon/otc-status-dashboard/internal/api/errors"
	"github.com/stackmon/otc-status-dashboard/internal/db"
)

var testAPI *API
var mock sqlmock.Sqlmock

func TestGetIncidentsHandler(t *testing.T) {
	initTests(t)

	str := "2024-09-01T11:45:26.371Z"

	testTime, err := time.Parse(time.RFC3339, str)
	require.NoError(t, err)

	prepareDB(t, testTime)

	var response = `{"data":[{"id":1,"title":"Incident title","impact":0,"components":[150],"start_date":"%s","system":false,"updates":[{"id":1,"status":"resolved","text":"Issue solved.","timestamp":"%s"}]}]}`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/incidents", nil)
	testAPI.r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	assert.Equal(t, fmt.Sprintf(response, str, str), w.Body.String())
}

func TestReturn404Handler(t *testing.T) {
	initTests(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/anyendpoint", nil)
	testAPI.r.ServeHTTP(w, req)

	assert.Equal(t, 404, w.Code)
	assert.Equal(t, `{"errMsg":"page not found"}`, w.Body.String())
}

func prepareDB(t *testing.T, testTime time.Time) {
	t.Helper()

	rows := sqlmock.NewRows([]string{"id", "text", "start_date", "end_date", "impact", "system"}).
		AddRow(1, "Incident title", testTime, nil, 0, false)
	mock.ExpectQuery("^SELECT (.+) FROM \"incident\"$").WillReturnRows(rows)

	rowsIncComp := sqlmock.NewRows([]string{"incident_id", "component_id"}).
		AddRow(1, 150)
	mock.ExpectQuery("^SELECT (.+) FROM \"incident_component_relation\"(.+)").WillReturnRows(rowsIncComp)

	rowsComp := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(150, "Cloud Container Engine")
	mock.ExpectQuery("^SELECT (.+) FROM \"component\"(.+)").WillReturnRows(rowsComp)

	rowsStatus := sqlmock.NewRows([]string{"id", "incident_id", "timestamp", "text", "status"}).
		AddRow(1, 1, testTime, "Issue solved.", "resolved")
	mock.ExpectQuery("^SELECT (.+) FROM \"incident_status\"").WillReturnRows(rowsStatus)

	rowsCompAttr := sqlmock.NewRows([]string{"id", "component_id", "name", "value"}).
		AddRows([][]driver.Value{
			{
				859, 150, "category", "Container",
			},
			{
				860, 150, "region", "EU-DE",
			},
			{
				861, 150, "type", "cce",
			},
		}...,
		)
	mock.ExpectQuery("^SELECT (.+) FROM \"component_attribute\"").WillReturnRows(rowsCompAttr)

	mock.NewRowsWithColumnDefinition()
}

func initTests(t *testing.T) {
	t.Helper()

	if testAPI != nil && mock != nil {
		t.Log("testAPI and mock are initialized")
	}

	t.Log("start initialisation")
	r := gin.Default()
	r.Use(ErrorHandle())
	r.NoRoute(errors.Return404)

	d, m, err := db.NewWithMock()
	require.NoError(t, err)

	testAPI = &API{r: r, db: d}
	testAPI.initRoutes()
	mock = m
}
