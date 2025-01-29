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
	v2Incidents    = "/v2/incidents"
	v2Availability = "/v2/availability"
)

func TestV2GetIncidentsHandler(t *testing.T) {
	t.Log("start to test GET /v2/incidents")
	r, _, _ := initTests(t)

	incidentStr := `{"id":1,"title":"Closed incident without any update","impact":1,"components":[1],"start_date":"2024-10-24T10:12:42Z","end_date":"2024-10-24T11:12:42Z","system":false,"updates":[{"status":"resolved","text":"close incident","timestamp":"2024-10-24T11:12:42.559346Z"}]}`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, v2Incidents, nil)

	r.ServeHTTP(w, req)

	incidents := map[string][]*v2.Incident{}

	t.Logf("Response status: %d", w.Code)
	t.Logf("Response headers: %v", w.Header())
	t.Logf("Response body: %s", w.Body.String())

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
	r, _, _ := initTests(t)

	var response = `[{"id":1,"name":"Cloud Container Engine","attributes":[{"name":"region","value":"EU-DE"},{"name":"category","value":"Container"},{"name":"type","value":"cce"}]},{"id":2,"name":"Cloud Container Engine","attributes":[{"name":"region","value":"EU-NL"},{"name":"category","value":"Container"},{"name":"type","value":"cce"}]},{"id":3,"name":"Elastic Cloud Server","attributes":[{"name":"region","value":"EU-DE"},{"name":"category","value":"Compute"},{"name":"type","value":"ecs"}]},{"id":4,"name":"Elastic Cloud Server","attributes":[{"name":"region","value":"EU-NL"},{"name":"category","value":"Compute"},{"name":"type","value":"ecs"}]},{"id":5,"name":"Distributed Cache Service","attributes":[{"name":"region","value":"EU-DE"},{"name":"category","value":"Database"},{"name":"type","value":"dcs"}]},{"id":6,"name":"Distributed Cache Service","attributes":[{"name":"region","value":"EU-NL"},{"name":"category","value":"Database"},{"name":"type","value":"dcs"}]}]`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v2/components", nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, response, w.Body.String())
}

func TestV2PostIncidentsHandlerNegative(t *testing.T) {
	t.Log("start to test incident creation and check json data for /v2/incidents")
	r, _, _ := initTests(t)

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
	r, _, _ := initTests(t)

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

	incidents := v2GetIncidents(t, r)
	for _, inc := range incidents {
		if inc.EndDate == nil {
			endDate := inc.StartDate.Add(time.Hour * 1)
			inc.EndDate = &endDate
			v2PatchIncident(t, r, inc)
		}
	}

	result := v2CreateIncident(t, r, &incidentCreateData)

	assert.Len(t, result.Result, len(incidentCreateData.Components))
	assert.Equal(t, "", result.Result[0].Error)
	assert.Equal(t, "", result.Result[1].Error)
	assert.Equal(t, len(incidents)+1, result.Result[0].IncidentID)
	assert.Equal(t, len(incidents)+1, result.Result[1].IncidentID)

	incident := v2GetIncident(t, r, result.Result[0].IncidentID)
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

	oldIncident := v2GetIncident(t, r, result.Result[0].IncidentID-1)
	assert.NotNil(t, oldIncident.EndDate)
	assert.Len(t, oldIncident.Components, 1)
	assert.NotNil(t, oldIncident.Updates)
	assert.Len(t, oldIncident.Updates, 2)
	assert.Equal(t, "SYSTEM", oldIncident.Updates[0].Status)
	assert.Equal(t, "SYSTEM", oldIncident.Updates[1].Status)
	assert.Equal(t, fmt.Sprintf("Cloud Container Engine (Container, EU-DE, cce) moved to <a href='/incidents/%d'>Test incident for dcs</a>", result.Result[0].IncidentID), oldIncident.Updates[0].Text)
	assert.Equal(t, fmt.Sprintf("Cloud Container Engine (Container, EU-NL, cce) moved to <a href='/incidents/%d'>Test incident for dcs</a>, Incident closed by system", result.Result[0].IncidentID), oldIncident.Updates[1].Text)

	incidentN3 := v2GetIncident(t, r, result.Result[0].IncidentID)
	assert.Nil(t, incidentN3.EndDate)
	assert.Len(t, incidentN3.Components, 2)
	assert.NotNil(t, incidentN3.Updates)
	assert.Len(t, incidentN3.Updates, 2)
	assert.Equal(t, "SYSTEM", incidentN3.Updates[0].Status)
	assert.Equal(t, "SYSTEM", incidentN3.Updates[1].Status)
	assert.Equal(t, fmt.Sprintf("Cloud Container Engine (Container, EU-DE, cce) moved from <a href='/incidents/%d'>Test incident for dcs</a>", result.Result[0].IncidentID-1), incidentN3.Updates[0].Text)
	assert.Equal(t, fmt.Sprintf("Cloud Container Engine (Container, EU-NL, cce) moved from <a href='/incidents/%d'>Test incident for dcs</a>", result.Result[0].IncidentID-1), incidentN3.Updates[1].Text)

	t.Log("create a new maintenance with the same components and higher impact, should create a new without components")

	impact = 0
	title = "Test maintenance for dcs"
	incidentCreateData.Title = title
	incidentCreateData.Description = "any description for maintenance incident"
	endDate := time.Now().AddDate(0, 0, 1).UTC()
	incidentCreateData.EndDate = &endDate

	result = v2CreateIncident(t, r, &incidentCreateData)
	assert.Equal(t, len(incidents)+3, result.Result[0].IncidentID)
	assert.Equal(t, len(incidents)+3, result.Result[1].IncidentID)

	maintenanceIncident := v2GetIncident(t, r, result.Result[0].IncidentID)
	assert.Equal(t, incidentCreateData.StartDate, maintenanceIncident.StartDate)
	assert.Equal(t, incidentCreateData.EndDate, maintenanceIncident.EndDate)
	assert.Equal(t, title, maintenanceIncident.Title)
	assert.Equal(t, impact, *maintenanceIncident.Impact)
	assert.Equal(t, system, *maintenanceIncident.System)
	assert.Equal(t, incidentCreateData.Description, maintenanceIncident.Updates[0].Text)
	assert.Equal(t, "description", maintenanceIncident.Updates[0].Status)

	incidentN3 = v2GetIncident(t, r, result.Result[0].IncidentID-1)
	assert.Nil(t, incidentN3.EndDate)
	assert.Len(t, incidentN3.Components, 2)
	assert.NotNil(t, incidentN3.Updates)
	assert.Len(t, incidentN3.Updates, 2)
	assert.Equal(t, "SYSTEM", incidentN3.Updates[0].Status)
	assert.Equal(t, "SYSTEM", incidentN3.Updates[1].Status)
	assert.Equal(t, fmt.Sprintf("Cloud Container Engine (Container, EU-DE, cce) moved from <a href='/incidents/%d'>Test incident for dcs</a>", incidentN3.ID-1), incidentN3.Updates[0].Text)
	assert.Equal(t, fmt.Sprintf("Cloud Container Engine (Container, EU-NL, cce) moved from <a href='/incidents/%d'>Test incident for dcs</a>", incidentN3.ID-1), incidentN3.Updates[1].Text)

	t.Log("check response, if incident component is not present in the opened incidents, should create a new incident")
	components = []int{3}
	impact = 1
	incidentCreateData = v2.IncidentData{
		Title:      "Test for another different component",
		Impact:     &impact,
		Components: components,
		StartDate:  startDate,
		System:     &system,
	}
	result = v2CreateIncident(t, r, &incidentCreateData)
	assert.Equal(t, 9, result.Result[0].IncidentID)
	assert.Equal(t, 3, result.Result[0].ComponentID)
}

func TestV2PatchIncidentHandlerNegative(t *testing.T) {
	t.Log("start to test negative cases for incident patching and check json data for /v2/incidents/42")
	r, _, _ := initTests(t)

	components := []int{1}
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

	resp := v2CreateIncident(t, r, &incidentCreateData)
	incID := resp.Result[0].IncidentID

	type testCase struct {
		ExpectedCode int
		Expected     string
		JSON         string
	}

	jsWrongOpenedStatus := `{
		"title": "OpenStack Upgrade in regions EU-DE/EU-NL",
	 	"impact": 1,
	 	"message": "Any message why the incident was updated.",
	 	"status": "in progress",
	 	"update_date": "2024-12-11T14:46:03.877Z",
	 	"start_date": "2024-12-11T14:46:03.877Z",
	 	"end_date": "2024-12-11T14:46:03.877Z"
	}`

	jsWrongOpenedStartDate := `{
	 "impact": 1,
	 "message": "Any message why the incident was updated.",
	 "status": "analysing",
	 "update_date": "2024-12-11T14:46:03.877Z",
	 "start_date": "2024-12-11T14:46:03.877Z"
	}`
	jsWrongOpenedStatusForChangingImpact := `{
	"impact": 0,
	"message": "Any message why the incident was updated.",
	"status": "analysing",
	"update_date": "2024-12-11T14:46:03.877Z"
}`
	jsWrongOpenedMaintenanceImpact := `{
	 "impact": 0,
	 "message": "Any message why the incident was updated.",
	 "status": "impact changed",
	 "update_date": "2024-12-11T14:46:03.877Z"
	}`
	testCases := map[string]*testCase{
		"negative testcase, wrong status for opened incident": {
			JSON:         jsWrongOpenedStatus,
			Expected:     `{"errMsg":"wrong status for incident"}`,
			ExpectedCode: 400,
		},
		"negative testcase, wrong start date for opened incident": {
			JSON:         jsWrongOpenedStartDate,
			Expected:     `{"errMsg":"can not change start date for open incident"}`,
			ExpectedCode: 400,
		},
		"negative testcase, wrong status for changing impact": {
			JSON:         jsWrongOpenedStatusForChangingImpact,
			Expected:     `{"errMsg":"wrong status for changing impact"}`,
			ExpectedCode: 400,
		},
		"negative testcase, can't change impact from incident to maintenance": {
			JSON:         jsWrongOpenedMaintenanceImpact,
			Expected:     `{"errMsg":"can not change impact to maintenance"}`,
			ExpectedCode: 400,
		},
	}

	for testName, c := range testCases {
		t.Logf("start test case: %s\n", testName)

		url := fmt.Sprintf("/v2/incidents/%d", incID)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPatch, url, strings.NewReader(c.JSON))
		r.ServeHTTP(w, req)

		assert.Equal(t, c.ExpectedCode, w.Code)
		assert.Equal(t, c.Expected, w.Body.String())
	}
}

func TestV2PatchIncidentHandler(t *testing.T) {
	t.Log("start to test incident patching")
	r, _, _ := initTests(t)

	components := []int{1}
	impact := 1
	title := "Test incident for patching test"
	startDate := time.Now().AddDate(0, 0, -2).UTC()
	system := false

	internalPatch := func(id int, p *v2.PatchIncidentData) *v2.Incident {
		d, err := json.Marshal(p)
		require.NoError(t, err)

		url := fmt.Sprintf("/v2/incidents/%d", id)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPatch, url, bytes.NewReader(d))

		r.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)

		inc := &v2.Incident{}
		err = json.Unmarshal(w.Body.Bytes(), inc)
		require.NoError(t, err)
		return inc
	}

	incidentCreateData := v2.IncidentData{
		Title:      title,
		Impact:     &impact,
		Components: components,
		StartDate:  startDate,
		System:     &system,
	}

	resp := v2CreateIncident(t, r, &incidentCreateData)
	incID := resp.Result[0].IncidentID

	newTitle := "patched incident title"
	t.Logf("patching incident title, from %s to %s", title, newTitle)

	pData := v2.PatchIncidentData{
		Title:      &newTitle,
		Message:    "update title",
		Status:     "analysing",
		UpdateDate: time.Now().UTC(),
	}

	inc := internalPatch(incID, &pData)
	assert.Equal(t, newTitle, inc.Title)

	newImpact := 2
	t.Logf("patching incident impact, from %d to %d", impact, newImpact)

	pData.Impact = &newImpact
	pData.Status = v2.IncidentImpactChanged

	inc = internalPatch(incID, &pData)
	assert.Equal(t, newImpact, *inc.Impact)

	t.Logf("close incident")
	pData.Status = v2.IncidentResolved
	updateDate := time.Now().UTC()
	pData.UpdateDate = updateDate

	inc = internalPatch(incID, &pData)
	assert.Equal(t, updateDate, *inc.EndDate)

	t.Logf("patching closed incident, change start date and end date")
	startDate = time.Now().AddDate(0, 0, -1).UTC()
	endDate := time.Now().UTC()

	pData.Status = v2.IncidentChanged
	pData.StartDate = &startDate
	pData.EndDate = &endDate

	inc = internalPatch(incID, &pData)
	assert.Equal(t, startDate, inc.StartDate)
	assert.Equal(t, endDate, *inc.EndDate)

	t.Logf("reopen closed incident")

	pData.Status = v2.IncidentReopened
	pData.StartDate = nil
	pData.EndDate = nil
	inc = internalPatch(incID, &pData)
	assert.Nil(t, inc.EndDate)

	t.Logf("final close the test incident")

	pData.Status = v2.IncidentResolved
	inc = internalPatch(incID, &pData)
	assert.NotNil(t, inc.EndDate)
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

func v2GetIncident(t *testing.T, r *gin.Engine, id int) *v2.Incident {
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

func v2GetIncidents(t *testing.T, r *gin.Engine) []*v2.Incident {
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

func v2PatchIncident(t *testing.T, r *gin.Engine, inc *v2.Incident) {
	t.Helper()

	patch := v2.PatchIncidentData{
		Message:    "closed",
		Status:     "resolved",
		UpdateDate: *inc.EndDate,
	}

	d, err := json.Marshal(patch)
	require.NoError(t, err)

	url := fmt.Sprintf("/v2/incidents/%d", inc.IncidentID.ID)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPatch, url, bytes.NewReader(d))

	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
}

func v2GetComponents(t *testing.T, r *gin.Engine) []v2.Component {
	t.Helper()

	url := "/v2/components"
	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err, "failed to create HTTP request")

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "unexpected HTTP status code")

	var components []v2.Component
	err = json.Unmarshal(w.Body.Bytes(), &components)
	require.NoError(t, err, "failed to unmarshal response body")

	return components
}

func TestV2CreateComponentAndList(t *testing.T) {
	t.Log("start to test component creation and listing")
	r, _, _ := initTests(t)

	// Test case 1: Successful component creation
	t.Log("Test case 1: Create new component successfully")
	newComponent := v2.PostComponentData{
		Name: "Domain Name System",
		Attributes: []v2.ComponentAttribute{
			{Name: "type", Value: "dns"},
			{Name: "region", Value: "EU-DE"},
			{Name: "category", Value: "Network"},
		},
	}

	w := httptest.NewRecorder()
	data, _ := json.Marshal(newComponent)
	req, _ := http.NewRequest(http.MethodPost, "/v2/components", bytes.NewReader(data))
	r.ServeHTTP(w, req)

	t.Logf("Create component response: status=%d, body=%s", w.Code, w.Body.String())
	assert.Equal(t, http.StatusCreated, w.Code)

	var createdComponent v2.Component
	err := json.Unmarshal(w.Body.Bytes(), &createdComponent)
	require.NoError(t, err)
	assert.Equal(t, newComponent.Name, createdComponent.Name)
	assert.Equal(t, len(newComponent.Attributes), len(createdComponent.Attributes))

	// Test case 2: Try to create the same component again
	t.Log("Test case 2: Try to create duplicate component")
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/v2/components", bytes.NewReader(data))
	r.ServeHTTP(w, req)

	t.Logf("Duplicate component response: status=%d, body=%s", w.Code, w.Body.String())
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "component exists")

	// Test case 3: Try to create component with invalid attributes (duplicate region)
	t.Log("Test case 3: Create component with invalid attributes")
	invalidComponent := v2.PostComponentData{
		Name: "Invalid Component",
		Attributes: []v2.ComponentAttribute{
			{Name: "region", Value: "EU-DE"},
			{Name: "region", Value: "EU-NL"}, // Duplicate attribute name
			{Name: "type", Value: "test"},
		},
	}

	data, _ = json.Marshal(invalidComponent)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/v2/components", bytes.NewReader(data))
	r.ServeHTTP(w, req)

	t.Logf("Invalid component response: status=%d, body=%s", w.Code, w.Body.String())
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "component attribute has invalid format")

	// List all components to verify
	components := v2GetComponents(t, r)
	t.Log("Final components list:")
	for _, comp := range components {
		t.Logf("Component ID=%d, Name=%s, Attributes=%+v",
			comp.ID, comp.Name, comp.Attributes)
	}
}

func TestV2GetComponentsAvailability(t *testing.T) {
	t.Log("start to test GET /v2/availability")
	r, _, _ := initTests(t)

	// Incident preparation
	t.Log("create an incident")

	components := []int{7}
	impact := 3
	title := "Test incident for dns N1"
	startDate := time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC)
	system := false

	// Incident N1
	incidentCreateDataN1 := v2.IncidentData{
		Title:      title,
		Impact:     &impact,
		Components: components,
		StartDate:  startDate,
		EndDate:    nil,
		System:     &system,
	}

	resultN1 := v2CreateIncident(t, r, &incidentCreateDataN1)

	assert.Len(t, resultN1.Result, len(incidentCreateDataN1.Components))

	// Incident closing
	incidentN1 := v2GetIncident(t, r, resultN1.Result[0].IncidentID)
	endDate := time.Date(2024, 12, 16, 12, 0, 0, 0, time.UTC)
	incidentN1.EndDate = &endDate
	v2PatchIncident(t, r, incidentN1)

	t.Logf("Incident patched: %+v", incidentN1)

	// Incident N2

	title = "Test incident for dns N2"
	startDate = time.Date(2024, 10, 16, 12, 0, 0, 0, time.UTC)
	endDate = time.Date(2024, 11, 16, 00, 00, 00, 0, time.UTC)

	incidentCreateDataN2 := v2.IncidentData{
		Title:      title,
		Impact:     &impact,
		Components: components,
		StartDate:  startDate,
		EndDate:    nil,
		System:     &system,
	}

	resultN2 := v2CreateIncident(t, r, &incidentCreateDataN2)

	assert.Len(t, resultN2.Result, len(incidentCreateDataN2.Components))

	// Incident closing
	incidentN2 := v2GetIncident(t, r, resultN2.Result[0].IncidentID)

	incidentN2.EndDate = &endDate
	v2PatchIncident(t, r, incidentN2)

	t.Logf("Incident patched: %+v", incidentN2)

	// Test case 1: Successful availability listing
	t.Log("Test case 1: List availability successfully")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v2/availability", nil)
	r.ServeHTTP(w, req)

	t.Logf("Availability response: status=%d, body=%s", w.Code, w.Body.String())
	assert.Equal(t, http.StatusOK, w.Code)

	var availability struct {
		Data []v2.ComponentAvailability `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &availability)
	require.NoError(t, err)
	assert.NotEmpty(t, availability)

	// Test case 2: Check if the availability data is correct
	targetMonths := map[int]bool{10: true, 11: true, 12: true}

	for _, compAvail := range availability.Data {
		if compAvail.ComponentID.ID == 7 {
			checkComponentAvailability(t, compAvail, targetMonths)
		}
	}
}

func checkComponentAvailability(t *testing.T, compAvail v2.ComponentAvailability, targetMonths map[int]bool) {
	for _, avail := range compAvail.Availability {
		if _, ok := targetMonths[avail.Month]; ok {
			assert.InEpsilon(t, 50.00000, avail.Percentage, 0.00001,
				"Availability percentage should be 50% for the target months")
			t.Logf("Availability for %v: %d-%d: %.2f%%", compAvail.Name, avail.Year, avail.Month, avail.Percentage)
		} else {
			assert.InEpsilon(t, 100.00000, avail.Percentage, 0.00001,
				"Availability percentage should be 100% for all months except the target months")
		}
	}
}
