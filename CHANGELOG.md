# Changelog

All notable changes to this project are documented in this file.

The format is based on Keep a Changelog. This project does not publish version tags from the rewrite itself until the maintainer asks.

## [Unreleased]

### Added

- Go rewrite of the Brasileirão read API, replacing the PHP table scraper. The previous PHP and front-end files are preserved under `legacy/`.
- PostgreSQL schema, versioned migrations, and a Docker Compose stack for Postgres, migrations, the API, and the worker.
- CBF adapter for Série A: competition catalog, match table, official standings, and schedule-change notes.
- CBF collection for Série B, Série C, and Série D, and for every year the public catalog lists. On 2026-10-31 that catalog covered 2018–2026, 36 seasons.
- Standing field `group_name`, the official group title. It is empty when the page has no group title. A single table is often published as `GRUPO ÚNICO`. Grouped divisions repeat `position` inside each group. Rows are ordered by the published group order, then by position.
- Worker selection `SYNC_COMPETITION=all` or a comma-separated list, and `SYNC_YEARS=all`. Compose sets both. Each season has its own timeout. One failed season does not replace the rows of the others.
- Read API under `/api/v1`, health checks, OpenAPI 3.1, and Swagger UI.
- Sync worker with a manual one-shot command, advisory locking, and retention of the last valid snapshot when a collection fails.

### Changed

- Updated Go dependencies and the build/test images to Go 1.26.10.
- Excluded HTML files from GitHub language statistics without removing them from the repository.
- Migration `002_standings_group.sql` replaces the standings unique key `(season_id, position)` with `(season_id, group_name, position)`.
- A year page with no standings table is followed by the first points phase published on that page. Knockout phases are skipped.
- Completeness limits are per division. Série A and Série B require 100 matches and 18–22 rows. Série C requires 40 matches and 8–40 rows. Série D requires 20 matches and 16–128 rows. `SYNC_MIN_MATCHES`, `SYNC_MIN_STANDINGS`, and `SYNC_MAX_STANDINGS` remain the fallback.

### Security

- API and worker containers run as a non-root user.
- Configuration comes from the environment. `.env.example` has local defaults only.
- Public routes are read-only and rate limited. Source URLs are not taken from clients.
- `govulncheck ./...` reports no known vulnerabilities with Go 1.26 and the updated dependencies.
