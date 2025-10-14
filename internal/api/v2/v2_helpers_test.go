package v2

import (
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	apiErrors "github.com/stackmon/otc-status-dashboard/internal/api/errors"
	"github.com/stackmon/otc-status-dashboard/internal/db"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func initTests(t *testing.T) (*gin.Engine, sqlmock.Sqlmock, *db.DB) {
	t.Helper()

	t.Log("start initialisation")
	d, m, err := db.NewWithMock()
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	r := gin.Default()
	r.NoRoute(apiErrors.Return404)

	log, _ := zap.NewDevelopment()
	initRoutes(t, r, d, log)

	return r, m, d
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
		v2Api.GET("incidents/:incidentID", GetIncidentHandler(dbInst, log))
		v2Api.PATCH("incidents/:incidentID", PatchIncidentHandler(dbInst, log))
		v2Api.PATCH("incidents/:incidentID/updates/:updateID",
			EventExistenceCheckForTests(dbInst, log),
			PatchEventUpdateTextHandler(dbInst, log),
		)
		v2Api.GET("events", GetEventsHandler(dbInst, log))
		v2Api.GET("availability", GetComponentsAvailabilityHandler(dbInst, log))
	}
}

func prepareIncident(t *testing.T, mock sqlmock.Sqlmock, testTime time.Time) {
	t.Helper()

	mock.ExpectQuery(`^SELECT count\(\*\) FROM "incident"$`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

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
		mock.ExpectQuery(`^SELECT count\(\*\) FROM "incident"`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectQuery(`^SELECT (.+) FROM "incident"`).
			WillReturnRows(sqlmock.NewRows([]string{"id", "text", "description", "start_date", "end_date", "impact", "system", "type"}))
		return
	}

	mock.ExpectQuery(`^SELECT count\(\*\) FROM "incident"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(len(result)))

	rowsInc, incidentIDs, componentIDs := prepareIncidentRows(result)
	mock.ExpectQuery(`^SELECT (.+) FROM "incident"`).
		WillReturnRows(rowsInc)

	rowsIncComp, rowsComp, rowsCompAttr, rowsStatus := prepareRelatedRows(result)

	mock.ExpectQuery(`^SELECT (.+) FROM "incident_component_relation"`).WithArgs(incidentIDs...).WillReturnRows(rowsIncComp)
	mock.ExpectQuery(`^SELECT (.+) FROM "component"`).WithArgs(componentIDs...).WillReturnRows(rowsComp)
	mock.ExpectQuery("^SELECT (.+) FROM \"component_attribute\"").WillReturnRows(rowsCompAttr)
	mock.ExpectQuery(`^SELECT (.+) FROM "incident_status"`).WithArgs(incidentIDs...).WillReturnRows(rowsStatus)
}

func prepareMockForEvents(t *testing.T, mock sqlmock.Sqlmock, result []*db.Incident, totalCount int) {
	t.Helper()

	mock.ExpectQuery(`^SELECT count\(\*\) FROM "incident"`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(totalCount))

	if len(result) == 0 {
		mock.ExpectQuery(`^SELECT (.+) FROM "incident"`).
			WillReturnRows(sqlmock.NewRows([]string{"id", "text", "description", "start_date", "end_date", "impact", "system", "type"}))
		return
	}

	rowsInc, incidentIDs, componentIDs := prepareIncidentRows(result)
	mock.ExpectQuery(`^SELECT (.+) FROM "incident"`).
		WillReturnRows(rowsInc)

	rowsIncComp, rowsComp, rowsCompAttr, rowsStatus := prepareRelatedRows(result)

	mock.ExpectQuery(`^SELECT (.+) FROM "incident_component_relation"`).
		WithArgs(incidentIDs...).
		WillReturnRows(rowsIncComp)

	uniqueComponentIDs := make(map[driver.Value]bool)
	for _, id := range componentIDs {
		uniqueComponentIDs[id] = true
	}
	if len(uniqueComponentIDs) > 0 {
		mock.ExpectQuery(`^SELECT (.+) FROM "component"`).WillReturnRows(rowsComp)
	}

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

func getYearAndMonth(year, month, offset int) (int, int) {
	newMonth := month - offset
	for newMonth <= 0 {
		year--
		newMonth += 12
	}
	return year, newMonth
}

func prepareMockForPatchEventUpdate(t *testing.T, mock sqlmock.Sqlmock, incident *db.Incident, updateID uint, updatedText string, updateIndex int) {
	t.Helper()

	// First mock for GetIncident in EventExistenceCheck middleware
	rowsInc, incidentIDs, componentIDs := prepareIncidentRows([]*db.Incident{incident})
	mock.ExpectQuery(`^SELECT (.+) FROM "incident"`).WillReturnRows(rowsInc)

	rowsIncComp, rowsComp, rowsCompAttr, rowsStatus := prepareRelatedRows([]*db.Incident{incident})

	mock.ExpectQuery(`^SELECT (.+) FROM "incident_component_relation"`).
		WithArgs(incidentIDs...).
		WillReturnRows(rowsIncComp)
	mock.ExpectQuery(`^SELECT (.+) FROM "component"`).
		WithArgs(componentIDs...).
		WillReturnRows(rowsComp)
	mock.ExpectQuery("^SELECT (.+) FROM \"component_attribute\"").
		WillReturnRows(rowsCompAttr)
	mock.ExpectQuery(`^SELECT (.+) FROM "incident_status"`).
		WithArgs(incidentIDs...).
		WillReturnRows(rowsStatus)

	// Second mock for GetEventUpdates in handler - get all updates
	statusRows := sqlmock.NewRows([]string{"id", "incident_id", "status", "text", "timestamp"})
	for _, status := range incident.Statuses {
		statusRows.AddRow(status.ID, status.IncidentID, status.Status, status.Text, status.Timestamp)
	}
	mock.ExpectQuery(`^SELECT \* FROM "incident_status" WHERE incident_id = \$1`).
		WithArgs(incident.ID).
		WillReturnRows(statusRows)

	// Get target update by index
	targetStatus := incident.Statuses[updateIndex]

	// Mock for update transaction
	returningRows := sqlmock.NewRows([]string{"id", "incident_id", "status", "text", "timestamp"})
	returningRows.AddRow(targetStatus.ID, targetStatus.IncidentID, targetStatus.Status, updatedText, targetStatus.Timestamp)

	mock.ExpectBegin()
	mock.ExpectQuery(`^UPDATE "incident_status" SET "text"=\$1 WHERE id = \$2 AND incident_id = \$3 RETURNING .*`).
		WithArgs(updatedText, targetStatus.ID, incident.ID).
		WillReturnRows(returningRows)
	mock.ExpectCommit()

	// Mock for Scan(&updated)
	returningRows = sqlmock.NewRows([]string{"id", "incident_id", "status", "text", "timestamp"})
	returningRows.AddRow(targetStatus.ID, targetStatus.IncidentID, targetStatus.Status, updatedText, targetStatus.Timestamp)
	mock.ExpectQuery(`^SELECT \* FROM "incident_status" WHERE id = \$1 AND incident_id = \$2`).
		WithArgs(updateID, incident.ID).
		WillReturnRows(returningRows)
}

func prepareMockForModifyEventUpdate(
	t *testing.T,
	mock sqlmock.Sqlmock,
	status db.IncidentStatus,
	updatedText string,
) {
	t.Helper()

	mock.ExpectBegin()

	returningRows := sqlmock.NewRows([]string{"id", "incident_id", "text", "status", "timestamp"}).
		AddRow(status.ID, status.IncidentID, updatedText, status.Status, status.Timestamp)
	mock.ExpectQuery(`^UPDATE "incident_status" SET "text"=\$1 WHERE id = \$2 AND incident_id = \$3 RETURNING .*`).
		WithArgs(updatedText, status.ID, status.IncidentID).
		WillReturnRows(returningRows)

	mock.ExpectCommit()
}

// EventExistenceCheckForTests duplicates logic from api.EventExistenceCheck but exists in package v2 tests.
func EventExistenceCheckForTests(dbInst *db.DB, _ *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var uri struct {
			ID uint `uri:"incidentID" binding:"required"`
		}
		if err := c.ShouldBindUri(&uri); err != nil {
			apiErrors.RaiseBadRequestErr(c, err)
			return
		}

		_, err := dbInst.GetIncident(int(uri.ID))
		if err != nil {
			if errors.Is(err, db.ErrDBIncidentDSNotExist) {
				apiErrors.RaiseStatusNotFoundErr(c, apiErrors.ErrIncidentDSNotExist)
				return
			}
			apiErrors.RaiseInternalErr(c, err)
			return
		}

		c.Next()
	}
}
