# Contributing

Thanks for your interest in improving the Exasol Grafana datasource. This project welcomes pull requests and issues.

## Before you start

- Read [DEVELOPMENT.md](./DEVELOPMENT.md) for prereqs, build commands, and the local Docker setup.
- For non-trivial changes, open an issue first to discuss the design.
- Security disclosures should follow [SECURITY.md](./SECURITY.md) — please do not file public issues for them.

## Workflow

1. Fork the repo and create a topic branch from `main`.
2. Make your change with a focused set of commits.
3. Run the full check matrix locally:
   ```bash
   gofmt -l ./pkg          # must print nothing
   go vet ./...
   go test ./...
   golangci-lint run ./...
   npm run typecheck
   npm run lint
   npm run test:ci
   npm run build
   ```
4. Push and open a pull request against `main`. CI will repeat these checks; both must pass.

## Commit and PR conventions

- Keep PR titles short (≤ 70 chars) and use the body to explain the why.
- Reference issue numbers in the body (`Closes #123`) when applicable.
- Don't bundle unrelated changes; smaller PRs review faster.
- Update `CHANGELOG.md` for user-visible changes under the unreleased section.
- Add or update tests alongside the code change. Backend tests live in `pkg/plugin/*_test.go` (use [`go-sqlmock`](https://github.com/DATA-DOG/go-sqlmock) for query-path coverage). Frontend tests use Jest; e2e tests use Playwright (`tests/`).

## Code style

- Go: standard `gofmt` and the lint set in `.golangci.yml`. No exceptions.
- TypeScript/React: the bundled ESLint + Prettier config (`npm run lint:fix`).
- Avoid adding new dependencies casually; explain the need in the PR description.

## What we'll likely push back on

- Adding new macros without tests in `pkg/plugin/macros_test.go`.
- Changes to type conversion without a corresponding case in the sqlmock test suite.
- Backwards-incompatible changes to `pkg/models/PluginSettings` JSON without a migration note.
- New runtime dependencies on libraries with restrictive licenses (this project is MIT).

## Getting help

If you're unsure about an approach, open a draft PR or comment on the related issue — feedback before the work is done is welcome.
