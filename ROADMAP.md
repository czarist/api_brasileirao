# Roadmap

MVP means a runnable Série A API: real CBF collection, PostgreSQL, read-only HTTP, OpenAPI, and Docker. Later phases are not implemented.

## 1. Legacy diagnosis and source validation

Objective: keep the previous behavior that is worth keeping, and choose a source from live responses.

Depends on: nothing.

- [ ] Read the PHP scraper, the static front end, and the Git history.
- [ ] Move that code to `legacy/` without rewriting history.
- [ ] Check `robots.txt`, the standings page, and the public JSON used by that page.
- [ ] Record the chosen source, URLs, and limits.

Acceptance: the source decision names live URLs and states what each URL returns.

## 2. Go foundation, Docker, and configuration

Objective: a pinned Go build, Compose stack, and boot-time config validation.

Depends on: phase 1.

- [ ] Go module, `cmd/api`, `cmd/worker`, `cmd/migrate`.
- [ ] Multi-stage Dockerfile, non-root runtime image, Compose with Postgres, migrate, API, and worker.
- [ ] `.env.example`, Makefile targets, `.dockerignore`, `.gitignore`.
- [ ] Startup validation for environment variables.

Acceptance: `make setup` and `make build` succeed. The runtime image does not run as root.

## 3. Model and migrations

Objective: stable internal ids and a forward-only schema.

Depends on: phase 2.

- [ ] Competitions, seasons, teams, aliases, season membership, rounds, matches, standings.
- [ ] External id map per source.
- [ ] Sync runs and per-season sync state.
- [ ] Null scores and null kickoffs. Status check constraint.

Acceptance: `make migrate` applies `migrations/001_init.sql` on a clean database.

## 4. Scraper and synchronization

Objective: idempotent collection that refuses to publish a broken snapshot.

Depends on: phases 1 and 3.

- [ ] CBF adapter separated from HTTP handlers.
- [ ] Catalog, full match table, standings HTML, and schedule-change feed.
- [ ] Retries, `Retry-After`, rate limit, body limit, same-host redirects.
- [ ] Worker interval, `worker -once`, initial run, advisory lock, graceful shutdown.
- [ ] Failed collection keeps the previous rows and marks the season stale.

Acceptance: a real Série A sync writes matches and standings. A second sync updates rows in place. A parser failure does not change those rows.

## 5. REST API

Objective: read endpoints over stored data only.

Depends on: phase 4.

- [ ] Competition, season, team, standings, round, match, and match detail routes.
- [ ] Filters, stable ordering, and pagination limits.
- [ ] Error body with `code`, `message`, and `request_id`.
- [ ] Source metadata, including never-collected and stale.
- [ ] Live vs ready health checks. Ready pings Postgres and does not call CBF.

Acceptance: the documented curl examples return the stored season, and no API request appears in the source client.

## 6. OpenAPI and Swagger UI

Objective: one contract that matches the handlers.

Depends on: phase 5.

- [ ] OpenAPI 3.1 document with schemas, nulls, pagination, examples, and errors.
- [ ] Swagger UI at `/docs` and the document at `/openapi.yaml`.
- [ ] CI test that the document parses and lists every route.

Acceptance: `/docs` renders the same routes the server registers.

## 7. Tests, security, and CI

Objective: default tests stay offline, and the pipeline matches local commands.

Depends on: phases 4 to 6.

- [ ] Parser fixtures, including a real standings fragment, a 0-0 score, a missing score, and an undefined date.
- [ ] Handler, filter, pagination, and error tests.
- [ ] Postgres integration test for idempotent upsert and preservation after a failed run.
- [ ] Rate limit, CORS, parameterized SQL, non-root container.
- [ ] GitHub Actions runs format, `go vet`, `go test -race`, `govulncheck`, OpenAPI, and image build.

Acceptance: `make check` passes, or the README states the exact command that could not run.

## 8. Integrated validation and documentation

Objective: a clean Compose project can collect and serve a season.

Depends on: phases 2 to 7.

- [ ] Build images, migrate, sync a real season, query endpoints, open Swagger.
- [ ] Restart and confirm the volume still has the data.
- [ ] Document source failure behavior.
- [ ] README, AGENTS, ROADMAP, and CHANGELOG match what is implemented.

Acceptance: the README quick start works from a clean checkout with Docker only.

## 9. After the MVP

Not part of the current implementation.

- Série B, C, and D through the same adapter shape.
- Seasons before the ones the chosen source lists.
- Scorers and player statistics.
- A second source adapter if CBF pages change or coverage is insufficient.
- Stronger in-progress detection when a source publishes that state. The enum exists; the CBF adapter does not emit `in_progress`.
