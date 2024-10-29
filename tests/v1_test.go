package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/stackmon/otc-status-dashboard/internal/api/v1"
)

func TestGetIncidentsHandler(t *testing.T) {
	t.Log("start to test GET /v1/incidents")
	r, _ := initTests(t)

	var response = `[{"id":1,"text":"Opened incident without any update","impact":1,"start_date":"2024-10-24 10:12","end_date":null,"updates":[]}]`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/incidents", nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, response, w.Body.String())
}

func TestGetComponentsStatusHandler(t *testing.T) {
	t.Log("start to test GET /v1/component_status")
	r, _ := initTests(t)

	var response = `[{"id":1,"attributes":[{"name":"region","value":"EU-DE"},{"name":"category","value":"Container"},{"name":"type","value":"cce"}],"name":"Cloud Container Engine","incidents":[{"id":1,"text":"Opened incident without any update","impact":1,"start_date":"2024-10-24 10:12","end_date":null,"updates":[]}]},{"id":2,"attributes":[{"name":"region","value":"EU-NL"},{"name":"category","value":"Container"},{"name":"type","value":"cce"}],"name":"Cloud Container Engine","incidents":[]},{"id":3,"attributes":[{"name":"region","value":"EU-DE"},{"name":"category","value":"Compute"},{"name":"type","value":"ecs"}],"name":"Elastic Cloud Server","incidents":[]},{"id":4,"attributes":[{"name":"region","value":"EU-NL"},{"name":"category","value":"Compute"},{"name":"type","value":"ecs"}],"name":"Elastic Cloud Server","incidents":[]},{"id":5,"attributes":[{"name":"region","value":"EU-DE"},{"name":"category","value":"Database"},{"name":"type","value":"dcs"}],"name":"Distributed Cache Service","incidents":[]},{"id":6,"attributes":[{"name":"region","value":"EU-NL"},{"name":"category","value":"Database"},{"name":"type","value":"dcs"}],"name":"Distributed Cache Service","incidents":[]}]`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/component_status", nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, response, w.Body.String())
}

func TestPostComponentsStatusHandler(t *testing.T) {
	t.Log("start to test POST requests to /v1/component_status")
	r, _ := initTests(t)

	type testCase struct {
		ExpectedCode int
		Expected     string
		JSON         string
	}

	testCases := map[string]*testCase{
		"positive testcase": {
			JSON:         `{"name":"Distributed Cache Service","text":"Incident","impact": 2,"attributes": [{"name":"region","value":"EU-NL"}]}`,
			Expected:     `{"id":2,"text":"Incident","impact":2,`,
			ExpectedCode: 200,
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
		//nolint:gocritic
		//"negative testcase, the incident with given impact and component already exists": {
		//	JSON:     `{"name":"Cloud Container Engine","text":"Incident","impact": 1,"attributes": [{"name":"region","value":"EU-DE"}]}`,
		//	Expected: `{"details":"Check your request parameters","existingIncidentId":1,"existingIncidentTitle":"Opened incident without any update","message":"Incident with this the component already exists","targetComponent":{"id":1,"name":"Cloud Container Engine","attributes":[{"name":"region","value":"EU-DE"},{"name":"category","value":"Container"},{"name":"type","value":"cce"}]}}`,
		//},
	}

	for title, c := range testCases {
		t.Logf("start test case: %s\n", title)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/v1/component_status", strings.NewReader(c.JSON))
		r.ServeHTTP(w, req)

		if title == "positive testcase" {
			inc := &v1.Incident{}
			err := json.Unmarshal(w.Body.Bytes(), inc)
			require.NoError(t, err)
			assert.Equal(t, 2, inc.ID)
			assert.Equal(t, "Incident", inc.Text)
			assert.Equal(t, 0, len(inc.Updates)) //nolint:testifylint
			assert.Equal(t, http.StatusOK, c.ExpectedCode)
			assert.True(t, strings.HasPrefix(w.Body.String(), c.Expected))
			continue
		}
		assert.Equal(t, c.ExpectedCode, w.Code)
		assert.Equal(t, c.Expected, w.Body.String())
	}
}
