package metric

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTimeseries(t *testing.T) {
	now := time.Date(2023, 10, 1, 12, 4, 5, 0, time.UTC)
	nowFunc = func() time.Time { return now }

	ts := NewTimeSeries[float64](time.Second, 3, AVG)
	ts.useRawTime = true
	ts.Add(1.0)

	now = now.Add(time.Second)
	ts.Add(2.0)

	require.JSONEq(t, `[`+
		`{"ts":"2023-10-01T12:04:05Z","value":1},`+
		`{"ts":"2023-10-01T12:04:06Z","value":2}`+
		`]`, ts.String())

	now = now.Add(time.Second)
	ts.Add(3.0)

	now = now.Add(time.Second)
	ts.Add(4.0)

	times, values := ts.Values()
	require.Equal(t, []time.Time{
		time.Date(2023, time.October, 1, 12, 4, 6, 0, time.UTC),
		time.Date(2023, time.October, 1, 12, 4, 7, 0, time.UTC),
		time.Date(2023, time.October, 1, 12, 4, 8, 0, time.UTC),
	}, times)
	require.Equal(t, []float64{2, 3, 4}, values)

	require.JSONEq(t, `[`+
		`{"ts":"2023-10-01T12:04:06Z","value":2},`+
		`{"ts":"2023-10-01T12:04:07Z","value":3},`+
		`{"ts":"2023-10-01T12:04:08Z","value":4}`+
		`]`, ts.String())

	now = now.Add(100 * time.Millisecond)
	ts.Add(5.0)

	now = now.Add(200 * time.Millisecond)
	ts.Add(4.8)

	require.JSONEq(t, `[`+
		`{"ts":"2023-10-01T12:04:06Z","value":2},`+
		`{"ts":"2023-10-01T12:04:07Z","value":3},`+
		`{"ts":"2023-10-01T12:04:08Z","value":4.6}`+
		`]`, ts.String())

	now = now.Add(1700 * time.Millisecond)
	ts.Add(6.0)

	require.JSONEq(t, `[`+
		`{"ts":"2023-10-01T12:04:08Z","value":4.6},`+
		`{"ts":"2023-10-01T12:04:09Z"},`+
		`{"ts":"2023-10-01T12:04:10Z","value":6}`+
		`]`, ts.String())

	now = now.Add(5 * time.Second)
	ts.Add(7.0)

	require.JSONEq(t, `[`+
		`{"ts":"2023-10-01T12:04:15Z","value":7.0}`+
		`]`, ts.String())
}

func TestTimeSeriesSubSeconds(t *testing.T) {
	ts := NewTimeSeries[float64](time.Second, 10, LAST)

	now := time.Date(2023, 10, 1, 12, 4, 5, 0, time.UTC)
	nowFunc = func() time.Time {
		ret := now
		now = now.Add(100 * time.Millisecond)
		return ret
	}

	for i := 1; i <= 10*10; i++ {
		ts.Add(float64(i))
	}

	require.JSONEq(t, `[`+
		`{"ts":"2023-10-01T12:04:06Z","value":10.000000},`+
		`{"ts":"2023-10-01T12:04:07Z","value":20.000000},`+
		`{"ts":"2023-10-01T12:04:08Z","value":30.000000},`+
		`{"ts":"2023-10-01T12:04:09Z","value":40.000000},`+
		`{"ts":"2023-10-01T12:04:10Z","value":50.000000},`+
		`{"ts":"2023-10-01T12:04:11Z","value":60.000000},`+
		`{"ts":"2023-10-01T12:04:12Z","value":70.000000},`+
		`{"ts":"2023-10-01T12:04:13Z","value":80.000000},`+
		`{"ts":"2023-10-01T12:04:14Z","value":90.000000},`+
		`{"ts":"2023-10-01T12:04:15Z","value":100.000000}`+
		`]`, ts.String())

	ss := ts.Snapshot(nil)
	require.Equal(t, []time.Time{
		time.Date(2023, 10, 1, 12, 4, 6, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 4, 7, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 4, 8, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 4, 9, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 4, 10, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 4, 11, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 4, 12, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 4, 13, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 4, 14, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 4, 15, 0, time.UTC),
	}, ss.Times)
	require.Equal(t, []float64{
		10.0, 20.0, 30.0, 40.0, 50.0,
		60.0, 70.0, 80.0, 90.0, 100.0,
	}, ss.Values)
	require.Equal(t, time.Second, ss.Interval)
	require.Equal(t, 10, ss.MaxCount)

	ptTime, ptValue := ts.Last()
	require.Equal(t, 100.0, ptValue)
	require.Equal(t, time.Date(2023, 10, 1, 12, 4, 15, 0, time.UTC), ptTime)

	ptTimes, _ := ts.LastN(0)
	require.Nil(t, ptTimes)
	ptTimes, _ = ts.LastN(-1)
	require.Nil(t, ptTimes)

	ptTimes, _ = ts.LastN(20)
	require.Equal(t, 10, len(ptTimes))

	ptTimes, ptValues := ts.After(time.Date(2023, 10, 1, 12, 4, 13, 0, time.UTC))
	require.Equal(t, 3, len(ptTimes))
	require.Equal(t, 80.0, ptValues[0])
	require.Equal(t, time.Date(2023, 10, 1, 12, 4, 13, 0, time.UTC), ptTimes[0])
	require.Equal(t, 90.0, ptValues[1])
	require.Equal(t, time.Date(2023, 10, 1, 12, 4, 14, 0, time.UTC), ptTimes[1])
	require.Equal(t, 100.0, ptValues[2])
	require.Equal(t, time.Date(2023, 10, 1, 12, 4, 15, 0, time.UTC), ptTimes[2])
}

func TestMultiTimeSeries(t *testing.T) {
	mts := MultiTimeSeries[float64]{
		NewTimeSeries[float64](time.Second, 10, LAST),
		NewTimeSeries[float64](10*time.Second, 6, LAST),
		NewTimeSeries[float64](60*time.Second, 5, LAST),
	}

	now := time.Date(2023, 10, 1, 12, 4, 5, 0, time.UTC)
	nowFunc = func() time.Time { return now }

	for i := 1; i <= 10*5*60; i++ {
		mts.Add(float64(i))
		now = now.Add(100 * time.Millisecond)
	}

	require.JSONEq(t, `[`+
		`[`+
		`{"ts":"2023-10-01T12:08:56Z","value":2910.000000},`+
		`{"ts":"2023-10-01T12:08:57Z","value":2920.000000},`+
		`{"ts":"2023-10-01T12:08:58Z","value":2930.000000},`+
		`{"ts":"2023-10-01T12:08:59Z","value":2940.000000},`+
		`{"ts":"2023-10-01T12:09:00Z","value":2950.000000},`+
		`{"ts":"2023-10-01T12:09:01Z","value":2960.000000},`+
		`{"ts":"2023-10-01T12:09:02Z","value":2970.000000},`+
		`{"ts":"2023-10-01T12:09:03Z","value":2980.000000},`+
		`{"ts":"2023-10-01T12:09:04Z","value":2990.000000},`+
		`{"ts":"2023-10-01T12:09:05Z","value":3000.000000}`+
		`],`+
		`[`+
		`{"ts":"2023-10-01T12:08:20Z","value":2550.000000},`+
		`{"ts":"2023-10-01T12:08:30Z","value":2650.000000},`+
		`{"ts":"2023-10-01T12:08:40Z","value":2750.000000},`+
		`{"ts":"2023-10-01T12:08:50Z","value":2850.000000},`+
		`{"ts":"2023-10-01T12:09:00Z","value":2950.000000},`+
		`{"ts":"2023-10-01T12:09:10Z","value":3000.000000}`+
		`],`+
		`[`+
		`{"ts":"2023-10-01T12:06:00Z","value":1150.000000},`+
		`{"ts":"2023-10-01T12:07:00Z","value":1750.000000},`+
		`{"ts":"2023-10-01T12:08:00Z","value":2350.000000},`+
		`{"ts":"2023-10-01T12:09:00Z","value":2950.000000},`+
		`{"ts":"2023-10-01T12:10:00Z","value":3000.000000}`+
		`]`+
		`]`, mts.String())
}
