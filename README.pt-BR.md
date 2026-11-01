<h1 align="center">API Brasileirão</h1>

<h3 align="center">A tabela oficial. A temporada na URL. Servida pelo PostgreSQL.</h3>

<p align="center">
  <a href="README.md">English</a> · <a href="README.pt-BR.md">Português (Brasil)</a>
</p>

<p align="center">
  <b>Uma API pública de leitura do Campeonato Brasileiro e das divisões adultas estaduais que já têm adaptador, escrita em Go.</b><br>
  Um worker coleta as páginas públicas das federações no PostgreSQL. A chamada HTTP só lê esse banco.
</p>

<p align="center">
  <a href="https://go.dev"><img alt="Go 1.26" src="https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go&logoColor=white"></a>
  <a href="https://www.postgresql.org"><img alt="Armazenamento: PostgreSQL" src="https://img.shields.io/badge/storage-PostgreSQL-4169E1?style=flat-square&logo=postgresql&logoColor=white"></a>
  <a href="https://docs.docker.com/compose/"><img alt="Sobe com Docker Compose" src="https://img.shields.io/badge/run-Docker%20Compose-2496ED?style=flat-square&logo=docker&logoColor=white"></a>
  <a href="docs/openapi.yaml"><img alt="Contrato: OpenAPI 3.1" src="https://img.shields.io/badge/contract-OpenAPI%203.1-6BA539?style=flat-square&logo=openapiinitiative&logoColor=white"></a>
  <img alt="API pública de leitura, versão de contrato 0.1.0" src="https://img.shields.io/badge/API-read--only%20%C2%B7%200.1.0-111111?style=flat-square">
  <a href="https://github.com/czarist/api_brasileirao/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/czarist/api_brasileirao/ci.yml?label=ci&style=flat-square&logo=githubactions&logoColor=white"></a>
</p>

<br>

<p align="center">
  <a href="#o-que-ela-serve">O que ela serve</a> ·
  <a href="#o-que-uma-resposta-significa">Respostas</a> ·
  <a href="#o-que-já-foi-coletado">Coletado</a> ·
  <a href="#o-que-ainda-falta">Em aberto</a> ·
  <a href="#começo-rápido">Começo rápido</a> ·
  <a href="#chamadas">Chamadas</a> ·
  <a href="#fonte">Fonte</a> ·
  <a href="#desenvolvimento">Desenvolvimento</a>
</p>

<br>

## O que ela serve

HTTP só de leitura para as competições profissionais adultas abaixo. O ano na URL escolhe a temporada. Competição ou temporada desconhecida responde 404. É uma cópia gravada, atualizada no intervalo do worker (1 hora por padrão). Não é um placar ao vivo.

Este projeto não tem vínculo com a CBF, a FMF, nenhum clube nem nenhuma emissora.

- **Campeonato Brasileiro.** Série A, Série B, Série C e Série D. Códigos `brasileirao-serie-a`, `brasileirao-serie-b`, `brasileirao-serie-c`, `brasileirao-serie-d`. Fonte `cbf`.
- **Copa do Brasil,** profissional. Código `copa-do-brasil`. A copa tem jogos e uma classificação vazia.
- **Campeonato Mineiro.** Módulo I, Módulo II e Segunda Divisão. Códigos `mineiro-modulo-i`, `mineiro-modulo-ii`, `mineiro-segunda-divisao`. Fonte `fmf`.
- **Candangão.** Série A e Série B. Códigos `candangao-serie-a`, `candangao-serie-b`. Fonte `ffdf`. O catálogo não tem Série C.
- **Campeonato Goiano.** 1ª Divisão, Divisão de Acesso e 3ª Divisão. Códigos `goiano-primeira-divisao`, `goiano-divisao-de-acesso`, `goiano-terceira-divisao`. Fonte `fgf-go`.
- **Campeonato Pernambucano.** A1, A2 e A3. Códigos `pernambucano-a1`, `pernambucano-a2`, `pernambucano-a3`. Fonte `fpf-pe`.
- **Campeonato Maranhense.** Série A e Série B. Códigos `maranhense-serie-a`, `maranhense-serie-b`. Fonte `fmf-ma`. O catálogo não tem Série C.
- **Mato-Grossense.** 1ª Divisão e II Divisão. Códigos `mato-grossense-primeira-divisao`, `mato-grossense-segunda-divisao`. Fonte `fmf-mt`. O catálogo não tem 3ª Divisão.
- **Campeonato Alagoano.** Série A, e Série B quando o catálogo nomeia uma Série B adulta. Códigos `alagoano-serie-a`, `alagoano-serie-b`. Fonte `faf-al`. O catálogo não tem Série C.
- **Baianão.** Série A e Série B. Códigos `baiano-serie-a`, `baiano-serie-b`. Fonte `fbf`. O catálogo não tem Série C adulta.
- **Campeonato Catarinense.** Série A, Série B e Série C. Códigos `catarinense-serie-a`, `catarinense-serie-b`, `catarinense-serie-c`. Fonte `fcf`.

`all` é essa lista, nessa ordem: o catálogo da CBF, depois o Mineiro, depois as divisões estaduais em `internal/catalog`. Base, feminino, amador e copas no site de uma federação ficam de fora. Divisão que o catálogo não nomeia não é inventada.

O scraper antigo em PHP e o front estático ficam em `legacy/`, só como referência.

## O que uma resposta significa

- **Placar ausente, horário ausente.** Placar e horário que a fonte não publicou vêm `null`. Um 0-0 de verdade continua 0-0.
- **Horários.** O início do jogo é gravado em UTC e devolvido em RFC 3339.
- **Tabela oficial.** Pontos, saldo de gols e os outros números são copiados, inclusive um saldo que não fecha com os gols. `group_name` é o nome oficial do grupo. Fica vazio quando a página não tem título de grupo. Uma tabela única costuma sair como `GRUPO ÚNICO`. Divisão com grupos repete `position` dentro de cada grupo. As linhas seguem a ordem publicada dos grupos e, dentro do grupo, a posição. A Copa do Brasil não tem tabela, então essa lista vem vazia. Uma fase posterior do Mineiro que só repete clubes já gravados na classificação é ignorada.
- **Cartões.** As tabelas da FMF, da FBF e da FCF não publicam cartões, então `yellow_cards` e `red_cards` são 0 nessas temporadas. Um `*` no fim do número é nota de rodapé. O inteiro da célula é o que fica gravado. Um 0 publicado continua 0.
- **Fonte.** `source.name` é o adaptador que gravou as linhas: `cbf`, `fmf`, `ffdf`, `fgf-go`, `fpf-pe`, `fmf-ma`, `fmf-mt`, `faf-al`, `fbf` ou `fcf`. A lista de competições usa o adaptador da linha mais nova.
- **Pênaltis.** O placar de pênaltis é copiado quando a fonte publica, inclusive um 0 de um dos lados. Jogo sem disputa fica com pênaltis `null`. Um par `(0)` num cartão antigo do Baianão é placeholder e não é gravado.
- **Aproveitamento no Baianão.** A página da FBF imprime um decimal. Ele é gravado como inteiro, arredondado pelo primeiro dígito decimal, metade para cima.
- **Aproveitamento no Catarinense.** A página da FCF não publica esse número, então o inteiro gravado é 0.
- **Adiado.** Jogo adiado mantém a rodada original.
- **Frescor.** `source.collection_status` é `ok` depois de uma sincronização que deu certo e `never_collected` quando ainda não há nada gravado. `source.stale` é verdadeiro quando o último sucesso é mais velho que `STALE_AFTER` (6 horas por padrão) ou quando uma coleta posterior falhou. As linhas anteriores ficam. O status de jogo `in_progress` existe no schema. O coletor atual não produz esse status.
- **Frequência.** O intervalo padrão do worker é 1 hora.

## O que já foi coletado

As contagens são jogos/linhas de classificação. Ano que o catálogo não lista não existe aqui. Temporada que falhou na validação não foi gravada, e uma temporada que falha não substitui as linhas de outro ano.

Conferido em 2026-10-31. Uma sincronização do catálogo gravou 36 temporadas, 2018–2026, das Séries A, B, C e D.

- **Série A e Série B.** 380 jogos e 20 linhas de classificação em cada um desses anos. `group_name` fica vazio na Série A 2018 e na Série B 2018 e 2019. As outras temporadas de A e B usam `GRUPO ÚNICO`. A Série A 2026 era liderada pelo Flamengo, com 73 pontos em 33 jogos.
- **Série C.** 20 linhas em todos os anos: `GRUPO A` e `GRUPO B` em 2018–2021, e um grupo em 2022–2026. Um HTTP 429 posterior na Série C 2024 deixou as linhas gravadas no lugar.
- **Série D.** 2020 nomeia os oito grupos de `GRUPO A-01` a `GRUPO A-08`. 2026 tem 16 grupos, `GRUPO A1` a `GRUPO A16`, e 96 linhas. 2018 e 2019 têm 17 grupos de 4. A primeira passagem de 2021 viu uma página de classificação vazia. A seguinte gravou 518 jogos e 64 linhas. A primeira linha era Castanhal, `GRUPO A01`, posição 1, 36 pontos.

No mesmo dia, um one-shot gravou a Copa do Brasil profissional de 2018–2026. Os jogos são 120 em 2018 e 2020, 118 em 2019, 122 em 2021–2025 e 154 em 2026. A classificação é vazia em todos os anos. A final de 2018 é Cruzeiro Esporte Clube 1-0 Corinthians e Corinthians 1-2 Cruzeiro Esporte Clube. 2026 tem quatro semifinais agendadas, com placar nulo e sem final. 2019 omitiu duas linhas, `813493` e `813503`, porque os dois lados eram o Bragantino, clube `35938`. A Série A ficou igual depois dessa sincronização.

Em 2026-11-01 um one-shot gravou as três divisões profissionais do Mineiro para cada ano do select da FMF: Módulo I 2013–2026, Módulo II 2014–2026 e Segunda Divisão 2014–2026. O Módulo I 2026 tem 53 jogos e 12 linhas nos grupos A, B e C. O Atlético SAF lidera o grupo A com 14 pontos. Um jogo do Módulo I 2020 tem placar nulo. O Módulo II 2026 grava o Ipatinga com -2 pontos. A Série A 2018–2026 continuou com 380 jogos e 20 linhas. As páginas de 2013 e 2014 não publicam `data-jogo` nem número de jogo, então esses jogos usam o código de mapa da página como id externo e `match_number` fica nulo.

No mesmo dia, one-shots separados gravaram as divisões adultas abaixo.

- **Candangão.** Série A 2023–2026: 51/10, 51/10, 50/10, 45/10. Série B 2023–2026: 35/12, 33/8, 28/8, 26/7. Fonte `ffdf`.
- **Goiano.** 1ª Divisão 2019–2024 e 2026: 86/12, 79/12, 74/12, 74/12, 80/12, 80/12, 70/12. 2025 não foi gravado: o jogo `41180` lista o mesmo clube duas vezes. Divisão de Acesso 2019–2026: 43/8, 11/4, 30/6, 56/8, 56/8, 56/8, 56/8, 90/10. A 3ª Divisão não tem card de 2020. 2019 e 2021–2026: 57/13, 50/12, 44/13, 52/11, 56/13, 66/12, 21/7. Fonte `fgf-go`.
- **Pernambucano.** A1 2020–2026: 57/10, 57/12, 57/10, 84/13, 53/10, 53/10, 38/8. A2 2018 e 2020–2025: 56/15, 59/13, 60/14, 105/25, 66/12, 52/10, 43/10. A3 2023: 21/7. A1 e A2 de 2019 não tinham jogos. A3 2024 não nomeia o clube do escudo `35767`. Fonte `fpf-pe`.
- **Maranhense.** Série A 2020–2025: 38/8, 38/8, 36/8, 36/8, 66/8, 62/8. Série B 2020–2023 e 2025: 30/4, 31/10, 26/7, 24/8, 35/11. Série A 2026 e Série B 2024 e 2026 não publicaram classificação. Fonte `fmf-ma`.
- **Mato-Grossense.** 1ª Divisão 2020–2026: 61/10, 60/10, 55/10, 55/10, 57/10, 48/9, 53/10. II Divisão 2021–2026: 13/4, 25/10, 26/10, 41/11, 26/10, 28/10. Sem II Divisão em 2020. Fonte `fmf-mt`.
- **Alagoano.** Série A 2020 e 2022–2026: 34/8, 38/8, 36/8, 36/8, 36/8, 34/8. O catálogo de 2021 não tem Série A adulta. Série B 2022–2024: 27/7, 51/10, 51/10. Os cards de B em 2025 e 2026 são Sub-23. Fonte `faf-al`. `GET /api/v1/competitions/alagoano-serie-a/seasons/2026/standings` devolveu o Asa, grupo `GRUPO A`, 17 pontos. A Série A 2026 continuou `cbf`, liderada pelo Flamengo com 73 pontos.
- **Baianão.** Série A 2011–2026: 114/12, 140/12, 76/11, 76/11, 56/12, 56/12, 61/11, depois 51/10 em 2018–2024, 50/10 em 2025 e 48/10 em 2026. O select de 2027 lista cinco jogos e não foi gravado. Série B 2011–2026: 46/10, 46/10, 46/10, 47/10, 38/9, 32/6, 32/6, 31/6, 32/6, 17/6, 19/6, 72/12, 51/10, 51/10, 50/10, 51/10. Fonte `fbf`. Clubes que compartilham `default.png` ficam com o nome publicado. `GET /api/v1/competitions/baiano-serie-a/seasons/2026/standings` devolveu o Bahia, grupo `Grupo 1`, 23 pontos, 9 jogos.
- **Catarinense.** Série A 2023–2026: 80/12, 80/12, 74/12, 70/12. Série B 2023–2026: 50/9, 55/10, 46/9, 90/10. Série C 2023–2026: 21/6, 8/3, 8/3, 14/4. O índice de anos lista só 2023–2026. Copas, Recopa, feminino e base ficam de fora. Fonte `fcf`. `GET /api/v1/competitions/catarinense-serie-a/seasons/2026/standings` devolveu o Brusque SAF, grupo `GRUPO A`, 12 pontos, 6 jogos.

## O que ainda falta

As outras federações estaduais não estão nesta coleta. A fase 13 do [ROADMAP.md](ROADMAP.md) acompanha cada uma: Acre, Amapá, Amazonas, Ceará, Espírito Santo, Mato Grosso do Sul, Pará, Paraíba, Paraná, Piauí, Rio de Janeiro, Rio Grande do Norte, Rio Grande do Sul, Rondônia, Roraima, São Paulo, Sergipe e Tocantins.

Acre e São Paulo responderam 403. A Paraíba fechou a conexão TLS. O Piauí falhou na validação do certificado. Roraima e Sergipe não resolveram o host. O coletor não contorna HTTP 403, verificação de certificado nem host que não resolveu. O `www.fgf.com.br` do Rio Grande do Sul é outro site, diferente do `fgf.esp.br` de Goiás.

Dentro de catálogos que já são coletados, estas temporadas não foram gravadas: Goiano 1ª Divisão 2025, Pernambucano A1 e A2 2019, Pernambucano A3 2024, Maranhense Série A 2026, Maranhense Série B 2024 e 2026, e Baianão Série A 2027. O motivo de cada uma está em [Coletado](#o-que-já-foi-coletado).

## Começo rápido

Docker é o caminho suportado. Na raiz do repositório:

```sh
make setup
make up
```

`make setup` copia `.env.example` para `.env` só quando `.env` não existe. `make up` sobe o PostgreSQL, aplica as migrations uma vez e então sobe a API em `127.0.0.1:8080` e o worker. O worker do Compose usa `SYNC_COMPETITION=all` e `SYNC_YEARS=all`. Cada temporada tem o próprio `SYNC_TIMEOUT` (3 minutos no Compose). Temporada que falha é registrada, as linhas anteriores dela ficam, e as outras temporadas seguem. `worker -once` sai com código diferente de zero quando alguma temporada selecionada falhou. Recriar o worker coleta de novo cada temporada selecionada, inclusive as que já estão gravadas.

`make scrape` coleta uma competição e um ano. O padrão é `brasileirao-serie-a` e `2026`.

```sh
make scrape COMPETITION=brasileirao-serie-d YEAR=2021
```

`make down` para os containers e mantém o volume do banco. `make reset CONFIRM=yes` apaga esse volume.

Outros alvos: `make logs`, `make migrate`, `make test`, `make fmt`, `make vet`, `make vuln`, `make openapi` e `make check`.

Numa máquina sem `make`, o mesmo fluxo é:

```sh
docker compose up -d --build
docker compose run --rm --build worker -once -competition brasileirao-serie-a -year 2026
```

`docker compose run --rm worker -once` usa o ambiente do Compose e coleta o catálogo inteiro. `-competition` aceita um código, uma lista separada por vírgula, ou `all`. `-year` escolhe um ano e substitui `SYNC_YEARS=all`. O padrão do processo, quando essas variáveis não estão definidas, continua sendo a Série A do ano corrente. O `.env.example` documenta esse padrão.

Códigos: `brasileirao-serie-a`, `brasileirao-serie-b`, `brasileirao-serie-c`, `brasileirao-serie-d`, `copa-do-brasil`, `mineiro-modulo-i`, `mineiro-modulo-ii`, `mineiro-segunda-divisao`, `candangao-serie-a`, `candangao-serie-b`, `goiano-primeira-divisao`, `goiano-divisao-de-acesso`, `goiano-terceira-divisao`, `pernambucano-a1`, `pernambucano-a2`, `pernambucano-a3`, `maranhense-serie-a`, `maranhense-serie-b`, `mato-grossense-primeira-divisao`, `mato-grossense-segunda-divisao`, `alagoano-serie-a`, `alagoano-serie-b`, `baiano-serie-a`, `baiano-serie-b`.

O cliente da CBF pede `SOURCE_BASE_URL` (padrão `https://www.cbf.com.br`). Cada cliente estadual fica preso ao próprio host e ignora essa variável.

A senha do banco de desenvolvimento em `compose.yaml` vale só nesta máquina. Não publique a porta 5432 nem reutilize essas credenciais fora daqui. A porta no host começa em 5432. Defina `POSTGRES_PORT` quando essa porta já estiver ocupada. Os containers continuam falando com `postgres:5432` na rede do Compose. Defina `CORS_ALLOWED_ORIGINS` com uma lista explícita de origens antes de expor a API, e `TRUST_PROXY=true` só atrás de um proxy que reescreve `X-Forwarded-For`.

## Chamadas

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

O Swagger UI fica em <http://127.0.0.1:8080/docs>. O contrato é [docs/openapi.yaml](docs/openapi.yaml).

Os filtros de jogo são `round`, `team_id`, `status`, `from` e `to`. `page_size` vai no máximo a 100. Os jogos saem por horário de início, com horário desconhecido por último, e depois por id.

`/health/live` confere se o processo está no ar. `/health/ready` consulta o PostgreSQL e não chama o site de nenhuma federação.

## Fonte

Conferido em 2026-10-30. `https://www.cbf.com.br/robots.txt` respondeu 404. A URL antiga de competição em `/futebol-brasileiro/competicoes/` também respondeu 404. As páginas que o site público usa agora são:

| Documento | Caminho | O que devolve |
| --- | --- | --- |
| Catálogo | `/api/cbf/campeonatos/anos` | JSON. O Campeonato Brasileiro é o id `42`. Os ids de categoria são `1` Série A, `2` Série B, `3` Série C e `5` Série D. A Copa do Brasil é o campeonato `24`, categoria profissional `4`. Ids de categoria não são únicos entre campeonatos. Anos e ids de campeonato são listas paralelas separadas por vírgula. |
| Jogos | `/api/cbf/jogos/tabela-detalhada/campeonato/{id}` | JSON, uma fase, todos os jogos da temporada. |
| Alterações de data | `/api/cbf/campeonatos/{id}/imt` | Lista JSON. `01/01/1900` e `1900-01-01 ::00` significam data indefinida. |
| Classificação | `/futebol-brasileiro/tabelas/campeonato-brasileiro/{slug}/{year}` | A tabela oficial, como tabela HTML ou como linhas de grupo embutidas na página. `{slug}` é `serie-a`, `serie-b`, `serie-c` ou `serie-d`. |

Os ids de campeonato de 2026 são Série A `1260611`, Série B `1260612`, Série C `1260613`, Série D `1260635` e Copa do Brasil `1260615`. O id do clube vem da URL do escudo `conteudo.cbf.com.br/clubes/{id}/escudo.jpg` na tabela HTML, e de `cod_time` numa linha de grupo embutida. No feed de jogos o id do clube vem da mesma URL de escudo.

O catálogo é pedido uma vez por coleta. Cada temporada pede então os jogos e as alterações de data. Uma divisão também pede a página de classificação do ano. Quando essa página não tem tabela reconhecida, o coletor pede o mesmo caminho mais o id da primeira fase de pontos (`fase_tipo` `pontuacao`) e não usa uma fase eliminatória. A Copa do Brasil não tem página de classificação, então esse pedido é pulado. Os pedidos esperam pelo menos `SOURCE_MIN_INTERVAL` e repetem HTTP 429 e 5xx com `Retry-After` quando ele vem na resposta. Página de bloqueio, tabela vazia ou lista curta de jogos falha essa temporada e não substitui as linhas gravadas. Uma linha de jogo com o mesmo id de clube nos dois lados é omitida. Dois clubes com o mesmo nome e escudos diferentes ficam os dois.

Série A e Série B aceitam um snapshot de pelo menos 100 jogos e 18–22 linhas de classificação. Série C aceita pelo menos 40 jogos e 8–40 linhas. Série D aceita pelo menos 20 jogos e 16–128 linhas. A Copa do Brasil aceita pelo menos 100 jogos e exatamente zero linhas de classificação. Módulo I e Módulo II aceitam pelo menos 30 jogos e 8–24 linhas. A Segunda Divisão aceita pelo menos 20 jogos e 8–48 linhas. As divisões do Distrito Federal, de Goiás e de Pernambuco aceitam pelo menos 8 jogos e 4–40 linhas. Maranhão, Mato Grosso, Alagoas e Bahia aceitam pelo menos 8 jogos e 4–48 linhas. Santa Catarina aceita pelo menos 8 jogos e 2–48 linhas.

As páginas do Mineiro são `https://www.fmf.com.br/Competicoes/ProxJogos.aspx?d=1` (Módulo I), `d=2` (Módulo II) e `d=3` (Segunda Divisão). A temporada é um POST de formulário com `request` igual à opção numérica da página. O coletor recusa qualquer outro host e qualquer outro caminho.

Cada adaptador estadual fica preso a um host. Os caminhos saem desse adaptador. Uma URL vinda do cliente nunca é buscada.

| Adaptador | `source.name` | Host | Competições |
| --- | --- | --- | --- |
| `cbf` | `cbf` | `www.cbf.com.br` | Série A–D, Copa do Brasil |
| `fmf` | `fmf` | `www.fmf.com.br` | Mineiro |
| `portal` | `ffdf` | `www.ffdf.com.br` | Candangão |
| `portal` | `fgf-go` | `fgf.esp.br` | Goiano |
| `portal` | `fpf-pe` | `www.fpf-pe.com.br` | Pernambucano |
| `uikit` | `fmf-ma` | `www.futebolmaranhense.com.br` | Maranhense |
| `uikit` | `fmf-mt` | `www.fmfmt.com.br` | Mato-Grossense |
| `fafal` | `faf-al` | `www.futeboldealagoas.net` | Alagoano |
| `fbf` | `fbf` | `www.fbf.org.br` | Baianão |
| `fcf` | `fcf` | `fcf.com.br` | Catarinense |

A sincronização pega um advisory lock do Postgres por competição e temporada. Página vazia, tabela quebrada ou resposta que não é HTML é coleta que falhou.

## Desenvolvimento

```
cmd/
  api/                 # servidor HTTP. Só lê o PostgreSQL.
  worker/              # sincronização agendada, e worker -once para uma coleta manual
  migrate/             # aplica o SQL versionado. A API e o worker não migram.
internal/
  domain/              # tipos e validação do snapshot
  scraper/             # cbf, fmf, portal, uikit, fafal, fbf, fcf. Os seletores ficam aqui.
  catalog/             # códigos das competições. "all" é esta lista.
  service/             # casos de uso de sincronização e de leitura
  repository/postgres/ # o único SQL
  httpapi/             # borda HTTP
  config/              # variáveis de ambiente, validadas na subida
migrations/            # SQL só para a frente
docs/openapi.yaml      # o contrato servido em /openapi.yaml
legacy/                # o scraper PHP anterior e o front estático
```

Os handlers nunca chamam o site de uma federação. `docs/openapi.yaml` é o contrato. Arquivos HTML ficam de fora das estatísticas de linguagem do GitHub pelo `.gitattributes`. Continuam versionados quando o projeto precisa deles.

```sh
make fmt
make vet
make test
make vuln
make openapi
make check
```

`go test ./...` não chama o site da CBF nem o de nenhuma federação estadual. Os testes de parser usam fixtures em `internal/scraper`. O teste de PostgreSQL cria um schema chamado `it_` mais hex aleatório, migra esse schema e derruba só ele. Ele roda quando `DATABASE_URL` está definida. `CBF_LIVE=1` fica reservado e não é obrigatório. Checagem ao vivo de um host novo segue a mesma ideia, fora da suíte padrão.

`go test -race` precisa de cgo. O profile `test` do Compose roda isso dentro da imagem Go 1.26.10. O módulo usa Go 1.26 porque o `golang.org/x/net` atual exige essa versão. `govulncheck ./...` não aponta vulnerabilidade conhecida com este toolchain e este conjunto de dependências.

O que ainda está aberto está no [ROADMAP.md](ROADMAP.md). O que mudou está no [CHANGELOG.md](CHANGELOG.md).

## Licença

Este repositório não inclui um arquivo de licença.
