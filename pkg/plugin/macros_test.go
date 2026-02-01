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
