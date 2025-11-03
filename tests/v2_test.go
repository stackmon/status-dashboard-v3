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

	apiErrors "github.com/stackmon/otc-status-dashboard/internal/api/errors"
	v2 "github.com/stackmon/otc-status-dashboard/internal/api/v2"
	"github.com/stackmon/otc-status-dashboard/internal/event"
)

const (
	v2IncidentsEndpoint    = "/v2/incidents"
	v2AvailabilityEndpoint = "/v2/availability"
	v2EventsEndpoint       = "/v2/events"
)

// V2IncidentsListResponse defines the expected structure for the GET /v2/incidents endpoint.
type V2IncidentsListResponse struct {
	Data    []*v2.Incident `json:"data"`
	Message string         `json:"message,omitempty"`
}

func TestV2GetIncidentsHandler(t *testing.T) {
	t.Logf("start to test GET %s", v2IncidentsEndpoint)
	r, _, _ := initTests(t)

	incidentStr := `{"id":1,"title":"Closed incident without any update","impact":1,"components":[1],"start_date":"2025-05-22T10:12:42Z","end_date":"2025-05-22T11:12:42Z","system":true,"type":"incident","updates":[{"id":0,"status":"resolved","text":"close incident","timestamp":"2025-05-22T11:12:42.559346Z"}],"status":"resolved"}`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, v2IncidentsEndpoint, nil)

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
  "type":"incident",
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
  "type":"incident",
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
  "system":false,
  "type":"incident"
}`
	jsWrongMaintenanceImpact := `{
  "title":"Maintenance with wrong impact",
  "impact":1,
  "components":[1],
  "start_date":"2024-11-25T09:32:14.075Z",
  "system":false,
  "type":"maintenance"
}`

	jsWrongIncidentImpact := `{
  "title":"Incident with maintenance impact",
  "impact":0,
  "components":[1],
  "start_date":"2024-11-25T09:32:14.075Z",
  "system":false,
  "type":"incident"
}`

	testCases := map[string]*testCase{
		"negative testcase, incident is not a maintenance and end_date is present": {
			JSON:         jsEndPresent,
			Expected:     `{"errMsg":"event end_date should be empty"}`,
			ExpectedCode: 400,
		},
		"negative testcase, updates are present": {
			JSON:         jsUpdatesPresent,
			Expected:     `{"errMsg":"event updates should be empty"}`,
			ExpectedCode: 400,
		},
		"negative testcase, wrong components ids": {
			JSON:         jsWrongComponents,
			Expected:     `{"errMsg":"component does not exist, component_id: 218"}`,
			ExpectedCode: 400,
		},
		"negative testcase, maintenance with non-zero impact": {
			JSON:         jsWrongMaintenanceImpact,
			Expected:     `{"errMsg":"impact must be 0 for type 'maintenance' or 'info' and gt 0 for 'incident'"}`,
			ExpectedCode: 400,
		},
		"negative testcase, incident with zero impact": {
			JSON:         jsWrongIncidentImpact,
			Expected:     `{"errMsg":"impact must be 0 for type 'maintenance' or 'info' and gt 0 for 'incident'"}`,
			ExpectedCode: 400,
		},
	}

	for title, c := range testCases {
		t.Logf("start test case: %s\n", title)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, v2IncidentsEndpoint, strings.NewReader(c.JSON))
		r.ServeHTTP(w, req)

		assert.Equal(t, c.ExpectedCode, w.Code)
		assert.Equal(t, c.Expected, w.Body.String())
	}
}

func TestV2PostIncidentsHandler(t *testing.T) {
	t.Log("start to test incident creation for /v2/incidents")
	r, _, _ := initTests(t)

	t.Log("check if all incidents have end date, if not, set it to start date + 1ms")
	incidents := v2GetIncidents(t, r)
	for _, inc := range incidents {
		if inc.EndDate == nil {
			endDate := inc.StartDate.Add(time.Millisecond * 1).UTC()
			inc.EndDate = &endDate
			v2PatchIncident(t, r, inc)
		}
		if inc.Type == event.TypeMaintenance {
			t.Log("the component is maintenance, cancel it")
			v2PatchIncident(t, r, inc, event.MaintenanceCancelled)
		}
	}

	components := []int{1, 2}
	impact := 1
	title := "Test incident creation for api V2 for components: 1, 2. Test 1."
	description := "any description for incident"
	startDate := time.Now().AddDate(0, 0, -1).UTC()
	system := false
	incType := event.TypeIncident

	incidentCreateData := v2.IncidentData{
		Title:       title,
		Description: description,
		Impact:      &impact,
		Components:  components,
		StartDate:   startDate,
		System:      &system,
		Type:        incType,
	}

	result := v2CreateIncident(t, r, &incidentCreateData)

	assert.Len(t, result.Result, len(incidentCreateData.Components))
	assert.Empty(t, result.Result[0].Error)
	assert.Empty(t, result.Result[1].Error)
	assert.Equal(t, len(incidents)+1, result.Result[0].IncidentID)
	assert.Equal(t, len(incidents)+1, result.Result[1].IncidentID)

	t.Log("check created incident data, incident id: ", result.Result[0].IncidentID)
	incident := v2GetIncident(t, r, result.Result[0].IncidentID)
	assert.Equal(t, incidentCreateData.StartDate.Truncate(time.Microsecond), incident.StartDate)
	assert.Equal(t, title, incident.Title)
	assert.Equal(t, impact, *incident.Impact)
	assert.Equal(t, system, *incident.System)
	assert.Nil(t, incident.EndDate)
	require.NotNil(t, incident.Type)
	assert.Equal(t, event.TypeIncident, incident.Type)
	require.NotNil(t, incident.Updates)
	assert.Equal(t, "The incident is detected.", incident.Updates[0].Text)

	t.Log("create a new incident with the same components and the same impact, should close previous and move components to the new")
	t.Log("current time:", time.Now().UTC())
	incidentCreateData.Title = "Test incident creation for api V2 for components: 1, 2. Test should close previous and move components to the new."
	result = v2CreateIncident(t, r, &incidentCreateData)
	assert.Equal(t, len(incidents)+2, result.Result[0].IncidentID)
	assert.Equal(t, len(incidents)+2, result.Result[1].IncidentID)

	oldIncident := v2GetIncident(t, r, result.Result[0].IncidentID-1)
	assert.NotNil(t, oldIncident.EndDate)
	assert.Len(t, oldIncident.Components, 1)
	assert.NotNil(t, oldIncident.Updates)
	assert.Len(t, oldIncident.Updates, 3)
	t.Logf("STATUS updates: %v", oldIncident.Updates)
	assert.Equal(t, event.IncidentDetected, oldIncident.Updates[0].Status)
	assert.Equal(t, event.OutDatedSystem, oldIncident.Updates[1].Status)
	assert.Equal(t, event.IncidentResolved, oldIncident.Updates[2].Status)
	assert.Equal(t, "The incident is detected.", oldIncident.Updates[0].Text)
	assert.Equal(t, fmt.Sprintf("Cloud Container Engine (Container, EU-DE, cce) moved to <a href='/incidents/%d'>Test incident creation for api V2 for components: 1, 2. Test should close previous and move components to the new.</a>", result.Result[0].IncidentID), oldIncident.Updates[1].Text)
	assert.Equal(t, fmt.Sprintf("Cloud Container Engine (Container, EU-NL, cce) moved to <a href='/incidents/%d'>Test incident creation for api V2 for components: 1, 2. Test should close previous and move components to the new.</a>, Incident closed by system", result.Result[0].IncidentID), oldIncident.Updates[2].Text)

	incidentN3 := v2GetIncident(t, r, result.Result[0].IncidentID)
	assert.Nil(t, incidentN3.EndDate)
	assert.Len(t, incidentN3.Components, 2)
	assert.NotNil(t, incidentN3.Updates)
	assert.Len(t, incidentN3.Updates, 3)
	assert.Equal(t, event.IncidentDetected, incidentN3.Updates[0].Status)
	assert.Equal(t, event.OutDatedSystem, incidentN3.Updates[1].Status)
	assert.Equal(t, event.OutDatedSystem, incidentN3.Updates[2].Status)
	assert.Equal(t, "The incident is detected.", incidentN3.Updates[0].Text)
	assert.Equal(t, fmt.Sprintf("Cloud Container Engine (Container, EU-DE, cce) moved from <a href='/incidents/%d'>Test incident creation for api V2 for components: 1, 2. Test 1.</a>", result.Result[0].IncidentID-1), incidentN3.Updates[1].Text)
	assert.Equal(t, fmt.Sprintf("Cloud Container Engine (Container, EU-NL, cce) moved from <a href='/incidents/%d'>Test incident creation for api V2 for components: 1, 2. Test 1.</a>", result.Result[0].IncidentID-1), incidentN3.Updates[2].Text)

	t.Log("create a new maintenance with the same components and higher impact, should create a new without components")

	impact = 0
	title = "Test maintenance creation for api V2 for the components: 1-Cloud Container Engine (Container, EU-DE, cce), 2-Cloud Container Engine (Container, EU-NL, cce)"
	incidentCreateData.Title = title
	incidentCreateData.Description = "any description for maintenance incident"
	endDate := time.Now().AddDate(0, 0, 1).UTC()
	incidentCreateData.EndDate = &endDate
	incidentCreateData.Type = event.TypeMaintenance

	result = v2CreateIncident(t, r, &incidentCreateData)
	assert.Equal(t, len(incidents)+3, result.Result[0].IncidentID)
	assert.Equal(t, len(incidents)+3, result.Result[1].IncidentID)

	maintenanceIncident := v2GetIncident(t, r, result.Result[0].IncidentID)
	assert.Equal(t, incidentCreateData.StartDate.Truncate(time.Microsecond), maintenanceIncident.StartDate)
	require.NotNil(t, incidentCreateData.EndDate)
	require.NotNil(t, maintenanceIncident.EndDate)
	assert.Equal(t, incidentCreateData.EndDate.Truncate(time.Microsecond), maintenanceIncident.EndDate.Truncate(time.Microsecond))
	assert.Equal(t, title, maintenanceIncident.Title)
	assert.Equal(t, impact, *maintenanceIncident.Impact)
	assert.Equal(t, system, *maintenanceIncident.System)
	assert.Equal(t, incidentCreateData.Description, maintenanceIncident.Description)
	assert.Equal(t, event.MaintenancePlanned, maintenanceIncident.Updates[0].Status)
	require.NotNil(t, maintenanceIncident.Type)
	assert.Equal(t, event.TypeMaintenance, maintenanceIncident.Type)
	assert.Equal(t, event.MaintenancePlanned, maintenanceIncident.Updates[0].Status)

	incidentN3 = v2GetIncident(t, r, result.Result[0].IncidentID-1)
	assert.Nil(t, incidentN3.EndDate)
	assert.Len(t, incidentN3.Components, 2)
	assert.NotNil(t, incidentN3.Updates)
	assert.Len(t, incidentN3.Updates, 3)
	assert.Equal(t, event.IncidentDetected, incidentN3.Updates[0].Status)
	assert.Equal(t, event.OutDatedSystem, incidentN3.Updates[1].Status)
	assert.Equal(t, event.OutDatedSystem, incidentN3.Updates[1].Status)
	assert.Equal(t, "The incident is detected.", incidentN3.Updates[0].Text)
	assert.Equal(t, fmt.Sprintf("Cloud Container Engine (Container, EU-DE, cce) moved from <a href='/incidents/%d'>Test incident creation for api V2 for components: 1, 2. Test 1.</a>", incidentN3.ID-1), incidentN3.Updates[1].Text)
	assert.Equal(t, fmt.Sprintf("Cloud Container Engine (Container, EU-NL, cce) moved from <a href='/incidents/%d'>Test incident creation for api V2 for components: 1, 2. Test 1.</a>", incidentN3.ID-1), incidentN3.Updates[2].Text)
	require.NotNil(t, incidentN3.Type)
	assert.Equal(t, event.TypeIncident, incidentN3.Type)

	t.Log("check response, if incident component is not present in the opened incidents, should create a new incident")
	components = []int{3}
	impact = 1
	incidentCreateData = v2.IncidentData{
		Title:       "Test for another different component id: 3.",
		Description: "Any description for incident with different component",
		Impact:      &impact,
		Components:  components,
		StartDate:   startDate,
		System:      &system,
		Type:        event.TypeIncident,
	}
	result = v2CreateIncident(t, r, &incidentCreateData)
	assert.Equal(t, 23, result.Result[0].IncidentID)
	assert.Equal(t, 3, result.Result[0].ComponentID)
}

func TestV2PatchIncidentHandlerNegative(t *testing.T) {
	t.Log("start to test negative cases for incident patching and check json data for /v2/incidents/42")
	r, _, _ := initTests(t)

	components := []int{1}
	impact := 1
	title := "Incident for negative tests for incident patching"
	startDate := time.Now().AddDate(0, 0, -1).UTC()
	system := false

	incidentCreateData := v2.IncidentData{
		Title:       title,
		Description: "any description for incident",
		Impact:      &impact,
		Components:  components,
		StartDate:   startDate,
		System:      &system,
		Type:        event.TypeIncident,
	}

	resp := v2CreateIncident(t, r, &incidentCreateData)
	incID10 := resp.Result[0].IncidentID

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
	 	"end_date": "2024-12-11T14:46:03.877Z",
		"type": "incident"
	}`
	jsWrongOpenedStartDate := `{
	 "impact": 1,
	 "message": "Any message why the incident was updated.",
	 "status": "analysing",
	 "update_date": "2024-12-11T14:46:03.877Z",
	 "start_date": "2024-12-11T14:46:03.877Z",
	 "type": "incident"
	}`
	jsWrongOpenedStatusForChangingImpact := `{
	"impact": 0,
	"message": "Any message why the incident was updated.",
	"status": "analysing",
	"update_date": "2024-12-11T14:46:03.877Z",
	"type": "maintenance"
	}`
	jsWrongOpenedMaintenanceImpact := `{
	 "impact": 0,
	 "message": "Any message why the incident was updated.",
	 "status": "impact changed",
	 "update_date": "2024-12-11T14:46:03.877Z",
	 "type": "maintenance"
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
			Expected:     `{"errMsg":"can not change impact to 0"}`,
			ExpectedCode: 400,
		},
	}

	for testName, c := range testCases {
		t.Logf("start test case: %s\n", testName)

		url := fmt.Sprintf("/v2/incidents/%d", incID10)

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
	description := "Test case Patch. Any description for incident"
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
		Title:       title,
		Description: description,
		Impact:      &impact,
		Components:  components,
		StartDate:   startDate,
		System:      &system,
		Type:        event.TypeIncident,
	}

	resp := v2CreateIncident(t, r, &incidentCreateData)
	incID := resp.Result[0].IncidentID

	newTitle := "patched incident title"
	newDescription := "patched incident description"
	t.Logf("patching incident title, from %s to %s", title, newTitle)

	pData := v2.PatchIncidentData{
		Title:       &newTitle,
		Description: &newDescription,
		Message:     "update title",
		Status:      "analysing",
		UpdateDate:  time.Now().UTC(),
	}

	inc := internalPatch(incID, &pData)
	assert.Equal(t, newTitle, inc.Title)
	assert.Equal(t, newDescription, inc.Description)

	newImpact := 2
	t.Logf("patching incident impact, from %d to %d", impact, newImpact)

	pData.Impact = &newImpact
	pData.Status = event.IncidentImpactChanged

	inc = internalPatch(incID, &pData)
	assert.Equal(t, newImpact, *inc.Impact)

	t.Logf("close incident")
	pData.Status = event.IncidentResolved
	updateDate := time.Now().UTC()
	pData.UpdateDate = updateDate

	inc = internalPatch(incID, &pData)
	require.NotNil(t, inc.EndDate)
	assert.Equal(t, updateDate.Truncate(time.Microsecond), inc.EndDate.Truncate(time.Microsecond))

	t.Logf("patching closed incident, change start date and end date")
	startDate = time.Now().AddDate(0, 0, -1).UTC()
	endDate := time.Now().UTC()

	pData.Status = event.IncidentChanged
	pData.StartDate = &startDate
	pData.EndDate = &endDate

	inc = internalPatch(incID, &pData)
	assert.Equal(t, startDate.Truncate(time.Microsecond), inc.StartDate)
	assert.Equal(t, event.IncidentChanged, inc.Status)
	require.NotNil(t, inc.EndDate)
	assert.Equal(t, endDate.Truncate(time.Microsecond), inc.EndDate.Truncate(time.Microsecond))

	t.Logf("reopen closed incident")

	pData.Status = event.IncidentReopened
	pData.StartDate = nil
	pData.EndDate = nil
	inc = internalPatch(incID, &pData)
	assert.Nil(t, inc.EndDate)

	t.Logf("final close the test incident")

	pData.Status = event.IncidentResolved
	inc = internalPatch(incID, &pData)
	assert.Equal(t, event.IncidentResolved, inc.Status)
	assert.NotNil(t, inc.EndDate)
}

func TestV2PostIncidentExtractHandler(t *testing.T) {
	t.Log("start to test component extraction from incident for the endpoint /v2/incidents/42/extract")
	r, _, _ := initTests(t)

	t.Log("check if all incidents have end date, if not, set it to start date + 1ms")
	incidents := v2GetIncidents(t, r)
	for _, inc := range incidents {
		if inc.EndDate == nil {
			endDate := inc.StartDate.Add(time.Millisecond * 1).UTC()
			inc.EndDate = &endDate
			v2PatchIncident(t, r, inc)
		}
		if inc.Type == event.TypeMaintenance {
			t.Log("the component is maintenance, cancel it")
			v2PatchIncident(t, r, inc, event.MaintenanceCancelled)
		}
	}

	components := []int{1, 2}
	impact := 1
	title := "Test component extraction for component dcs"
	description := "Test incident for extraction"
	startDate := time.Now().AddDate(0, 0, -1).UTC()
	system := false

	incidentCreateData := v2.IncidentData{
		Title:       title,
		Description: description,
		Impact:      &impact,
		Components:  components,
		StartDate:   startDate,
		System:      &system,
		Type:        event.TypeIncident,
	}

	t.Log("create a initial incident", incidentCreateData)
	result := v2CreateIncident(t, r, &incidentCreateData)

	t.Logf("prepare to extract components: %d from incident %d", 2, result.Result[0].IncidentID)
	type IncidentData struct {
		Components []int `json:"components"`
	}
	movedComponents := IncidentData{Components: []int{2}}
	data, err := json.Marshal(movedComponents)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, v2IncidentsEndpoint+fmt.Sprintf("/%d/extract", result.Result[0].IncidentID), bytes.NewReader(data))
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	t.Log("check the new incident created by extraction")
	newInc := &v2.Incident{}
	err = json.Unmarshal(w.Body.Bytes(), newInc)
	require.NoError(t, err)
	assert.Len(t, newInc.Components, 1)
	assert.Equal(t, incidentCreateData.Impact, newInc.Impact)
	assert.Equal(t, description, newInc.Description)
	assert.Equal(t, fmt.Sprintf("Cloud Container Engine (Container, EU-NL, cce) moved from <a href='/incidents/%d'>Test component extraction for component dcs</a>", result.Result[0].IncidentID), newInc.Updates[0].Text)

	t.Log("check the old incident with a record about extraction")
	createdInc := v2GetIncident(t, r, result.Result[0].IncidentID)
	assert.Equal(t, "The incident is detected.", createdInc.Updates[0].Text)
	assert.Equal(t, fmt.Sprintf("Cloud Container Engine (Container, EU-NL, cce) moved to <a href='/incidents/%d'>Test component extraction for component dcs</a>", newInc.ID), createdInc.Updates[1].Text)

	t.Log("start negative case, try to extract all components from the incident, should return error")
	// start negative case
	movedComponents = IncidentData{Components: []int{1}}
	data, err = json.Marshal(movedComponents)
	require.NoError(t, err)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, v2IncidentsEndpoint+fmt.Sprintf("/%d/extract", result.Result[0].IncidentID), bytes.NewReader(data))
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"errMsg":"can not move all components to the new incident, keep at least one"}`, w.Body.String())
}

func v2CreateIncident(t *testing.T, r *gin.Engine, inc *v2.IncidentData) *v2.PostIncidentResp {
	t.Helper()

	data, err := json.Marshal(inc)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, v2IncidentsEndpoint, bytes.NewReader(data))
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

	assert.Equal(t, http.StatusOK, w.Code)

	data := map[string][]*v2.Incident{}
	err := json.Unmarshal(w.Body.Bytes(), &data)
	require.NoError(t, err)

	return data["data"]
}

func v2PatchIncident(t *testing.T, r *gin.Engine, inc *v2.Incident, status ...event.Status) {
	t.Helper()

	st := event.IncidentResolved

	if len(status) == 1 {
		st = status[0]
	}

	patch := v2.PatchIncidentData{
		Message:    "closed",
		Status:     st,
		UpdateDate: *inc.EndDate,
	}

	d, err := json.Marshal(patch)
	require.NoError(t, err)

	url := fmt.Sprintf("/v2/incidents/%d", inc.ID)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPatch, url, bytes.NewReader(d))

	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
}

func Testv2GetComponents(t *testing.T, r *gin.Engine) []v2.Component {
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
	assert.Equal(t, http.StatusCreated, w.Code)

	var createdComponent v2.Component
	err := json.Unmarshal(w.Body.Bytes(), &createdComponent)
	require.NoError(t, err)
	assert.Equal(t, newComponent.Name, createdComponent.Name)
	assert.Len(t, newComponent.Attributes, len(createdComponent.Attributes))

	// Test case 2: Try to create the same component again
	t.Log("Test case 2: Try to create duplicate component")
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodPost, "/v2/components", bytes.NewReader(data))
	r.ServeHTTP(w, req)

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

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "component attribute has invalid format")
}

func TestV2GetIncidentsFilteredHandler(t *testing.T) {
	t.Log("start to test GET /v2/incidents with filters")
	r, _, _ := initTests(t)

	type filterTestCase struct {
		name          string
		queryParams   map[string]string
		expectedIDs   []int
		expectedCount int
	}

	testCases := []filterTestCase{
		{
			name:          "No filters",
			queryParams:   nil,
			expectedIDs:   []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27},
			expectedCount: 27,
		},
		{
			name:        "Filter by start_date",
			queryParams: map[string]string{"start_date": time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)},
			// Incidents starting on or after 2025-02-01
			expectedIDs:   []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27},
			expectedCount: 27,
		},
		{
			name:        "Filter by end_date",
			queryParams: map[string]string{"end_date": time.Date(2025, 5, 23, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)},
			// Incidents starting on or before 2025-05-23
			expectedIDs:   []int{1},
			expectedCount: 1,
		},
		{
			name:          "Filter by impact minor (1)",
			queryParams:   map[string]string{"impact": "1"},
			expectedIDs:   []int{1, 13, 20, 21, 23, 24, 26, 27},
			expectedCount: 8,
		},
		{
			name:          "Filter by impact major (2)",
			queryParams:   map[string]string{"impact": "2"},
			expectedIDs:   []int{2, 4, 7, 9, 10, 15, 16, 19, 25},
			expectedCount: 9,
		},
		{
			name:          "Filter by impact maintenance (0)",
			queryParams:   map[string]string{"impact": "0"},
			expectedIDs:   []int{6, 8, 17, 22},
			expectedCount: 4,
		},
		{
			name:          "Filter by component_id 1",
			queryParams:   map[string]string{"components": "1"},
			expectedIDs:   []int{1, 5, 22, 24, 25, 26},
			expectedCount: 6,
		},
		{
			name:          "Filter by non-existent component_id 8",
			queryParams:   map[string]string{"components": "8"},
			expectedIDs:   []int{},
			expectedCount: 0,
		},
		{
			name:          "Filter by system true",
			queryParams:   map[string]string{"system": "true"},
			expectedIDs:   []int{1, 7, 10, 11, 12, 13, 14, 15, 16, 18},
			expectedCount: 10,
		},
		{
			name:          "Filter by system false",
			queryParams:   map[string]string{"system": "false"},
			expectedIDs:   []int{2, 3, 4, 5, 6, 8, 9, 17, 19, 20, 21, 22, 23, 24, 25, 26, 27},
			expectedCount: 17,
		},
		{
			name:          "Filter by active true",
			queryParams:   map[string]string{"active": "true"},
			expectedIDs:   []int{26, 27},
			expectedCount: 2,
		},
		{
			name:          "Combination: active true and impact 1",
			queryParams:   map[string]string{"active": "true", "impact": "1"},
			expectedIDs:   []int{26, 27},
			expectedCount: 2,
		},
		{
			name:          "Combination: component_id 3 and system true",
			queryParams:   map[string]string{"components": "3", "system": "true"},
			expectedIDs:   []int{7, 12, 14, 16},
			expectedCount: 4,
		},
		{
			name:        "Date range: 2025-05-01 to 2025-05-24",
			queryParams: map[string]string{"start_date": time.Date(2025, 5, 01, 0, 0, 0, 0, time.UTC).Format(time.RFC3339), "end_date": time.Date(2025, 5, 24, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)},
			// Incidents starting between 2025-05-01 and 2025-05-24 (inclusive for start_date)
			// No pre-existing incidents in this range.
			expectedIDs:   []int{1},
			expectedCount: 1,
		},
		{
			name:          "Filter by impact 3 (outage)",
			queryParams:   map[string]string{"impact": "3"},
			expectedIDs:   []int{3, 5, 11, 12, 14, 18},
			expectedCount: 6,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, v2IncidentsEndpoint, nil)

			q := req.URL.Query()
			for k, v := range tc.queryParams {
				q.Add(k, v)
			}
			req.URL.RawQuery = q.Encode()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "Unexpected status code for: "+tc.name)

			var responseData V2IncidentsListResponse
			err := json.Unmarshal(w.Body.Bytes(), &responseData)
			require.NoError(t, err, "Failed to unmarshal response for: "+tc.name)

			actualIncidents := responseData.Data
			assert.Len(t, actualIncidents, tc.expectedCount, "Unexpected number of incidents for: "+tc.name)

			// When incidents are found or not, the message field should ideally be empty.
			assert.Empty(t, responseData.Message, "Expected no message for: "+tc.name)

			actualIDs := make([]int, len(actualIncidents))
			for i, inc := range actualIncidents {
				actualIDs[i] = inc.ID
			}
			assert.ElementsMatch(t, tc.expectedIDs, actualIDs, "Unexpected incident IDs for: "+tc.name)
		})
	}
}

func TestV2GetEventsHandler(t *testing.T) {
	t.Logf("start to test GET %s with pagination", v2EventsEndpoint)
	r, _, _ := initTests(t)

	type V2EventsListResponse struct {
		Data       []*v2.Incident `json:"data"`
		Pagination struct {
			PageIndex      int `json:"pageIndex"`
			RecordsPerPage int `json:"recordsPerPage"`
			TotalRecords   int `json:"totalRecords"`
			TotalPages     int `json:"totalPages"`
		} `json:"pagination"`
	}

	// Get all incidents for better debugging from /v2/incidents endpoint
	allIncidents := v2GetIncidents(t, r)
	t.Logf("Initial incidents in DB: %+v", len(allIncidents))
	totalIncidents := len(allIncidents)
	expectedpages := totalIncidents / 10
	if totalIncidents%10 != 0 {
		expectedpages++
	}

	testCases := []struct {
		name               string
		queryParams        string
		expectedStatusCode int
		expectedTotal      int
		expectedPages      int
		expectedItemsCount int
		expectedLimit      int
		expectedPage       int
	}{
		{
			name:               "Default pagination",
			queryParams:        "",
			expectedStatusCode: http.StatusOK,
			expectedTotal:      totalIncidents,
			expectedPages:      1,
			expectedItemsCount: totalIncidents,
			expectedLimit:      50, // default limit
			expectedPage:       1,  // default page
		},
		{
			name:               "Pagination with limit 10, page 1",
			queryParams:        "?limit=10&page=1",
			expectedStatusCode: http.StatusOK,
			expectedTotal:      totalIncidents,
			expectedPages:      expectedpages,
			expectedItemsCount: 10,
			expectedLimit:      10,
			expectedPage:       1,
		},
		{
			name:               "Pagination with limit 10, page 2",
			queryParams:        "?limit=10&page=2",
			expectedStatusCode: http.StatusOK,
			expectedTotal:      totalIncidents,
			expectedPages:      expectedpages,
			expectedItemsCount: 10,
			expectedLimit:      10,
			expectedPage:       2,
		},
		{
			name:               "Pagination with limit 20, page 1",
			queryParams:        "?limit=20&page=1",
			expectedStatusCode: http.StatusOK,
			expectedTotal:      totalIncidents,
			expectedPages:      2,
			expectedItemsCount: 20,
			expectedLimit:      20,
			expectedPage:       1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, v2EventsEndpoint+tc.queryParams, nil)
			r.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatusCode, w.Code)

			var response V2EventsListResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Len(t, response.Data, tc.expectedItemsCount)
			assert.Equal(t, tc.expectedTotal, response.Pagination.TotalRecords)
			assert.Equal(t, tc.expectedPages, response.Pagination.TotalPages)
			assert.Equal(t, tc.expectedLimit, response.Pagination.RecordsPerPage)
			assert.Equal(t, tc.expectedPage, response.Pagination.PageIndex)
		})
	}
}

func TestV2PostMaintenanceHandler(t *testing.T) {
	t.Log("start to test maintenance creation for /v2/incidents")
	r, _, _ := initTests(t)

	t.Log("create a maintenance")

	components := []int{1, 2}
	impact := 0
	title := "Test maintenance incident for dcs"
	description := "Test maintenance description"
	startDate := time.Now().Add(time.Hour * 1).UTC()
	endDate := time.Now().Add(time.Hour * 2).UTC()
	system := false

	incidentCreateData := v2.IncidentData{
		Title:       title,
		Description: description,
		Impact:      &impact,
		Components:  components,
		StartDate:   startDate,
		EndDate:     &endDate,
		System:      &system,
		Type:        event.TypeMaintenance,
	}

	result := v2CreateIncident(t, r, &incidentCreateData)
	assert.Len(t, incidentCreateData.Components, len(result.Result))

	incident := v2GetIncident(t, r, result.Result[0].IncidentID)
	assert.Equal(t, incidentCreateData.StartDate.Truncate(time.Microsecond), incident.StartDate)
	assert.Equal(t, incidentCreateData.EndDate.Truncate(time.Microsecond), *incident.EndDate)
	assert.Equal(t, title, incident.Title)
	assert.Equal(t, impact, *incident.Impact)
	assert.Equal(t, system, *incident.System)
	assert.Equal(t, description, incident.Description)
	assert.NotNil(t, incident.Updates)
	assert.Equal(t, event.MaintenancePlanned, incident.Updates[0].Status)
}

func TestV2PostInfoWithExistingEventsHandler(t *testing.T) {
	t.Log("start to test 'info' incident creation when an 'incident' and a 'maintenance' for the same component already exist")
	r, _, _ := initTests(t)

	// 1. Preparation: Close any existing open incidents for a clean state.
	incidentsBeforeTest := v2GetIncidents(t, r)
	for _, inc := range incidentsBeforeTest {
		if inc.EndDate == nil {
			t.Logf("Closing pre-existing open incident ID: %d for test setup", inc.ID)
			endDate := inc.StartDate.Add(time.Hour * 1).UTC()
			inc.EndDate = &endDate
			v2PatchIncident(t, r, inc)
		}
	}

	// 2. Create an active "incident" type event.
	t.Log("Create an active 'incident' type event")
	incidentComponentID := 1
	initialIncidentImpact := 1
	initialIncidentTitle := "Initial incident event"
	initialIncidentDescription := "Description for the initial incident"
	initialIncidentStartDate := time.Now().AddDate(0, 0, -1).UTC()
	initialIncidentSystem := false

	initialIncidentData := v2.IncidentData{
		Title:       initialIncidentTitle,
		Description: initialIncidentDescription,
		Impact:      &initialIncidentImpact,
		Components:  []int{incidentComponentID},
		StartDate:   initialIncidentStartDate,
		System:      &initialIncidentSystem,
		Type:        event.TypeIncident,
	}

	initialIncidentResp := v2CreateIncident(t, r, &initialIncidentData)
	require.NotNil(t, initialIncidentResp, "Failed to create initial incident")
	require.Len(t, initialIncidentResp.Result, 1, "Initial incident response should have one result")
	initialIncidentID := initialIncidentResp.Result[0].IncidentID
	t.Logf("Created active 'incident' with ID: %d for component %d", initialIncidentID, incidentComponentID)

	// 3. Create a planned "maintenance" type event for the SAME component.
	t.Log("Step 3: Create a planned 'maintenance' type event for the same component")
	maintenanceImpact := 0 // Maintenance impact is typically 0
	maintenanceTitle := "Planned Maintenance for Component"
	maintenanceDescription := "Description for the maintenance event"
	maintenanceStartDate := time.Now().AddDate(0, 0, 7).UTC()
	maintenanceEndDate := time.Now().AddDate(0, 0, 7).Add(time.Hour * 2).UTC()
	maintenanceSystem := false

	maintenanceIncidentData := v2.IncidentData{
		Title:       maintenanceTitle,
		Description: maintenanceDescription,
		Impact:      &maintenanceImpact,
		Components:  []int{incidentComponentID},
		StartDate:   maintenanceStartDate,
		EndDate:     &maintenanceEndDate,
		System:      &maintenanceSystem,
		Type:        "maintenance",
	}

	maintenanceIncidentResp := v2CreateIncident(t, r, &maintenanceIncidentData)
	require.NotNil(t, maintenanceIncidentResp, "Failed to create 'maintenance' incident")
	require.Len(t, maintenanceIncidentResp.Result, 1, "'Maintenance' incident response should have one result")
	maintenanceIncidentID := maintenanceIncidentResp.Result[0].IncidentID
	t.Logf("Created planned 'maintenance' with ID: %d for component %d, starting at %s", maintenanceIncidentID, incidentComponentID, maintenanceStartDate)

	// 4. Create a new "info" type event for the SAME component.
	t.Log("Step 4: Create a new 'info' type event for the same component")
	infoImpact := 0
	infoTitle := "Informational Update During IsActive Incident and Before Maintenance"
	infoDescription := "Description for the info event"
	infoStartDate := time.Now().Add(time.Minute * -30).UTC()
	infoEndDate := time.Now().Add(time.Minute * 30).UTC()
	infoSystem := false

	infoIncidentData := v2.IncidentData{
		Title:       infoTitle,
		Description: infoDescription,
		Impact:      &infoImpact,
		Components:  []int{incidentComponentID},
		StartDate:   infoStartDate,
		EndDate:     &infoEndDate,
		System:      &infoSystem,
		Type:        "info",
	}

	infoIncidentResp := v2CreateIncident(t, r, &infoIncidentData)
	require.NotNil(t, infoIncidentResp, "Failed to create 'info' incident")
	require.Len(t, infoIncidentResp.Result, 1, "'Info' incident response should have one result")
	infoIncidentID := infoIncidentResp.Result[0].IncidentID
	t.Logf("Created 'info' incident with ID: %d for component %d", infoIncidentID, incidentComponentID)

	// Assertions.
	t.Log("Step 5: Perform assertions")
	assert.NotEqual(t, initialIncidentID, infoIncidentID, "Info incident should have a new, distinct ID")
	assert.NotEqual(t, maintenanceIncidentID, infoIncidentID, "Info incident should have a new, distinct ID from maintenance")

	// Verify the 'info' incident.
	fetchedInfoIncident := v2GetIncident(t, r, infoIncidentID)
	assert.Equal(t, infoTitle, fetchedInfoIncident.Title)
	assert.Equal(t, infoDescription, fetchedInfoIncident.Description)
	assert.Equal(t, event.TypeInformation, fetchedInfoIncident.Type)
	assert.Equal(t, infoImpact, *fetchedInfoIncident.Impact)
	assert.Contains(t, fetchedInfoIncident.Components, incidentComponentID)
	require.NotNil(t, fetchedInfoIncident.EndDate)
	assert.True(t, infoEndDate.Truncate(time.Second).Equal(fetchedInfoIncident.EndDate.Truncate(time.Second)))

	// Verify the initial 'incident' event is still open.
	fetchedInitialIncident := v2GetIncident(t, r, initialIncidentID)
	assert.Equal(t, initialIncidentTitle, fetchedInitialIncident.Title)
	assert.Equal(t, initialIncidentDescription, fetchedInitialIncident.Description)
	assert.Equal(t, event.TypeIncident, fetchedInitialIncident.Type)
	assert.Nil(t, fetchedInitialIncident.EndDate, "Initial 'incident' event should still be open")
	assert.Contains(t, fetchedInitialIncident.Components, incidentComponentID, "Initial 'incident' should still have its component")
	assert.Len(t, fetchedInitialIncident.Components, 1, "Initial 'incident' should only have its original component")

	// Verify the planned 'maintenance' event is still scheduled.
	fetchedMaintenanceIncident := v2GetIncident(t, r, maintenanceIncidentID)
	assert.Equal(t, maintenanceTitle, fetchedMaintenanceIncident.Title)
	assert.Equal(t, maintenanceDescription, fetchedMaintenanceIncident.Description)
	assert.Equal(t, event.TypeMaintenance, fetchedMaintenanceIncident.Type)
	require.NotNil(t, fetchedMaintenanceIncident.EndDate, "Maintenance event should have an end date")
	assert.True(t, maintenanceEndDate.Truncate(time.Second).Equal(fetchedMaintenanceIncident.EndDate.Truncate(time.Second)), "Maintenance end date mismatch")
	assert.Contains(t, fetchedMaintenanceIncident.Components, incidentComponentID, "Maintenance event should still have its component")
	assert.Len(t, fetchedMaintenanceIncident.Components, 1, "Maintenance event should only have its original component")
}

func TestV2GetComponentsAvailability(t *testing.T) {
	truncateIncidents(t)
	t.Logf("start to test GET %s", v2AvailabilityEndpoint)
	r, _, _ := initTests(t)

	// Incident preparation
	t.Log("create an incident")

	components := []int{7}
	impact := 3
	title := "Test incident for dns N1"
	startDate := time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)
	system := false

	// Incident N1
	incidentCreateDataN1 := v2.IncidentData{
		Title:      title,
		Impact:     &impact,
		Components: components,
		StartDate:  startDate,
		EndDate:    nil,
		System:     &system,
		Type:       event.TypeIncident,
	}

	resultN1 := v2CreateIncident(t, r, &incidentCreateDataN1)

	assert.Len(t, resultN1.Result, len(incidentCreateDataN1.Components))

	// Incident closing
	incidentN1 := v2GetIncident(t, r, resultN1.Result[0].IncidentID)
	endDate := time.Date(2025, 7, 16, 12, 0, 0, 0, time.UTC)
	incidentN1.EndDate = &endDate
	v2PatchIncident(t, r, incidentN1)

	t.Logf("Incident patched: %+v", incidentN1)

	// Incident N2

	title = "Test incident for dns N2"
	startDate = time.Date(2025, 8, 16, 12, 0, 0, 0, time.UTC)
	endDate = time.Date(2025, 9, 16, 00, 00, 00, 0, time.UTC)

	incidentCreateDataN2 := v2.IncidentData{
		Title:      title,
		Impact:     &impact,
		Components: components,
		StartDate:  startDate,
		EndDate:    nil,
		System:     &system,
		Type:       event.TypeIncident,
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
	req, _ := http.NewRequest(http.MethodGet, v2AvailabilityEndpoint, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var availability struct {
		Data []v2.ComponentAvailability `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &availability)
	require.NoError(t, err)
	assert.NotEmpty(t, availability)

	// Test case 2: Check if the availability data is correct
	targetMonths := map[int]bool{7: true, 8: true, 9: true}

	for _, compAvail := range availability.Data {
		if compAvail.ID == 7 {
			checkComponentAvailability(t, compAvail, targetMonths)
		}
	}
}

func checkComponentAvailability(t *testing.T, compAvail v2.ComponentAvailability, targetMonths map[int]bool) {
	for _, avail := range compAvail.Availability {
		if _, ok := targetMonths[avail.Month]; ok {
			assert.InEpsilon(t, 50.00000, avail.Percentage, 0.00001,
				"Availability percentage should be 50% for the target months")
			// t.Logf("Availability for %v: %d-%d: %.2f%%", compAvail.Name, avail.Year, avail.Month, avail.Percentage)
		} else {
			assert.InEpsilon(t, 100.00000, avail.Percentage, 0.00001,
				"Availability percentage should be 100% for all months except the target months")
		}
	}
}

func TestV2PatchEventUpdateHandler(t *testing.T) {
	t.Log("start to test PATCH /v2/incidents/:incidentID/updates/:updateID")
	r, _, _ := initTests(t)

	// Clean up database before test to ensure a clean state for this test case.
	truncateIncidents(t)

	components := []int{1}
	impact := 1
	title := "Incident for testing update patch"
	startDate := time.Now().UTC()
	system := false
	incidentCreateData := v2.IncidentData{
		Title:      title,
		Impact:     &impact,
		Components: components,
		StartDate:  startDate,
		System:     &system,
		Type:       event.TypeIncident,
	}

	createResp := v2CreateIncident(t, r, &incidentCreateData)
	require.NotNil(t, createResp, "Failed to create incident for test")
	require.Len(t, createResp.Result, 1)
	incidentID := createResp.Result[0].IncidentID

	// The created incident has one update with index 0
	initialIncident := v2GetIncident(t, r, incidentID)
	require.Len(t, initialIncident.Updates, 1)

	testCases := []struct {
		name           string
		incidentID     int
		updateIndex    int
		body           string
		expectedStatus int
		expectedBody   string
		checkResponse  func(t *testing.T, body []byte)
	}{
		{
			name:           "Successful update",
			incidentID:     incidentID,
			updateIndex:    0,
			body:           `{"text": "The text of this update has been successfully changed."}`,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body []byte) {
				var update v2.EventUpdateData
				err := json.Unmarshal(body, &update)
				require.NoError(t, err)
				assert.Equal(t, 0, update.ID)
				assert.Equal(t, "The text of this update has been successfully changed.", update.Text)
				assert.Equal(t, event.IncidentDetected, update.Status)
			},
		},
		{
			name:           "Incident not found",
			incidentID:     99999,
			updateIndex:    0,
			body:           `{"text": "This should fail."}`,
			expectedStatus: http.StatusNotFound,
			expectedBody:   fmt.Sprintf(`{"errMsg":"%s"}`, apiErrors.ErrIncidentDSNotExist),
		},
		{
			name:           "Update index not found",
			incidentID:     incidentID,
			updateIndex:    99,
			body:           `{"text": "This should also fail."}`,
			expectedStatus: http.StatusNotFound,
			expectedBody:   fmt.Sprintf(`{"errMsg":"%s"}`, apiErrors.ErrUpdateDSNotExist),
		},
		{
			name:           "Invalid update index (negative)",
			incidentID:     incidentID,
			updateIndex:    -1,
			body:           `{"text": "This should also fail."}`,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   (`{"errMsg":"Key: 'updateData.UpdateID' Error:Field validation for 'UpdateID' failed on the 'gte' tag"}`),
		},
		{
			name:           "Empty text in body",
			incidentID:     incidentID,
			updateIndex:    0,
			body:           `{"text": ""}`,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   (`{"errMsg":"Key: 'PatchEventUpdateData.Text' Error:Field validation for 'Text' failed on the 'required' tag"}`),
		},
		{
			name:           "Missing text field in body",
			incidentID:     incidentID,
			updateIndex:    0,
			body:           `{}`,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   (`{"errMsg":"Key: 'PatchEventUpdateData.Text' Error:Field validation for 'Text' failed on the 'required' tag"}`),
		},
		{
			name:           "ff",
			incidentID:     incidentID,
			updateIndex:    0,
			body:           `{"text": "invalid json`,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"errMsg":"unexpected EOF"}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			url := fmt.Sprintf("/v2/incidents/%d/updates/%d", tc.incidentID, tc.updateIndex)
			req, _ := http.NewRequest(http.MethodPatch, url, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)

			if tc.checkResponse != nil {
				tc.checkResponse(t, w.Body.Bytes())
			}

			if tc.expectedBody != "" {
				assert.JSONEq(t, tc.expectedBody, w.Body.String())
			}
		})
	}
}
