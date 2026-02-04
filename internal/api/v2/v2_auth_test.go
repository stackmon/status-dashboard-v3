package v2

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/stackmon/otc-status-dashboard/internal/api/rbac"
	"github.com/stackmon/otc-status-dashboard/internal/db"
)

func TestCreatorFieldExposedToAuthenticated(t *testing.T) {
	d, m, err := db.NewWithMock()
	require.NoError(t, err)
	log := zap.NewNop()

	now := time.Now().UTC()
	creator := "user@example.com"
	contactEmail := "contact@example.com"

	m.ExpectQuery(`^SELECT count\(\*\) FROM "incident"`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	rowsInc := sqlmock.NewRows([]string{"id", "text", "description", "start_date", "end_date", "impact", "system", "type", "created_by", "contact_email"}).
		AddRow(1, "Title", "Desc", now, now.Add(1*time.Hour), 1, false, "maintenance", creator, contactEmail)
	m.ExpectQuery(`^SELECT (.+) FROM "incident"`).WillReturnRows(rowsInc)

	m.ExpectQuery(`^SELECT (.+) FROM "incident_component_relation"`).WillReturnRows(sqlmock.NewRows([]string{"incident_id", "component_id"}))
	m.ExpectQuery(`^SELECT (.+) FROM "incident_status"`).WillReturnRows(sqlmock.NewRows([]string{"id", "incident_id", "timestamp", "text", "status"}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v2/events", nil)

	c.Set("role", rbac.Operator)

	handler := GetEventsHandler(d, log)
	handler(c)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"creator":"user@example.com"`)
}

func TestCreatorFieldHiddenFromUnauthenticated(t *testing.T) {
	d, m, err := db.NewWithMock()
	require.NoError(t, err)
	log := zap.NewNop()

	now := time.Now().UTC()
	creator := "user@example.com"
	contactEmail := "contact@example.com"

	m.ExpectQuery(`^SELECT count\(\*\) FROM "incident"`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	rowsInc := sqlmock.NewRows([]string{"id", "text", "description", "start_date", "end_date", "impact", "system", "type", "created_by", "contact_email"}).
		AddRow(1, "Title", "Desc", now, now.Add(1*time.Hour), 1, false, "maintenance", creator, contactEmail)
	m.ExpectQuery(`^SELECT (.+) FROM "incident"`).WillReturnRows(rowsInc)

	m.ExpectQuery(`^SELECT (.+) FROM "incident_component_relation"`).WillReturnRows(sqlmock.NewRows([]string{"incident_id", "component_id"}))
	m.ExpectQuery(`^SELECT (.+) FROM "incident_status"`).WillReturnRows(sqlmock.NewRows([]string{"id", "incident_id", "timestamp", "text", "status"}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/v2/events", nil)

	handler := GetEventsHandler(d, log)
	handler(c)

	assert.Equal(t, 200, w.Code)
	assert.NotContains(t, w.Body.String(), "creator")
	assert.NotContains(t, w.Body.String(), "user@example.com")
}
