# API Brasileirão

Read-only HTTP API for Campeonato Brasileiro Série A. A worker collects the public CBF pages and JSON feeds into PostgreSQL. The API serves that database. It does not scrape when a client calls it, and it is not a live-score service.

This project is not affiliated with the CBF, any club, or any broadcaster. The repository does not include a license file.

The previous PHP scraper and the static front end are kept under `legacy/` for reference.

## What a response means

- Scores and kickoff times that the source has not published are `null`. A real 0-0 stays 0-0.
- Kickoff times are stored in UTC and returned as RFC 3339.
- Standings are the official table. Points and goal difference are copied, not recalculated.
- Postponed matches keep their original round.
- `source.collection_status` is `ok` after a successful sync and `never_collected` when nothing has been published.
- `source.stale` is true when the last success is older than `STALE_AFTER` (6 hours by default) or a later collection failed. The previous rows stay in place.
- The match status `in_progress` exists in the schema and is not produced by the current collector.
- The default worker interval is 1 hour. That is the update frequency to expect.

## Run with Docker

Docker is the supported path. From the repository root:

```sh
make setup
make up
make scrape COMPETITION=brasileirao-serie-a YEAR=2026
```

`make setup` copies `.env.example` to `.env` only when `.env` is missing. `make up` starts PostgreSQL, applies migrations once, then starts the API on `127.0.0.1:8080` and the worker. The Compose file pins the worker to the 2026 Série A season and collects it on startup, then every hour.

`make down` stops the containers and keeps the database volume. `make reset CONFIRM=yes` deletes that volume.

Other targets: `make logs`, `make migrate`, `make test`, `make fmt`, `make vet`, `make vuln`, `make openapi`, and `make check`.

On a machine without `make`, the same flow is:

```sh
docker compose up -d --build
docker compose run --rm --build worker -once -competition brasileirao-serie-a -year 2026
```

The development database password in `compose.yaml` is local-only. Do not publish port 5432 or reuse those credentials outside this machine. The host port defaults to 5432. Set `POSTGRES_PORT` when that port is already taken; containers still connect to `postgres:5432` on the Compose network. Set `CORS_ALLOWED_ORIGINS` to an explicit origin list before exposing the API, and set `TRUST_PROXY=true` only behind a proxy that overwrites `X-Forwarded-For`.

## Calls

```sh
curl -s http://127.0.0.1:8080/health/live
curl -s http://127.0.0.1:8080/health/ready
curl -s http://127.0.0.1:8080/api/v1/competitions
curl -s http://127.0.0.1:8080/api/v1/competitions/brasileirao-serie-a/seasons/2026/standings
curl -s "http://127.0.0.1:8080/api/v1/competitions/brasileirao-serie-a/seasons/2026/matches?round=1&page=1&page_size=20"
curl -s http://127.0.0.1:8080/openapi.yaml
```

Swagger UI is at <http://127.0.0.1:8080/docs>. Match filters are `round`, `team_id`, `status`, `from`, and `to`. `page_size` is at most 100. Matches are ordered by kickoff, with unknown kickoffs last, then by id.

`/health/live` only checks that the process is up. `/health/ready` pings PostgreSQL and does not call the CBF site.

## Source

Checked on 2026-10-30. `https://www.cbf.com.br/robots.txt` returned 404. The old competition URL under `/futebol-brasileiro/competicoes/` also returned 404. The pages the public site uses now are:

| Document | Path | What it returns |
| --- | --- | --- |
| Catalog | `/api/cbf/campeonatos/anos` | JSON. Campeonato Brasileiro is id `42`. Série A is category id `1`. Years and championship ids are parallel comma-separated lists. |
| Fixtures | `/api/cbf/jogos/tabela-detalhada/campeonato/{id}` | JSON, one phase, every match in the season. |
| Schedule changes | `/api/cbf/campeonatos/{id}/imt` | JSON list. `01/01/1900` and `1900-01-01 ::00` mean the date is undefined. |
| Standings | `/futebol-brasileiro/tabelas/campeonato-brasileiro/serie-a/{year}` | HTML table. There is no JSON standings feed. |

The 2026 Série A championship id in that catalog is `1260611`. Club ids come from the badge URL `conteudo.cbf.com.br/clubes/{id}/escudo.jpg`. The collector sends four requests per season, waits at least `SOURCE_MIN_INTERVAL` between them, and retries HTTP 429 and 5xx with `Retry-After` when it is present. A block page, an empty table, or a short fixture list is a failed collection and does not replace stored rows.

## Tests

`go test ./...` does not call the CBF website. Parser tests use fixtures in `internal/scraper/cbf/testdata`. The PostgreSQL test creates a schema named `it_` plus random hex, migrates that schema, and drops only that schema. It runs when `DATABASE_URL` is set. `CBF_LIVE=1` is reserved and is not required.

`go test -race` needs cgo. The Compose `test` profile runs it inside the Go 1.26.10 image. The module uses Go 1.26 because the current `golang.org/x/net` release requires it. `govulncheck ./...` reports no known vulnerabilities with this toolchain and dependency set.

## Layout

`cmd/api`, `cmd/worker`, and `cmd/migrate` are one Go module and one image, with different commands. HTTP handlers are in `internal/httpapi` and only read `internal/repository/postgres`. Collection lives in `internal/scraper/cbf`. `docs/openapi.yaml` is the contract served at `/openapi.yaml`.
