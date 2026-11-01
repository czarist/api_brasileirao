<h1 align="center">API Brasileirão</h1>

<h3 align="center">The official table. The season in the path. Served from PostgreSQL.</h3>

<p align="center">
  <a href="README.md">English</a> · <a href="README.pt-BR.md">Português (Brasil)</a>
</p>

<p align="center">
  <b>A public read API for the Brazilian Soccer League, divisions A–D, and the adult state divisions that have an adapter, written in Go.</b><br>
  A worker collects those public federation pages into PostgreSQL. An HTTP call only reads that database.
</p>

<p align="center">
  <a href="https://go.dev"><img alt="Go 1.26" src="https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go&logoColor=white"></a>
  <a href="https://www.postgresql.org"><img alt="Storage: PostgreSQL" src="https://img.shields.io/badge/storage-PostgreSQL-4169E1?style=flat-square&logo=postgresql&logoColor=white"></a>
  <a href="https://docs.docker.com/compose/"><img alt="Run with Docker Compose" src="https://img.shields.io/badge/run-Docker%20Compose-2496ED?style=flat-square&logo=docker&logoColor=white"></a>
  <a href="docs/openapi.yaml"><img alt="Contract: OpenAPI 3.1" src="https://img.shields.io/badge/contract-OpenAPI%203.1-6BA539?style=flat-square&logo=openapiinitiative&logoColor=white"></a>
  <img alt="Public read API, contract version 0.1.0" src="https://img.shields.io/badge/API-read--only%20%C2%B7%200.1.0-111111?style=flat-square">
  <a href="https://github.com/czarist/api_brasileirao/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/czarist/api_brasileirao/ci.yml?label=ci&style=flat-square&logo=githubactions&logoColor=white"></a>
</p>

<br>

<p align="center">
  <a href="#what-it-serves">What it serves</a> ·
  <a href="#what-a-response-means">Responses</a> ·
  <a href="#collected">Collected</a> ·
  <a href="#still-open">Still open</a> ·
  <a href="#quick-start">Quick start</a> ·
  <a href="#calls">Calls</a> ·
  <a href="#source">Source</a> ·
  <a href="#development">Development</a>
</p>

<br>

## What it serves

Read-only HTTP for the adult professional competitions below. The year in the path selects the season. Unknown competitions and seasons return 404. This is a stored snapshot, refreshed on the worker interval (one hour by default). It is not a live-score service.

This project is not affiliated with the CBF, the FMF, any club, or any broadcaster.

- **Brazilian Soccer League.** Divisions A, B, C, and D. Codes `brasileirao-serie-a`, `brasileirao-serie-b`, `brasileirao-serie-c`, `brasileirao-serie-d`. Source `cbf`.
- **Brazil Cup,** professional. Code `copa-do-brasil`. The cup has fixtures and an empty standings list.
- **Campeonato Mineiro.** Módulo I, Módulo II, and Segunda Divisão. Codes `mineiro-modulo-i`, `mineiro-modulo-ii`, `mineiro-segunda-divisao`. Source `fmf`.
- **Candangão.** Série A and Série B. Codes `candangao-serie-a`, `candangao-serie-b`. Source `ffdf`. The catalog has no Série C.
- **Campeonato Goiano.** 1ª Divisão, Divisão de Acesso, and 3ª Divisão. Codes `goiano-primeira-divisao`, `goiano-divisao-de-acesso`, `goiano-terceira-divisao`. Source `fgf-go`.
- **Campeonato Pernambucano.** A1, A2, and A3. Codes `pernambucano-a1`, `pernambucano-a2`, `pernambucano-a3`. Source `fpf-pe`.
- **Campeonato Maranhense.** Série A and Série B. Codes `maranhense-serie-a`, `maranhense-serie-b`. Source `fmf-ma`. The catalog has no Série C.
- **Mato-Grossense.** 1ª Divisão and II Divisão. Codes `mato-grossense-primeira-divisao`, `mato-grossense-segunda-divisao`. Source `fmf-mt`. The catalog has no 3ª Divisão.
- **Campeonato Alagoano.** Série A, plus Série B where that catalog names an adult Série B. Codes `alagoano-serie-a`, `alagoano-serie-b`. Source `faf-al`. The catalog has no Série C.
- **Baianão.** Série A and Série B. Codes `baiano-serie-a`, `baiano-serie-b`. Source `fbf`. The catalog has no adult Série C.

`all` is that list, in that order: the CBF catalog, then Mineiro, then the state divisions in `internal/catalog`. Youth, women's, amateur, and cup competitions on a state site stay out. A division the catalog does not name is not invented.

The previous PHP scraper and the static front end are kept under `legacy/` for reference.

## What a response means

- **Missing score, missing kickoff.** Scores and kickoff times the source has not published are `null`. A real 0-0 stays 0-0.
- **Times.** Kickoffs are stored in UTC and returned as RFC 3339.
- **Official table.** Points, goal difference, and the other figures are copied, including a goal difference that does not match the raw goals. `group_name` is the official group title. It is empty when the page has no group title. A single table is often published as `GRUPO ÚNICO`. Grouped divisions repeat `position` inside each group. Rows follow the published group order, then position. Brazil Cup has no table, so that list is empty. A later Mineiro phase table that only repeats clubs already in the classification is skipped.
- **Cards.** The FMF and FBF tables do not publish cards, so `yellow_cards` and `red_cards` are 0 for those seasons. A trailing `*` on a figure is a footnote. The integer in the cell is stored. A published 0 stays 0.
- **Source.** `source.name` is the adapter that stored the rows: `cbf`, `fmf`, `ffdf`, `fgf-go`, `fpf-pe`, `fmf-ma`, `fmf-mt`, `faf-al`, or `fbf`. A list of every competition uses the adapter of the newest stored row.
- **Penalties.** Penalty scores are copied when the source publishes them, including a published 0 on one side. A match without a shoot-out has null penalty scores. A `(0)` pair on an old Bahia card is a placeholder and is not stored.
- **Win percentage on the Baianão.** The FBF page prints a decimal. It is stored as an integer, rounded by the first decimal digit, half up.
- **Postponed.** Postponed matches keep their original round.
- **Freshness.** `source.collection_status` is `ok` after a successful sync and `never_collected` when nothing has been stored. `source.stale` is true when the last success is older than `STALE_AFTER` (6 hours by default) or a later collection failed. The previous rows stay. The match status `in_progress` exists in the schema. The current collector does not produce it.
- **Update frequency.** The default worker interval is 1 hour.

## Collected

Counts are matches/standings. A year the catalog does not list is absent. A season that failed validation was not stored, and a failed season does not replace rows already stored for another year.

Checked on 2026-10-31. A catalog sync stored 36 seasons, 2018–2026, for Brazilian Soccer League divisions A, B, C, and D.

- **Divisions A and B.** 380 matches and 20 standings in each of those years. `group_name` is empty for division A 2018 and for division B 2018 and 2019. The other A and B seasons use `GRUPO ÚNICO`. Division A 2026 was led by Flamengo with 73 points from 33 matches.
- **Division C.** 20 standings rows in every year: `GRUPO A` and `GRUPO B` in 2018–2021, and one group in 2022–2026. A later HTTP 429 for division C 2024 left the stored rows in place.
- **Division D.** 2020 names its eight groups `GRUPO A-01` through `GRUPO A-08`. 2026 has 16 groups, `GRUPO A1` through `GRUPO A16`, and 96 rows. 2018 and 2019 have 17 groups of 4. The first pass for 2021 saw an empty standings page. The next pass stored 518 matches and 64 standings. The first row was Castanhal, `GRUPO A01`, position 1, 36 points.

The same day, a one-shot stored the professional Brazil Cup for 2018–2026. Match counts are 120 in 2018 and 2020, 118 in 2019, 122 in 2021–2025, and 154 in 2026. Standings are empty in every year. The 2018 final is Cruzeiro Esporte Clube 1-0 Corinthians and Corinthians 1-2 Cruzeiro Esporte Clube. 2026 has four scheduled semi-finals with null scores and no final. 2019 omitted two rows, `813493` and `813503`, because both sides were Bragantino with club id `35938`. Division A was unchanged after that sync.

On 2026-11-01 a one-shot stored the three professional Mineiro divisions for every year on the FMF select: Módulo I 2013–2026, Módulo II 2014–2026, and Segunda Divisão 2014–2026. Módulo I 2026 has 53 matches and 12 standings in groups A, B, and C. Atlético SAF leads group A with 14 points. One Módulo I 2020 match has null scores. Módulo II 2026 stores Ipatinga at -2 points. Division A 2018–2026 stayed at 380 matches and 20 standings. Pages from 2013 and 2014 publish no `data-jogo` and no game number, so those matches use the page's map code as the external id and `match_number` is null.

The same day, separate one-shots stored the adult state divisions below.

- **Candangão.** Série A 2023–2026: 51/10, 51/10, 50/10, 45/10. Série B 2023–2026: 35/12, 33/8, 28/8, 26/7. Source `ffdf`.
- **Goiano.** 1ª Divisão 2019–2024 and 2026: 86/12, 79/12, 74/12, 74/12, 80/12, 80/12, 70/12. 2025 was not stored: match `41180` lists the same club twice. Divisão de Acesso 2019–2026: 43/8, 11/4, 30/6, 56/8, 56/8, 56/8, 56/8, 90/10. 3ª Divisão has no 2020 card. 2019 and 2021–2026: 57/13, 50/12, 44/13, 52/11, 56/13, 66/12, 21/7. Source `fgf-go`.
- **Pernambucano.** A1 2020–2026: 57/10, 57/12, 57/10, 84/13, 53/10, 53/10, 38/8. A2 2018 and 2020–2025: 56/15, 59/13, 60/14, 105/25, 66/12, 52/10, 43/10. A3 2023: 21/7. A1 and A2 2019 had no matches. A3 2024 names no club for badge `35767`. Source `fpf-pe`.
- **Maranhense.** Série A 2020–2025: 38/8, 38/8, 36/8, 36/8, 66/8, 62/8. Série B 2020–2023 and 2025: 30/4, 31/10, 26/7, 24/8, 35/11. Série A 2026 and Série B 2024 and 2026 published no classification. Source `fmf-ma`.
- **Mato-Grossense.** 1ª Divisão 2020–2026: 61/10, 60/10, 55/10, 55/10, 57/10, 48/9, 53/10. II Divisão 2021–2026: 13/4, 25/10, 26/10, 41/11, 26/10, 28/10. No 2020 II Divisão. Source `fmf-mt`.
- **Alagoano.** Série A 2020 and 2022–2026: 34/8, 38/8, 36/8, 36/8, 36/8, 34/8. The 2021 catalog has no adult Série A. Série B 2022–2024: 27/7, 51/10, 51/10. The 2025 and 2026 B cards are Sub-23. Source `faf-al`. `GET /api/v1/competitions/alagoano-serie-a/seasons/2026/standings` returned Asa, group `GRUPO A`, 17 points. Division A 2026 stayed `cbf`, led by Flamengo with 73 points.
- **Baianão.** Série A 2011–2026: 114/12, 140/12, 76/11, 76/11, 56/12, 56/12, 61/11, then 51/10 for 2018–2024, 50/10 in 2025, and 48/10 in 2026. The 2027 select lists five fixtures and was not stored. Série B 2011–2026: 46/10, 46/10, 46/10, 47/10, 38/9, 32/6, 32/6, 31/6, 32/6, 17/6, 19/6, 72/12, 51/10, 51/10, 50/10, 51/10. Source `fbf`. Clubs that share `default.png` keep the published name. `GET /api/v1/competitions/baiano-serie-a/seasons/2026/standings` returned Bahia, group `Grupo 1`, 23 points, 9 played.

## Still open

The other state federations are not in this collection. [ROADMAP.md](ROADMAP.md) phase 13 tracks each one: Acre, Amapá, Amazonas, Ceará, Espírito Santo, Mato Grosso do Sul, Pará, Paraíba, Paraná, Piauí, Rio de Janeiro, Rio Grande do Norte, Rio Grande do Sul, Rondônia, Roraima, Santa Catarina, São Paulo, Sergipe, and Tocantins.

Acre and São Paulo returned 403. Paraíba closed the TLS connection. Piauí failed certificate validation. Roraima and Sergipe did not resolve. The collector does not bypass HTTP 403, certificate checks, or a host that did not resolve. Rio Grande do Sul `www.fgf.com.br` is a different site from Goiás `fgf.esp.br`.

Inside catalogs that are already collected, these seasons were not stored: Goiano 1ª Divisão 2025, Pernambucano A1 and A2 2019, Pernambucano A3 2024, Maranhense Série A 2026, Maranhense Série B 2024 and 2026, and Baianão Série A 2027. The reason for each one is in [Collected](#collected).

## Quick start

Docker is the supported path. From the repository root:

```sh
make setup
make up
```

`make setup` copies `.env.example` to `.env` only when `.env` is missing. `make up` starts PostgreSQL, applies migrations once, then starts the API on `127.0.0.1:8080` and the worker. The Compose worker sets `SYNC_COMPETITION=all` and `SYNC_YEARS=all`. Each season has its own `SYNC_TIMEOUT` (3 minutes in Compose). A failed season is logged, its previous rows stay, and the other seasons continue. `worker -once` exits non-zero when any selected season failed. Recreating the worker collects every selected season, including seasons already stored.

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

`docker compose run --rm worker -once` uses the Compose environment and collects the full catalog. `-competition` accepts one code, a comma-separated list, or `all`. `-year` selects one year and overrides `SYNC_YEARS=all`. The process default, when those variables are unset, remains the current division A season. `.env.example` documents that default.

Codes: `brasileirao-serie-a`, `brasileirao-serie-b`, `brasileirao-serie-c`, `brasileirao-serie-d`, `copa-do-brasil`, `mineiro-modulo-i`, `mineiro-modulo-ii`, `mineiro-segunda-divisao`, `candangao-serie-a`, `candangao-serie-b`, `goiano-primeira-divisao`, `goiano-divisao-de-acesso`, `goiano-terceira-divisao`, `pernambucano-a1`, `pernambucano-a2`, `pernambucano-a3`, `maranhense-serie-a`, `maranhense-serie-b`, `mato-grossense-primeira-divisao`, `mato-grossense-segunda-divisao`, `alagoano-serie-a`, `alagoano-serie-b`, `baiano-serie-a`, `baiano-serie-b`.

The CBF client requests `SOURCE_BASE_URL` (default `https://www.cbf.com.br`). Each state client is locked to its own host and ignores that variable.

The development database password in `compose.yaml` is local-only. Do not publish port 5432 or reuse those credentials outside this machine. The host port defaults to 5432. Set `POSTGRES_PORT` when that port is already taken. Containers still connect to `postgres:5432` on the Compose network. Set `CORS_ALLOWED_ORIGINS` to an explicit origin list before exposing the API, and set `TRUST_PROXY=true` only behind a proxy that overwrites `X-Forwarded-For`.

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

Swagger UI is at <http://127.0.0.1:8080/docs>. The contract is [docs/openapi.yaml](docs/openapi.yaml).

Match filters are `round`, `team_id`, `status`, `from`, and `to`. `page_size` is at most 100. Matches are ordered by kickoff, with unknown kickoffs last, then by id.

`/health/live` checks that the process is up. `/health/ready` pings PostgreSQL and does not call a federation site.

## Source

Checked on 2026-10-30. `https://www.cbf.com.br/robots.txt` returned 404. The old competition URL under `/futebol-brasileiro/competicoes/` also returned 404. The pages the public site uses now are:

| Document | Path | What it returns |
| --- | --- | --- |
| Catalog | `/api/cbf/campeonatos/anos` | JSON. The Brazilian Soccer League is id `42`. Category ids are `1` division A, `2` division B, `3` division C, and `5` division D. Brazil Cup is championship `24`, professional category `4`. Category ids are not unique across championships. Years and championship ids are parallel comma-separated lists. |
| Fixtures | `/api/cbf/jogos/tabela-detalhada/campeonato/{id}` | JSON, one phase, every match in the season. |
| Schedule changes | `/api/cbf/campeonatos/{id}/imt` | JSON list. `01/01/1900` and `1900-01-01 ::00` mean the date is undefined. |
| Standings | `/futebol-brasileiro/tabelas/campeonato-brasileiro/{slug}/{year}` | The official table, as an HTML table or as group rows embedded in the page. `{slug}` is `serie-a`, `serie-b`, `serie-c`, or `serie-d`. |

The 2026 championship ids are division A `1260611`, division B `1260612`, division C `1260613`, division D `1260635`, and Brazil Cup `1260615`. Club ids come from the badge URL `conteudo.cbf.com.br/clubes/{id}/escudo.jpg` on the HTML table, and from `cod_time` in an embedded group row. In the fixture feed the club id comes from the same badge URL.

The catalog is requested once per collection. Each season then requests fixtures and schedule changes. A division also requests the standings year page. When that page has no parsed table, the collector requests the same path plus the id of the first points phase (`fase_tipo` `pontuacao`) and does not use a knockout phase. Brazil Cup has no standings page, so that request is skipped. Requests wait at least `SOURCE_MIN_INTERVAL` and retry HTTP 429 and 5xx with `Retry-After` when it is present. A block page, an empty table, or a short fixture list fails that season and does not replace its stored rows. A fixture row that lists the same club id as home and away is omitted. Two clubs that share a name and have different badge ids both stay.

Divisions A and B accept a snapshot of at least 100 matches and 18–22 standings rows. Division C accepts at least 40 matches and 8–40 rows. Division D accepts at least 20 matches and 16–128 rows. Brazil Cup accepts at least 100 matches and exactly zero standings rows. Módulo I and Módulo II accept at least 30 matches and 8–24 rows. Segunda Divisão accepts at least 20 matches and 8–48 rows. The Distrito Federal, Goiás, and Pernambuco divisions accept at least 8 matches and 4–40 rows. Maranhão, Mato Grosso, Alagoas, and Bahia accept at least 8 matches and 4–48 rows.

The Mineiro pages are `https://www.fmf.com.br/Competicoes/ProxJogos.aspx?d=1` (Módulo I), `d=2` (Módulo II), and `d=3` (Segunda Divisão). A season is a form POST of `request` set to the numeric option on that page. The collector refuses any other host and any other path.

Each state adapter is locked to one host. Paths come from that adapter. A client-supplied URL is never fetched.

| Adapter | `source.name` | Host | Competitions |
| --- | --- | --- | --- |
| `cbf` | `cbf` | `www.cbf.com.br` | Brazilian Soccer League divisions A–D, Brazil Cup |
| `fmf` | `fmf` | `www.fmf.com.br` | Mineiro |
| `portal` | `ffdf` | `www.ffdf.com.br` | Candangão |
| `portal` | `fgf-go` | `fgf.esp.br` | Goiano |
| `portal` | `fpf-pe` | `www.fpf-pe.com.br` | Pernambucano |
| `uikit` | `fmf-ma` | `www.futebolmaranhense.com.br` | Maranhense |
| `uikit` | `fmf-mt` | `www.fmfmt.com.br` | Mato-Grossense |
| `fafal` | `faf-al` | `www.futeboldealagoas.net` | Alagoano |
| `fbf` | `fbf` | `www.fbf.org.br` | Baianão |

Sync takes a Postgres advisory lock per competition and season. An empty page, a broken table, or a non-HTML response is a failed collection.

## Development

```
cmd/
  api/                 # HTTP server. Reads PostgreSQL only.
  worker/              # scheduled sync, and worker -once for a manual sync
  migrate/             # applies versioned SQL. The API and the worker do not migrate.
internal/
  domain/              # types and snapshot validation
  scraper/             # cbf, fmf, portal, uikit, fafal, fbf. Selectors stay here.
  catalog/             # competition codes. "all" is this list.
  service/             # sync and read use cases
  repository/postgres/ # the only SQL
  httpapi/             # HTTP boundary
  config/              # environment variables, validated at startup
migrations/            # forward-only SQL
docs/openapi.yaml      # the contract served at /openapi.yaml
legacy/                # the previous PHP scraper and the static front end
```

Handlers never call a federation site. `docs/openapi.yaml` is the contract. HTML files are excluded from GitHub's language statistics via `.gitattributes`. They remain tracked when the project needs them.

```sh
make fmt
make vet
make test
make vuln
make openapi
make check
```

`go test ./...` does not call the CBF site or any state federation site. Parser tests use fixtures under `internal/scraper`. The PostgreSQL test creates a schema named `it_` plus random hex, migrates that schema, and drops only that schema. It runs when `DATABASE_URL` is set. `CBF_LIVE=1` is reserved and is not required. Live checks for a new host follow the same opt-in idea and stay out of the default test run.

`go test -race` needs cgo. The Compose `test` profile runs it inside the Go 1.26.10 image. The module uses Go 1.26 because the current `golang.org/x/net` release requires it. `govulncheck ./...` reports no known vulnerabilities with this toolchain and dependency set.

Open work is listed in [ROADMAP.md](ROADMAP.md). What changed is listed in [CHANGELOG.md](CHANGELOG.md).

## License

This repository does not include a license file.
