package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v2 "github.com/stackmon/otc-status-dashboard/internal/api/v2"
)

const (
	v2Incidents = "/v2/incidents"
)

func TestV2GetIncidentsHandler(t *testing.T) {
	t.Log("start to test GET /v2/incidents")
	r, _ := initTests(t)

	incidentStr := `{"id":1,"title":"Closed incident without any update","impact":1,"components":[1],"start_date":"2024-10-24T10:12:42Z","end_date":"2024-10-24T11:12:42Z","system":false,"updates":[{"status":"resolved","text":"close incident","timestamp":"2024-10-24T11:12:42.559346Z"}]}`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, v2Incidents, nil)

	r.ServeHTTP(w, req)

	incidents := map[string][]*v2.Incident{}

	assert.Equal(t, 200, w.Code)

	err := json.Unmarshal(w.Body.Bytes(), &incidents)
	require.NoError(t, err)
	for _, inc := range incidents["data"] {
		if inc.ID == 1 {
			b, errM := json.Marshal(inc)
			require.NoError(t, errM)
			assert.Equal(t, incidentStr, string(b))
			return
		}
	}
	require.NoError(t, fmt.Errorf("incident 1 is not found"))
}

func TestV2GetComponentsHandler(t *testing.T) {
	t.Log("start to test GET /v2/components")
	r, _ := initTests(t)

	var response = `[{"id":1,"name":"Cloud Container Engine","attributes":[{"name":"region","value":"EU-DE"},{"name":"category","value":"Container"},{"name":"type","value":"cce"}]},{"id":2,"name":"Cloud Container Engine","attributes":[{"name":"region","value":"EU-NL"},{"name":"category","value":"Container"},{"name":"type","value":"cce"}]},{"id":3,"name":"Elastic Cloud Server","attributes":[{"name":"region","value":"EU-DE"},{"name":"category","value":"Compute"},{"name":"type","value":"ecs"}]},{"id":4,"name":"Elastic Cloud Server","attributes":[{"name":"region","value":"EU-NL"},{"name":"category","value":"Compute"},{"name":"type","value":"ecs"}]},{"id":5,"name":"Distributed Cache Service","attributes":[{"name":"region","value":"EU-DE"},{"name":"category","value":"Database"},{"name":"type","value":"dcs"}]},{"id":6,"name":"Distributed Cache Service","attributes":[{"name":"region","value":"EU-NL"},{"name":"category","value":"Database"},{"name":"type","value":"dcs"}]}]`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v2/components", nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, response, w.Body.String())
}

func TestV2PostIncidentsHandlerNegative(t *testing.T) {
	t.Log("start to test incident creation and check json data for /v2/incidents")
	r, _ := initTests(t)

	type testCase struct {
		ExpectedCode int
		Expected     string
		JSON         string
	}

	jsEndPresent := `{
  "title":"OpenStack Upgrade in regions EU-DE/EU-NL",
  "impact":1,
  "components":[
    1
  ],
  "start_date":"2024-11-25T09:32:14.075Z",
  "end_date":"2024-11-25T09:32:14.075Z",
  "system":false,
  "updates":[
    {
      "id":163,
      "status":"resolved",
      "text":"issue resolved",
      "timestamp":"2024-11-25T09:32:14.075Z"
    }
  ]
}`
	jsUpdatesPresent := `{
  "title":"OpenStack Upgrade in regions EU-DE/EU-NL",
  "impact":1,
  "components":[
    1
  ],
  "start_date":"2024-11-25T09:32:14.075Z",
  "system":false,
  "updates":[
    {
      "id":163,
      "status":"resolved",
      "text":"issue resolved",
      "timestamp":"2024-11-25T09:32:14.075Z"
    }
  ]
}`
	jsWrongComponents := `{
  "title":"OpenStack Upgrade in regions EU-DE/EU-NL",
  "impact":1,
  "components":[
    218,
    254
  ],
  "start_date":"2024-11-25T09:32:14.075Z",
  "system":false
}`

	testCases := map[string]*testCase{
		"negative testcase, incident is not a maintenance and end_date is present": {
			JSON:         jsEndPresent,
			Expected:     `{"errMsg":"incident end_date should be empty"}`,
			ExpectedCode: 400,
		},
		"negative testcase, updates are present": {
			JSON:         jsUpdatesPresent,
			Expected:     `{"errMsg":"incident updates should be empty"}`,
			ExpectedCode: 400,
		},
		"negative testcase, wrong components ids": {
			JSON:         jsWrongComponents,
			Expected:     `{"errMsg":"component does not exist, component_id: 218"}`,
			ExpectedCode: 400,
		},
	}

	for title, c := range testCases {
		t.Logf("start test case: %s\n", title)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, v2Incidents, strings.NewReader(c.JSON))
		r.ServeHTTP(w, req)

		assert.Equal(t, c.ExpectedCode, w.Code)
		assert.Equal(t, c.Expected, w.Body.String())
	}
}

func TestV2PostIncidentsHandler(t *testing.T) {
	t.Log("start to test incident creation for /v2/incidents")
	r, _ := initTests(t)

	t.Log("create an incident")

	components := []int{1, 2}
	impact := 1
	title := "Test incident for dcs"
	startDate := time.Now().AddDate(0, 0, -1).UTC()
	system := false

	incidentCreateData := v2.IncidentData{
		Title:      title,
		Impact:     &impact,
		Components: components,
		StartDate:  startDate,
		System:     &system,
	}

	incidents := V2GetIncidents(t, r)
	for _, inc := range incidents {
		if inc.EndDate == nil {
			endDate := inc.StartDate.Add(time.Hour * 1)
			inc.EndDate = &endDate
			V2PatchIncident(t, r, inc)
		}
	}

	result := v2CreateIncident(t, r, &incidentCreateData)

	assert.Len(t, result.Result, len(incidentCreateData.Components))
	assert.Equal(t, "", result.Result[0].Error)
	assert.Equal(t, "", result.Result[1].Error)
	assert.Equal(t, len(incidents)+1, result.Result[0].IncidentID)
	assert.Equal(t, len(incidents)+1, result.Result[1].IncidentID)

	incident := V2GetIncident(t, r, result.Result[0].IncidentID)
	assert.Equal(t, incidentCreateData.StartDate, incident.StartDate)
	assert.Equal(t, title, incident.Title)
	assert.Equal(t, impact, *incident.Impact)
	assert.Equal(t, system, *incident.System)
	assert.Nil(t, incident.EndDate)
	assert.Nil(t, incident.Updates)

	t.Log("create a new incident with the same components and the same impact, should close previous and move components to the new")
	result = v2CreateIncident(t, r, &incidentCreateData)
	assert.Equal(t, len(incidents)+2, result.Result[0].IncidentID)
	assert.Equal(t, len(incidents)+2, result.Result[1].IncidentID)

	oldIncident := V2GetIncident(t, r, result.Result[0].IncidentID-1)
	assert.NotNil(t, oldIncident.EndDate)
	assert.Len(t, oldIncident.Components, 1)
	assert.NotNil(t, oldIncident.Updates)
	assert.Len(t, oldIncident.Updates, 2)
	assert.Equal(t, "SYSTEM", oldIncident.Updates[0].Status)
	assert.Equal(t, "SYSTEM", oldIncident.Updates[1].Status)
	assert.Equal(t, fmt.Sprintf("Cloud Container Engine (Container, EU-DE, cce) moved to <a href='/incidents/%d'>Test incident for dcs</a>", result.Result[0].IncidentID), oldIncident.Updates[0].Text)
	assert.Equal(t, fmt.Sprintf("Cloud Container Engine (Container, EU-NL, cce) moved to <a href='/incidents/%d'>Test incident for dcs</a>, Incident closed by system", result.Result[0].IncidentID), oldIncident.Updates[1].Text)

	incidentN3 := V2GetIncident(t, r, result.Result[0].IncidentID)
	assert.Nil(t, incidentN3.EndDate)
	assert.Len(t, incidentN3.Components, 2)
	assert.NotNil(t, incidentN3.Updates)
	assert.Len(t, incidentN3.Updates, 2)
	assert.Equal(t, "SYSTEM", incidentN3.Updates[0].Status)
	assert.Equal(t, "SYSTEM", incidentN3.Updates[1].Status)
	assert.Equal(t, fmt.Sprintf("Cloud Container Engine (Container, EU-DE, cce) moved from <a href='/incidents/%d'>Test incident for dcs</a>", result.Result[0].IncidentID-1), incidentN3.Updates[0].Text)
	assert.Equal(t, fmt.Sprintf("Cloud Container Engine (Container, EU-NL, cce) moved from <a href='/incidents/%d'>Test incident for dcs</a>", result.Result[0].IncidentID-1), incidentN3.Updates[1].Text)

	t.Log("create a new maintenance with the same components and higher impact, should create a new without components ")

	impact = 0
	title = "Test maintenance for dcs"
	incidentCreateData.Title = title
	incidentCreateData.Description = "any description for maintenance incident"
	endDate := time.Now().AddDate(0, 0, 1).UTC()
	incidentCreateData.EndDate = &endDate

	result = v2CreateIncident(t, r, &incidentCreateData)
	assert.Equal(t, len(incidents)+3, result.Result[0].IncidentID)
	assert.Equal(t, len(incidents)+3, result.Result[1].IncidentID)

	maintenanceIncident := V2GetIncident(t, r, result.Result[0].IncidentID)
	assert.Equal(t, incidentCreateData.StartDate, maintenanceIncident.StartDate)
	assert.Equal(t, incidentCreateData.EndDate, maintenanceIncident.EndDate)
	assert.Equal(t, title, maintenanceIncident.Title)
	assert.Equal(t, impact, *maintenanceIncident.Impact)
	assert.Equal(t, system, *maintenanceIncident.System)
	assert.Equal(t, incidentCreateData.Description, maintenanceIncident.Updates[0].Text)
	assert.Equal(t, "description", maintenanceIncident.Updates[0].Status)

	incidentN3 = V2GetIncident(t, r, result.Result[0].IncidentID-1)
	assert.Nil(t, incidentN3.EndDate)
	assert.Len(t, incidentN3.Components, 2)
	assert.NotNil(t, incidentN3.Updates)
	assert.Len(t, incidentN3.Updates, 2)
	assert.Equal(t, "SYSTEM", incidentN3.Updates[0].Status)
	assert.Equal(t, "SYSTEM", incidentN3.Updates[1].Status)
	assert.Equal(t, fmt.Sprintf("Cloud Container Engine (Container, EU-DE, cce) moved from <a href='/incidents/%d'>Test incident for dcs</a>", incidentN3.ID-1), incidentN3.Updates[0].Text)
	assert.Equal(t, fmt.Sprintf("Cloud Container Engine (Container, EU-NL, cce) moved from <a href='/incidents/%d'>Test incident for dcs</a>", incidentN3.ID-1), incidentN3.Updates[1].Text)
}

func v2CreateIncident(t *testing.T, r *gin.Engine, inc *v2.IncidentData) *v2.PostIncidentResp {
	t.Helper()

	data, err := json.Marshal(inc)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, v2Incidents, bytes.NewReader(data))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		return nil
	}

	assert.Equal(t, http.StatusOK, w.Code)

	respCreated := &v2.PostIncidentResp{}
	err = json.Unmarshal(w.Body.Bytes(), respCreated)
	require.NoError(t, err)

	return respCreated
}

func V2GetIncident(t *testing.T, r *gin.Engine, id int) *v2.Incident {
	t.Helper()
	url := fmt.Sprintf("/v2/incidents/%d", id)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, url, nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var incident v2.Incident
	err := json.Unmarshal(w.Body.Bytes(), &incident)
	require.NoError(t, err)

	return &incident
}

func V2GetIncidents(t *testing.T, r *gin.Engine) []*v2.Incident {
	t.Helper()
	url := "/v2/incidents"
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, url, nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	data := map[string][]*v2.Incident{}
	err := json.Unmarshal(w.Body.Bytes(), &data)
	require.NoError(t, err)

	return data["data"]
}

func V2PatchIncident(t *testing.T, r *gin.Engine, inc *v2.Incident) {
	t.Helper()

	d, err := json.Marshal(inc)
	require.NoError(t, err)

	url := fmt.Sprintf("/v2/incidents/%d", inc.IncidentID.ID)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPatch, url, bytes.NewReader(d))

	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
}
