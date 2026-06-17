package plugin

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/gtime"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
)

const (
	exasolEpochAnchor      = "TIMESTAMP '1970-01-01 00:00:00'"
	defaultIntervalSeconds = 1
)

type exasolInterval struct {
	raw   string
	value int64
	unit  string
}

var exasolMacros = sqlutil.Macros{
	"time":            exasolMacro("time"),
	"timeFilter":      exasolMacro("timeFilter"),
	"timeFrom":        exasolMacro("timeFrom"),
	"timeTo":          exasolMacro("timeTo"),
	"timeGroup":       exasolMacro("timeGroup"),
	"timeGroupAlias":  exasolMacro("timeGroupAlias"),
	"interval":        exasolMacro("interval"),
	"interval_ms":     exasolMacro("interval_ms"),
	"unixEpochFilter": exasolMacro("unixEpochFilter"),
	"unixEpochGroup":  exasolMacro("unixEpochGroup"),
}

func interpolateQuery(rawSQL string, query backend.DataQuery) (string, error) {
	sqlQuery := &sqlutil.Query{
		RawSQL:        rawSQL,
		RefID:         query.RefID,
		Interval:      query.Interval,
		TimeRange:     query.TimeRange,
		MaxDataPoints: query.MaxDataPoints,
	}
	return sqlutil.Interpolate(sqlQuery, exasolMacros)
}

func exasolMacro(name string) sqlutil.MacroFunc {
	return func(query *sqlutil.Query, args []string) (string, error) {
		return evaluateExasolMacro(query, name, args)
	}
}

func evaluateExasolMacro(query *sqlutil.Query, name string, args []string) (string, error) {
	fromExpr := exasolEpochMsToTimestampExpr(query.TimeRange.From.UnixMilli())
	toExpr := exasolEpochMsToTimestampExpr(query.TimeRange.To.UnixMilli())

	switch name {
	case "time":
		columnExpr, err := macroTimeColumnArg(args, name)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s AS \"time\"", columnExpr), nil
	case "timeFilter":
		columnExpr, err := macroTimeColumnArg(args, name)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s >= %s AND %s <= %s", columnExpr, fromExpr, columnExpr, toExpr), nil
	case "timeFrom":
		return macroExasolBoundary(args, fromExpr, ">=", name)
	case "timeTo":
		return macroExasolBoundary(args, toExpr, "<=", name)
	case "timeGroup":
		return macroExasolTimeGroupExpr(args)
	case "timeGroupAlias":
		groupExpr, err := macroExasolTimeGroupExpr(args)
		if err != nil {
			return "", err
		}
		return groupExpr + ` AS "time"`, nil
	case "interval":
		return macroExasolInterval(query), nil
	case "interval_ms":
		return macroExasolIntervalMS(query), nil
	case "unixEpochFilter":
		return macroExasolUnixEpochFilter(query, args)
	case "unixEpochGroup":
		return macroExasolUnixEpochGroup(args)
	default:
		return "", fmt.Errorf("unknown macro %q", name)
	}
}

// macroExasolInterval renders the dashboard/alert interval as a human string
// (e.g. "5m", "1h"). Falls back to "1s" when the SDK passes a zero interval
// (common for alert evaluation without a dashboard context).
func macroExasolInterval(query *sqlutil.Query) string {
	if query.Interval <= 0 {
		return fmt.Sprintf("%ds", defaultIntervalSeconds)
	}
	return gtime.FormatInterval(query.Interval)
}

// macroExasolIntervalMS renders the dashboard/alert interval as integer
// milliseconds. Falls back to 1000 (1 second) on a zero interval.
func macroExasolIntervalMS(query *sqlutil.Query) string {
	if query.Interval <= 0 {
		return strconv.FormatInt(int64(time.Second/time.Millisecond), 10)
	}
	return strconv.FormatInt(query.Interval.Milliseconds(), 10)
}

// macroExasolUnixEpochFilter expands `$__unixEpochFilter(col)` to a numeric
// range over Unix epoch seconds. Useful when the column stores epoch as
// DECIMAL/INTEGER rather than a native TIMESTAMP.
func macroExasolUnixEpochFilter(query *sqlutil.Query, args []string) (string, error) {
	columnExpr, err := macroTimeColumnArg(args, "unixEpochFilter")
	if err != nil {
		return "", err
	}
	from := query.TimeRange.From.Unix()
	to := query.TimeRange.To.Unix()
	return fmt.Sprintf("%s >= %d AND %s <= %d", columnExpr, from, columnExpr, to), nil
}

// macroExasolUnixEpochGroup buckets a numeric epoch column by `interval` seconds.
// Equivalent to FLOOR(col / N) * N for the second-resolution bucket size.
func macroExasolUnixEpochGroup(args []string) (string, error) {
	if len(args) != 2 && len(args) != 3 {
		return "", fmt.Errorf("%w: expected 2 or 3 arguments, received %d", sqlutil.ErrorBadArgumentCount, len(args))
	}
	columnExpr, err := macroTimeColumnArg(args[:1], "unixEpochGroup")
	if err != nil {
		return "", err
	}
	interval, err := parseExasolInterval(args[1])
	if err != nil {
		return "", err
	}
	bucketMillis, err := interval.fixedMillis()
	if err != nil {
		return "", err
	}
	bucketSeconds := bucketMillis / 1000
	if bucketSeconds <= 0 {
		return "", fmt.Errorf("unixEpochGroup requires a bucket of at least 1 second, got %q", args[1])
	}
	return fmt.Sprintf("FLOOR(%s / %d) * %d", columnExpr, bucketSeconds, bucketSeconds), nil
}

func macroExasolBoundary(args []string, boundaryExpr string, operator string, macroName string) (string, error) {
	if len(args) == 0 || (len(args) == 1 && strings.TrimSpace(args[0]) == "") {
		return boundaryExpr, nil
	}
	if len(args) == 1 {
		columnExpr, err := macroTimeColumnArg(args, macroName)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s %s %s", columnExpr, operator, boundaryExpr), nil
	}

	return "", fmt.Errorf("%w: expected 0 or 1 argument, received %d", sqlutil.ErrorBadArgumentCount, len(args))
}

func macroExasolTimeGroupExpr(args []string) (string, error) {
	if len(args) != 2 && len(args) != 3 {
		return "", fmt.Errorf("%w: expected 2 or 3 arguments, received %d", sqlutil.ErrorBadArgumentCount, len(args))
	}

	columnExpr, err := macroTimeColumnArg(args[:1], "timeGroup")
	if err != nil {
		return "", err
	}

	return exasolTimeGroupExpr(columnExpr, args[1])
}

func exasolTimeGroupExpr(columnExpr string, rawInterval string) (string, error) {
	interval, err := parseExasolInterval(rawInterval)
	if err != nil {
		return "", err
	}

	if truncUnit, ok := interval.calendarTruncUnit(); ok {
		return fmt.Sprintf("DATE_TRUNC('%s', %s)", truncUnit, columnExpr), nil
	}

	bucketMillis, err := interval.fixedMillis()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"ADD_SECONDS(%s, FLOOR(SECONDS_BETWEEN(%s, %s) * 1000 / %d) * %d / 1000.0)",
		exasolEpochAnchor,
		columnExpr,
		exasolEpochAnchor,
		bucketMillis,
		bucketMillis,
	), nil
}

func macroTimeColumnArg(args []string, macroName string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("%w: expected 1 argument, received %d", sqlutil.ErrorBadArgumentCount, len(args))
	}

	columnExpr := strings.TrimSpace(args[0])
	if columnExpr == "" {
		return "", fmt.Errorf("%w: expected non-empty time column argument for macro %s", sqlutil.ErrorBadArgumentCount, macroName)
	}

	return columnExpr, nil
}

func parseExasolInterval(raw string) (exasolInterval, error) {
	trimmed := strings.Trim(strings.TrimSpace(raw), `"'`)
	if trimmed == "" {
		return exasolInterval{}, fmt.Errorf("unsupported interval %q: expected value like 5m, 1h, 1d, 1w, 1M, or 1y", raw)
	}

	suffixStart := 0
	for suffixStart < len(trimmed) && trimmed[suffixStart] >= '0' && trimmed[suffixStart] <= '9' {
		suffixStart++
	}
	if suffixStart == 0 || suffixStart == len(trimmed) {
		return exasolInterval{}, fmt.Errorf("unsupported interval %q: expected value like 5m, 1h, 1d, 1w, 1M, or 1y", raw)
	}

	value, err := strconv.ParseInt(trimmed[:suffixStart], 10, 64)
	if err != nil || value <= 0 {
		return exasolInterval{}, fmt.Errorf("unsupported interval %q: expected a positive interval", raw)
	}

	unit := trimmed[suffixStart:]
	switch unit {
	case "ms", "s", "m", "h", "d", "w", "M", "y":
		return exasolInterval{raw: trimmed, value: value, unit: unit}, nil
	default:
		return exasolInterval{}, fmt.Errorf("unsupported interval %q: expected value like 5m, 1h, 1d, 1w, 1M, or 1y", raw)
	}
}

func (interval exasolInterval) calendarTruncUnit() (string, bool) {
	switch {
	case interval.value == 1 && interval.unit == "y":
		return "year", true
	case interval.value == 1 && interval.unit == "M":
		return "month", true
	case interval.value == 1 && interval.unit == "w":
		return "week", true
	case interval.value == 1 && interval.unit == "d":
		return "day", true
	case interval.value == 1 && interval.unit == "h":
		return "hour", true
	case interval.value == 1 && interval.unit == "m":
		return "minute", true
	case interval.value == 1 && interval.unit == "s":
		return "second", true
	default:
		return "", false
	}
}

func (interval exasolInterval) fixedMillis() (int64, error) {
	switch interval.unit {
	case "ms":
		return interval.value, nil
	case "s":
		return interval.value * 1000, nil
	case "m":
		return interval.value * 60 * 1000, nil
	case "h":
		return interval.value * 60 * 60 * 1000, nil
	case "d":
		return interval.value * 24 * 60 * 60 * 1000, nil
	case "w":
		return interval.value * 7 * 24 * 60 * 60 * 1000, nil
	case "M", "y":
		return 0, fmt.Errorf("unsupported interval %q: only 1M and 1y are supported for calendar bucketing", interval.raw)
	default:
		return 0, fmt.Errorf("unsupported interval unit %q", interval.unit)
	}
}

func exasolEpochMsToTimestampExpr(epochMs int64) string {
	return fmt.Sprintf("ADD_SECONDS(%s, %d / 1000)", exasolEpochAnchor, epochMs)
}
