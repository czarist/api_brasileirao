# API Brasileirão

Read-only HTTP API for Campeonato Brasileiro Série A, Série B, Série C, and Série D, the professional Copa do Brasil, and the adult professional state divisions that have an adapter: Mineiro (Módulo I, Módulo II, Segunda Divisão), Candangão Série A and B, Goiano (1ª Divisão, Divisão de Acesso, 3ª Divisão), Pernambucano A1, A2, and A3, Maranhense Série A and B, Mato-Grossense 1ª Divisão and II Divisão, Alagoano Série A plus Série B where that catalog names it, and Baianão Série A and Série B. A worker collects those public pages into PostgreSQL. The API serves that database. It does not scrape when a client calls it, and it is not a live-score service. The year in the path selects the season. Unknown competitions and seasons return 404.

This project is not affiliated with the CBF, the FMF, any club, or any broadcaster. The repository does not include a license file.

The previous PHP scraper and the static front end are kept under `legacy/` for reference.

## What a response means

- Scores and kickoff times that the source has not published are `null`. A real 0-0 stays 0-0.
- Kickoff times are stored in UTC and returned as RFC 3339.
- Standings are the official table. Points and goal difference are copied, not recalculated. `group_name` is the official group title. It is empty when the page has no group title. A single table is often published as `GRUPO ÚNICO`. Grouped divisions repeat `position` inside each group. Rows are ordered by the published group order, then by position. Copa do Brasil has no table, so that list is empty. A Mineiro later-phase table that only repeats clubs from the classification is not stored.
- The FMF standings table does not publish cards. `yellow_cards` and `red_cards` are 0 for those seasons. A trailing `*` on a figure is a footnote. The number in the cell is stored.
- `source.name` is the adapter that stored the rows: `cbf`, `fmf`, `ffdf`, `fgf-go`, `fpf-pe`, `fmf-ma`, `fmf-mt`, `faf-al`, or `fbf`. A list of every competition uses the adapter of the newest stored row.
- Penalty scores are copied when the source publishes them, including a published 0. A match without a penalty shoot-out has null penalty scores.
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

`make setup` copies `.env.example` to `.env` only when `.env` is missing. `make up` starts PostgreSQL, applies migrations once, then starts the API on `127.0.0.1:8080` and the worker. The Compose worker sets `SYNC_COMPETITION=all` and `SYNC_YEARS=all`. `all` is the CBF catalog, then Mineiro, then the state divisions in `internal/catalog`. Each season has its own `SYNC_TIMEOUT` (3 minutes in Compose). A failed season is logged, its previous rows stay, and the other seasons continue. `worker -once` exits non-zero when any selected season failed. Recreating the worker collects every selected season, including the leagues already stored. The hourly worker that is already running was left on its previous binary on purpose.

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

`docker compose run --rm worker -once` uses the Compose environment and collects the full catalog. `-competition` accepts one code, a comma-separated list, or `all`. `-year` selects one year and overrides `SYNC_YEARS=all`. Codes: `brasileirao-serie-a`, `brasileirao-serie-b`, `brasileirao-serie-c`, `brasileirao-serie-d`, `copa-do-brasil`, `mineiro-modulo-i`, `mineiro-modulo-ii`, `mineiro-segunda-divisao`. The process default, when those variables are unset, remains the current Série A season. `.env.example` documents that default. The FMF client always uses `https://www.fmf.com.br`. It does not follow `SOURCE_BASE_URL`.

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
curl -s "http://127.0.0.1:8080/api/v1/competitions/copa-do-brasil/seasons/2026/matches?status=scheduled"
curl -s http://127.0.0.1:8080/api/v1/competitions/copa-do-brasil/seasons/2026/standings
curl -s http://127.0.0.1:8080/api/v1/competitions/mineiro-modulo-i/seasons/2026/standings
curl -s http://127.0.0.1:8080/openapi.yaml
```

Swagger UI is at <http://127.0.0.1:8080/docs>. Match filters are `round`, `team_id`, `status`, `from`, and `to`. `page_size` is at most 100. Matches are ordered by kickoff, with unknown kickoffs last, then by id.

`/health/live` only checks that the process is up. `/health/ready` pings PostgreSQL and does not call the CBF site.

## Source

Checked on 2026-10-30. `https://www.cbf.com.br/robots.txt` returned 404. The old competition URL under `/futebol-brasileiro/competicoes/` also returned 404. The pages the public site uses now are:

| Document | Path | What it returns |
| --- | --- | --- |
| Catalog | `/api/cbf/campeonatos/anos` | JSON. Campeonato Brasileiro is id `42`. Category ids are `1` Série A, `2` Série B, `3` Série C, and `5` Série D. Copa do Brasil is championship `24`, professional category `4`. Category ids are not unique across championships. Years and championship ids are parallel comma-separated lists. |
| Fixtures | `/api/cbf/jogos/tabela-detalhada/campeonato/{id}` | JSON, one phase, every match in the season. |
| Schedule changes | `/api/cbf/campeonatos/{id}/imt` | JSON list. `01/01/1900` and `1900-01-01 ::00` mean the date is undefined. |
| Standings | `/futebol-brasileiro/tabelas/campeonato-brasileiro/{slug}/{year}` | The official table, as an HTML table or as group rows embedded in the page. `{slug}` is `serie-a`, `serie-b`, `serie-c`, or `serie-d`. |

The 2026 championship ids are Série A `1260611`, Série B `1260612`, Série C `1260613`, Série D `1260635`, and Copa do Brasil `1260615`. Club ids come from the badge URL `conteudo.cbf.com.br/clubes/{id}/escudo.jpg` on the HTML table, and from `cod_time` in an embedded group row. In the fixture feed the club id comes from the same badge URL.

The catalog is requested once per collection. Each season then requests fixtures and schedule changes. A division also requests the standings year page. When that page has no parsed table, the collector requests the same path plus the id of the first points phase (`fase_tipo` `pontuacao`) and does not use a knockout phase. Copa do Brasil has no standings page, so that request is skipped. Requests wait at least `SOURCE_MIN_INTERVAL` and retry HTTP 429 and 5xx with `Retry-After` when it is present. A block page, an empty table, or a short fixture list fails that season and does not replace its stored rows. A fixture row that lists the same club id as home and away is omitted. Two clubs that share a name and have different badge ids both stay.

Série A and Série B accept a snapshot of at least 100 matches and 18–22 standings rows. Série C accepts at least 40 matches and 8–40 rows. Série D accepts at least 20 matches and 16–128 rows. Copa do Brasil accepts at least 100 matches and exactly zero standings rows. Módulo I and Módulo II accept at least 30 matches and 8–24 rows. Segunda Divisão accepts at least 20 matches and 8–48 rows. The Distrito Federal, Goiás, and Pernambuco divisions accept at least 8 matches and 4–40 rows. Maranhão, Mato Grosso, Alagoas, and Bahia accept at least 8 matches and 4–48 rows.

The Mineiro pages are `https://www.fmf.com.br/Competicoes/ProxJogos.aspx?d=1` (Módulo I), `d=2` (Módulo II), and `d=3` (Segunda Divisão). A season is a form POST of `request` set to the numeric option on that page. The collector refuses any other host and any other path. Youth, women's, and amateur competitions on the same site are not collected.

Checked on 2026-10-31. A catalog sync stored 36 seasons, 2018–2026, for all four divisions. Série A and Série B have 380 matches and 20 standings in each of those years. `group_name` is empty for Série A 2018 and for Série B 2018 and 2019. The other A and B seasons use `GRUPO ÚNICO`. Série A 2026 was led by Flamengo with 73 points from 33 matches. Série C has 20 standings rows in every year: `GRUPO A` and `GRUPO B` in 2018–2021, and one group in 2022–2026. Série D 2020 names its eight groups `GRUPO A-01` through `GRUPO A-08`. Série D 2026 has 16 groups, `GRUPO A1` through `GRUPO A16`, and 96 rows. Série D 2018 and 2019 have 17 groups of 4. The first pass for Série D 2021 saw an empty standings page. The next pass stored 518 matches and 64 standings. The first row was Castanhal, `GRUPO A01`, position 1, 36 points. A later HTTP 429 for Série C 2024 left the stored rows in place. The same rows were still served after a restart.

The same day, a one-shot sync stored the professional Copa do Brasil for 2018–2026. Match counts are 120 in 2018 and 2020, 118 in 2019, 122 in 2021–2025, and 154 in 2026. Standings are empty in every year. The 2018 final is Cruzeiro Esporte Clube 1-0 Corinthians and Corinthians 1-2 Cruzeiro Esporte Clube. 2026 has four scheduled semi-finals with null scores and no final. 2019 omitted two rows, `813493` and `813503`, because both sides were Bragantino with club id `35938`. Série A was unchanged after that sync.

On 2026-11-01 a one-shot stored the three professional Mineiro divisions for every year on the FMF select: Módulo I 2013–2026, Módulo II 2014–2026, and Segunda Divisão 2014–2026. Módulo I 2026 has 53 matches and 12 standings in groups A, B, and C. Atlético SAF leads group A with 14 points. One Módulo I 2020 match has null scores. Módulo II 2026 stores Ipatinga at -2 points. Série A 2018–2026 stayed at 380 matches and 20 standings.

The same day, separate one-shots stored the adult divisions below. Counts are matches/standings. A year the catalog does not list is absent. A season that failed was not stored.

- Candangão Série A 2023–2026: 51/10, 51/10, 50/10, 45/10. Série B 2023–2026: 35/12, 33/8, 28/8, 26/7. Source `ffdf`. No Série C.
- Goiano 1ª Divisão 2019–2024 and 2026: 86/12, 79/12, 74/12, 74/12, 80/12, 80/12, 70/12. 2025 was not stored: match `41180` lists the same club twice. Divisão de Acesso 2019–2026: 43/8, 11/4, 30/6, 56/8, 56/8, 56/8, 56/8, 90/10. 3ª Divisão has no 2020 card. 2019, 2021–2026: 57/13, 50/12, 44/13, 52/11, 56/13, 66/12, 21/7. Source `fgf-go`.
- Pernambucano A1 2020–2026: 57/10, 57/12, 57/10, 84/13, 53/10, 53/10, 38/8. A2 2018 and 2020–2025: 56/15, 59/13, 60/14, 105/25, 66/12, 52/10, 43/10. A3 2023: 21/7. A1 and A2 2019 had no matches. A3 2024 names no club for badge `35767`. Source `fpf-pe`.
- Maranhense Série A 2020–2025: 38/8, 38/8, 36/8, 36/8, 66/8, 62/8. Série B 2020–2023 and 2025: 30/4, 31/10, 26/7, 24/8, 35/11. Série A 2026 and Série B 2024 and 2026 published no classification. No Série C. Source `fmf-ma`.
- Mato-Grossense 1ª Divisão 2020–2026: 61/10, 60/10, 55/10, 55/10, 57/10, 48/9, 53/10. II Divisão 2021–2026: 13/4, 25/10, 26/10, 41/11, 26/10, 28/10. No 2020 II Divisão and no 3ª Divisão. Source `fmf-mt`.
- Alagoano Série A 2020 and 2022–2026: 34/8, 38/8, 36/8, 36/8, 36/8, 34/8. The 2021 catalog has no adult Série A. Série B 2022–2024: 27/7, 51/10, 51/10. The 2025 and 2026 B cards are Sub-23. No Série C. Source `faf-al`. `GET /api/v1/competitions/alagoano-serie-a/seasons/2026/standings` returned Asa, group `GRUPO A`, 17 points. Série A 2026 stayed `cbf`, led by Flamengo with 73 points.
- Baianão Série A 2011–2026: 114/12, 140/12, 76/11, 76/11, 56/12, 56/12, 61/11, then 51/10 for 2018–2024, 50/10 in 2025, and 48/10 in 2026. The 2027 select lists five fixtures and was not stored. Série B 2011–2026: 46/10, 46/10, 46/10, 47/10, 38/9, 32/6, 32/6, 31/6, 32/6, 17/6, 19/6, 72/12, 51/10, 51/10, 50/10, 51/10. No Série C. Source `fbf`. The table does not publish cards, so yellow and red are 0. The published win percentage is a decimal stored as an integer, rounded by the first decimal digit. A `(0)` pair on an old score card is a placeholder and is not stored; a shoot-out with a non-zero figure is stored, including a 0 on the other side. Clubs that share `default.png` keep the published name. `GET /api/v1/competitions/baiano-serie-a/seasons/2026/standings` returned Bahia, group `Grupo 1`, 23 points, 9 played.

The other state federations are not in this collection.

## Tests

`go test ./...` does not call the CBF or FMF websites. Parser tests use fixtures in `internal/scraper/cbf/testdata` and `internal/scraper/fmf/testdata`. The PostgreSQL test creates a schema named `it_` plus random hex, migrates that schema, and drops only that schema. It runs when `DATABASE_URL` is set. `CBF_LIVE=1` is reserved and is not required.

`go test -race` needs cgo. The Compose `test` profile runs it inside the Go 1.26.10 image. The module uses Go 1.26 because the current `golang.org/x/net` release requires it. `govulncheck ./...` reports no known vulnerabilities with this toolchain and dependency set.

## Layout

`cmd/api`, `cmd/worker`, and `cmd/migrate` are one Go module and one image, with different commands. HTTP handlers are in `internal/httpapi` and only read `internal/repository/postgres`. Collection lives in `internal/scraper/cbf`. `docs/openapi.yaml` is the contract served at `/openapi.yaml`. HTML files are excluded from GitHub's language statistics via `.gitattributes`; they remain tracked when needed by the project.
