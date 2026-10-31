
Recomece o projeto https://github.com/czarist/api_brasileirao no mesmo repositório, reimplementando em Go.

A proposta é uma API REST pública para consultar dados do Campeonato Brasileiro obtidos por web scraping, com documentação OpenAPI e Swagger UI, persistência e atualização automática.

Quero uma implementação funcional, testada e fácil de executar. Vá além do planejamento e implemente o MVP.

## 1. Inspeção inicial e skills

Antes de modificar:

- Leia AGENTS.md, README, código, configurações, licença e instruções existentes.
- Identifique as fontes e funcionalidades da implementação anterior que merecem ser preservadas.
- Verifique o estado do Git e preserve alterações locais existentes.
- Trabalhe em uma branch de reescrita. Preserve o histórico, sem force push ou exclusão indiscriminada do projeto.

Descubra e acione as skills disponíveis relacionadas a:

- Go idiomático e arquitetura de backend.
- Web scraping e extração de dados.
- Design de APIs REST e OpenAPI/Swagger.
- PostgreSQL e migrações.
- Testes, concorrência e análise de segurança.
- Docker, GitHub Actions e documentação.

Use os nomes reais das skills encontradas. Não invente skills ou alegue execução de ferramentas indisponíveis. Se faltar alguma skill, prossiga com as ferramentas disponíveis e documente a limitação.

## 2. Escopo

Priorize o Brasileirão Série A, com estrutura preparada para outras divisões e temporadas sem implementar tudo antecipadamente.

O MVP deve oferecer:

- Competições e temporadas disponíveis.
- Times participantes por temporada.
- Classificação.
- Rodadas.
- Jogos, resultados e próximos confrontos.
- Detalhamento de uma partida.

Artilharia e estatísticas adicionais ficam no roadmap, salvo se a fonte já fornecer esses dados de maneira simples e confiável.

Não prometa placares em tempo real. Documente a frequência efetiva de atualização.

## 3. Validação das fontes

Antes de construir o scraper, investigue as fontes reais disponíveis e compare:

- Cobertura dos dados necessários.
- Estabilidade da estrutura.
- HTML estático, dados estruturados embutidos ou renderização JavaScript.
- Identificadores disponíveis.
- Histórico de temporadas.
- Restrições de acesso e condições de reutilização.

Escolha uma fonte principal e documente a decisão com URLs e limitações verificadas.

Prefira HTTP e parsing de HTML/dados estruturados. Use navegador headless somente se houver necessidade demonstrada.

Respeite robots.txt, limites e condições das fontes. Não implemente contorno de CAPTCHA, autenticação ou bloqueios.

Separe cada fonte em um adaptador substituível. Não misture seletores HTML com handlers HTTP ou regras de domínio.

Se a fonte não cobrir um recurso, documente a ausência. Não invente dados ou use mocks como se fossem resultados reais.

## 4. Stack e arquitetura

Use:

- Versão estável e suportada de Go, verificada no início.
- PostgreSQL, com migrações versionadas.
- net/http ou um roteador leve, justificando a escolha.
- slog para logs estruturados.
- OpenAPI 3.x e Swagger UI.
- Dockerfile multi-stage e Docker Compose.

Escolha bibliotecas mantidas, verificando compatibilidade e documentação oficial. Evite dependências desnecessárias.

Organize o código de maneira idiomática, por exemplo:

- cmd/api
- cmd/worker
- internal/domain
- internal/service
- internal/repository
- internal/scraper
- internal/http
- internal/config
- migrations
- docs

Adapte a estrutura quando necessário. Evite microserviços, abstrações genéricas excessivas e interfaces sem uma necessidade concreta.

API e worker podem compartilhar o mesmo módulo e imagem, com comandos separados.

## 5. Modelo dos dados

Modele competição, temporada, time, participação na temporada, rodada, partida e classificação.

Cuide especialmente de:

- IDs internos estáveis, independentes de nomes e slugs.
- Mapeamento dos IDs externos por fonte.
- Variações de nome e abreviação dos clubes.
- Placar desconhecido como null, sem confundir com 0 a 0.
- Datas ainda não definidas como null.
- Estados normalizados: agendado, em andamento, encerrado, adiado, suspenso e cancelado, conforme a informação disponível.
- Jogos adiados mantendo a rodada original.
- Alterações posteriores de horário, estádio e resultado.
- Datas armazenadas em UTC e expostas em RFC 3339.
- Origem, instante da coleta e instante da última atualização bem-sucedida.

Não suponha que todas as temporadas tenham exatamente o mesmo formato.

Preserve a classificação oficial coletada, incluindo possíveis punições ou ajustes de pontos. Não substitua silenciosamente a tabela oficial por cálculos próprios.

## 6. Scraping e sincronização

As requisições públicas da API não devem executar scraping diretamente.

Implemente:

- Worker com intervalos configuráveis.
- Comando de sincronização manual via CLI.
- Execução inicial para popular o banco.
- Coleta por competição e temporada.
- Upserts idempotentes e restrições únicas.
- Timeouts, cancelamento por context e limite de concorrência.
- Retentativas limitadas, com backoff e jitter para falhas transitórias.
- Respeito a Retry-After.
- Limite de requisições por fonte.
- Proteção contra execuções simultâneas da mesma sincronização.
- Encerramento gracioso.

Valide os dados antes de persistir. Uma página vazia, seletor quebrado ou resposta de bloqueio não pode apagar os dados válidos.

Use transações para publicar conjuntos coerentes de dados. Registre tentativas, falhas e última sincronização bem-sucedida.

Mantenha dados anteriores quando uma coleta falhar e sinalize sua defasagem. Diferencie “nunca houve coleta” de “última coleta está desatualizada”.

Não adicione Redis ou filas externas no MVP sem necessidade demonstrada.

## 7. API REST

Use prefixo /api/v1 e nomes consistentes em inglês.

Proposta de endpoints:

- GET /api/v1/competitions
- GET /api/v1/competitions/{competition_id}/seasons
- GET /api/v1/competitions/{competition_id}/seasons/{year}/teams
- GET /api/v1/competitions/{competition_id}/seasons/{year}/standings
- GET /api/v1/competitions/{competition_id}/seasons/{year}/rounds
- GET /api/v1/competitions/{competition_id}/seasons/{year}/matches
- GET /api/v1/matches/{match_id}
- GET /health/live
- GET /health/ready
- GET /openapi.yaml
- GET /docs

Permita filtros de partidas por rodada, time, estado e intervalo de datas. Defina paginação, ordenação estável e limites máximos.

Padronize:

- Respostas JSON.
- Validação de parâmetros.
- Códigos HTTP.
- Erros com código, mensagem e request_id.
- Metadados de paginação.
- Fonte, última coleta bem-sucedida e indicador de dados desatualizados.

Defina claramente o comportamento para:

- Recurso inexistente.
- Temporada sem dados coletados.
- Lista válida sem resultados.
- Dados antigos disponíveis após falha da fonte.
- Banco indisponível.

Não retorne sucesso com dados inventados ou silenciosamente vazios quando houve falha operacional.

## 8. Swagger e contrato

Mantenha uma única fonte de verdade para o contrato OpenAPI e a implementação, evitando documentação divergente.

Documente:

- Todos os endpoints, parâmetros e filtros.
- Schemas, enums e campos nullable.
- Paginação.
- Exemplos de sucesso e erro.
- Formatos e fusos das datas.
- Frequência de atualização.
- Semântica dos dados desatualizados.
- Limitações de cobertura.

Disponibilize Swagger UI funcional e valide a especificação no CI.

## 9. Segurança e operação

Inclua:

- Configuração por variáveis de ambiente e .env.example.
- Validação das configurações no boot.
- Timeouts do servidor HTTP.
- Request ID, recuperação de panic e logs estruturados.
- Rate limiting configurável.
- CORS configurável.
- Consultas SQL parametrizadas.
- Limites de tamanho das respostas externas.
- URLs de fontes controladas pela configuração, sem aceitar URLs arbitrárias do cliente.
- Container executando sem root.
- Ausência de segredos no repositório.

A liveness deve representar o processo ativo. A readiness deve verificar as dependências necessárias para atender consultas, sem depender de uma chamada ao site de origem.

Registre duração e resultado das sincronizações e quantidade de registros processados. Não exponha endpoints administrativos de escrita sem proteção.

## 10. Testes e CI

Implemente testes relevantes:

- Parsers com fixtures reais de HTML ou dados estruturados.
- Página vazia, estrutura alterada e conteúdo incompleto.
- Jogos adiados e placares ausentes.
- Normalização dos times.
- Idempotência e atualização de registros.
- Preservação dos dados após falha de coleta.
- Handlers, filtros, paginação e erros.
- Persistência com PostgreSQL de teste.
- Concorrência e cancelamento quando aplicáveis.

Os testes padrão não devem depender da disponibilidade da fonte externa. Separe verificações reais da fonte como testes opcionais.

Configure GitHub Actions para:

- Verificar formatação.
- Executar go vet.
- Executar testes com race detector.
- Executar govulncheck.
- Validar OpenAPI.
- Compilar aplicação e imagem Docker.

Corrija falhas encontradas. Relate claramente qualquer verificação que não tenha sido possível executar.

## 11. Documentação e entrega

Atualize o README em português com:

- Objetivo e arquitetura.
- Stack e justificativas breves.
- Fonte dos dados e limitações.
- Pré-requisitos.
- Como subir com Docker Compose.
- Como aplicar migrações.
- Como realizar a primeira coleta.
- Como iniciar API e worker.
- Como abrir o Swagger.
- Exemplos curl.
- Como testar.
- Configurações disponíveis.
- Frequência de atualização e tratamento de indisponibilidade.

Crie comandos simples para build, execução, migrações, sincronização e testes.

Preserve a licença existente. Não escolha outra licença sem uma decisão explícita do mantenedor. Documente que a API é independente e não implica vínculo oficial com as entidades esportivas.

Crie um roadmap enxuto para outras divisões, histórico adicional, artilharia e fontes alternativas.

## 12. Critérios de conclusão

Considere o MVP concluído quando:

1. O ambiente subir seguindo o README.
2. A sincronização coletar e persistir dados reais de uma temporada suportada.
3. A API consultar o banco sem disparar scraping.
4. Os endpoints principais e filtros funcionarem.
5. O Swagger refletir o comportamento implementado.
6. Falhas da fonte preservarem os últimos dados válidos.
7. Os testes e verificações executáveis passarem.
8. A documentação permitir que outra pessoa reproduza a execução.

Comece apresentando um diagnóstico curto e as decisões principais, depois implemente em etapas. Não pare apenas na criação de documentação ou scaffold.

Ao finalizar, informe o que foi implementado, quais fontes foram verificadas, como executar, quais testes passaram e quais limitações permanecem. Não faça deploy nem publique alterações remotas sem solicitação.



## 13. Documentação obrigatória e regras permanentes

Crie ou atualize na raiz do repositório:

- AGENTS.md
- README.md
- ROADMAP.md
- CHANGELOG.md
- .env.example
- .gitignore
- .dockerignore
- Dockerfile
- compose.yaml
- Makefile

Leia os arquivos existentes antes de alterá-los. Preserve informações relevantes e adapte instruções antigas à reescrita em Go.

Esses arquivos devem refletir a implementação real. Não documente funcionalidades planejadas como se já estivessem disponíveis.

### AGENTS.md

Crie este arquivo no início do trabalho e siga suas instruções nas etapas seguintes.

Inclua regras objetivas para qualquer agente que trabalhar no projeto:

1. Contexto e arquitetura:

   - Objetivo do projeto, stack e estrutura de diretórios.
   - Responsabilidade de cada camada.
   - Fluxo: fonte externa → scraper → validação → banco → API.
   - Handlers públicos não podem disparar scraping.
   - Manter Go idiomático, dependências justificadas e arquitetura simples.
2. Fluxo de trabalho:

   - Ler AGENTS.md e ROADMAP.md antes de implementar.
   - Verificar o estado do Git e preservar alterações de terceiros.
   - Identificar o item do roadmap atendido pela alteração.
   - Implementar em incrementos funcionais.
   - Atualizar documentação e changelog junto com a mudança.
   - Não fazer force push, deploy, troca de licença ou publicar alterações remotas sem solicitação.
   - Não considerar uma tarefa concluída sem verificar seus critérios de aceite.
   - Registrar bloqueios concretos sem marcar tarefas incompletas como finalizadas.
3. Convenções:

   - Código, identificadores, rotas e comentários técnicos em inglês.
   - Documentação destinada ao mantenedor em português.
   - gofmt obrigatório.
   - context.Context nas operações de I/O.
   - Erros tratados explicitamente e contextualizados, sem expor detalhes internos ao cliente.
   - Datas persistidas em UTC.
   - Configuração externa por variáveis de ambiente, validada no boot.
   - Nunca versionar credenciais, arquivos .env reais ou dados sensíveis.
   - Não editar manualmente arquivos gerados: documentar e executar o comando de geração.
4. Dados e scraping:

   - Adaptadores de fontes separados do domínio e da API.
   - Sincronização idempotente.
   - Preservar dados válidos quando uma coleta falhar.
   - Não tratar ausência de placar como zero.
   - Não inventar estatísticas ou cobertura de temporadas.
   - Mudanças nos parsers exigem atualização das fixtures e testes relevantes.
   - Não depender da fonte externa nos testes padrão.
5. Banco e contrato:

   - Alterações de schema por migrações versionadas.
   - Não alterar migrações já aplicadas: criar novas.
   - Documentar migrações destrutivas e seu impacto.
   - Mudanças de endpoints ou schemas exigem atualização do OpenAPI.
   - Preservar compatibilidade dentro de /api/v1 ou documentar explicitamente a quebra.
6. Verificação:

   - Registrar os comandos oficiais de build, testes, lint, migrações e sincronização.
   - Executar verificações pertinentes antes de concluir.
   - Informar verificações não executadas e o motivo.
   - Preferir o ambiente Docker padronizado para reprodução.

Não transforme AGENTS.md em diário de atividades. Mantenha nele apenas instruções duráveis e verificáveis.

### README.md

O README deve ser a porta de entrada do projeto e conter:

- Objetivo e escopo efetivamente implementado.
- Situação atual da reescrita em Go.
- Stack e arquitetura resumida.
- Pré-requisitos e início rápido com Docker.
- Configuração das variáveis de ambiente.
- Migrações, primeira coleta, API e worker.
- Endereço do Swagger e exemplos curl.
- Comandos de desenvolvimento e testes.
- Fontes dos dados, frequência de atualização e limitações.
- Persistência dos dados e funcionamento dos volumes.
- Solução de problemas comuns.
- Links relativos para AGENTS.md, ROADMAP.md e CHANGELOG.md.

O início rápido deve funcionar em um clone limpo, sem exigir Go ou PostgreSQL instalados no host.

### ROADMAP.md

Crie o roadmap antes da implementação detalhada e atualize-o conforme o trabalho avançar.

Organize em fases:

1. Diagnóstico do legado e validação da fonte.
2. Fundação Go, Docker e configuração.
3. Modelagem e migrações.
4. Scraper e sincronização.
5. API REST.
6. OpenAPI e Swagger UI.
7. Testes, segurança e CI.
8. Validação integrada e documentação.
9. Evoluções posteriores ao MVP.

Para cada fase, registre:

- Objetivo.
- Tarefas concretas com checkboxes.
- Dependências.
- Critérios de aceite observáveis.

Inclua as tarefas previstas no prompt nas fases correspondentes. Diferencie claramente MVP de melhorias futuras.

Marque como concluído somente o que foi implementado e verificado. Quando houver bloqueio, explique a causa e o próximo passo. Mantenha o estado final do roadmap atualizado ao entregar.

### CHANGELOG.md

Use uma estrutura inspirada em Keep a Changelog:

- Seção inicial [Unreleased].
- Categorias Added, Changed, Deprecated, Removed, Fixed e Security, apenas quando houver conteúdo.
- Entradas concisas descrevendo mudanças relevantes para quem usa ou mantém o projeto.

Registre a reescrita em Go, nova arquitetura, ambiente Docker, endpoints e demais mudanças conforme forem implementados.

Não invente releases ou datas históricas. Não registre funcionalidades futuras como entregues. Não crie tag de versão ou release remoto automaticamente.

## 14. Docker obrigatório

Docker deve ser o caminho oficial para desenvolvimento, testes e execução.

Implemente:

- Dockerfile multi-stage.
- Estágios para desenvolvimento/testes e imagem final enxuta.
- Imagem final sem ferramentas de compilação e executando sem root.
- Certificados CA e suporte aos fusos horários necessários.
- Tratamento correto de sinais e encerramento gracioso.
- Imagens-base com versões explícitas; não usar latest.

O compose.yaml deve oferecer:

- postgres: banco com healthcheck e volume nomeado.
- migrate: execução pontual das migrações.
- api: servidor HTTP.
- worker: sincronização agendada.
- Serviço ou profile para testes e verificações.

API e worker devem compartilhar a mesma imagem e selecionar seus comandos explicitamente.

Cuidados obrigatórios:

- Não executar migrações simultaneamente em cada processo.
- Ordenar a inicialização pela prontidão do banco e conclusão das migrações.
- API e worker devem tratar perda temporária de conexão após a inicialização.
- Não fixar container_name nem exigir caminhos absolutos da máquina.
- Expor apenas as portas necessárias; banco publicado no host somente se necessário ao desenvolvimento.
- Não embutir segredos na imagem ou nos argumentos de build.
- Manter configuração de desenvolvimento separada das orientações de produção.
- Preservar o volume do banco nas operações normais de parada.
- Documentar explicitamente qualquer comando que apague dados.
- Permitir reconstruir o ambiente de maneira reproduzível.

Não introduza Kubernetes, Redis, filas ou outros serviços sem necessidade demonstrada.

## 15. Comandos padronizados

Disponibilize no Makefile comandos como:

- make help
- make setup
- make build
- make up
- make down
- make logs
- make migrate
- make scrape
- make test
- make fmt
- make vet
- make vuln
- make check

Esses comandos devem usar Docker e funcionar sem uma instalação local de Go.

O comando de sincronização deve permitir informar competição e temporada. Documente exemplos reais.

make setup pode preparar o .env a partir do exemplo, mas nunca sobrescrever uma configuração existente.

make down não pode remover volumes. Caso exista um comando de reset, dê um nome explícito, documente seu efeito e exija confirmação antes de apagar dados.

Mantenha os comandos usados localmente alinhados aos utilizados no CI, evitando verificações diferentes entre os ambientes.

## 16. Validação final em ambiente limpo

Antes de concluir:

1. Faça o build das imagens.
2. Suba PostgreSQL, aplique migrações e inicie os serviços.
3. Execute uma sincronização real da fonte validada.
4. Consulte endpoints com os dados persistidos.
5. Verifique Swagger UI e OpenAPI.
6. Reinicie os serviços e confirme a persistência.
7. Verifique o comportamento quando a fonte estiver indisponível.
8. Execute os testes e demais verificações disponíveis.
9. Confira se README, AGENTS, ROADMAP e CHANGELOG correspondem ao resultado.

Use um banco ou projeto Compose isolado para testes destrutivos. Não apague volumes existentes do usuário.

Caso o ambiente impeça algum passo, registre exatamente o que não foi validado e como reproduzir a verificação. Não declare sucesso sem evidência.


## Idioma obrigatório do projeto

Mantenha todo o projeto em inglês, incluindo:

- Código, identificadores, comentários e mensagens de erro.
- Rotas, parâmetros, campos JSON e documentação OpenAPI/Swagger.
- README.md, AGENTS.md, ROADMAP.md e CHANGELOG.md.
- Descrições de configurações, saída dos comandos e logs.
- Mensagens de commit e títulos e descrições de pull requests, quando solicitados.

Preserve nomes próprios oficiais de clubes, competições, estádios e entidades. Não traduza esses nomes artificialmente.

Inclua essa política no AGENTS.md para orientar futuras contribuições. Esta instrução substitui qualquer orientação anterior que determine documentação em português.

A comunicação comigo pode continuar em português.

salvar o código antigo em uma pasta chamada "legacy" só pra lembrança, criar tags e commitar cada parte, continue fazendo sem perguntar nada, siga seus instintos, use as skills da pasta
