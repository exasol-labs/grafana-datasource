package plugin

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

func TestQueryData(t *testing.T) {
	ds := Datasource{}

	resp, err := ds.QueryData(
		context.Background(),
		&backend.QueryDataRequest{
			Queries: []backend.DataQuery{
				{RefID: "A"},
			},
		},
	)
	if err != nil {
		t.Error(err)
	}

	if len(resp.Responses) != 1 {
		t.Fatal("QueryData must return a response")
	}
}

func TestParseExasolTimeString(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "timestamp with microseconds", input: "2024-06-18 17:22:13.123000"},
		{name: "timestamp without fractional", input: "2024-06-18 17:22:13"},
		{name: "date", input: "2024-06-18"},
		{name: "rfc3339", input: "2024-06-18T17:22:13Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, ok := parseExasolTimeString(tt.input)
			if !ok {
				t.Fatalf("expected %q to parse", tt.input)
			}
			if parsed.IsZero() {
				t.Fatalf("expected parsed time for %q to be non-zero", tt.input)
			}
		})
	}
}

func TestParseExasolTimeStringInvalid(t *testing.T) {
	if parsed, ok := parseExasolTimeString("not-a-time"); ok || !parsed.Equal(time.Time{}) {
		t.Fatal("expected invalid time parse to fail")
	}
}

func TestParseExasolTimeStringExtendedFormats(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "timestamp with nanoseconds", input: "2024-06-18 17:22:13.123456789"},
		{name: "timestamp with offset and space", input: "2024-06-18 17:22:13.123456 +00:00"},
		{name: "timestamp with offset compact", input: "2024-06-18 17:22:13+02:00"},
		{name: "rfc3339 nano", input: "2024-06-18T17:22:13.123456789Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, ok := parseExasolTimeString(tt.input)
			if !ok || parsed.IsZero() {
				t.Fatalf("expected %q to parse in extended format", tt.input)
			}
		})
	}
}

func TestParseExasolTimeValueVariants(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	testCases := []struct {
		name  string
		value interface{}
	}{
		{name: "time", value: now},
		{name: "time pointer", value: &now},
		{name: "string", value: "2024-06-18 17:22:13.123000"},
		{name: "bytes", value: []byte("2024-06-18 17:22:13.123000")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parsed, ok := parseExasolTimeValue(tc.value)
			if !ok || parsed == nil || parsed.IsZero() {
				t.Fatalf("expected value %#v to parse as time", tc.value)
			}
		})
	}
}

func TestConvertToStringFieldHandlesBytes(t *testing.T) {
	field := convertToStringField("s", []interface{}{[]byte("abc"), "def"})
	if field.Type() != data.FieldTypeNullableString {
		t.Fatalf("expected nullable string field, got %s", field.Type())
	}
	v0, ok0 := field.ConcreteAt(0)
	v1, ok1 := field.ConcreteAt(1)
	if !ok0 || v0.(string) != "abc" {
		t.Fatalf("expected first value to be 'abc', got %#v", v0)
	}
	if !ok1 || v1.(string) != "def" {
		t.Fatalf("expected second value to be 'def', got %#v", v1)
	}
}

func TestConvertToBoolFieldCoercions(t *testing.T) {
	field := convertToBoolField("b", []interface{}{true, "false", []byte("1"), int64(0), float64(2)})
	if field.Type() != data.FieldTypeNullableBool {
		t.Fatalf("expected nullable bool field, got %s", field.Type())
	}

	expected := []bool{true, false, true, false, true}
	for i, exp := range expected {
		v, ok := field.ConcreteAt(i)
		if !ok || v.(bool) != exp {
			t.Fatalf("unexpected bool at index %d: got %#v want %v", i, v, exp)
		}
	}
}

func TestNumericParsersFromStrings(t *testing.T) {
	floatField := convertToFloatField("f", []interface{}{"3.14", []byte("2.5")})
	if floatField.Type() != data.FieldTypeNullableFloat64 {
		t.Fatalf("expected nullable float field, got %s", floatField.Type())
	}
	f0, ok0 := floatField.ConcreteAt(0)
	f1, ok1 := floatField.ConcreteAt(1)
	if !ok0 || f0.(float64) != 3.14 {
		t.Fatalf("expected 3.14, got %#v", f0)
	}
	if !ok1 || f1.(float64) != 2.5 {
		t.Fatalf("expected 2.5, got %#v", f1)
	}

	intField := convertToIntField("i", []interface{}{"42", []byte("7.0")})
	if intField.Type() != data.FieldTypeNullableInt64 {
		t.Fatalf("expected nullable int field, got %s", intField.Type())
	}
	i0, ok0 := intField.ConcreteAt(0)
	i1, ok1 := intField.ConcreteAt(1)
	if !ok0 || i0.(int64) != 42 {
		t.Fatalf("expected 42, got %#v", i0)
	}
	if !ok1 || i1.(int64) != 7 {
		t.Fatalf("expected 7, got %#v", i1)
	}
}

func TestParseFloatLike(t *testing.T) {
	cases := map[string]float64{
		"3.14":     3.14,
		"1,234.56": 1234.56,
		"1,23":     1.23,
	}
	for input, expected := range cases {
		got, ok := parseFloatLike(input)
		if !ok || got != expected {
			t.Fatalf("expected %q => %v, got %v (ok=%v)", input, expected, got, ok)
		}
	}
}

func TestFormatStringValueVariants(t *testing.T) {
	s := "abc"
	raw := sql.RawBytes("raw")
	now := time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)

	cases := []interface{}{s, &s, []byte("bytes"), raw, now, &now}
	for _, c := range cases {
		got, ok := formatStringValue(c)
		if !ok || got == "" {
			t.Fatalf("expected formatStringValue to format %#v", c)
		}
	}
}

func TestParseTextValueIsStrictlyTextLike(t *testing.T) {
	s := "abc"
	raw := sql.RawBytes("raw")

	for _, c := range []interface{}{s, &s, []byte("bytes"), raw} {
		got, ok := parseTextValue(c)
		if !ok || got == "" {
			t.Fatalf("expected parseTextValue to parse %#v", c)
		}
	}

	for _, c := range []interface{}{time.Now(), true, int64(7), float64(3.5)} {
		if got, ok := parseTextValue(c); ok || got != "" {
			t.Fatalf("expected parseTextValue to reject non-text value %#v", c)
		}
	}
}

func TestNumericParsersWithPointerStrings(t *testing.T) {
	s1 := "42"
	s2 := "3.5"
	if i, ok := parseIntValue(&s1); !ok || i != 42 {
		t.Fatalf("expected int parser to parse pointer string 42, got %v (ok=%v)", i, ok)
	}
	if f, ok := parseFloatValue(&s2); !ok || f != 3.5 {
		t.Fatalf("expected float parser to parse pointer string 3.5, got %v (ok=%v)", f, ok)
	}
	if _, ok := parseFloatValue(time.Now()); ok {
		t.Fatal("expected float parser to reject time values")
	}
	if _, ok := parseIntValue(time.Now()); ok {
		t.Fatal("expected int parser to reject time values")
	}
}

func TestHasNoRows(t *testing.T) {
	if !hasNoRows([][]interface{}{}) {
		t.Fatal("expected empty column data to be treated as no rows")
	}
	if !hasNoRows([][]interface{}{{}}) {
		t.Fatal("expected empty first column to be treated as no rows")
	}
	if hasNoRows([][]interface{}{{1}, {"x"}}) {
		t.Fatal("expected non-empty column data to have rows")
	}
}

func TestConvertTypedFieldByTypeUsesSharedParsers(t *testing.T) {
	floatField := convertTypedFieldByType("f", []interface{}{"3.14", []byte("2.5")}, data.FieldTypeNullableFloat64)
	if floatField.Type() != data.FieldTypeNullableFloat64 {
		t.Fatalf("expected float field type, got %s", floatField.Type())
	}
	if v, ok := floatField.ConcreteAt(0); !ok || v.(float64) != 3.14 {
		t.Fatalf("expected float[0]=3.14, got %#v", v)
	}
	if v, ok := floatField.ConcreteAt(1); !ok || v.(float64) != 2.5 {
		t.Fatalf("expected float[1]=2.5, got %#v", v)
	}

	intField := convertTypedFieldByType("i", []interface{}{"42", []byte("7.0")}, data.FieldTypeNullableInt64)
	if intField.Type() != data.FieldTypeNullableInt64 {
		t.Fatalf("expected int field type, got %s", intField.Type())
	}
	if v, ok := intField.ConcreteAt(0); !ok || v.(int64) != 42 {
		t.Fatalf("expected int[0]=42, got %#v", v)
	}
	if v, ok := intField.ConcreteAt(1); !ok || v.(int64) != 7 {
		t.Fatalf("expected int[1]=7, got %#v", v)
	}

	boolField := convertTypedFieldByType("b", []interface{}{"true", []byte("0"), int64(1)}, data.FieldTypeNullableBool)
	if boolField.Type() != data.FieldTypeNullableBool {
		t.Fatalf("expected bool field type, got %s", boolField.Type())
	}
	if v, ok := boolField.ConcreteAt(0); !ok || v.(bool) != true {
		t.Fatalf("expected bool[0]=true, got %#v", v)
	}
	if v, ok := boolField.ConcreteAt(1); !ok || v.(bool) != false {
		t.Fatalf("expected bool[1]=false, got %#v", v)
	}
	if v, ok := boolField.ConcreteAt(2); !ok || v.(bool) != true {
		t.Fatalf("expected bool[2]=true, got %#v", v)
	}
}

func TestTransformToWideFormatKeepsNanosecondTimestamps(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 123000000, time.UTC)
	t1 := t0.Add(time.Nanosecond)

	columns := []string{"time", "value"}
	columnData := [][]interface{}{
		{t0, t1},
		{float64(1), float64(2)},
	}
	fields := []*data.Field{
		convertToTimeField(columns[0], columnData[0]),
		convertToFloatField(columns[1], columnData[1]),
	}

	frame := transformToWideFormat(columns, fields, columnData, 0, []int{}, []int{1})
	if len(frame.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(frame.Fields))
	}
	if frame.Fields[0].Len() != 2 {
		t.Fatalf("expected 2 distinct timestamps, got %d", frame.Fields[0].Len())
	}
}

func TestTransformToWideFormatParsesLabelBytes(t *testing.T) {
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	columns := []string{"time", "cluster_name", "metric"}
	columnData := [][]interface{}{
		{ts},
		{[]byte("cluster-a")},
		{float64(10)},
	}
	fields := []*data.Field{
		convertToTimeField(columns[0], columnData[0]),
		convertToStringField(columns[1], columnData[1]),
		convertToFloatField(columns[2], columnData[2]),
	}

	frame := transformToWideFormat(columns, fields, columnData, 0, []int{1}, []int{2})
	if len(frame.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(frame.Fields))
	}
	labels := frame.Fields[1].Labels
	if labels == nil {
		t.Fatal("expected labels on value field")
	}
	if got := labels["cluster_name"]; got != "cluster-a" {
		t.Fatalf("expected parsed byte label 'cluster-a', got %q", got)
	}
}

func TestConvertToTypedNilFieldByDBType(t *testing.T) {
	tests := []struct {
		name             string
		dbType           string
		decimalPrecision int64
		decimalScale     int64
		hasDecimalMeta   bool
		expectedType     data.FieldType
	}{
		{name: "timestamp", dbType: "TIMESTAMP", expectedType: data.FieldTypeNullableTime},
		{name: "date", dbType: "DATE", expectedType: data.FieldTypeNullableTime},
		{name: "decimal int fits int64", dbType: "DECIMAL", decimalPrecision: 9, decimalScale: 0, hasDecimalMeta: true, expectedType: data.FieldTypeNullableInt64},
		{name: "decimal int at int64 boundary", dbType: "DECIMAL", decimalPrecision: 18, decimalScale: 0, hasDecimalMeta: true, expectedType: data.FieldTypeNullableInt64},
		{name: "decimal int beyond int64 falls back to float", dbType: "DECIMAL", decimalPrecision: 19, decimalScale: 0, hasDecimalMeta: true, expectedType: data.FieldTypeNullableFloat64},
		{name: "decimal with scale becomes float", dbType: "DECIMAL", decimalPrecision: 9, decimalScale: 2, hasDecimalMeta: true, expectedType: data.FieldTypeNullableFloat64},
		{name: "decimal high precision stays float", dbType: "DECIMAL", decimalPrecision: 30, decimalScale: 10, hasDecimalMeta: true, expectedType: data.FieldTypeNullableFloat64},
		{name: "double", dbType: "DOUBLE", expectedType: data.FieldTypeNullableFloat64},
		{name: "boolean", dbType: "BOOLEAN", expectedType: data.FieldTypeNullableBool},
		{name: "varchar", dbType: "VARCHAR", expectedType: data.FieldTypeNullableString},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := convertToTypedNilField("c", 3, tt.dbType, tt.decimalPrecision, tt.decimalScale, tt.hasDecimalMeta)
			if field.Type() != tt.expectedType {
				t.Fatalf("expected field type %s, got %s", tt.expectedType, field.Type())
			}
			if field.Len() != 3 {
				t.Fatalf("expected field length 3, got %d", field.Len())
			}
		})
	}
}

func TestBuildTypedFieldFromMetadataKeepsVarcharAsString(t *testing.T) {
	values := []interface{}{"00123", "false", "2024-06-18"}
	field := buildTypedFieldFromMetadata("code", values, "VARCHAR", 0, 0, false)
	if field.Type() != data.FieldTypeNullableString {
		t.Fatalf("expected varchar to stay string, got %s", field.Type())
	}
	if v, ok := field.ConcreteAt(0); !ok || v.(string) != "00123" {
		t.Fatalf("expected first varchar value to remain '00123', got %#v", v)
	}
}

func TestBuildTypedFieldFromMetadataUnknownTypeStaysString(t *testing.T) {
	values := []interface{}{"1.9", "2.5"}
	field := buildTypedFieldFromMetadata("metric", values, "UNKNOWN_TYPE", 0, 0, false)
	if field.Type() != data.FieldTypeNullableString {
		t.Fatalf("expected unknown type to stay string, got %s", field.Type())
	}
	if v, ok := field.ConcreteAt(0); !ok || v.(string) != "1.9" {
		t.Fatalf("expected first value to stay '1.9', got %#v", v)
	}
}

func TestBuildTypedFieldFromMetadataHighPrecisionDecimalIsFloat(t *testing.T) {
	values := []interface{}{float64(1234567890123456.8)}
	field := buildTypedFieldFromMetadata("amount", values, "DECIMAL", 30, 10, true)
	if field.Type() != data.FieldTypeNullableFloat64 {
		t.Fatalf("expected high precision decimal to be float64, got %s", field.Type())
	}
	if v, ok := field.ConcreteAt(0); !ok || v.(float64) != values[0].(float64) {
		t.Fatalf("expected decimal float value, got %#v", v)
	}
}

func TestBuildTypedFieldFromMetadataDecimalBeyondInt64Range(t *testing.T) {
	values := []interface{}{float64(1e20)}
	field := buildTypedFieldFromMetadata("big", values, "DECIMAL", 20, 0, true)
	if field.Type() != data.FieldTypeNullableFloat64 {
		t.Fatalf("expected DECIMAL(20,0) to fall back to float64, got %s", field.Type())
	}
}
