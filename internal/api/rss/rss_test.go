package rss

import (
	"database/sql/driver"
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

func TestRSSHandlerAllIncs(t *testing.T) {
	r, m := initTests(t)

	startDate := "2025-02-01T00:00:01.371Z"

	testTime, err := time.Parse(time.RFC3339, startDate)
	require.NoError(t, err)

	allIncidents(t, m, testTime)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/rss/", nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/rss+xml")
}

func TestRSSHandlerRegion(t *testing.T) {
	r, m := initTests(t)

	startDate := "2025-02-01T00:00:01.371Z"

	testTime, err := time.Parse(time.RFC3339, startDate)
	require.NoError(t, err)

	incidentsByRegion(t, m, testTime)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/rss/?mt=EU-DE", nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/rss+xml")
}

func TestRSSHandlerComponent(t *testing.T) {
	r, m := initTests(t)

	startDate := "2025-02-01T00:00:01.371Z"

	testTime, err := time.Parse(time.RFC3339, startDate)
	require.NoError(t, err)

	incidentsByComponent(t, m, testTime)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/rss/?mt=EU-DE&srv=Component_A", nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/rss+xml")
}

func allIncidents(t *testing.T, mock sqlmock.Sqlmock, testTime time.Time) {
	t.Helper()

	rowsInc := sqlmock.NewRows([]string{"id", "text", "start_date", "end_date", "impact", "system"}).
		AddRow(1, "Incident title A", testTime, testTime.Add(time.Hour*72), 1, false).
		AddRow(2, "Incident title B", testTime, testTime.Add(time.Hour*72), 1, false)
	mock.ExpectQuery("^SELECT (.+) FROM \"incident\"$").WillReturnRows(rowsInc)

	rowsIncComp := sqlmock.NewRows([]string{"incident_id", "component_id"}).
		AddRow(1, 150).
		AddRow(2, 151)
	mock.ExpectQuery("^SELECT (.+) FROM \"incident_component_relation\"(.+)").WillReturnRows(rowsIncComp)

	rowsComp := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(150, "Component_A").
		AddRow(151, "Component_B")
	mock.ExpectQuery("^SELECT (.+) FROM \"component\"(.+)").WillReturnRows(rowsComp)

	rowsStatus := sqlmock.NewRows([]string{"id", "incident_id", "timestamp", "text", "status"}).
		AddRow(1, 1, testTime.Add(time.Hour*72), "Issue solved.", "resolved").
		AddRow(2, 2, testTime.Add(time.Hour*72), "Issue solved.", "resolved")
	mock.ExpectQuery("^SELECT (.+) FROM \"incident_status\"").WillReturnRows(rowsStatus)

	rowsCompAttr := sqlmock.NewRows([]string{"id", "component_id", "name", "value"}).
		AddRows([][]driver.Value{
			{859, 150, "category", "A"},
			{860, 150, "region", "EU-DE"},
			{861, 150, "type", "b"},
			{862, 151, "category", "B"},
			{863, 151, "region", "EU-NL"},
			{864, 151, "type", "a"},
		}...)
	mock.ExpectQuery(`^SELECT (.+) FROM \"component_attribute\"`).WillReturnRows(rowsCompAttr)
}

func incidentsByRegion(t *testing.T, mock sqlmock.Sqlmock, testTime time.Time) {
	t.Helper()

	rowsInc := sqlmock.NewRows([]string{"id", "text", "start_date", "end_date", "impact", "system"}).
		AddRow(1, "Incident title A", testTime, testTime.Add(time.Hour*72), 1, false).
		AddRow(2, "Incident title B", testTime, testTime.Add(time.Hour*72), 1, false)

	mock.ExpectQuery(`^SELECT (.+) FROM "incident" `+
		`JOIN incident_component_relation icr ON icr.incident_id = incident.id `+
		`JOIN component_attribute ca ON ca.component_id = icr.component_id `+
		`WHERE ca.name = \$1 AND ca.value = \$2 ORDER BY incident.id desc LIMIT \$3$`).
		WithArgs("region", "EU-DE", 10).
		WillReturnRows(rowsInc)

	rowsIncComp := sqlmock.NewRows([]string{"incident_id", "component_id"}).
		AddRow(1, 150).
		AddRow(2, 151)
	mock.ExpectQuery(`^SELECT \* FROM "incident_component_relation" WHERE "incident_component_relation"."incident_id" IN \(\$1,\$2\)$`).
		WithArgs(1, 2).
		WillReturnRows(rowsIncComp)

	rowsComp := sqlmock.NewRows([]string{"id"}).
		AddRow(150).
		AddRow(151)
	mock.ExpectQuery(`^SELECT "id" FROM "component" WHERE "component"."id" IN \(\$1,\$2\)$`).
		WithArgs(150, 151).
		WillReturnRows(rowsComp)

	rowsStatus := sqlmock.NewRows([]string{"id", "incident_id", "timestamp", "text", "status"}).
		AddRow(1, 1, testTime.Add(time.Hour*72), "Issue solved.", "resolved").
		AddRow(2, 2, testTime.Add(time.Hour*72), "Issue solved.", "resolved")
	mock.ExpectQuery(`^SELECT (.+) FROM "incident_status" WHERE "incident_status"."incident_id" IN (.+)$`).
		WillReturnRows(rowsStatus)
}

func incidentsByComponent(t *testing.T, mock sqlmock.Sqlmock, testTime time.Time) {
	t.Helper()

	rowsComp := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(150, "Component_A")
	mock.ExpectQuery(`^SELECT \* FROM "component" WHERE name = \$1 `+
		`AND id = \(SELECT component.id FROM "component" `+
		`JOIN component_attribute ca ON ca.component_id = component.id `+
		`WHERE ca.value = \$2 AND component.name = \$3\) `+
		`ORDER BY "component"."id" LIMIT \$4$`).
		WithArgs("Component_A", "EU-DE", "Component_A", 1).
		WillReturnRows(rowsComp)

	rowsCompAttr := sqlmock.NewRows([]string{"id", "component_id", "name", "value"}).
		AddRow(859, 150, "category", "A").
		AddRow(860, 150, "region", "EU-DE").
		AddRow(861, 150, "type", "b")
	mock.ExpectQuery(`^SELECT \* FROM "component_attribute" WHERE "component_attribute"."component_id" = \$1$`).
		WithArgs(150).
		WillReturnRows(rowsCompAttr)

	rowsInc := sqlmock.NewRows([]string{"id", "text", "start_date", "end_date", "impact", "system"}).
		AddRow(1, "Incident title A", testTime, testTime.Add(time.Hour*72), 1, false).
		AddRow(2, "Incident title B", testTime.Add(time.Hour*72), testTime.Add(time.Hour*100), 1, false)
	mock.ExpectQuery(`^SELECT (.+) FROM "incident" `+
		`JOIN incident_component_relation icr ON icr.incident_id = incident.id `+
		`WHERE icr.component_id = \$1 ORDER BY incident.id desc LIMIT \$2$`).
		WithArgs(150, 10).
		WillReturnRows(rowsInc)

	rowsIncComp := sqlmock.NewRows([]string{"incident_id", "component_id"}).
		AddRow(1, 150).
		AddRow(2, 150)
	mock.ExpectQuery(`^SELECT \* FROM "incident_component_relation" WHERE "incident_component_relation"."incident_id" IN \(\$1,\$2\)$`).
		WithArgs(1, 2).
		WillReturnRows(rowsIncComp)

	rowsCompByID := sqlmock.NewRows([]string{"id"}).
		AddRow(150)
	mock.ExpectQuery(`^SELECT "id" FROM "component" WHERE "component"."id" = \$1$`).
		WithArgs(150).
		WillReturnRows(rowsCompByID)

	rowsStatus := sqlmock.NewRows([]string{"id", "incident_id", "timestamp", "text", "status"}).
		AddRow(1, 1, testTime.Add(time.Hour*72), "Issue solved.", "resolved").
		AddRow(2, 2, testTime.Add(time.Hour*172), "Issue solved.", "resolved")
	mock.ExpectQuery(`^SELECT (.+) FROM "incident_status" WHERE "incident_status"."incident_id" IN \(\$1,\$2\)$`).
		WithArgs(1, 2).
		WillReturnRows(rowsStatus)
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

	rssFEED := c.Group("rss")
	{
		rssFEED.GET("/", HandleRSS(dbInst, log))
	}
}
