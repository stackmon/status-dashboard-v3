package v1

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCustomTimeFormat(t *testing.T) {
	timeRFC3339Str := "2024-09-01T11:45:26.371Z"
	parsedTime, err := time.Parse(time.RFC3339, timeRFC3339Str)
	require.NoError(t, err)
	inc := &Incident{
		IncidentData: IncidentData{
			StartDate: SD2Time(parsedTime),
			EndDate:   nil,
		},
	}

	data, err := json.Marshal(inc)
	require.NoError(t, err)
	assert.JSONEq(t, "{\"id\":0,\"text\":\"\",\"impact\":null,\"start_date\":\"2024-09-01 11:45\",\"end_date\":null,\"updates\":null}", string(data))

	inc = &Incident{}
	err = json.Unmarshal(data, &inc)
	require.NoError(t, err)
	assert.Equal(t, parsedTime.YearDay(), time.Time(inc.StartDate).YearDay())
	assert.Equal(t, parsedTime.Hour(), time.Time(inc.StartDate).Hour())
	assert.Equal(t, parsedTime.Minute(), time.Time(inc.StartDate).Minute())
	assert.NotEqual(t, parsedTime.Second(), time.Time(inc.StartDate).Second())
	assert.Nil(t, inc.EndDate)
}
