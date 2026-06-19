package plugin

import (
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/require"
)

func testMacroQuery() backend.DataQuery {
	return backend.DataQuery{
		TimeRange: backend.TimeRange{
			From: time.UnixMilli(1700000000000).UTC(),
			To:   time.UnixMilli(1700003600000).UTC(),
		},
	}
}

func TestInterpolateQuery_TimeFilter(t *testing.T) {
	got, err := interpolateQuery(
		"SELECT * FROM EXA_USAGE_LAST_DAY WHERE $__timeFilter(MEASURE_TIME)",
		testMacroQuery(),
	)
	require.NoError(t, err)
	require.Equal(
		t,
		"SELECT * FROM EXA_USAGE_LAST_DAY WHERE MEASURE_TIME >= ADD_SECONDS(TIMESTAMP '1970-01-01 00:00:00', 1700000000000 / 1000) AND MEASURE_TIME <= ADD_SECONDS(TIMESTAMP '1970-01-01 00:00:00', 1700003600000 / 1000)",
		got,
	)
}

func TestInterpolateQuery_TimeFromToNoArgs(t *testing.T) {
	got, err := interpolateQuery(
		"SELECT $__timeFrom() AS start_ts, $__timeTo() AS end_ts",
		testMacroQuery(),
	)
	require.NoError(t, err)
	require.Equal(
		t,
		"SELECT ADD_SECONDS(TIMESTAMP '1970-01-01 00:00:00', 1700000000000 / 1000) AS start_ts, ADD_SECONDS(TIMESTAMP '1970-01-01 00:00:00', 1700003600000 / 1000) AS end_ts",
		got,
	)
}

func TestInterpolateQuery_TimeAlias(t *testing.T) {
	got, err := interpolateQuery(
		`SELECT $__time(MEASURE_TIME), USERS_AVG FROM EXA_USAGE_HOURLY`,
		testMacroQuery(),
	)
	require.NoError(t, err)
	require.Equal(t, `SELECT MEASURE_TIME AS "time", USERS_AVG FROM EXA_USAGE_HOURLY`, got)
}

func TestInterpolateQuery_TimeGroupAlias(t *testing.T) {
	got, err := interpolateQuery(
		`SELECT $__timeGroupAlias(MEASURE_TIME, '5m'), AVG(USERS_AVG) FROM EXA_USAGE_HOURLY GROUP BY 1`,
		testMacroQuery(),
	)
	require.NoError(t, err)
	require.Equal(
		t,
		`SELECT ADD_SECONDS(TIMESTAMP '1970-01-01 00:00:00', FLOOR(SECONDS_BETWEEN(MEASURE_TIME, TIMESTAMP '1970-01-01 00:00:00') * 1000 / 300000) * 300000 / 1000.0) AS "time", AVG(USERS_AVG) FROM EXA_USAGE_HOURLY GROUP BY 1`,
		got,
	)
}

func TestInterpolateQuery_TimeGroupCalendarMonth(t *testing.T) {
	got, err := interpolateQuery(
		`SELECT $__timeGroup(MEASURE_TIME, '1M') AS bucket FROM EXA_USAGE_HOURLY`,
		testMacroQuery(),
	)
	require.NoError(t, err)
	require.Equal(t, `SELECT DATE_TRUNC('month', MEASURE_TIME) AS bucket FROM EXA_USAGE_HOURLY`, got)
}

func TestInterpolateQuery_TimeGroupAcceptsOptionalFillArgument(t *testing.T) {
	got, err := interpolateQuery(
		`SELECT $__timeGroup(MEASURE_TIME, '5m', 0) AS bucket FROM EXA_USAGE_HOURLY`,
		testMacroQuery(),
	)
	require.NoError(t, err)
	require.Equal(
		t,
		`SELECT ADD_SECONDS(TIMESTAMP '1970-01-01 00:00:00', FLOOR(SECONDS_BETWEEN(MEASURE_TIME, TIMESTAMP '1970-01-01 00:00:00') * 1000 / 300000) * 300000 / 1000.0) AS bucket FROM EXA_USAGE_HOURLY`,
		got,
	)
}

func TestInterpolateQuery_TimeGroupRejectsMultiMonthBucketing(t *testing.T) {
	_, err := interpolateQuery(
		`SELECT $__timeGroup(MEASURE_TIME, '2M') FROM EXA_USAGE_HOURLY`,
		testMacroQuery(),
	)
	require.ErrorContains(t, err, `unsupported interval "2M": only 1M and 1y are supported for calendar bucketing`)
}

func TestInterpolateQuery_IntervalMacros(t *testing.T) {
	q := testMacroQuery()
	q.Interval = 5 * time.Minute

	got, err := interpolateQuery(`SELECT $__interval AS i, $__interval_ms AS ims`, q)
	require.NoError(t, err)
	require.Equal(t, `SELECT 5m AS i, 300000 AS ims`, got)
}

func TestInterpolateQuery_IntervalFallback(t *testing.T) {
	q := testMacroQuery()
	q.Interval = 0

	got, err := interpolateQuery(`SELECT $__interval AS i, $__interval_ms AS ims`, q)
	require.NoError(t, err)
	require.Equal(t, `SELECT 1s AS i, 1000 AS ims`, got)
}

func TestInterpolateQuery_UnixEpochFilter(t *testing.T) {
	got, err := interpolateQuery(
		`SELECT * FROM EVENTS WHERE $__unixEpochFilter(TS)`,
		testMacroQuery(),
	)
	require.NoError(t, err)
	require.Equal(
		t,
		`SELECT * FROM EVENTS WHERE TS >= 1700000000 AND TS <= 1700003600`,
		got,
	)
}

func TestInterpolateQuery_UnixEpochGroup(t *testing.T) {
	got, err := interpolateQuery(
		`SELECT $__unixEpochGroup(TS, '5m') AS bucket FROM EVENTS GROUP BY 1`,
		testMacroQuery(),
	)
	require.NoError(t, err)
	require.Equal(t, `SELECT FLOOR(TS / 300) * 300 AS bucket FROM EVENTS GROUP BY 1`, got)
}

func TestInterpolateQuery_UnixEpochGroupRejectsSubSecondBucket(t *testing.T) {
	_, err := interpolateQuery(
		`SELECT $__unixEpochGroup(TS, '500ms') FROM EVENTS`,
		testMacroQuery(),
	)
	require.ErrorContains(t, err, "unixEpochGroup requires a bucket of at least 1 second")
}
