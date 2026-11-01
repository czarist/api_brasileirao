# Changelog

All notable changes to this project are documented in this file.

The format is based on Keep a Changelog. This project does not publish version tags from the rewrite itself until the maintainer asks.

## [Unreleased]

### Added

- Go rewrite of the Brasileirão read API, replacing the PHP table scraper. The previous PHP and front-end files are preserved under `legacy/`.
- PostgreSQL schema, versioned migrations, and a Docker Compose stack for Postgres, migrations, the API, and the worker.
- CBF adapter for Série A: competition catalog, match table, official standings, and schedule-change notes.
- CBF collection for Série B, Série C, and Série D, and for every year the public catalog lists. On 2026-10-31 that catalog covered 2018–2026, 36 seasons.
- CBF collection for the professional Copa do Brasil (championship `24`, category `4`) for every catalog year. On 2026-10-31 that was 2018–2026. The competition is knockout, so a season stores matches and no standings. The standings route returns an empty list. A fixture row with the same club id on both sides is omitted. In 2019 that removed matches `813493` and `813503`, both published as Bragantino against Bragantino, leaving 118 matches. 2026 stored 154 matches, including four scheduled semi-finals with null scores and no final.
- FMF collection for the adult professional Campeonato Mineiro: Módulo I (`mineiro-modulo-i`, 2013–2026), Módulo II (`mineiro-modulo-ii`, 2014–2026), and Segunda Divisão (`mineiro-segunda-divisao`, 2014–2026). Those are the published names. The adapter does not invent a Série A, B, or C. On 2026-11-01 a one-shot stored all 40 seasons the FMF year select listed. The classification table is stored once per club. Later phase tables that only repeat those clubs are skipped. Matches keep every published phase. The FMF table does not publish cards, so yellow and red cards are 0. A trailing `*` on a figure is a footnote; the number in the cell is stored. Módulo II 2026 stores Ipatinga at -2 points. Pages from 2013 and 2014 publish no `data-jogo` and no game number. Those matches use the page's map code as the external id, and `match_number` is null. `source.name` is `fmf` for those seasons. The long-running worker container was not recreated, so its hourly job still runs the previous binary.
- FFDF, Goiás FGF (`fgf.esp.br`), and FPF-PE collection through one portal adapter. Candangão Série A and Série B, 2023–2026, source `ffdf`. Goiano 1ª Divisão, Divisão de Acesso, and 3ª Divisão under those names, source `fgf-go`. Pernambucano A1, A2, and A3, source `fpf-pe`. No Candangão Série C. Goiano 1ª Divisão 2025 was not stored because match `41180` lists the same club twice. Pernambucano A1 and A2 2019 had no matches. Pernambucano A3 2024 was not stored because badge `35767` has no published name.
- Maranhão and Mato Grosso collection through the UIKit adapter. Maranhense Série A 2020–2025 and Série B 2020–2023 and 2025, source `fmf-ma`. Série A 2026 and Série B 2024 and 2026 published no classification and were not stored. Mato-Grossense 1ª Divisão 2020–2026 and II Divisão 2021–2026, source `fmf-mt`. No Maranhense Série C and no Mato-Grossense 3ª Divisão.
- FAF collection for Alagoano Série A (`alagoano-serie-a`) in 2020 and 2022–2026, and Série B (`alagoano-serie-b`) in 2022–2024. Source `faf-al`. The 2021 catalog has no adult Série A. The 2025 and 2026 Série B cards are Sub-23 and are not collected. Shoot-out scores are stored beside the 90-minute score. `GET /api/v1/competitions/alagoano-serie-a/seasons/2026/standings` returned source `faf-al`, Asa, `GRUPO A`, 17 points. Série A 2026 stayed source `cbf`.
- FBF collection for Baianão Série A (`baiano-serie-a`) in 2011–2026 and Série B (`baiano-serie-b`) in 2011–2026. Source `fbf`. There is no adult Série C. Série A 2027 is on the select with five fixtures and was not stored. The table does not publish cards, so yellow and red are 0. The published win percentage is a decimal; the integer column keeps it rounded by the first decimal digit. A `(0)` pair printed on older score cards is a placeholder and is not stored. A shoot-out with at least one non-zero figure is stored, including a published 0 on the other side. Clubs that share the placeholder badge `default.png` are stored under the published name. `GET /api/v1/competitions/baiano-serie-a/seasons/2026/standings` returned source `fbf`, Bahia, `Grupo 1`, 23 points. Série A 2026 stayed source `cbf`. The hourly worker container was not recreated.
- Roadmap phase 13 tracks every state federation that still has no adapter: Acre, Amapá, Amazonas, Ceará, Espírito Santo, Mato Grosso do Sul, Pará, Paraíba, Paraná, Piauí, Rio de Janeiro, Rio Grande do Norte, Rio Grande do Sul, Rondônia, Roraima, Santa Catarina, São Paulo, Sergipe, and Tocantins. Acre and São Paulo returned 403. Paraíba closed the TLS connection. Piauí failed certificate validation. Roraima and Sergipe did not resolve. Also still open inside collected catalogs: Goiano 1ª Divisão 2025, Pernambucano A1 and A2 2019, Pernambucano A3 2024, and Maranhense Série A 2026 plus Série B 2024 and 2026.
- Standing field `group_name`, the official group title. It is empty when the page has no group title. A single table is often published as `GRUPO ÚNICO`. Grouped divisions repeat `position` inside each group. Rows are ordered by the published group order, then by position.
- Worker selection `SYNC_COMPETITION=all` or a comma-separated list, and `SYNC_YEARS=all`. Compose sets both. Each season has its own timeout. One failed season does not replace the rows of the others.
- Read API under `/api/v1`, health checks, OpenAPI 3.1, and Swagger UI.
- Sync worker with a manual one-shot command, advisory locking, and retention of the last valid snapshot when a collection fails.

### Changed

- Updated Go dependencies and the build/test images to Go 1.26.10.
- Excluded HTML files from GitHub language statistics without removing them from the repository.
- Migration `002_standings_group.sql` replaces the standings unique key `(season_id, position)` with `(season_id, group_name, position)`.
- A year page with no standings table is followed by the first points phase published on that page. Knockout phases are skipped.
- Completeness limits are per division. Série A and Série B require 100 matches and 18–22 rows. Série C requires 40 matches and 8–40 rows. Série D requires 20 matches and 16–128 rows. Copa do Brasil requires 100 matches and zero standings rows. Módulo I and Módulo II require 30 matches and 8–24 rows. Segunda Divisão requires 20 matches and 8–48 rows. Portal divisions require 8 matches and 4–40 rows. UIKit, FAF, and FBF divisions require 8 matches and 4–48 rows. `SYNC_MIN_MATCHES`, `SYNC_MIN_STANDINGS`, and `SYNC_MAX_STANDINGS` remain the fallback.
- `source.name` is the adapter stored on the rows (`cbf`, `fmf`, `ffdf`, `fgf-go`, `fpf-pe`, `fmf-ma`, `fmf-mt`, `faf-al`, or `fbf`). A response that lists every competition uses the adapter of the newest stored row.

### Security

- API and worker containers run as a non-root user.
- Configuration comes from the environment. `.env.example` has local defaults only.
- Public routes are read-only and rate limited. Source URLs are not taken from clients.
- `govulncheck ./...` reports no known vulnerabilities with Go 1.26 and the updated dependencies.
