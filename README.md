# API Brasileirão

Read-only HTTP API for Campeonato Brasileiro Série A, Série B, Série C, and Série D. A worker collects the public CBF pages and JSON feeds into PostgreSQL. The API serves that database. It does not scrape when a client calls it, and it is not a live-score service. The year in the path selects the season. Unknown competitions and seasons return 404.

This project is not affiliated with the CBF, any club, or any broadcaster. The repository does not include a license file.

The previous PHP scraper and the static front end are kept under `legacy/` for reference.

## What a response means

- Scores and kickoff times that the source has not published are `null`. A real 0-0 stays 0-0.
- Kickoff times are stored in UTC and returned as RFC 3339.
- Standings are the official table. Points and goal difference are copied, not recalculated. `group_name` is the official group title. It is empty when the page has no group title. A single table is often published as `GRUPO ÚNICO`. Grouped divisions repeat `position` inside each group. Rows are ordered by the published group order, then by position.
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
```

`make setup` copies `.env.example` to `.env` only when `.env` is missing. `make up` starts PostgreSQL, applies migrations once, then starts the API on `127.0.0.1:8080` and the worker. The Compose worker sets `SYNC_COMPETITION=all` and `SYNC_YEARS=all`. It collects every catalog season for the four divisions on startup, then every hour. Each season has its own `SYNC_TIMEOUT` (3 minutes in Compose). A failed season is logged, its previous rows stay, and the other seasons continue. `worker -once` exits non-zero when any selected season failed.

`make scrape` collects one competition and one year. The defaults are `brasileirao-serie-a` and `2026`.

```sh
make scrape COMPETITION=brasileirao-serie-d YEAR=2021
```

`make down` stops the containers and keeps the database volume. `make reset CONFIRM=yes` deletes that volume.

Other targets: `make logs`, `make migrate`, `make test`, `make fmt`, `make vet`, `make vuln`, `make openapi`, and `make check`.

On a machine without `make`, the same flow is:

```sh
docker compose up -d --build
docker compose run --rm --build worker -once -competition brasileirao-serie-a -year 2026
```

`docker compose run --rm worker -once` uses the Compose environment and collects the full catalog. `-competition` accepts one code, a comma-separated list, or `all`. `-year` selects one year and overrides `SYNC_YEARS=all`. Codes: `brasileirao-serie-a`, `brasileirao-serie-b`, `brasileirao-serie-c`, `brasileirao-serie-d`. The process default, when those variables are unset, remains the current Série A season. `.env.example` documents that default.

The development database password in `compose.yaml` is local-only. Do not publish port 5432 or reuse those credentials outside this machine. The host port defaults to 5432. Set `POSTGRES_PORT` when that port is already taken; containers still connect to `postgres:5432` on the Compose network. Set `CORS_ALLOWED_ORIGINS` to an explicit origin list before exposing the API, and set `TRUST_PROXY=true` only behind a proxy that overwrites `X-Forwarded-For`.

## Calls

```sh
curl -s http://127.0.0.1:8080/health/live
curl -s http://127.0.0.1:8080/health/ready
curl -s http://127.0.0.1:8080/api/v1/competitions
curl -s http://127.0.0.1:8080/api/v1/competitions/brasileirao-serie-a/seasons/2026/standings
curl -s http://127.0.0.1:8080/api/v1/competitions/brasileirao-serie-a/seasons/2026/teams
curl -s http://127.0.0.1:8080/api/v1/competitions/brasileirao-serie-d/seasons/2026/standings
curl -s "http://127.0.0.1:8080/api/v1/competitions/brasileirao-serie-a/seasons/2026/matches?round=1&page=1&page_size=20"
curl -s http://127.0.0.1:8080/openapi.yaml
```

Swagger UI is at <http://127.0.0.1:8080/docs>. Match filters are `round`, `team_id`, `status`, `from`, and `to`. `page_size` is at most 100. Matches are ordered by kickoff, with unknown kickoffs last, then by id.

`/health/live` only checks that the process is up. `/health/ready` pings PostgreSQL and does not call the CBF site.

## Source

Checked on 2026-10-30. `https://www.cbf.com.br/robots.txt` returned 404. The old competition URL under `/futebol-brasileiro/competicoes/` also returned 404. The pages the public site uses now are:

| Document | Path | What it returns |
| --- | --- | --- |
| Catalog | `/api/cbf/campeonatos/anos` | JSON. Campeonato Brasileiro is id `42`. Category ids are `1` Série A, `2` Série B, `3` Série C, and `5` Série D. Years and championship ids are parallel comma-separated lists. |
| Fixtures | `/api/cbf/jogos/tabela-detalhada/campeonato/{id}` | JSON, one phase, every match in the season. |
| Schedule changes | `/api/cbf/campeonatos/{id}/imt` | JSON list. `01/01/1900` and `1900-01-01 ::00` mean the date is undefined. |
| Standings | `/futebol-brasileiro/tabelas/campeonato-brasileiro/{slug}/{year}` | The official table, as an HTML table or as group rows embedded in the page. `{slug}` is `serie-a`, `serie-b`, `serie-c`, or `serie-d`. |

The 2026 championship ids are Série A `1260611`, Série B `1260612`, Série C `1260613`, and Série D `1260635`. Club ids come from the badge URL `conteudo.cbf.com.br/clubes/{id}/escudo.jpg` on the HTML table, and from `cod_time` in an embedded group row.

The catalog is requested once per collection. Each season then requests fixtures, schedule changes, and the standings year page. When that page has no parsed table, the collector requests the same path plus the id of the first points phase (`fase_tipo` `pontuacao`) and does not use a knockout phase. Requests wait at least `SOURCE_MIN_INTERVAL` and retry HTTP 429 and 5xx with `Retry-After` when it is present. A block page, an empty table, or a short fixture list fails that season and does not replace its stored rows.

Série A and Série B accept a snapshot of at least 100 matches and 18–22 standings rows. Série C accepts at least 40 matches and 8–40 rows. Série D accepts at least 20 matches and 16–128 rows.

Checked on 2026-10-31. A catalog sync stored 36 seasons, 2018–2026, for all four divisions. Série A and Série B have 380 matches and 20 standings in each of those years. `group_name` is empty for Série A 2018 and for Série B 2018 and 2019. The other A and B seasons use `GRUPO ÚNICO`. Série A 2026 was led by Flamengo with 73 points from 33 matches. Série C has 20 standings rows in every year: `GRUPO A` and `GRUPO B` in 2018–2021, and one group in 2022–2026. Série D 2020 names its eight groups `GRUPO A-01` through `GRUPO A-08`. Série D 2026 has 16 groups, `GRUPO A1` through `GRUPO A16`, and 96 rows. Série D 2018 and 2019 have 17 groups of 4. The first pass for Série D 2021 saw an empty standings page. The next pass stored 518 matches and 64 standings. The first row was Castanhal, `GRUPO A01`, position 1, 36 points. A later HTTP 429 for Série C 2024 left the stored rows in place. The same rows were still served after a restart.

## Tests

`go test ./...` does not call the CBF website. Parser tests use fixtures in `internal/scraper/cbf/testdata`. The PostgreSQL test creates a schema named `it_` plus random hex, migrates that schema, and drops only that schema. It runs when `DATABASE_URL` is set. `CBF_LIVE=1` is reserved and is not required.

`go test -race` needs cgo. The Compose `test` profile runs it inside the Go 1.26.10 image. The module uses Go 1.26 because the current `golang.org/x/net` release requires it. `govulncheck ./...` reports no known vulnerabilities with this toolchain and dependency set.

## Layout

`cmd/api`, `cmd/worker`, and `cmd/migrate` are one Go module and one image, with different commands. HTTP handlers are in `internal/httpapi` and only read `internal/repository/postgres`. Collection lives in `internal/scraper/cbf`. `docs/openapi.yaml` is the contract served at `/openapi.yaml`. HTML files are excluded from GitHub's language statistics via `.gitattributes`; they remain tracked when needed by the project.
