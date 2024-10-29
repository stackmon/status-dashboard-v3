package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/stackmon/otc-status-dashboard/internal/api/v1"
)

func TestGetIncidentsHandler(t *testing.T) {
	t.Log("start to test GET /v1/incidents")
	r, _ := initTests(t)

	var response = `[{"id":1,"text":"Closed incident without any update","impact":1,"start_date":"2024-10-24 10:12","end_date":"2024-10-24 11:12","updates":[{"status":"resolved","text":"close incident","timestamp":"2024-10-24 11:12"}]}]`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/incidents", nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, response, w.Body.String())
}

func TestGetComponentsStatusHandler(t *testing.T) {
	t.Log("start to test GET /v1/component_status")
	r, _ := initTests(t)

	var response = `[{"id":1,"attributes":[{"name":"region","value":"EU-DE"},{"name":"category","value":"Container"},{"name":"type","value":"cce"}],"name":"Cloud Container Engine","incidents":[{"id":1,"text":"Closed incident without any update","impact":1,"start_date":"2024-10-24 10:12","end_date":"2024-10-24 11:12","updates":[{"status":"resolved","text":"close incident","timestamp":"2024-10-24 11:12"}]}]},{"id":2,"attributes":[{"name":"region","value":"EU-NL"},{"name":"category","value":"Container"},{"name":"type","value":"cce"}],"name":"Cloud Container Engine","incidents":[]},{"id":3,"attributes":[{"name":"region","value":"EU-DE"},{"name":"category","value":"Compute"},{"name":"type","value":"ecs"}],"name":"Elastic Cloud Server","incidents":[]},{"id":4,"attributes":[{"name":"region","value":"EU-NL"},{"name":"category","value":"Compute"},{"name":"type","value":"ecs"}],"name":"Elastic Cloud Server","incidents":[]},{"id":5,"attributes":[{"name":"region","value":"EU-DE"},{"name":"category","value":"Database"},{"name":"type","value":"dcs"}],"name":"Distributed Cache Service","incidents":[]},{"id":6,"attributes":[{"name":"region","value":"EU-NL"},{"name":"category","value":"Database"},{"name":"type","value":"dcs"}],"name":"Distributed Cache Service","incidents":[]}]`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/component_status", nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, response, w.Body.String())
}

func TestPostComponentsStatusHandler(t *testing.T) {
	t.Log("start to test incident creation and check json data for /v1/component_status")
	r, _ := initTests(t)

	type testCase struct {
		ExpectedCode int
		Expected     string
		JSON         string
	}

	testCases := map[string]*testCase{
		"positive testcase, create a new incident": {
			JSON:         `{"name":"Distributed Cache Service","text":"Incident","impact": 2,"attributes": [{"name":"region","value":"EU-NL"}]}`,
			Expected:     `{"id":2,"text":"Incident","impact":2,`,
			ExpectedCode: 201,
		},
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

		if title == "positive testcase, create a new incident" {
			inc := &v1.Incident{}
			err := json.Unmarshal(w.Body.Bytes(), inc)
			require.NoError(t, err)
			assert.Equal(t, 2, inc.ID)
			assert.Equal(t, "Incident", inc.Text)
			assert.Equal(t, 0, len(inc.Updates)) //nolint:testifylint
			assert.Equal(t, c.ExpectedCode, w.Code)
			assert.True(t, strings.HasPrefix(w.Body.String(), c.Expected))
			continue
		}
		assert.Equal(t, c.ExpectedCode, w.Code)
		assert.Equal(t, c.Expected, w.Body.String())
	}
}

func TestPostComponentsStatusHandlerBL(t *testing.T) {
	t.Log("start to test incident creation, modification by /v1/component_status")
	r, _ := initTests(t)

	t.Log("create an incident")

	compName := "Distributed Cache Service"
	attr := []*v1.ComponentAttribute{{Name: "region", Value: "EU-NL"}}
	impact := 1
	text := "Test incident for dcs"

	componentCreateData := &v1.ComponentStatusPost{
		Name:       compName,
		Impact:     impact,
		Text:       text,
		Attributes: attr,
	}

	incID, _ := createIncidentByComponent(t, r, componentCreateData)

	t.Log("create a new incident with the same component and the same impact, should get an error")
	_, body := createIncidentByComponent(t, r, componentCreateData)
	confStruct := &v1.ConflictResponse{}
	err := json.Unmarshal(body, confStruct)
	require.NoError(t, err)

	checkConflictMsg(t, confStruct, incID, text)

	t.Log("create a new incident with the same component and higher impact, should update the impact")
	componentCreateData.Impact = 2
	newIncID, body := createIncidentByComponent(t, r, componentCreateData)
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
		Attributes: attr,
	}
	newIncID, body = createIncidentByComponent(t, r, componentCreateData)
	assert.Equal(t, incID, newIncID)
	newInc = &v1.Incident{}
	err = json.Unmarshal(body, newInc)
	require.NoError(t, err)
	//TODO: it's strange, that we can't check the count of components, fix it
	assert.Equal(t, componentCreateData.Impact, *newInc.Impact)
	assert.Len(t, newInc.Updates, 2)
	for _, u := range newInc.Updates {
		if strings.HasPrefix(u.Text, "Cloud Container Engine") {
			assert.Equal(t, "Cloud Container Engine (Container, EU-NL, cce) added", u.Text)
			assert.Equal(t, "SYSTEM", u.Status)
		}
	}
}

func createIncidentByComponent(t *testing.T, r *gin.Engine, inc *v1.ComponentStatusPost) (int, []byte) {
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

func checkConflictMsg(t *testing.T, confStruct *v1.ConflictResponse, incID int, text string) {
	assert.Equal(t, "Incident with this the component already exists", confStruct.Msg)
	assert.Equal(t, "Check your request parameters", confStruct.Details)
	assert.Equal(t, incID, confStruct.ExistingIncidentID)
	assert.Equal(t, text, confStruct.ExistingIncidentTitle)
}
