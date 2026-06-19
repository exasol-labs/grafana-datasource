# Changelog

## 1.0.0 (Unreleased)

Initial release.

### Features

- Backend SQL execution against Exasol via the official `exasol-driver-go`
- Grafana SQL macros (PostgreSQL-style subset):
  - `$__time(column)`, `$__timeFilter(column)`
  - `$__timeFrom()`, `$__timeTo()`
  - `$__timeGroup(column, interval[, fill])`
  - `$__timeGroupAlias(column, interval[, fill])`
- Query output formats:
  - `Table` (raw typed columns)
  - `Time series` (Grafana Wide frame, auto-pivoted by label columns)
- Configurable TLS:
  - Optional skip-verify toggle for test environments
  - Optional server certificate fingerprint pinning for self-signed clusters
- Configurable connection pool (max open / idle / lifetime) and per-query timeout
- Health check via lightweight `SELECT 1` probe
- Compatible with Grafana >= 10.4.0
