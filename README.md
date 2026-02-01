# Exasol Datasource Plugin for Grafana

This plugin lets Grafana query Exasol directly via SQL.

Plugin ID: `exasol-exasol-datasource`

Repository policy files:
- [DISCLAIMER.md](./DISCLAIMER.md)
- [SECURITY.md](./SECURITY.md)
- [LICENSE](./LICENSE)

## Prerequisites

- Node.js `>=22`
- Go `>=1.24`
- Docker and Docker Compose (for local Grafana in containers)

## Build the plugin

From this directory:

```bash
npm install
npm run build
npm run build:backend
npm run package
```

Notes:

- `npm run build` builds frontend assets into `dist/`
- `npm run build:backend` builds backend binaries for all target platforms into `dist/`
- `npm run package` creates a validator-ready archive at `build/exasol-exasol-datasource-<version>.zip`
- If you have `mage` installed, `mage -v` is equivalent

To validate the packaged plugin locally:

```bash
npm run validate:plugin
```

If you already rebuilt the archive and only want to validate the current root ZIP:

```bash
npm run validate:plugin:local
```

## Run Grafana with Docker and load this plugin

Use a `docker-compose.yml` like this:

```yaml
services:
  grafana:
    image: grafana/grafana:latest
    container_name: grafana
    ports:
      - "3000:3000"
    environment:
      - GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=exasol-exasol-datasource
    volumes:
      - /Users/dren.daka/dev/exasol-grafana-second/dist:/var/lib/grafana/plugins/exasol-exasol-datasource
```

Start Grafana:

```bash
docker compose up -d
```

Then open `http://localhost:3000`.

## Install into an already running Grafana container

```bash
docker exec -it <grafana_container> mkdir -p /var/lib/grafana/plugins/exasol-exasol-datasource
docker cp dist/. <grafana_container>:/var/lib/grafana/plugins/exasol-exasol-datasource/
docker restart <grafana_container>
```

Make sure the container was started with:

```bash
-e GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=exasol-exasol-datasource
```

## Verify plugin load

- In Grafana UI: `Administration -> Plugins` and search for `Exasol`
- Or check logs:

```bash
docker logs <grafana_container> | grep exasol-exasol-datasource
```

## Configure datasource

In `Connections -> Data sources -> Add data source -> Exasol`:

- `Host` (for example `exasol` or `db.example.com`)
- `Port` (`8563` by default)
- `User`
- `Password`
- Optional: `Skip TLS Verify` for test environments only

Click `Save & test`.

In panel queries, choose the query `Format`:

- `Table` (default): raw SQL results
- `Time series`: requires at least one Exasol `DATE`/`TIMESTAMP` column and at least one numeric column, returned as wide time series

Notes for `Time series` format:

- String columns are treated as labels, not values
- Queries with only `time` plus string columns are not valid time series; use `Table` format or Grafana annotations for event-style data
- The plugin identifies time fields from Exasol column metadata, so the time column should come from a native temporal column

## Supported macros

The datasource supports a PostgreSQL-style subset of Grafana SQL macros for native Exasol temporal columns:

- `$__time(column)`:
  aliases a native temporal column as `"time"`
- `$__timeFilter(column)`:
  expands to a Grafana time-range predicate
- `$__timeFrom()` and `$__timeTo()`:
  expand to the dashboard time-range boundaries as Exasol timestamp expressions
- `$__timeGroup(column, '5m'[, fill])`:
  buckets a native temporal column by interval
- `$__timeGroupAlias(column, '5m'[, fill])`:
  same as `$__timeGroup`, but aliases the bucketed expression as `"time"`

## How to use the macros

Use the macros with native Exasol `DATE` or `TIMESTAMP` columns.

Basic time filtering:

```sql
SELECT
  $__time(INTERVAL_START),
  USERS_AVG,
  USERS_MAX,
  CLUSTER_NAME
FROM EXA_USAGE_HOURLY
WHERE $__timeFilter(INTERVAL_START)
ORDER BY INTERVAL_START
```

Time-series bucketing:

```sql
SELECT
  $__timeGroupAlias(INTERVAL_START, '5m'),
  AVG(USERS_AVG) AS users_avg,
  MAX(USERS_MAX) AS users_max,
  CLUSTER_NAME
FROM EXA_USAGE_HOURLY
WHERE $__timeFilter(INTERVAL_START)
GROUP BY 1, 4
ORDER BY 1
```

Annotation-style events:

```sql
SELECT
  $__time(MEASURE_TIME),
  EVENT_TYPE AS text,
  CLUSTER_NAME AS tags
FROM EXA_SYSTEM_EVENTS
WHERE $__timeFilter(MEASURE_TIME)
ORDER BY MEASURE_TIME
```

Examples:

```sql
SELECT
  $__timeGroupAlias(MEASURE_TIME, '5m'),
  AVG(USERS_AVG) AS value,
  CLUSTER_NAME
FROM EXA_USAGE_HOURLY
WHERE $__timeFilter(MEASURE_TIME)
GROUP BY 1, 3
ORDER BY 1
```

```sql
SELECT
  $__time(MEASURE_TIME),
  EVENT_TYPE AS text,
  CLUSTER_NAME AS tags
FROM EXA_SYSTEM_EVENTS
WHERE $__timeFilter(MEASURE_TIME)
ORDER BY MEASURE_TIME
```

Macro notes:

- Supported fixed-size grouping intervals are `ms`, `s`, `m`, `h`, `d`, and `w`
- Calendar grouping is supported for `1M` and `1y`
- Multi-month and multi-year buckets such as `2M` or `2y` are not supported
- PostgreSQL unix-epoch macros such as `$__unixEpochFilter` are intentionally not implemented because this datasource treats native Exasol temporal types as time fields rather than numeric epoch columns

## Development commands

```bash
npm run dev
npm run lint
npm run test:ci
go test ./...
```
