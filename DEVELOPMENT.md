# Development Guide

This document covers building, running, and testing the Exasol datasource plugin locally. For end-user installation and configuration, see [README.md](./README.md).

## Prerequisites

- Node.js `>= 22` (`.nvmrc` provided)
- Go `>= 1.24` (CI builds against the version in `go.mod`)
- Docker + Docker Compose (for running Grafana locally)
- An Exasol cluster reachable from your machine, or `host.docker.internal` from inside the container

## Repository layout

```
src/                    Frontend (React + TypeScript)
pkg/plugin/             Backend datasource (Go)
pkg/models/             Backend settings model
tests/                  Playwright e2e specs
docs/examples.sql       Reference queries
provisioning/           Grafana provisioning fixtures for e2e
.config/                create-plugin webpack/jest/eslint config
```

## Build

```bash
npm install
npm run build           # frontend → dist/
npm run build:backend   # backend binaries for all platforms → dist/
npm run package         # builds + zips to build/exasol-exasol-datasource-<version>.zip
```

`mage -v` is equivalent to `npm run build:backend` if you have mage installed locally.

## Validate the packaged plugin

```bash
npm run validate:plugin          # rebuilds + runs @grafana/plugin-validator
npm run validate:plugin:local    # validates an already-built ZIP at repo root
```

## Run Grafana with the plugin in Docker

The committed `docker-compose.yaml` mounts `./dist` into the plugin directory and disables the catalog gate for unsigned plugins:

```bash
docker compose up -d
```

Then open <http://localhost:3000> (anonymous Admin is enabled in the compose file).

The relevant env vars:

| Variable | Purpose |
| --- | --- |
| `GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=exasol-exasol-datasource` | Permit loading the unsigned local build |
| `GF_DEFAULT_APP_MODE=development` | Lets Grafana treat unsigned plugins as valid during dev |
| `GF_PLUGINS_PLUGIN_ADMIN_ENABLED=true` | Keep the plugin catalog UI accessible |
| `GF_PLUGINS_PLUGIN_ADMIN_EXTERNAL_MANAGE_ENABLED=false` | Disable the "manage via grafana.com" gate |

If a Save & Test inside the container needs to reach Exasol on your host, use `host.docker.internal` as the host (macOS / Windows). Linux users typically need `--network host` or a literal host IP.

## Install into an already-running Grafana container

```bash
docker exec -it <grafana_container> mkdir -p /var/lib/grafana/plugins/exasol-exasol-datasource
docker cp dist/. <grafana_container>:/var/lib/grafana/plugins/exasol-exasol-datasource/
docker restart <grafana_container>
```

Make sure the container was started with `-e GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=exasol-exasol-datasource`.

## Verify the plugin is loaded

UI: **Administration → Plugins** and search "Exasol".

Logs:

```bash
docker logs <grafana_container> | grep exasol-exasol-datasource
```

## Testing

```bash
# Frontend
npm run typecheck
npm run lint
npm run test:ci

# Backend
go test ./...
gofmt -l ./pkg          # must be empty
golangci-lint run ./... # see .golangci.yml

# End-to-end (requires a running Grafana + Exasol)
npm run e2e
```

Backend unit tests use [`go-sqlmock`](https://github.com/DATA-DOG/go-sqlmock) (see `pkg/plugin/datasource_sqlmock_test.go`). The real Exasol driver is exercised only by the e2e suite and the `provisioning/` fixtures.

## Watch mode (frontend)

```bash
npm run dev
```

Webpack rebuilds `dist/` on every change. Grafana picks up the new module without a restart (a hard browser refresh is sometimes required).

## Architecture overview

```
┌──────────────────────────────────┐
│           Browser (React)        │
│  ConfigEditor, QueryEditor       │
│  src/components/*.tsx            │
└────────────┬─────────────────────┘
             │ Grafana JS bridge (DataSourceWithBackend)
┌────────────▼─────────────────────┐
│      Grafana server (Go)         │
│      Plugin SDK gRPC bridge      │
└────────────┬─────────────────────┘
             │ datasource.Manage()
┌────────────▼─────────────────────┐
│    pkg/plugin (this codebase)    │
│  - QueryData / CheckHealth       │
│  - Macro interpolation           │
│  - Type conversion + pivoting    │
└────────────┬─────────────────────┘
             │ database/sql
┌────────────▼─────────────────────┐
│  github.com/exasol/exasol-driver │
└──────────────────────────────────┘
```

## Macro implementation notes

Macros live in `pkg/plugin/macros.go` and use the SDK's `sqlutil.Macros` framework.

| Macro | Notes |
| --- | --- |
| `$__time`, `$__timeFilter`, `$__timeFrom`, `$__timeTo` | Native `TIMESTAMP`/`DATE` columns only |
| `$__timeGroup`, `$__timeGroupAlias` | Fixed: `ms/s/m/h/d/w`; calendar: `1M`, `1y` only |
| `$__interval`, `$__interval_ms` | Sourced from `query.Interval`; fall back to `1s` / `1000` when zero (alerting) |
| `$__unixEpochFilter`, `$__unixEpochGroup` | For columns storing Unix epoch as `DECIMAL` / `INTEGER` |

Time-range expressions use the constant `exasolEpochAnchor = TIMESTAMP '1970-01-01 00:00:00'` and Exasol's `ADD_SECONDS`/`SECONDS_BETWEEN` functions instead of dialect-specific epoch helpers.

## Releasing

1. Bump `version` in `package.json` and add a `CHANGELOG.md` entry.
2. Commit and tag: `git tag v<x.y.z> && git push --tags`.
3. The `release.yml` workflow runs `grafana/plugin-actions/build-plugin`. Add `GRAFANA_ACCESS_POLICY_TOKEN` to repository secrets to enable signing.

## Common dev pitfalls

- **"plugin is unsigned" in Grafana**: expected for local builds; `GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS` permits it.
- **Catalog page complains "not published to grafana.com/plugins"**: cosmetic; use **Connections → Data sources** to add the datasource instead of the catalog detail page.
- **`Failed to get Exasol connection: dial tcp …`**: the host you typed isn't reachable from inside the Docker container. Use `host.docker.internal` on macOS/Windows.
- **`go mod tidy` bumping the `go` directive**: a transitive dependency requires a newer Go toolchain; either upgrade your local toolchain or pin the dependency.
