# Roadmap

The running API covers Série A, Série B, Série C, and Série D for every year the CBF catalog lists: real collection, PostgreSQL, read-only HTTP, OpenAPI, and Docker. Phase 10 is not implemented.

## 1. Legacy diagnosis and source validation

Objective: keep the previous behavior that is worth keeping, and choose a source from live responses.

Depends on: nothing.

- [x] Read the PHP scraper, the static front end, and the Git history.
- [x] Move that code to `legacy/` without rewriting history.
- [x] Check `robots.txt`, the standings page, and the public JSON used by that page.
- [x] Record the chosen source, URLs, and limits.

Acceptance: the source decision names live URLs and states what each URL returns.

## 2. Go foundation, Docker, and configuration

Objective: a pinned Go build, Compose stack, and boot-time config validation.

Depends on: phase 1.

- [x] Go module, `cmd/api`, `cmd/worker`, `cmd/migrate`.
- [x] Multi-stage Dockerfile, non-root runtime image, Compose with Postgres, migrate, API, and worker.
- [x] `.env.example`, Makefile targets, `.dockerignore`, `.gitignore`.
- [x] Startup validation for environment variables.

Acceptance: `docker compose build` succeeded and the runtime image user is `65532:65532`. This machine has no `make`, so `make setup` and `make build` were not executed. The README lists the Compose equivalents.

## 3. Model and migrations

Objective: stable internal ids and a forward-only schema.

Depends on: phase 2.

- [x] Competitions, seasons, teams, aliases, season membership, rounds, matches, standings.
- [x] External id map per source.
- [x] Sync runs and per-season sync state.
- [x] Null scores and null kickoffs. Status check constraint.

Acceptance: the migrate container applied `migrations/001_init.sql` on a clean volume and skipped it on the next start.

## 4. Scraper and synchronization

Objective: idempotent collection that refuses to publish a broken snapshot.

Depends on: phases 1 and 3.

- [x] CBF adapter separated from HTTP handlers.
- [x] Catalog, full match table, standings HTML, and schedule-change feed.
- [x] Retries, `Retry-After`, rate limit, body limit, same-host redirects.
- [x] Worker interval, `worker -once`, initial run, advisory lock, graceful shutdown.
- [x] Failed collection keeps the previous rows and marks the season stale.

Acceptance: the 2026 Série A sync stored 380 matches and 20 standings. A second sync kept the same team ids. A one-shot run against `https://example.invalid` left those rows in place and set `stale` to true.

## 5. REST API

Objective: read endpoints over stored data only.

Depends on: phase 4.

- [x] Competition, season, team, standings, round, match, and match detail routes.
- [x] Filters, stable ordering, and pagination limits.
- [x] Error body with `code`, `message`, and `request_id`.
- [x] Source metadata, including never-collected and stale.
- [x] Live vs ready health checks. Ready pings Postgres and does not call CBF.

Acceptance: the documented curl examples returned the stored 2026 season. `cmd/api` does not import the scraper. Stopping Postgres made `/health/ready` return 503 while `/health/live` stayed 200.

## 6. OpenAPI and Swagger UI

Objective: one contract that matches the handlers.

Depends on: phase 5.

- [x] OpenAPI 3.1 document with schemas, nulls, pagination, examples, and errors.
- [x] Swagger UI at `/docs` and the document at `/openapi.yaml`.
- [x] CI test that the document parses and lists every route.

Acceptance: `/docs` returned 200 and loads the vendored Swagger UI against `/openapi.yaml`. `TestOpenAPIDocument` passed.

## 7. Tests, security, and CI

Objective: default tests stay offline, and the pipeline matches local commands.

Depends on: phases 4 to 6.

- [x] Parser fixtures, including a real standings fragment, a 0-0 score, a missing score, and an undefined date.
- [x] Handler, filter, pagination, and error tests.
- [x] Postgres integration test for idempotent upsert and preservation after a failed run.
- [x] Rate limit, CORS, parameterized SQL, non-root container.
- [x] Update outdated Go dependencies and align build/test images with Go 1.26.
- [x] Exclude HTML files from GitHub language statistics while keeping required files tracked.
- [ ] GitHub Actions runs format, `go vet`, `go test -race`, `govulncheck`, OpenAPI, and image build.

Acceptance: on 2026-10-31, `gofmt -l` was empty, `go vet ./...` passed, and `go test ./...` passed, including the PostgreSQL integration test against the Compose database on `127.0.0.1:5433`. The Docker test profile passed `go test -race ./...`. `govulncheck ./...` reported no known vulnerabilities with Go 1.26. `make check` was not run because `make` is not installed. The workflow file was not executed on GitHub.

## 8. Integrated validation and documentation

Objective: a clean Compose project can collect and serve a season.

Depends on: phases 2 to 7.

- [x] Build images, migrate, sync a real season, query endpoints, open Swagger.
- [x] Restart and confirm the volume still has the data.
- [x] Document source failure behavior.
- [x] README, AGENTS, ROADMAP, and CHANGELOG match what is implemented.

Acceptance: `docker compose up -d --build` migrated, synced 2026 Série A, and served `/docs`. `docker compose down` without `-v` kept the volume, and the same team and match ids were still there after the next start. On this machine port 5432 is already taken by a local PostgreSQL service, so Compose was published with `POSTGRES_PORT=5433`.

## 9. Série B, C, D, and every catalog year

Objective: serve official standings, teams, and matches for the four divisions across every year the CBF catalog lists.

Depends on: phases 4 to 8.

- [x] Série B, Série C, and Série D through the same adapter. Category ids are 2, 3, and 5.
- [x] Every catalog year. On 2026-10-31 the catalog listed 2018–2026. Compose sets `SYNC_COMPETITION=all` and `SYNC_YEARS=all`.
- [x] Grouped tables. `group_name` is the official title and is empty only when the page has none. Positions repeat inside a group. Migration `002_standings_group.sql`.
- [x] A year page with no table is followed by the first points phase. Knockout phases are skipped.
- [x] One failed season keeps the last valid rows for that season and does not discard the others.

Acceptance, checked on 2026-10-31 against the Compose database and the API:

- 36 seasons. Série A and Série B, 2018–2026: 380 matches and 20 standings each. `group_name` is empty for Série A 2018 and for Série B 2018 and 2019. The other A and B seasons use `GRUPO ÚNICO`. Série A 2026 leader was Flamengo, 73 points, 33 played.
- Série C: 2018 and 2019 have 194 matches, 20 standings, `GRUPO A` and `GRUPO B`. 2020 and 2021 have 206 matches, 20 standings, the same two groups. 2022–2026 have 216 matches and 20 standings in one group: `GRUPO A` in 2022 and 2024–2026, `GRUPO ÚNICO` in 2023.
- Série D: 2018 and 2019 have 266 matches, 68 standings, 17 groups of 4 (`GRUPO A01`–`GRUPO A17`). 2020 has 518 matches, 64 standings, 8 groups (`GRUPO A-01`–`GRUPO A-08`). 2021 has 518 matches and 64 standings in `GRUPO A01`–`GRUPO A08`. 2022–2025 have 510 matches, 64 standings, 8 groups (`GRUPO A1`–`GRUPO A8`). 2026 has 610 matches, 96 standings, 16 groups (`GRUPO A1`–`GRUPO A16`, positions 1–6).
- The first pass for Série D 2021 returned `standings table is empty` and exited `worker -once` with one failure. The next pass stored 518 matches and 64 standings. `GET /api/v1/competitions/brasileirao-serie-d/seasons/2021/standings` returned 200, first row Castanhal, `GRUPO A01`, position 1, 36 points.
- A later pass received HTTP 429 for Série C 2024 and left the stored 216 matches and 20 standings in place.
- After a restart on 2026-10-31 the same volume was still there. `docker compose up -d --build` applied the migrations, served `/health/ready` and `/docs`, and returned the stored standings. Série B 2018 returned 20 teams. The startup collect finished all 36 seasons again. Série A and Série B stayed at 380 matches and 20 standings. It ended with Série D 2026 at 610 matches and 96 standings.

## 10. Later

Not part of the current implementation.

- [ ] Seasons before the ones the chosen source lists.
- [ ] Scorers and player statistics.
- [ ] A second source adapter if CBF pages change or coverage is insufficient.
- [ ] Stronger in-progress detection when a source publishes that state. The enum exists; the CBF adapter does not emit `in_progress`.
