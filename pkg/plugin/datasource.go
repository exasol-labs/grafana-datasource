package plugin

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/exasol/exasol-driver-go"
	"github.com/exasol/exasol/pkg/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// Make sure Datasource implements required interfaces. This is important to do
// since otherwise we will only get a not implemented error response from plugin in
// runtime. In this example datasource instance implements backend.QueryDataHandler,
// backend.CheckHealthHandler interfaces. Plugin should not implement all these
// interfaces - only those which are required for a particular task.
var (
	_ backend.QueryDataHandler      = (*Datasource)(nil)
	_ backend.CheckHealthHandler    = (*Datasource)(nil)
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
)

const startupPingTimeout = 5 * time.Second

// NewDatasource creates a new datasource instance.
func NewDatasource(_ context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	config, err := models.LoadPluginSettings(settings)
	if err != nil {
		return nil, fmt.Errorf("failed to load plugin settings: %w", err)
	}

	// Create Exasol database connection
	// Note: ValidateServerCertificate(true) = validate cert, ValidateServerCertificate(false) = skip validation
	port := config.Port
	if port == 0 {
		port = 8563 // Default Exasol port
	}
	connectionString := exasol.NewConfig(config.User, config.Secrets.Password).
		Host(config.DatabaseHost).
		Port(port).
		Schema(config.Schema).
		ValidateServerCertificate(!config.InsecureSkipVerify).
		String()

	db, err := sql.Open("exasol", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Test the connection with a timeout to avoid hanging startup on network issues.
	pingCtx, cancel := context.WithTimeout(context.Background(), startupPingTimeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.DefaultLogger.Warn("Failed to close Exasol database after ping failure", "error", closeErr.Error())
		}
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.DefaultLogger.Info("Successfully connected to Exasol database", "host", config.DatabaseHost)

	return &Datasource{
		db: db,
	}, nil
}

// Datasource is an Exasol datasource which can respond to data queries and reports its health.
type Datasource struct {
	db *sql.DB
}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created. As soon as datasource settings change detected by SDK old datasource instance will
// be disposed and a new one will be created using NewDatasource factory function.
func (d *Datasource) Dispose() {
	if d.db != nil {
		if err := d.db.Close(); err != nil {
			log.DefaultLogger.Warn("Failed to close Exasol database connection", "error", err.Error())
			return
		}
		log.DefaultLogger.Info("Closed Exasol database connection")
	}
}

// QueryData handles multiple queries and returns multiple responses.
// req contains the queries []DataQuery (where each query contains RefID as a unique identifier).
// The QueryDataResponse contains a map of RefID to the response for each query, and each response
// contains Frames ([]*Frame).
func (d *Datasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	// create response struct
	response := backend.NewQueryDataResponse()

	// loop over queries and execute them individually.
	for _, q := range req.Queries {
		res := d.query(ctx, q)

		// save the response in a hashmap
		// based on with RefID as identifier
		response.Responses[q.RefID] = res
	}

	return response, nil
}

type queryModel struct {
	QueryText string `json:"queryText"`
	Format    string `json:"format"`
}

const (
	queryFormatTable      = "table"
	queryFormatTimeSeries = "time_series"
	// Keep session timezone deterministic so Grafana UTC time range aligns with Exasol temporal values.
	sessionTimeZoneUTC = "ALTER SESSION SET TIME_ZONE='UTC'"
)

func (d *Datasource) query(ctx context.Context, query backend.DataQuery) backend.DataResponse {
	var response backend.DataResponse

	// Unmarshal the JSON into our queryModel.
	var qm queryModel

	err := json.Unmarshal(query.JSON, &qm)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}

	// Check if query text is provided
	if qm.QueryText == "" {
		return backend.ErrDataResponse(backend.StatusBadRequest, "query text is required")
	}
	if qm.Format == "" {
		qm.Format = queryFormatTable
	}
	if qm.Format != queryFormatTable && qm.Format != queryFormatTimeSeries {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("invalid format: %s", qm.Format))
	}

	expandedQuery, err := interpolateQuery(qm.QueryText, query)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("query interpolation failed: %v", err.Error()))
	}

	log.DefaultLogger.Debug("Executing query", "query", expandedQuery)

	// Pin query execution to a single connection so session-level timezone applies to this query.
	conn, err := d.db.Conn(ctx)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("failed to get database connection: %v", err.Error()))
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.DefaultLogger.Warn("Failed to close Exasol query connection", "error", err.Error())
		}
	}()

	// Force UTC in session to avoid timezone drift between Grafana picker timestamps and Exasol rendering.
	if _, err := conn.ExecContext(ctx, sessionTimeZoneUTC); err != nil {
		log.DefaultLogger.Warn("Failed to set Exasol session time zone to UTC; continuing with server/session default", "error", err.Error())
	}

	// Execute the SQL query
	rows, err := conn.QueryContext(ctx, expandedQuery)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("query execution failed: %v", err.Error()))
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.DefaultLogger.Warn("Failed to close Exasol query rows", "error", err.Error())
		}
	}()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("failed to get columns: %v", err.Error()))
	}

	// Get column types
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("failed to get column types: %v", err.Error()))
	}

	// Create data frame
	frame := data.NewFrame("response")

	// Create slices for each column
	columnData := make([][]interface{}, len(columns))
	for i := range columnData {
		columnData[i] = make([]interface{}, 0)
	}

	// Read all rows
	for rows.Next() {
		// Create a slice of interface{} to hold each column value
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		// Scan the row
		if err := rows.Scan(valuePtrs...); err != nil {
			return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("failed to scan row: %v", err.Error()))
		}

		// Append values to column data
		for i, val := range values {
			columnData[i] = append(columnData[i], val)
		}
	}

	if err := rows.Err(); err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("row iteration error: %v", err.Error()))
	}

	// Identify field types for Wide format transformation
	timeColIndex := -1
	stringColIndices := []int{}
	numericColIndices := []int{}

	fields := make([]*data.Field, len(columns))
	for i, colName := range columns {
		dbType := columnTypes[i].DatabaseTypeName()

		// Debug: Log column classification
		if len(columnData[i]) > 0 {
			sampleSize := 3
			if len(columnData[i]) < sampleSize {
				sampleSize = len(columnData[i])
			}
			log.DefaultLogger.Debug("Column data sample",
				"column", colName,
				"dbType", dbType,
				"sampleValues", columnData[i][:sampleSize])
		}

		field := convertToTypedField(colName, columnData[i], columnTypes[i], qm.Format)
		fields[i] = field

		// Classify columns for time series transformation based on converted field types.
		// This makes CAST/view-projected columns work even if DB type names vary.
		switch field.Type() {
		case data.FieldTypeNullableTime, data.FieldTypeTime:
			if timeColIndex == -1 {
				timeColIndex = i
			}
		case data.FieldTypeNullableString, data.FieldTypeString:
			stringColIndices = append(stringColIndices, i)
		case data.FieldTypeNullableFloat64, data.FieldTypeFloat64,
			data.FieldTypeNullableInt64, data.FieldTypeInt64,
			data.FieldTypeNullableBool, data.FieldTypeBool:
			numericColIndices = append(numericColIndices, i)
		}
	}

		// PostgreSQL-style behavior:
		// - Table format: return raw typed columns without time-series shaping.
		// - Time series format: require time + numeric columns and return wide frame.
		if qm.Format == queryFormatTable {
			frame.Fields = append(frame.Fields, fields...)
			response.Frames = append(response.Frames, frame)
			log.DefaultLogger.Debug("Frame configured as table format", "fields", len(frame.Fields))
			return response
		}

	// No rows in range should produce empty data instead of type-shape errors.
	// This is common for alerting windows and should map to NoData behavior upstream.
	if hasNoRows(columnData) {
		response.Frames = append(response.Frames, data.NewFrame("response"))
		return response
	}

	if timeColIndex < 0 || len(numericColIndices) == 0 {
		detected := make([]string, 0, len(fields))
		for _, f := range fields {
			detected = append(detected, fmt.Sprintf("%s:%s", f.Name, f.Type()))
		}
		return backend.ErrDataResponse(
			backend.StatusBadRequest,
			fmt.Sprintf("time series format requires at least one time column and one numeric column (detected: %s)", strings.Join(detected, ", ")),
		)
	}
	// Transform to Wide format for time series.
	if timeColIndex >= 0 && len(numericColIndices) > 0 && len(columnData[0]) > 0 {
		wideFrame := transformToWideFormat(columns, fields, columnData, timeColIndex, stringColIndices, numericColIndices)
		response.Frames = append(response.Frames, wideFrame)
		log.DefaultLogger.Debug("Frame configured as Wide format time series",
			"timeColumn", columns[timeColIndex],
			"labelColumns", len(stringColIndices),
			"valueColumns", len(numericColIndices),
			"rows", len(columnData[0]))
	}

	return response
}

// transformToWideFormat pivots data into Wide format with one time column + value columns with labels
func transformToWideFormat(columns []string, fields []*data.Field, columnData [][]interface{},
	timeColIndex int, stringColIndices []int, numericColIndices []int) *data.Frame {

	frame := data.NewFrame("response")
	numRows := len(columnData[0])

	// Map to track unique time points and their indices
	type timeKey int64
	timeMap := make(map[timeKey]int)
	timeValues := []*time.Time{}

	// Map to track unique series (by label combination)
	type seriesKey string
	type seriesData struct {
		labels data.Labels
		values map[int]map[int]interface{} // [valueColIdx][timeIdx] = value
	}
	seriesMap := make(map[seriesKey]*seriesData)

	// Process each row to build the wide structure
	for rowIdx := 0; rowIdx < numRows; rowIdx++ {
		// Extract time value
		timeVal := columnData[timeColIndex][rowIdx]
		timePtr, _ := parseExasolTimeValue(timeVal)

		if timePtr == nil {
			continue
		}

		// Get or create time index
		tKey := timeKey(timePtr.UnixNano())
		timeIdx, exists := timeMap[tKey]
		if !exists {
			timeIdx = len(timeValues)
			timeValues = append(timeValues, timePtr)
			timeMap[tKey] = timeIdx
		}

		// Build labels from string columns
		labels := data.Labels{}
		labelParts := []string{}
		for _, strIdx := range stringColIndices {
			val := columnData[strIdx][rowIdx]
			strVal := ""
			if val != nil {
				if parsed, ok := formatStringValue(val); ok {
					strVal = parsed
				} else {
					strVal = fmt.Sprintf("%v", val)
				}
			}
			labels[columns[strIdx]] = strVal
			labelParts = append(labelParts, strVal)
		}

		// Create series key
		sKey := seriesKey(fmt.Sprintf("%v", labelParts))

		// Get or create series
		series, exists := seriesMap[sKey]
		if !exists {
			series = &seriesData{
				labels: labels,
				values: make(map[int]map[int]interface{}),
			}
			seriesMap[sKey] = series
		}

		// Store values for each numeric column
		for valIdx, numIdx := range numericColIndices {
			if series.values[valIdx] == nil {
				series.values[valIdx] = make(map[int]interface{})
			}
			series.values[valIdx][timeIdx] = columnData[numIdx][rowIdx]
		}
	}

	// Build the wide frame
	// 1. Add time field
	timeField := data.NewField(columns[timeColIndex], nil, timeValues)
	frame.Fields = append(frame.Fields, timeField)

	// 2. Add value fields for each series
	for _, series := range seriesMap {
		for valIdx, numIdx := range numericColIndices {
			// Create value array aligned with time array
			valueArray := make([]interface{}, len(timeValues))
			for timeIdx := range timeValues {
				if val, ok := series.values[valIdx][timeIdx]; ok {
					valueArray[timeIdx] = val
				}
			}

			// Convert to typed field using the original field's type
			valueField := convertTypedFieldByType(columns[numIdx], valueArray, fields[numIdx].Type())

			// Attach labels
			if len(series.labels) > 0 {
				valueField.Labels = series.labels
			}

			frame.Fields = append(frame.Fields, valueField)
		}
	}

	// Set as Wide time series
	frame.SetMeta(&data.FrameMeta{
		Type: data.FrameTypeTimeSeriesWide,
	})

	return frame
}

// convertTypedFieldByType creates a field from existing field type (for Wide format value columns)
func convertTypedFieldByType(name string, values []interface{}, fieldType data.FieldType) *data.Field {
	switch fieldType {
	case data.FieldTypeNullableFloat64, data.FieldTypeFloat64:
		return convertToFloatField(name, values)

	case data.FieldTypeNullableInt64, data.FieldTypeInt64:
		return convertToIntField(name, values)

	case data.FieldTypeNullableBool, data.FieldTypeBool:
		return convertToBoolField(name, values)

	case data.FieldTypeNullableTime, data.FieldTypeTime:
		return convertToTimeField(name, values)

	default:
		return convertToStringField(name, values)
	}
}

// Helper functions for type conversion

func convertToTypedNilField(name string, rowCount int, dbTypeName string, decimalPrecision int64, decimalScale int64, hasDecimalMeta bool, format string) *data.Field {
	normalizedDBType := strings.ToUpper(strings.TrimSpace(dbTypeName))

	switch {
	case strings.HasPrefix(normalizedDBType, "TIMESTAMP") || normalizedDBType == "DATE":
		return data.NewField(name, nil, make([]*time.Time, rowCount))

	case strings.HasPrefix(normalizedDBType, "DECIMAL"):
		if shouldPreserveDecimalAsString(decimalPrecision, decimalScale, hasDecimalMeta, format) {
			return data.NewField(name, nil, make([]*string, rowCount))
		}
		if hasDecimalMeta && decimalScale == 0 {
			return data.NewField(name, nil, make([]*int64, rowCount))
		}
		return data.NewField(name, nil, make([]*float64, rowCount))

	case strings.HasPrefix(normalizedDBType, "DOUBLE") ||
		strings.HasPrefix(normalizedDBType, "FLOAT") ||
		strings.HasPrefix(normalizedDBType, "REAL") ||
		strings.HasPrefix(normalizedDBType, "NUMBER"):
		return data.NewField(name, nil, make([]*float64, rowCount))

	case strings.HasPrefix(normalizedDBType, "BOOLEAN") || normalizedDBType == "BOOL":
		return data.NewField(name, nil, make([]*bool, rowCount))

	default:
		return data.NewField(name, nil, make([]*string, rowCount))
	}
}

func convertToTimeField(name string, values []interface{}) *data.Field {
	// Driver usually returns TIMESTAMP/DATE as string, but database/sql can also surface []byte/time.Time.
	timeValues := make([]*time.Time, len(values))
	for i, val := range values {
		if val != nil {
			if t, ok := parseExasolTimeValue(val); ok {
				timeValues[i] = t
			}
		}
	}
	return data.NewField(name, nil, timeValues)
}

func parseExasolTimeString(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}

	layouts := []string{
		"2006-01-02 15:04:05.999999999 -07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05 -07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05.999",
		"2006-01-02 15:04:05",
		"2006-01-02",
		time.RFC3339Nano,
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func parseExasolTimeValue(value interface{}) (*time.Time, bool) {
	switch v := value.(type) {
	case time.Time:
		t := v
		return &t, true
	case *time.Time:
		if v != nil {
			return v, true
		}
	case *string:
		if v != nil {
			if t, ok := parseExasolTimeString(*v); ok {
				return &t, true
			}
		}
	case string:
		if t, ok := parseExasolTimeString(v); ok {
			return &t, true
		}
	case sql.RawBytes:
		if t, ok := parseExasolTimeString(string(v)); ok {
			return &t, true
		}
	case []byte:
		if t, ok := parseExasolTimeString(string(v)); ok {
			return &t, true
		}
	default:
		if str, ok := parseTextValue(v); ok {
			if t, ok := parseExasolTimeString(str); ok {
				return &t, true
			}
		}
	}
	return nil, false
}

func convertToNumericField(name string, values []interface{}) *data.Field {
	// Driver returns DECIMAL as int64 (scale=0) or float64 (scale>0)
	// Check first non-nil value to determine type
	for _, val := range values {
		if val != nil {
			if _, ok := val.(int64); ok {
				return convertToIntField(name, values)
			}
			if _, ok := val.(float64); ok {
				return convertToFloatField(name, values)
			}
			break
		}
	}
	// Default to float if can't determine
	return convertToFloatField(name, values)
}

func convertToDecimalField(name string, values []interface{}, decimalPrecision int64, decimalScale int64, hasDecimalMeta bool, format string) *data.Field {
	if shouldPreserveDecimalAsString(decimalPrecision, decimalScale, hasDecimalMeta, format) {
		return convertToStringField(name, values)
	}
	if hasDecimalMeta && decimalScale == 0 {
		return convertToIntField(name, values)
	}
	return convertToNumericField(name, values)
}

func shouldPreserveDecimalAsString(decimalPrecision int64, decimalScale int64, hasDecimalMeta bool, format string) bool {
	if !hasDecimalMeta {
		return false
	}
	if format != queryFormatTable {
		return false
	}
	if decimalScale == 0 {
		return decimalPrecision > 18
	}
	return decimalPrecision > 15
}

func convertToFloatField(name string, values []interface{}) *data.Field {
	// Driver returns DOUBLE as float64, DECIMAL (with scale>0) as float64
	float64Values := make([]*float64, len(values))
	for i, val := range values {
		if val != nil {
			if f64, ok := parseFloatValue(val); ok {
				float64Values[i] = &f64
			}
		}
	}
	return data.NewField(name, nil, float64Values)
}

func convertToIntField(name string, values []interface{}) *data.Field {
	// Driver returns DECIMAL (scale=0) as int64
	int64Values := make([]*int64, len(values))
	for i, val := range values {
		if val != nil {
			if i64, ok := parseIntValue(val); ok {
				int64Values[i] = &i64
			}
		}
	}
	return data.NewField(name, nil, int64Values)
}

func convertToBoolField(name string, values []interface{}) *data.Field {
	// Driver usually returns BOOLEAN as bool, but tolerate textual and numeric bool-like values.
	boolValues := make([]*bool, len(values))
	for i, val := range values {
		if val != nil {
			if v, ok := parseBoolValue(val); ok {
				boolValues[i] = &v
			}
		}
	}
	return data.NewField(name, nil, boolValues)
}

func convertToStringField(name string, values []interface{}) *data.Field {
	// Driver can return string or []byte depending on scan path.
	stringValues := make([]*string, len(values))
	for i, val := range values {
		if val != nil {
			if str, ok := formatStringValue(val); ok {
				stringValues[i] = &str
			}
		}
	}
	return data.NewField(name, nil, stringValues)
}

func parseTextValue(value interface{}) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case *string:
		if v != nil {
			return *v, true
		}
		return "", false
	case sql.RawBytes:
		return string(v), true
	case []byte:
		return string(v), true
	default:
		return "", false
	}
}

func formatStringValue(value interface{}) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case *string:
		if v != nil {
			return *v, true
		}
		return "", false
	case sql.RawBytes:
		return string(v), true
	case []byte:
		return string(v), true
	case time.Time:
		return v.Format(time.RFC3339Nano), true
	case *time.Time:
		if v != nil {
			return v.Format(time.RFC3339Nano), true
		}
		return "", false
	case bool:
		return strconv.FormatBool(v), true
	case int:
		return strconv.FormatInt(int64(v), 10), true
	case int8:
		return strconv.FormatInt(int64(v), 10), true
	case int16:
		return strconv.FormatInt(int64(v), 10), true
	case int32:
		return strconv.FormatInt(int64(v), 10), true
	case int64:
		return strconv.FormatInt(v, 10), true
	case uint:
		return strconv.FormatUint(uint64(v), 10), true
	case uint8:
		return strconv.FormatUint(uint64(v), 10), true
	case uint16:
		return strconv.FormatUint(uint64(v), 10), true
	case uint32:
		return strconv.FormatUint(uint64(v), 10), true
	case uint64:
		return strconv.FormatUint(v, 10), true
	case float32:
		return strconv.FormatFloat(float64(v), 'g', -1, 32), true
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64), true
	case fmt.Stringer:
		return v.String(), true
	default:
		return "", false
	}
}

func parseIntLike(value string) (int64, bool) {
	v := strings.TrimSpace(value)
	if v == "" {
		return 0, false
	}
	if i64, err := strconv.ParseInt(v, 10, 64); err == nil {
		return i64, true
	}
	if f64, ok := parseFloatLike(v); ok {
		i64 := int64(f64)
		return i64, true
	}
	return 0, false
}

func parseFloatLike(value string) (float64, bool) {
	v := strings.TrimSpace(value)
	if v == "" {
		return 0, false
	}
	if f64, err := strconv.ParseFloat(v, 64); err == nil {
		return f64, true
	}
	if strings.Contains(v, ",") {
		// If there is no dot, treat comma as decimal separator first (e.g. 1,23).
		if !strings.Contains(v, ".") {
			normalized := strings.ReplaceAll(v, ",", ".")
			if f64, err := strconv.ParseFloat(normalized, 64); err == nil {
				return f64, true
			}
		}
		// Then try commas as thousands separators (e.g. 1,234.56).
		normalized := strings.ReplaceAll(v, ",", "")
		if f64, err := strconv.ParseFloat(normalized, 64); err == nil {
			return f64, true
		}
	}
	return 0, false
}

func parseBoolValue(value interface{}) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case int64:
		return v != 0, true
	case float64:
		return v != 0, true
	case string:
		return parseBoolString(v)
	case *string:
		if v != nil {
			return parseBoolString(*v)
		}
		return false, false
	case sql.RawBytes:
		return parseBoolString(string(v))
	case []byte:
		return parseBoolString(string(v))
	default:
		if str, ok := parseTextValue(v); ok {
			return parseBoolString(str)
		}
		return false, false
	}
}

func parseFloatValue(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case int:
		return float64(v), true
	default:
		if str, ok := parseTextValue(v); ok {
			return parseFloatLike(str)
		}
		return 0, false
	}
}

func parseIntValue(value interface{}) (int64, bool) {
	switch v := value.(type) {
	case int64:
		return v, true
	case int32:
		return int64(v), true
	case int:
		return int64(v), true
	case float64:
		return int64(v), true
	case float32:
		return int64(v), true
	default:
		if str, ok := parseTextValue(v); ok {
			return parseIntLike(str)
		}
		return 0, false
	}
}

func parseBoolString(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "t", "yes", "y":
		return true, true
	case "0", "false", "f", "no", "n":
		return false, true
	default:
		return false, false
	}
}

// convertToTypedField converts a column of interface{} values to a typed data.Field
func convertToTypedField(name string, values []interface{}, colType *sql.ColumnType, format string) *data.Field {
	dbTypeName := ""
	var decimalPrecision int64
	var decimalScale int64
	hasDecimalMeta := false
	if colType != nil {
		dbTypeName = colType.DatabaseTypeName()
		if precision, scale, ok := colType.DecimalSize(); ok {
			decimalPrecision = int64(precision)
			decimalScale = int64(scale)
			hasDecimalMeta = true
		}
	}

	return buildTypedFieldFromMetadata(name, values, dbTypeName, decimalPrecision, decimalScale, hasDecimalMeta, format)
}

func buildTypedFieldFromMetadata(name string, values []interface{}, dbTypeName string, decimalPrecision int64, decimalScale int64, hasDecimalMeta bool, format string) *data.Field {
	if len(values) == 0 {
		return convertToTypedNilField(name, 0, dbTypeName, decimalPrecision, decimalScale, hasDecimalMeta, format)
	}

	hasNonNil := false
	for _, v := range values {
		if v != nil {
			hasNonNil = true
			break
		}
	}
	if !hasNonNil {
		return convertToTypedNilField(name, len(values), dbTypeName, decimalPrecision, decimalScale, hasDecimalMeta, format)
	}

	normalizedDBType := strings.ToUpper(strings.TrimSpace(dbTypeName))

	switch {
	case strings.HasPrefix(normalizedDBType, "TIMESTAMP") || normalizedDBType == "DATE":
		return convertToTimeField(name, values)

	case strings.HasPrefix(normalizedDBType, "DECIMAL"):
		return convertToDecimalField(name, values, decimalPrecision, decimalScale, hasDecimalMeta, format)

	case strings.HasPrefix(normalizedDBType, "DOUBLE") ||
		strings.HasPrefix(normalizedDBType, "FLOAT") ||
		strings.HasPrefix(normalizedDBType, "REAL") ||
		strings.HasPrefix(normalizedDBType, "NUMBER"):
		return convertToFloatField(name, values)

	case strings.Contains(normalizedDBType, "CHAR"):
		return convertToStringField(name, values)

	case strings.HasPrefix(normalizedDBType, "BOOLEAN") || normalizedDBType == "BOOL":
		return convertToBoolField(name, values)

	case strings.HasPrefix(normalizedDBType, "INTERVAL") ||
		strings.HasPrefix(normalizedDBType, "GEOMETRY") ||
		strings.HasPrefix(normalizedDBType, "HASHTYPE"):
		return convertToStringField(name, values)
	}
	log.DefaultLogger.Debug("Unknown database type, preserving as string",
		"column", name,
		"dbType", dbTypeName)
	return convertToStringField(name, values)
}

func hasNoRows(columnData [][]interface{}) bool {
	return len(columnData) == 0 || len(columnData[0]) == 0
}

// CheckHealth handles health checks sent from Grafana to the plugin.
// The main use case for these health checks is the test button on the
// datasource configuration page which allows users to verify that
// a datasource is working as expected.
func (d *Datasource) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	res := &backend.CheckHealthResult{}

	// Try to ping the database
	if err := d.db.PingContext(ctx); err != nil {
		res.Status = backend.HealthStatusError
		res.Message = fmt.Sprintf("Failed to connect to Exasol database: %v", err)
		return res, nil
	}

	// Try a simple query
	var result int
	err := d.db.QueryRowContext(ctx, "SELECT 1 FROM DUAL").Scan(&result)
	if err != nil {
		res.Status = backend.HealthStatusError
		res.Message = fmt.Sprintf("Database query failed: %v", err)
		return res, nil
	}

	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Successfully connected to Exasol database",
	}, nil
}
