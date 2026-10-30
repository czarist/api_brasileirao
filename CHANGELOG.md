# Changelog

All notable changes to this project are documented in this file.

The format is based on Keep a Changelog. This project does not publish version tags from the rewrite itself until the maintainer asks.

## [Unreleased]

### Added

- Go rewrite of the Brasileirão read API, replacing the PHP table scraper. The previous PHP and front-end files are preserved under `legacy/`.
- PostgreSQL schema, versioned migrations, and a Docker Compose stack for Postgres, migrations, the API, and the worker.
- CBF adapter for Série A: competition catalog, match table, official standings, and schedule-change notes.
- Read API under `/api/v1`, health checks, OpenAPI 3.1, and Swagger UI.
- Sync worker with a manual one-shot command, advisory locking, and retention of the last valid snapshot when a collection fails.

### Security

- API and worker containers run as a non-root user.
- Configuration comes from the environment. `.env.example` has local defaults only.
- Public routes are read-only and rate limited. Source URLs are not taken from clients.
- `govulncheck` on Go 1.24.3 reports findings in the standard library, `golang.org/x/net` v0.50.0, and `golang.org/x/text` v0.34.0. Those fixes require Go 1.25 or newer, which this module does not use.
