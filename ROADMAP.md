# Roadmap

The running API covers Série A, Série B, Série C, Série D, the professional Copa do Brasil, the professional Campeonato Mineiro, and the adult state divisions in phase 13, including Baianão Série A and Série B: real collection, PostgreSQL, read-only HTTP, OpenAPI, and Docker. Phase 10 is not implemented. The state federations that are not listed in phase 13 are not collected.

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
- [ ] A second source adapter if CBF pages change or coverage is insufficient. The FMF adapter in phase 12 covers the Mineiro professional divisions only. It does not close this item.
- [ ] Stronger in-progress detection when a source publishes that state. The enum exists; the CBF adapter does not emit `in_progress`.

## 11. Copa do Brasil results

Objective: serve the professional Copa do Brasil match results for every year the CBF catalog lists, including the current year.

Depends on: phases 4 to 9.

- [x] Championship `24`, professional category `4`. Other categories that reuse the name are not collected.
- [x] Matches, phases, scores, and penalty scores from the public fixture feed. No standings table is invented. The standings route returns an empty list.
- [x] A fixture row that lists the same club id on both sides is omitted. Two clubs that share a name and have different badge ids both stay.

Acceptance, checked on 2026-10-31 against the Compose database and the API:

- Nine seasons, 2018–2026. Match counts: 2018 and 2020 have 120. 2019 has 118. 2021–2025 have 122. 2026 has 154. Standings are 0 in every year. `GET /api/v1/competitions/copa-do-brasil/seasons/2026/standings` returned 200 and an empty list.
- 2018 final, served by the matches route: Cruzeiro Esporte Clube 1-0 Corinthians, then Corinthians 1-2 Cruzeiro Esporte Clube. Both finished, with null penalty scores.
- 2026 has four scheduled semi-finals with null scores: Palmeiras vs Vasco da Gama Saf, Grêmio vs Atlético Mineiro, and the return legs. The feed has no final.
- 2019 omitted matches `813493` and `813503`. Both sides used club id `35938` and the name Bragantino. The other 118 matches were stored.
- Série A 2018–2026 stayed at 380 matches and 20 standings. Série A 2026 was still led by Flamengo with 73 points.

## 12. Campeonato Mineiro

Objective: serve the adult professional divisions published by the Federação Mineira de Futebol, under the names on that catalog.

Depends on: phases 4 to 9.

- [x] Módulo I, Módulo II, and Segunda Divisão. Youth, women's, and amateur competitions on the same site are not collected. No Série A, B, or C is invented.
- [x] Every year the FMF select lists. On 2026-11-01 that was Módulo I 2013–2026 and the other two divisions 2014–2026.
- [x] Matches for every published phase. The classification table is stored once per club. A later table that only repeats those clubs is skipped.
- [ ] Every remaining state federation. Each one is an open task in phase 13. Distrito Federal, Goiás, Pernambuco, Maranhão, Mato Grosso, Alagoas, Bahia, Santa Catarina, Paraná, and Rio Grande do Sul are collected.

Acceptance, checked on 2026-11-01 against the Compose database and the API:

- 40 seasons. Módulo I 2026 has 53 matches and 12 standings. Group A is led by Atlético SAF with 14 points. `GET /api/v1/competitions/mineiro-modulo-i/seasons/2026/matches?page_size=2` returned total 53. The first row is Uberlândia S.A.F 0-0 Tombense, kickoff `2026-01-10T19:00:00Z`, phase `CLASSIFICATÓRIA`.
- Módulo I 2020 has 75 matches, one of them with null scores. Módulo II 2026 has 74 matches and stores Ipatinga at -2 points.
- Série A 2018–2026 stayed at 380 matches and 20 standings.
- The long-running worker container was not recreated. Its hourly job still runs the previous binary.

## 13. State federations, one catalog at a time

Objective: serve the adult top divisions each state federation publishes, under the name on that catalog. Do not invent a division the page does not list.

Depends on: phases 4 to 9.

- [x] Distrito Federal, FFDF, `www.ffdf.com.br`, source `ffdf`. Candangão Série A and Série B, 2023–2026. No Série C.
- [x] Goiás, FGF, `fgf.esp.br`, source `fgf-go`. This is not `www.fgf.com.br`. 1ª Divisão, Divisão de Acesso, and 3ª Divisão. 1ª Divisão 2025 was not stored. 3ª Divisão has no 2020 card.
- [x] Pernambuco, FPF, `www.fpf-pe.com.br`, source `fpf-pe`. A1, A2, and A3 when the catalog lists them. A1 and A2 2019 and A3 2024 were not stored.
- [x] Maranhão, `www.futebolmaranhense.com.br`, source `fmf-ma`. Série A and Série B. No Série C. Three seasons published no classification and were not stored.
- [x] Mato Grosso, `www.fmfmt.com.br`, source `fmf-mt`. 1ª Divisão 2020–2026 and II Divisão 2021–2026. No 3ª Divisão.
- [x] Alagoas, FAF, `www.futeboldealagoas.net`, source `faf-al`. Série A for 2020 and 2022–2026. Série B for 2022–2024. The 2021 catalog has no adult Série A. Later B cards are Sub-23. No Série C.
- [x] Bahia, FBF, `www.fbf.org.br`, source `fbf`. Baianão Série A 2011–2026 and Série B 2011–2026. Série A 2027 listed five fixtures and was not stored. No Série C.
- [ ] Acre. The probed hosts returned 403 or did not resolve. Do not bypass the 403.
- [ ] Amapá, `fafamapa.com.br`. The professional pages are a landing site, not a match and standings catalog. No Série C in the menu.
- [ ] Amazonas, `fafamazonas.com.br`. Adult Série A 2023–2026 and Série B 2022–2026 are published. No adult Série C. Each season page is one round, with the knockout blocks repeated on the round pages.
- [ ] Ceará. No current public season catalog was confirmed.
- [ ] Espírito Santo, `futebolcapixaba.com`. The page did not show adult division names.
- [ ] Mato Grosso do Sul, `www.futebolms.com.br`. The page links out to a second host. That host stays off the allowlist until a full public season page is fetched.
- [ ] Pará, `www.fpfpara.com.br`. Collect A1, A2, and A3. Pré-A3 is a fourth published name. Do not store it as Série C.
- [ ] Paraíba. The TLS connection closed. Do not disable certificate checks.
- [x] Paraná, FPF, `federacaopr.com.br`, source `fpf-pr`. Paranaense, Segundona, and Terceirona, 2026 only. Copa Paraná stays out. The names are not renamed to Série A, B, or C. The table does not publish cards. The win percentage is the published decimal, rounded by the first decimal digit.
- [ ] Piauí. Certificate validation failed. Do not disable certificate checks.
- [ ] Rio de Janeiro, `www.fferj.com.br`. Five masculine divisions are published: Série A, A2, B1, B2, and C. Copa Rio, women's, and youth stay out. A second host stays off the allowlist until a full public season page is fetched.
- [ ] Rio Grande do Norte, `www.fnf.org.br`. No competition catalog was confirmed.
- [x] Rio Grande do Sul, FGF, `www.fgf.com.br`, source `fgf-rs`. This is not Goiás `fgf.esp.br`. Gauchão 2019–2026, Gauchão Série A2 2019–2026, and Gauchão Série B 2019 and 2021–2026. Copa FGF and Recopa stay out. No Série C. Empty year shells and Gauchão 2018 (one published match) were not stored. The Série B 2020 year page is the 2019 competition and was not stored. The table publishes cards and the win percentage. A blank position badge keeps the row order. A row whose stat cells are all empty is omitted. A match external id is the division id, the catalog year, and the jogo slug.
- [ ] Rondônia, `www.ffer.com.br`. No match catalog was confirmed.
- [ ] Roraima, `www.futebolroraimense.com.br`. The host resolves. The fetched page has no standings table. Do not invent Série A, B, or C.
- [x] Santa Catarina, FCF, `fcf.com.br`, source `fcf`. Série A, Série B, and Série C, 2023–2026. Cups, Recopa, women's, and youth stay out. The year index lists no earlier season. The table does not publish cards or a win percentage, so those fields are 0. Série C 2024 and 2025 publish 3 clubs. A named phase with no round number is stored as round 1.
- [ ] São Paulo, `www.futebolpaulista.com.br`. The host returned 403. Do not bypass it. The adult divisions are A1, A2, and A3.
- [ ] Sergipe, `fsf-se.com.br`. The championship index publishes adult Sergipano A1, A2, and, where the card exists, A3. Copa Lotese and Copa Governo do Estado stay out.
- [ ] Tocantins, `www.ftf.org.br`. No competition catalog was found.
- [ ] Goiano 1ª Divisão 2025. Match `41180` lists the same club twice, so that season was not stored.
- [ ] Pernambucano A1 and A2 2019 published no matches. A3 2024 badge `35767` has no published name.
- [ ] Maranhense Série A 2026 and Série B 2024 and 2026 published no classification.

Acceptance, checked on 2026-11-01 against the Compose database and the API:

- Alagoano Série A 2026: 34 matches and 8 standings. `GET /api/v1/competitions/alagoano-serie-a/seasons/2026/standings` returned source `faf-al`, Asa, `GRUPO A`, 17 points, 7 played.
- Baianão Série A 2026: 48 matches and 10 standings. `GET /api/v1/competitions/baiano-serie-a/seasons/2026/standings` returned source `fbf`, Bahia, `Grupo 1`, 23 points, 9 played. Série B 2013 has 46 matches and 10 standings.
- Catarinense Série A 2023–2026: 80/12, 80/12, 74/12, 70/12. Série B 2023–2026: 50/9, 55/10, 46/9, 90/10. Série C 2023–2026: 21/6, 8/3, 8/3, 14/4. `GET /api/v1/competitions/catarinense-serie-a/seasons/2026/standings` returned source `fcf`, Brusque SAF, `GRUPO A`, 12 points, 6 played, yellow 0, red 0, win percentage 0. Chapecoense 1-1 Brusque SAF kicked off at `2026-01-06T22:00:00Z`.
- Paranaense 2026: 54 matches and 12 standings. `GET /api/v1/competitions/paranaense/seasons/2026/standings` returned source `fpf-pr`, Londrina, `1ª FASE - GRUPO A`, 22 points, 12 played, win percentage 61. Segundona 2026: 59/10, led by Paraná Clube with 37 points. Terceirona 2026: 44/11.
- Gauchão 2019–2026: 80/12, 74/12, 72/12, 72/12, 72/12, 76/12, 72/12, 64/12. Série A2 2019–2026: 126/16, 108/16, 126/16, 126/16, 126/16, 126/16, 119/16, 134/16. Série B 2019 and 2021–2026: 74/12, 66/14, 62/16, 62/16, 30/8, 38/9, 47/10. `GET /api/v1/competitions/gauchao/seasons/2026/standings` returned source `fgf-rs`, Internacional, `GRUPO A`, 15 points, 6 played, yellow 17, red 0, win percentage 83. Juventude 1-1 Ypiranga kicked off at `2026-01-10T19:00:00Z`. Gauchão 2018 and Série B 2020 are absent. Série B 2019 has 74 matches.
- Série A 2026 stayed source `cbf`, led by Flamengo with 73 points.
- The hourly worker container `df275f704aeb` was still image `3d5a314de6ce` after the Bahia, Santa Catarina, Paraná, and Rio Grande do Sul one-shots. Docker Desktop died once; the same containers were started again.
