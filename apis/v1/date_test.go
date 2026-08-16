package v1

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDate(t *testing.T) {
	d, err := ParseDate("2026-08-16")
	require.NoError(t, err)
	assert.True(t, time.Time(d).Equal(time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)))

	_, err = ParseDate("not-a-date")
	assert.Error(t, err)

	_, err = ParseDate("2026-08-16T00:00:00Z")
	assert.Error(t, err)
}

func TestDate_MarshalJSON(t *testing.T) {
	d := Date(time.Date(2026, 8, 16, 15, 4, 5, 0, time.UTC))

	b, err := json.Marshal(d)
	require.NoError(t, err)
	assert.Equal(t, `"2026-08-16"`, string(b))
}

func TestDate_UnmarshalJSON(t *testing.T) {
	var d Date
	require.NoError(t, json.Unmarshal([]byte(`"2026-08-16"`), &d))
	assert.True(t, time.Time(d).Equal(time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)))

	err := json.Unmarshal([]byte(`"not-a-date"`), &d)
	assert.Error(t, err)

	err = json.Unmarshal([]byte(`123`), &d)
	assert.Error(t, err)
}

func TestDate_RoundTrip(t *testing.T) {
	type wrapper struct {
		D Date `json:"d"`
	}

	in := wrapper{D: Date(time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC))}
	b, err := json.Marshal(in)
	require.NoError(t, err)
	assert.JSONEq(t, `{"d":"2026-08-16"}`, string(b))

	var out wrapper
	require.NoError(t, json.Unmarshal(b, &out))
	assert.Equal(t, in, out)
}

func TestDate_BeginningAndEnd(t *testing.T) {
	d, err := ParseDate("2026-08-16")
	require.NoError(t, err)

	beginning := d.Beginning()
	assert.Equal(t, time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC), beginning)

	end := d.End()
	assert.Equal(t, time.Date(2026, 8, 16, 23, 59, 59, 999999999, time.UTC), end)

	assert.True(t, beginning.Before(end))
}
