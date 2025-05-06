package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/stackmon/otc-status-dashboard/internal/api/v1"
	"github.com/stackmon/otc-status-dashboard/internal/db"
)

func TestV1GetIncidentsHandler(t *testing.T) {
	t.Log("start to test GET /v1/incidents")
	r, _, _ := initTests(t)

	var response = `[{"id":1,"text":"Closed incident without any update","impact":1,"start_date":"2024-10-24 10:12","end_date":"2024-10-24 11:12","updates":[{"status":"resolved","text":"close incident","timestamp":"2024-10-24 11:12"}]}]`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/incidents", nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, response, w.Body.String())
}

func TestV1GetComponentsStatusHandler(t *testing.T) {
	t.Log("start to test GET /v1/component_status")
	r, _, _ := initTests(t)

	var response = `[{"id":1,"attributes":[{"name":"region","value":"EU-DE"},{"name":"category","value":"Container"},{"name":"type","value":"cce"}],"name":"Cloud Container Engine","incidents":[{"id":1,"text":"Closed incident without any update","impact":1,"start_date":"2024-10-24 10:12","end_date":"2024-10-24 11:12","updates":[{"status":"resolved","text":"close incident","timestamp":"2024-10-24 11:12"}]}]},{"id":2,"attributes":[{"name":"region","value":"EU-NL"},{"name":"category","value":"Container"},{"name":"type","value":"cce"}],"name":"Cloud Container Engine","incidents":[]},{"id":3,"attributes":[{"name":"region","value":"EU-DE"},{"name":"category","value":"Compute"},{"name":"type","value":"ecs"}],"name":"Elastic Cloud Server","incidents":[]},{"id":4,"attributes":[{"name":"region","value":"EU-NL"},{"name":"category","value":"Compute"},{"name":"type","value":"ecs"}],"name":"Elastic Cloud Server","incidents":[]},{"id":5,"attributes":[{"name":"region","value":"EU-DE"},{"name":"category","value":"Database"},{"name":"type","value":"dcs"}],"name":"Distributed Cache Service","incidents":[]},{"id":6,"attributes":[{"name":"region","value":"EU-NL"},{"name":"category","value":"Database"},{"name":"type","value":"dcs"}],"name":"Distributed Cache Service","incidents":[]}]`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/component_status", nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, response, w.Body.String())
}

func TestV1PostComponentsStatusHandlerNegative(t *testing.T) {
	t.Log("start to test incident creation and check json data for /v1/component_status")
	r, _, _ := initTests(t)

	type testCase struct {
		ExpectedCode int
		Expected     string
		JSON         string
	}

	testCases := map[string]*testCase{
		"negative testcase, region is invalid": {
			JSON:         `{"name":"Distributed Cache Service","text":"Incident","impact": 2,"attributes": [{"name":"region","value":"EU-NL123"}]}`,
			Expected:     `{"errMsg":"component does not exist"}`,
			ExpectedCode: 400,
		},
		"negative testcase, attribute region is missing": {
			JSON:         `{"name":"Distributed Cache Service","text":"Incident","impact": 2,"attributes": [{"name":"type","value":"dcs"}]}`,
			Expected:     `{"errMsg":"component attribute region is missing or invalid"}`,
			ExpectedCode: 400,
		},
		"negative testcase, component name is wrong": {
			JSON:         `{"name":"New Distributed Cache Service","text":"Incident","impact": 2,"attributes": [{"name":"region","value":"EU-NL"}]}`,
			Expected:     `{"errMsg":"component does not exist"}`,
			ExpectedCode: 400,
		},
	}

	for title, c := range testCases {
		t.Logf("start test case: %s\n", title)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/v1/component_status", strings.NewReader(c.JSON))
		r.ServeHTTP(w, req)

		assert.Equal(t, c.ExpectedCode, w.Code)
		assert.Equal(t, c.Expected, w.Body.String())
	}
}

func TestV1PostComponentsStatusHandler(t *testing.T) {
	t.Log("start to test incident creation, modification by /v1/component_status")
	r, dbIns, _ := initTests(t)

	t.Log("create an incident")

	compName := "Distributed Cache Service"
	attrEUNL := []*v1.ComponentAttribute{{Name: "region", Value: "EU-NL"}}
	impact := 1
	text := "Test incident for dcs"

	componentCreateData := &v1.ComponentStatusPost{
		Name:       compName,
		Impact:     impact,
		Text:       text,
		Attributes: attrEUNL,
	}

	incID, _ := createIncidentByComponentV1(t, r, componentCreateData)

	t.Log("create a new incident with the same component and the same impact, should get an error")
	_, body := createIncidentByComponentV1(t, r, componentCreateData)
	confStruct := &v1.ConflictResponse{}
	err := json.Unmarshal(body, confStruct)
	require.NoError(t, err)

	checkConflictMsgV1(t, confStruct, incID, text)

	t.Log("create a new incident with the same component and higher impact, should update the impact")
	componentCreateData.Impact = 2
	newIncID, body := createIncidentByComponentV1(t, r, componentCreateData)
	assert.Equal(t, incID, newIncID)
	newInc := &v1.Incident{}
	err = json.Unmarshal(body, newInc)
	require.NoError(t, err)
	assert.Equal(t, componentCreateData.Impact, *newInc.Impact)
	assert.Len(t, newInc.Updates, 1)
	assert.Equal(t, "SYSTEM", newInc.Updates[0].Status)
	assert.Equal(t, "impact changed from 1 to 2", newInc.Updates[0].Text)
	assert.NotNil(t, newInc.Updates[0].Timestamp)

	t.Log("create a new incident with another component and same impact, should add component to the incident")
	compName2 := "Cloud Container Engine"
	componentCreateData = &v1.ComponentStatusPost{
		Name:       compName2,
		Impact:     2,
		Text:       text,
		Attributes: attrEUNL,
	}
	activeIncidentID, body := createIncidentByComponentV1(t, r, componentCreateData)
	assert.Equal(t, incID, activeIncidentID)
	newInc = &v1.Incident{}
	err = json.Unmarshal(body, newInc)
	require.NoError(t, err)
	//TODO: it's strange, that we can't check the count of components, fix it (maybe never, because this api is outdated)
	assert.Equal(t, componentCreateData.Impact, *newInc.Impact)
	assert.Len(t, newInc.Updates, 2)
	for _, u := range newInc.Updates {
		if strings.HasPrefix(u.Text, "Cloud Container Engine") {
			assert.Equal(t, "Cloud Container Engine (Container, EU-NL, cce) added", u.Text)
			assert.Equal(t, "SYSTEM", u.Status)
		}
	}

	t.Log("create a new incident with another component and higher impact, should create a new incident with higher impact")
	compName3 := "Elastic Cloud Server"
	text = "Test incident for ecs"
	componentCreateData = &v1.ComponentStatusPost{
		Name:       compName3,
		Impact:     3,
		Text:       text,
		Attributes: attrEUNL,
	}
	newIncID, _ = createIncidentByComponentV1(t, r, componentCreateData)
	assert.NotEqual(t, incID, newIncID)

	t.Log("start to test component movement between incidents")
	t.Log("close incident with impact 3")
	closeIncidentV1(t, r, dbIns, newIncID)

	t.Log("extract component to the new incident with higher impact")
	componentCreateData = &v1.ComponentStatusPost{
		Name:       compName,
		Impact:     3,
		Text:       text,
		Attributes: attrEUNL,
	}
	newIncID, _ = createIncidentByComponentV1(t, r, componentCreateData)
	assert.NotEqual(t, newIncID, activeIncidentID)
	checkIncidentsDataAfterMoveV1(t, r)

	t.Log("extract component to the existed incident with higher impact, close the old incident")
	componentCreateData = &v1.ComponentStatusPost{
		Name:       compName2,
		Impact:     3,
		Text:       text,
		Attributes: attrEUNL,
	}
	newIncID, _ = createIncidentByComponentV1(t, r, componentCreateData)
	assert.NotEqual(t, newIncID, activeIncidentID)
	checkIncidentsDataAfterMoveAndClosedIncidentV1(t, r)

	t.Log("decrease incident impact from 3 to 2")
	decreaseIncidentImpactV1(t, r, dbIns, newIncID)

	t.Log("create an incident with another components with higher impact")

	attrEUDE := []*v1.ComponentAttribute{{Name: "region", Value: "EU-DE"}}
	text = "Test incident for moving component between incidents"

	componentCreateData = &v1.ComponentStatusPost{
		Name:       compName,
		Impact:     3,
		Text:       text,
		Attributes: attrEUDE,
	}
	newIncID, _ = createIncidentByComponentV1(t, r, componentCreateData)
	assert.NotEqual(t, newIncID, activeIncidentID)

	componentCreateData.Name = compName2
	activeIncidentID, _ = createIncidentByComponentV1(t, r, componentCreateData)
	assert.Equal(t, activeIncidentID, newIncID)

	incidents := getIncidentsAPIV1(t, r)
	assert.Len(t, incidents, 5)

	t.Log("send create request, should move component to the incident with higher impact")
	componentCreateData = &v1.ComponentStatusPost{
		Name:       compName,
		Impact:     3,
		Text:       text,
		Attributes: attrEUNL,
	}
	_, _ = createIncidentByComponentV1(t, r, componentCreateData)
	checkIncidentsDataAfterMovingComponentBetweenIncidentsV1(t, r, dbIns)
}

func createIncidentByComponentV1(t *testing.T, r *gin.Engine, inc *v1.ComponentStatusPost) (int, []byte) {
	t.Helper()

	data, err := json.Marshal(inc)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/component_status", bytes.NewReader(data))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		return 0, w.Body.Bytes()
	}

	assert.Equal(t, http.StatusCreated, w.Code)

	respCreated := &v1.Incident{}
	err = json.Unmarshal(w.Body.Bytes(), respCreated)
	require.NoError(t, err)

	assert.Equal(t, inc.Impact, *respCreated.Impact)
	assert.Equal(t, inc.Text, respCreated.Text)
	assert.Nil(t, respCreated.EndDate)

	return respCreated.ID, w.Body.Bytes()
}

func checkConflictMsgV1(t *testing.T, confStruct *v1.ConflictResponse, incID int, text string) {
	t.Helper()
	assert.Equal(t, "Incident with this the component already exists", confStruct.Msg)
	assert.Equal(t, "Check your request parameters", confStruct.Details)
	assert.Equal(t, incID, confStruct.ExistingIncidentID)
	assert.Equal(t, text, confStruct.ExistingIncidentTitle)
}

func closeIncidentV1(t *testing.T, r *gin.Engine, dbIns *db.DB, id int) {
	t.Helper()
	tNow := time.Now()
	inc := &db.Incident{
		ID:      uint(id),
		EndDate: &tNow,
		Statuses: []db.IncidentStatus{
			{
				IncidentID: uint(id),
				Status:     "resolved",
				Text:       "closed for a next test",
				Timestamp:  tNow,
			},
		},
	}
	err := dbIns.ModifyIncident(inc)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/incidents", nil)

	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	incidents := getIncidentsAPIV1(t, r)

	for _, i := range incidents {
		if i.ID == id {
			endTime := time.Time(*i.EndDate)
			assert.Equal(t, tNow.YearDay(), endTime.YearDay())
			assert.Equal(t, tNow.Hour(), endTime.Hour())
			assert.Equal(t, tNow.Minute(), endTime.Minute())
			assert.Len(t, i.Updates, 1)
		}
	}
}

func checkIncidentsDataAfterMoveV1(t *testing.T, r *gin.Engine) {
	t.Helper()

	incidents := getIncidentsAPIV1(t, r)

	for _, inc := range incidents {
		switch inc.ID {
		case 4:
			assert.Nil(t, inc.EndDate)
			assert.Equal(t, 3, *inc.Impact)
			assert.Len(t, inc.Updates, 1)
			assert.Equal(t, "SYSTEM", inc.Updates[0].Status)
			assert.Equal(t, "Distributed Cache Service (Database, EU-NL, dcs) moved from <a href='/incidents/2'>Test incident for dcs</a>", inc.Updates[0].Text)
		case 2:
			assert.Nil(t, inc.EndDate)
			assert.Equal(t, 2, *inc.Impact)
			assert.Len(t, inc.Updates, 3)
			assert.Equal(t, "Distributed Cache Service (Database, EU-NL, dcs) moved to <a href='/incidents/4'>Test incident for ecs</a>", inc.Updates[2].Text)
		}
	}
}

func checkIncidentsDataAfterMovingComponentBetweenIncidentsV1(t *testing.T, r *gin.Engine, dbIns *db.DB) {
	t.Helper()

	incidents := getIncidentsAPIV1(t, r)

	for _, inc := range incidents {
		switch inc.ID {
		case 4:
			assert.Nil(t, inc.EndDate)
			assert.Equal(t, 2, *inc.Impact)
			assert.Len(t, inc.Updates, 3)
			assert.Equal(t, "Distributed Cache Service (Database, EU-NL, dcs) moved to <a href='/incidents/5'>Test incident for moving component between incidents</a>", inc.Updates[2].Text)
		case 5:
			assert.Nil(t, inc.EndDate)
			assert.Equal(t, 3, *inc.Impact)
			assert.Len(t, inc.Updates, 2)
			assert.Equal(t, "Distributed Cache Service (Database, EU-NL, dcs) moved from <a href='/incidents/4'>Test incident for ecs</a>", inc.Updates[1].Text)
		}
	}

	inc, err := dbIns.GetIncident(4)
	require.NoError(t, err)
	assert.Len(t, inc.Components, 1)

	inc, err = dbIns.GetIncident(5)
	require.NoError(t, err)
	assert.Len(t, inc.Components, 3)
}

func checkIncidentsDataAfterMoveAndClosedIncidentV1(t *testing.T, r *gin.Engine) {
	t.Helper()

	incidents := getIncidentsAPIV1(t, r)

	for _, inc := range incidents {
		switch inc.ID {
		case 4:
			assert.Nil(t, inc.EndDate)
			assert.Equal(t, 3, *inc.Impact)
			assert.Len(t, inc.Updates, 2)
			assert.Equal(t, "SYSTEM", inc.Updates[0].Status)
			assert.Equal(t, "SYSTEM", inc.Updates[1].Status)
			assert.Equal(t, "Distributed Cache Service (Database, EU-NL, dcs) moved from <a href='/incidents/2'>Test incident for dcs</a>", inc.Updates[0].Text)
			assert.Equal(t, "Cloud Container Engine (Container, EU-NL, cce) moved from <a href='/incidents/2'>Test incident for dcs</a>", inc.Updates[1].Text)
		case 2:
			assert.NotNil(t, inc.EndDate)
			assert.Equal(t, 2, *inc.Impact)
			assert.Len(t, inc.Updates, 4)
			assert.Equal(t, "Distributed Cache Service (Database, EU-NL, dcs) moved to <a href='/incidents/4'>Test incident for ecs</a>", inc.Updates[2].Text)
			assert.Equal(t, "Cloud Container Engine (Container, EU-NL, cce) moved to <a href='/incidents/4'>Test incident for ecs</a>, Incident closed by system", inc.Updates[3].Text)
		}
	}
}

func getIncidentsAPIV1(t *testing.T, r *gin.Engine) []*v1.Incident {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/incidents", nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var incidents []*v1.Incident
	err := json.Unmarshal(w.Body.Bytes(), &incidents)
	require.NoError(t, err)

	return incidents
}

func decreaseIncidentImpactV1(t *testing.T, r *gin.Engine, dbIns *db.DB, id int) {
	t.Helper()
	impact := 2
	inc := &db.Incident{ID: uint(id), Impact: &impact}

	err := dbIns.ModifyIncident(inc)
	require.NoError(t, err)

	incidents := getIncidentsAPIV1(t, r)
	for _, i := range incidents {
		if i.ID == id {
			assert.Equal(t, impact, *i.Impact)
		}
	}

	require.NoError(t, err)
}
