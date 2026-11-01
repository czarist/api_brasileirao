# Agent guide

This file is the durable instruction set for anyone changing this repository.
The project language is English: code, identifiers, comments, API fields,
OpenAPI, logs, command output, and these documents. Keep official proper names
of clubs, competitions, stadiums, and organizations as published by the source.
Talk to the maintainer in whatever language they use.

## Context and architecture

Public read API for the Campeonato Brasileiro, Copa do Brasil, and the adult
professional divisions published by the state federations that have an adapter.
Collected today: CBF Série A–D and Copa do Brasil; FMF Minas Gerais (Módulo I,
Módulo II, Segunda Divisão); FFDF Candangão Série A and B; FGF Goiás (fgf.esp.br)
1ª Divisão, Divisão de Acesso, and 3ª Divisão; FPF Pernambuco A1, A2, and A3;
FMF Maranhão Série A and B; FMF Mato Grosso 1ª Divisão and II Divisão; FAF
Alagoas Série A and, only for years the catalog names it, Série B; FBF Bahia
Baianão Série A and Série B; FCF Santa Catarina Série A, Série B, and Série C
for the years on https://fcf.com.br/competicoes/profissionais/. Federations
without a working public catalog are not collected.
Data is collected from those public sites and stored in PostgreSQL. HTTP
handlers only read the database.

```
CBF public HTTP       ->  cbf adapter    ->  validation  ->  PostgreSQL  ->  API
FMF public HTTP       ->  fmf adapter    ->
FFDF, FGF, FPF-PE     ->  portal adapter ->
FMF-MA, FMF-MT        ->  uikit adapter  ->
FAF public HTTP       ->  fafal adapter  ->
FBF public HTTP       ->  fbf adapter    ->
FCF public HTTP       ->  fcf adapter    ->
```

Layout:

- `cmd/api` serves HTTP.
- `cmd/worker` runs scheduled sync and `worker -once` for a manual sync.
- `cmd/migrate` applies versioned SQL migrations. API and worker do not migrate.
- `internal/domain` holds types and snapshot validation.
- `internal/scraper/cbf` is the CBF adapter. Selectors and JSON shapes stay here.
- `internal/scraper/fmf` is the Minas Gerais adapter. It requests only `https://www.fmf.com.br/Competicoes/ProxJogos.aspx?d=1`, `d=2`, or `d=3`. It does not use `SOURCE_BASE_URL`.
- `internal/scraper/portal` collects FFDF (`www.ffdf.com.br`), Goiás FGF (`fgf.esp.br`, not `www.fgf.com.br`), and FPF-PE (`www.fpf-pe.com.br`). Each client is locked to its host.
- `internal/scraper/uikit` collects Maranhão (`www.futebolmaranhense.com.br`) and Mato Grosso (`www.fmfmt.com.br`). Each client is locked to its host.
- `internal/scraper/fafal` collects Alagoas from `https://www.futeboldealagoas.net` only.
- `internal/scraper/fbf` collects the adult Baianão from `https://www.fbf.org.br` only. Division paths are `/competicoes/1` and `/competicoes/2`. It does not use `SOURCE_BASE_URL`.
- `internal/scraper/fcf` collects adult Catarinense Série A, B, and C from `https://fcf.com.br` only. Paths are `/competicoes/profissionais/`, `/{year}/`, and `/{year}/{id}/`. It does not use `SOURCE_BASE_URL`. Cups, Recopa, women's, and youth options are skipped.
- `internal/catalog` expands `all` to the CBF codes, then FMF, portal, UIKit, FAF, FBF, and FCF. `cbf.ParseCodes("all")` stays the five CBF codes. Do not pass `all` to a one-shot unless a full re-walk is intended.
- `internal/service` orchestrates sync and read use cases.
- `internal/repository/postgres` is the only SQL.
- `internal/httpapi` is the HTTP boundary.
- `internal/config` loads and validates environment variables.
- `migrations` holds forward-only SQL.
- `docs/openapi.yaml` is the API contract. `docs/swagger.html` renders it.

Dependencies in use, and why:

- `jackc/pgx/v5` for PostgreSQL.
- `golang.org/x/net/html` to parse the standings table.
- `golang.org/x/time/rate` for source and API rate limits.
- `gopkg.in/yaml.v3` to parse the OpenAPI document in tests.

Go 1.22+ `net/http` ServeMux provides method and path parameters, so there is no
router library. Logging uses `log/slog`. Do not add Redis, a queue, or another
service without a demonstrated need.

## Workflow

- Read `AGENTS.md` and `ROADMAP.md` before implementing.
- Check `git status` and keep unrelated local changes.
- Name the roadmap item the change completes.
- Ship working increments. Update `ROADMAP.md`, `README.md`, and `CHANGELOG.md` with the change.
- Do not force-push, deploy, change the license, or publish remote changes unless the maintainer asks.
- The repository has no license file. Do not add one without an explicit maintainer decision.
- A task is done only when its acceptance checks have been run.
- Record a concrete blocker instead of marking unfinished work done.
- Do not edit an applied migration. Add a new file. Describe any destructive migration and its impact.
- Do not hand-edit generated files. This repository does not generate the OpenAPI document; edit `docs/openapi.yaml` and keep the handlers aligned with it.
- Preserve `/api/v1` compatibility or document the break in the changelog.

## Conventions

- Run `gofmt` on Go code.
- Pass `context.Context` through I/O.
- Wrap errors with context. Do not return internal details to API clients.
- Store timestamps in UTC and render them as RFC 3339.
- Configure the process with environment variables validated at startup.
- Never commit `.env`, credentials, or scraped private data.
- A missing score is SQL `NULL`, never `0`.
- A missing kickoff is SQL `NULL`.
- Keep the official standings figures, including a goal difference that does not match the raw goals.
- Prefer the Docker workflow in the Makefile for reproduction.

## Data and scraping

- Public handlers must not call the source.
- Sync is idempotent and guarded by a Postgres advisory lock per competition and season.
- A failed or incomplete collection must not delete or replace the last valid rows.
- An empty page, a broken table, or a non-HTML/non-JSON block response is a failed collection.
- Do not invent scorers, seasons, or clubs the source did not publish.
- Parser changes need fixture and test updates.
- Default tests must not call the source websites. Live checks are opt-in with `CBF_LIVE=1`.
- Source URLs come from configuration and fixed path templates. Do not pass a client-supplied URL to the fetcher.
- Standings come from the division year page. When that page has no table, the collector follows the first points phase on the page and skips knockout phases.
- Copa do Brasil is championship `24`, professional category `4`. It has no standings page. Do not invent a table. Omit a fixture row that lists the same club id on both sides.
- The Mineiro divisions keep the published names. Do not invent a Série C, a club, or a match the page did not publish. Store the classification once per club. The FMF table does not publish cards, so those counts stay 0. A trailing `*` is a footnote, not part of the number.
- State adapters keep the published division name. Do not add a Série C, A2, or A3 the catalog did not list. A year with no adult card is omitted. A season with no matches or no standings is not stored. A later phase table that repeats clubs already stored is skipped. Do not bypass a 403, a TLS failure, or a DNS failure.
- Do not call `cms.cbf.com.br` or keep a bearer token found in page scripts.
- Respect `Retry-After`, the configured request interval, and response size limits. Do not bypass CAPTCHA, authentication, or access controls.

## Verification

Official commands, all through Docker:

- `make setup`
- `make build`
- `make up`
- `make down`
- `make logs`
- `make migrate`
- `make scrape COMPETITION=brasileirao-serie-a YEAR=2026`
- `make test`
- `make fmt`
- `make vet`
- `make vuln`
- `make openapi`
- `make check`

`make down` does not remove volumes. `make reset CONFIRM=yes` deletes the Compose volumes and the database data inside them.

`make test` creates a temporary schema and drops it. It must not be pointed at a database you cannot discard, but it does not drop the `public` schema.

Before finishing a change, run the checks that the change can affect and say which checks did not run.
