package plugin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/stretchr/testify/require"
)

// newMockedDatasource wires a sqlmock-backed *sql.DB into a Datasource so we
// can exercise query() end to end without touching a real Exasol instance.
func newMockedDatasource(t *testing.T) (*Datasource, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(false))
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})
	return &Datasource{db: db, queryTimeoutSecs: 5}, mock
}

func tableQuery(sql string) backend.DataQuery {
	return backend.DataQuery{
		RefID: "A",
		JSON:  []byte(`{"queryText":"` + sql + `","format":"table"}`),
		TimeRange: backend.TimeRange{
			From: time.UnixMilli(1700000000000).UTC(),
			To:   time.UnixMilli(1700003600000).UTC(),
		},
	}
}

func timeSeriesQuery(sql string) backend.DataQuery {
	return backend.DataQuery{
		RefID: "A",
		JSON:  []byte(`{"queryText":"` + sql + `","format":"time_series"}`),
		TimeRange: backend.TimeRange{
			From: time.UnixMilli(1700000000000).UTC(),
			To:   time.UnixMilli(1700003600000).UTC(),
		},
	}
}

func TestQuery_TableFormat_TypedColumns(t *testing.T) {
	ds, mock := newMockedDatasource(t)

	ts := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRowsWithColumnDefinition(
		sqlmock.NewColumn("MEASURE_TIME").OfType("TIMESTAMP", time.Time{}),
		sqlmock.NewColumn("USERS_AVG").OfType("DECIMAL", float64(0)).WithPrecisionAndScale(9, 2),
		sqlmock.NewColumn("USERS_MAX").OfType("DECIMAL", int64(0)).WithPrecisionAndScale(9, 0),
		sqlmock.NewColumn("LOAD").OfType("DOUBLE", float64(0)),
		sqlmock.NewColumn("CLUSTER_NAME").OfType("VARCHAR", ""),
		sqlmock.NewColumn("ACTIVE").OfType("BOOLEAN", false),
	).AddRow(ts, float64(1.5), int64(10), float64(0.42), "clusterA", true)

	mock.ExpectExec("ALTER SESSION").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	resp := ds.query(context.Background(), tableQuery("SELECT *"))
	require.NoError(t, resp.Error, "table query should not error")
	require.Len(t, resp.Frames, 1)

	frame := resp.Frames[0]
	require.Equal(t, 6, len(frame.Fields))
	require.Equal(t, data.FieldTypeNullableTime, frame.Fields[0].Type())
	require.Equal(t, data.FieldTypeNullableFloat64, frame.Fields[1].Type(), "DECIMAL(9,2) should map to float64")
	require.Equal(t, data.FieldTypeNullableInt64, frame.Fields[2].Type(), "DECIMAL(9,0) should map to int64")
	require.Equal(t, data.FieldTypeNullableFloat64, frame.Fields[3].Type())
	require.Equal(t, data.FieldTypeNullableString, frame.Fields[4].Type())
	require.Equal(t, data.FieldTypeNullableBool, frame.Fields[5].Type())

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuery_TimeSeriesFormat_WideFrameDeterministic(t *testing.T) {
	ds, mock := newMockedDatasource(t)

	t1 := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Minute)

	build := func() *sqlmock.Rows {
		return sqlmock.NewRowsWithColumnDefinition(
			sqlmock.NewColumn("MEASURE_TIME").OfType("TIMESTAMP", time.Time{}),
			sqlmock.NewColumn("VALUE").OfType("DOUBLE", float64(0)),
			sqlmock.NewColumn("CLUSTER_NAME").OfType("VARCHAR", ""),
		).
			AddRow(t1, float64(1.0), "clusterB").
			AddRow(t1, float64(2.0), "clusterA").
			AddRow(t2, float64(3.0), "clusterB").
			AddRow(t2, float64(4.0), "clusterA")
	}

	// Run twice; field order must be identical thanks to series-key sorting.
	var firstNames []string
	for i := 0; i < 2; i++ {
		mock.ExpectExec("ALTER SESSION").WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery("SELECT").WillReturnRows(build())

		resp := ds.query(context.Background(), timeSeriesQuery("SELECT *"))
		require.NoError(t, resp.Error)
		require.Len(t, resp.Frames, 1)
		frame := resp.Frames[0]
		require.Equal(t, data.FrameTypeTimeSeriesWide, frame.Meta.Type)
		require.Equal(t, 3, len(frame.Fields), "time + 2 series")

		names := []string{frame.Fields[1].Labels["CLUSTER_NAME"], frame.Fields[2].Labels["CLUSTER_NAME"]}
		if i == 0 {
			firstNames = names
		} else {
			require.Equal(t, firstNames, names, "wide-frame series order must be deterministic across runs")
		}
	}

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuery_TimeSeriesFormat_EmptyResultReturnsEmptyFrame(t *testing.T) {
	ds, mock := newMockedDatasource(t)

	rows := sqlmock.NewRowsWithColumnDefinition(
		sqlmock.NewColumn("MEASURE_TIME").OfType("TIMESTAMP", time.Time{}),
		sqlmock.NewColumn("VALUE").OfType("DOUBLE", float64(0)),
	)
	mock.ExpectExec("ALTER SESSION").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	resp := ds.query(context.Background(), timeSeriesQuery("SELECT *"))
	require.NoError(t, resp.Error)
	require.Len(t, resp.Frames, 1)
	require.Equal(t, 0, len(resp.Frames[0].Fields), "empty result should produce an empty frame, not an error")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuery_TimeSeriesFormat_RequiresTimeAndNumeric(t *testing.T) {
	ds, mock := newMockedDatasource(t)

	rows := sqlmock.NewRowsWithColumnDefinition(
		sqlmock.NewColumn("CLUSTER_NAME").OfType("VARCHAR", ""),
		sqlmock.NewColumn("EVENT").OfType("VARCHAR", ""),
	).AddRow("clusterA", "boot")

	mock.ExpectExec("ALTER SESSION").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	resp := ds.query(context.Background(), timeSeriesQuery("SELECT *"))
	require.Error(t, resp.Error)
	require.Contains(t, resp.Error.Error(), "time series format requires")
}

func TestQuery_DriverError_ReturnsBadRequest(t *testing.T) {
	ds, mock := newMockedDatasource(t)

	mock.ExpectExec("ALTER SESSION").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT").WillReturnError(errors.New("E-EGOD-11: bad sql"))

	resp := ds.query(context.Background(), tableQuery("SELECT BAD"))
	require.Error(t, resp.Error)
	require.Contains(t, resp.Error.Error(), "query execution failed")
}

func TestQuery_MissingQueryText_ReturnsBadRequest(t *testing.T) {
	ds, _ := newMockedDatasource(t)

	resp := ds.query(context.Background(), backend.DataQuery{
		RefID: "A",
		JSON:  []byte(`{"queryText":"","format":"table"}`),
	})
	require.Error(t, resp.Error)
	require.Contains(t, resp.Error.Error(), "query text is required")
}

func TestQuery_InvalidFormat_ReturnsBadRequest(t *testing.T) {
	ds, _ := newMockedDatasource(t)

	resp := ds.query(context.Background(), backend.DataQuery{
		RefID: "A",
		JSON:  []byte(`{"queryText":"SELECT 1","format":"json"}`),
	})
	require.Error(t, resp.Error)
	require.Contains(t, resp.Error.Error(), "invalid format")
}
