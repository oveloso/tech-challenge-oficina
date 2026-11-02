# Oficina Tech Challenge

API da fase 1 do FIAP SOAT para gerenciar clientes, veículos, serviços, estoque e ordens de serviço de uma oficina mecânica. O fluxo inclui diagnóstico, orçamento versionado, aprovação ou rejeição pelo cliente, execução dos serviços e entrega do veículo.

A aplicação é um monólito em camadas com **Go 1.26.0 ou superior, Gin 1.12.0, PostgreSQL 18 e pgx**. As **40 operações** estão descritas em OpenAPI e podem ser executadas pelo Swagger servido pelo próprio backend.

## Objetivo

Dar à oficina controle sobre o atendimento de ponta a ponta: o que entrou, em que estado está cada veículo, o que o cliente aprovou e quais peças saíram do estoque. O MVP cobre três fluxos, levantados no Event Storming: criação da OS com diagnóstico e orçamento, acompanhamento da OS pelo funcionário e pelo cliente até a entrega, e gestão de peças e insumos.

## Execução local

Execute os comandos a partir da raiz do projeto. Escolha Docker Compose ou execução nativa. Os scripts de configuração criam `.env` com credenciais aleatórias e preservam um arquivo já existente; os valores reais não são versionados.

### Docker Compose

Requisito: Docker Engine ou Docker Desktop com o daemon em execução e Docker Compose v2.

No PowerShell:

```powershell
./scripts/configure.ps1
docker compose config --quiet
docker compose up --build -d
docker compose ps
```

No Linux/macOS, substitua a primeira linha por `sh scripts/configure.sh`; esse script requer OpenSSL. O Compose cria PostgreSQL, executa migrações e seed e inicia a API. O seed cria o administrador definido em `.env` e, com `DEMO_DATA=true`, dados de demonstração: 3 clientes e veículos, 3 serviços, peças e insumo, 3 OS já entregues (alimentam as métricas de tempo médio) e 2 OS recebidas. Uma delas pede 2 discos de freio com saldo 1, para demonstrar o 409 de estoque insuficiente. Executá-lo novamente não substitui a senha de um administrador existente.

API e banco são publicados apenas em `127.0.0.1`, nas portas `API_PORT` e `DB_PORT` do `.env` — por padrão, 8080 e 5432. Dentro do contêiner, a API usa `HOST=0.0.0.0`. Para encerrar preservando os dados:

```powershell
docker compose down
```

**Validação:** o job [compose-smoke da CI](https://github.com/oveloso/tech-challenge-oficina/actions/runs/41526702477) constrói e sobe o ambiente Docker a cada PR, confere seed, saúde, Swagger e OpenAPI e autentica o administrador.

### Windows com Go e PostgreSQL

Requisitos: Go 1.26.0 ou superior, PostgreSQL 18 iniciado, PowerShell e `go`/`psql` disponíveis no `PATH`.

```powershell
./scripts/configure.ps1
psql -h 127.0.0.1 -p 5432 -U postgres -d postgres
```

No `psql`, prepare os bancos uma vez. Em `\password`, informe o valor de `POSTGRES_PASSWORD` do seu `.env`; a senha não entra no comando SQL:

```text
CREATE ROLE oficina LOGIN;
\password oficina
CREATE DATABASE oficina OWNER oficina;
CREATE DATABASE oficina_test OWNER oficina;
\q
```

Os nomes acima correspondem ao `.env` gerado. Se já houver usuário/bancos preparados ou você alterar esses nomes, use os valores correspondentes. `oficina_test` deve ser exclusivo para testes. Em seguida:

```powershell
./scripts/run.ps1 -Action seed
./scripts/run.ps1 -Action api
```

O script lê `.env`, monta a conexão sem exibir a senha e configura o contrato OpenAPI. A API fica em primeiro plano; `Ctrl+C` a encerra. As migrações são aplicadas automaticamente. Para outro banco ou porta, use, por exemplo, `./scripts/run.ps1 -Action api -DatabasePort 5433 -DatabaseName oficina`.

Na execução nativa, `HOST` é `127.0.0.1` por padrão, e o script mantém esse vínculo local. Não é necessário abrir a aplicação para a rede do Windows.

### Endereços e configuração

Com a porta padrão:

| Recurso | Endereço |
|---|---|
| Swagger interativo | [http://127.0.0.1:8080/docs](http://127.0.0.1:8080/docs) |
| Contrato OpenAPI | [http://127.0.0.1:8080/openapi.yaml](http://127.0.0.1:8080/openapi.yaml) |
| Saúde da API e conexão com banco | [http://127.0.0.1:8080/health](http://127.0.0.1:8080/health) |
| Prefixo REST | `/api/v1` |

Swagger carrega CSS e JavaScript de `swagger-ui-dist` **5.33.1** pela CDN jsDelivr e precisa de internet para esses assets. O contrato estático e as operações da API são locais.

`.env` configura credenciais do banco e administrador, `JWT_SECRET`, dados de demonstração e portas. O backend também aceita `DATABASE_URL`, `HOST`, `PORT`, `OPENAPI_PATH`, `CORS_ORIGINS` e `CUSTOMER_TOKEN_TTL` pelo ambiente. A validade padrão do token do cliente é `30d`; CORS aceita apenas origens explícitas. Os scripts e o Compose já definem os valores necessários para os modos descritos acima.

## Autenticação e acesso do cliente

No Swagger, execute `POST /auth/login` com `ADMIN_USERNAME` e `ADMIN_PASSWORD` do seu `.env`. Copie o `accessToken` retornado para **Authorize → AdminBearer**, sem escrever o prefixo `Bearer`. O JWT vale por uma hora e autoriza `/admin`. Não há cadastro público de funcionários nem refresh token.

Este exemplo PowerShell lê as credenciais do `.env` e guarda o token em memória, sem colocar senhas literais no comando:

```powershell
$oficinaConfig = @{}
Get-Content .env | ForEach-Object {
    if ($_ -match '^([A-Z_]+)=(.*)$') {
        $oficinaConfig[$matches[1]] = $matches[2]
    }
}
$apiBase = "http://127.0.0.1:$($oficinaConfig.API_PORT)/api/v1"
$loginBody = @{
    username = $oficinaConfig.ADMIN_USERNAME
    password = $oficinaConfig.ADMIN_PASSWORD
} | ConvertTo-Json
$login = Invoke-RestMethod -Method Post -Uri "$apiBase/auth/login" -ContentType 'application/json' -Body $loginBody
$adminHeaders = @{ Authorization = "Bearer $($login.accessToken)" }
Invoke-RestMethod -Uri "$apiBase/admin/service-orders" -Headers $adminHeaders
```

O funcionário emite acesso em `POST /admin/service-orders/{serviceOrderId}/customer-access`, informando `expectedVersion`. A resposta contém `token`, `expiresAt` e `workOrderVersion`. Use esse token em **Authorize → CustomerBearer** para consultar e decidir o orçamento em `/customer`. A OS é identificada pela credencial; CPF, placa e IDs enviados pelo consumidor não concedem acesso.

O token do cliente é aleatório, tem 256 bits e só é retornado na emissão; o banco guarda seu hash SHA-256. Emitir outro invalida o anterior. O envio ao cliente é externo ao sistema. Ambos os tipos de token são enviados em `Authorization: Bearer <token>`, nunca na URL, e não são intercambiáveis. Swagger mantém as autorizações apenas na memória da página.

## Operações e roteiro de demonstração

Além do Swagger, a pasta [docs/api/bruno](docs/api/bruno) traz uma collection do [Bruno](https://www.usebruno.com/) com o fluxo de demonstração em ordem: login, cadastro de cliente e veículo, abertura da OS, orçamento, token do cliente, recusa com motivo, aprovação, execução com baixa de estoque, entrega, erros esperados e métricas. Cada requisição guarda o token, os IDs e o `expectedVersion` para a próxima. Abra a pasta no Bruno, escolha o ambiente `local` e informe `adminPassword` (o `ADMIN_PASSWORD` do `.env`). Pela linha de comando, dentro da pasta: `npx @usebruno/cli@4.2.1 run -r --env local --env-var adminPassword=<senha>`.

Todos os caminhos abaixo usam o prefixo `/api/v1`. Os schemas, exemplos, filtros e respostas estão no [contrato OpenAPI](docs/api/openapi.yaml).

| Grupo | Operações | Quantidade |
|---|---|---:|
| Saúde | Consultar disponibilidade | 1 |
| Autenticação | Login administrativo | 1 |
| Clientes | Criar, listar, consultar, atualizar e desativar | 5 |
| Veículos | Criar, listar, consultar, atualizar e desativar | 5 |
| Serviços | Criar, listar, consultar, atualizar e desativar | 5 |
| Estoque | Cinco operações cadastrais, consultar e registrar movimentos | 7 |
| Ordens de serviço | Criar/listar/consultar, substituir itens, iniciar/reabrir diagnóstico, publicar orçamento, emitir acesso, iniciar/concluir serviço, finalizar e entregar | 12 |
| Cliente autorizado | Consultar sua OS e decidir o orçamento | 2 |
| Métricas | Tempo de execução por serviço e por OS | 2 |
| **Total** | | **40** |

O seed facilita uma demonstração inicial; os registros também podem ser criados pelo Swagger. Use os IDs reais das respostas e, em cada alteração administrativa de uma OS existente, envie sua `version` atual como `expectedVersion`. Após emitir acesso, use `workOrderVersion` ou consulte a OS novamente.

1. Consulte `/health`, faça login e autorize `AdminBearer`.
2. Explore clientes, veículos, serviços e itens de estoque: crie ou use os registros de demonstração, consulte detalhes e filtros e atualize os dados. Para demonstrar `DELETE`, use um cadastro auxiliar: a exclusão é lógica, e `PATCH` com `active: true` reativa o registro.
3. Registre reposição ou ajuste em `/admin/inventory-items/{id}/movements` e consulte o histórico. Saldo não é alterado por edição cadastral.
4. Crie uma OS em `POST /admin/service-orders` com cliente, veículo pertencente a ele, `problemDescription`, `serviceIds` e `parts`. Listas vazias são aceitas na abertura; o orçamento exige ao menos um serviço.
5. Execute `POST /admin/service-orders/{id}/diagnosis/start`. Se necessário, substitua as listas completas em `PUT /admin/service-orders/{id}/items`. Publique o orçamento com `POST /admin/service-orders/{id}/quotes`; o servidor congela nomes e preços e calcula o total.
6. Emita o token de cliente, autorize `CustomerBearer` e consulte `GET /customer/service-order`. Em `POST /customer/service-order/quote-decision`, envie o `currentQuote.id` como `quoteId` e `decision: "REJECTED"`. A OS retorna ao diagnóstico. Revise itens, publique outra versão e decida com `"APPROVED"`.
7. Para demonstrar uma revisão adicional antes de executar, use `POST /admin/service-orders/{id}/diagnosis/reopen`. O orçamento anterior é superado; publique e aprove a nova versão. A aprovação mantém a OS em `AWAITING_APPROVAL` e não reserva estoque.
8. Inicie cada serviço em `POST /admin/service-orders/{id}/services/{serviceOrderServiceId}/start`. Use o `id` do item em `services`, que é diferente do `serviceId` do catálogo. O primeiro início baixa todas as peças na mesma transação e muda a OS para `IN_EXECUTION`. Saldo insuficiente retorna 409 sem efeitos parciais.
9. Conclua cada serviço no caminho correspondente terminado em `/complete`. Depois, finalize a OS em `POST /admin/service-orders/{id}/complete` e registre a entrega em `POST /admin/service-orders/{id}/deliver`.
10. Consulte `/admin/metrics/service-execution` e `/admin/metrics/service-orders` com `from` e `to` em RFC 3339 abrangendo a demonstração. O intervalo inclui `from` e exclui `to`; sem amostras, a média é `null`. Consulte também os movimentos de estoque e o histórico da OS.

O fluxo percorre `RECEIVED → IN_DIAGNOSIS → AWAITING_APPROVAL → IN_EXECUTION → COMPLETED → DELIVERED`. Não existe alteração livre de status. Valores monetários são centavos inteiros de BRL e datas são UTC. Erros usam `application/problem+json`; versões desatualizadas e conflitos de negócio retornam 409.

## Testes, cobertura e segurança

Use um banco PostgreSQL de teste separado. No Windows, após criar `oficina_test` conforme as instruções nativas:

```powershell
./scripts/run.ps1 -Action test
Push-Location backend
go tool cover -func=coverage.out
go tool cover -html=coverage.out -o coverage.html
Pop-Location
```

O script escolhe `<POSTGRES_DB>_test`, define `TEST_DATABASE_URL` e executa `go test -count=1 -coverpkg=./... -coverprofile=coverage.out ./...`. Os testes aplicam migrações, criam credenciais sintéticas e abrem um servidor HTTP temporário; não exigem a API de demonstração em execução. Eles criam dados no banco de teste. Outro nome pode ser informado com `-DatabaseName`.

Se estiver usando o PostgreSQL do Compose, crie o banco de teste uma vez com os nomes padrão e execute o mesmo script com Go instalado no host:

```powershell
docker compose exec db createdb -U oficina -O oficina oficina_test
./scripts/run.ps1 -Action test
```

Sem Go instalado, `./scripts/test-docker.ps1` executa a mesma suíte em contêineres descartáveis (Go 1.27.3 e PostgreSQL 18) e mostra a cobertura global e a do domínio.

Sem `TEST_DATABASE_URL`, a suíte integrada informa `SKIP`; executar somente `go test ./...` não comprova integração com banco. Há também uma modalidade contra API externa, configurada por `API_BASE_URL`, `TEST_ADMIN_USERNAME` e `TEST_ADMIN_PASSWORD`, para `go test -count=1 -v ./tests -run '^TestAPIIntegration$'` dentro de `backend`.

Outras verificações, dentro de `backend`:

```powershell
go vet ./...
go build ./cmd/...
go test -count=1 -coverprofile=critical.out ./internal/domain
go tool cover -func=critical.out
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
go run github.com/securego/gosec/v2/cmd/gosec@v2.30.0 ./...
```

A medição de 02/11/2026 em Linux/Docker, com detector de corridas, registrou **88,4% de cobertura global** e **99,0% no domínio**, com PostgreSQL real. Formatação, `go vet`, build e testes passaram. O contrato tem 40 operações e é servido pelo Swagger. Os comandos, resultados e limites estão no [registro de verificação](docs/evidencias/verificacao.md).

Evidências: [testes](docs/evidencias/testes.txt), [cobertura](docs/evidencias/cobertura.txt), [vulnerabilidades em dependências](docs/evidencias/govulncheck.txt) e [análise de segurança do código](docs/evidencias/gosec.txt).

A [CI](.github/workflows/ci.yml) configura PostgreSQL 18 e executa formatação, `go vet`, build, testes com detector de corridas, cobertura, `govulncheck` e `gosec`; o job `compose-smoke` constrói e sobe o ambiente Docker. Os jobs `backend` e `compose-smoke` passaram na [execução 41526702477](https://github.com/oveloso/tech-challenge-oficina/actions/runs/41526702477). Os resultados de segurança e demais limites são detalhados em [Requisitos e evidências](docs/entrega-fase-1.md).

## Organização e decisões

```text
backend/
  cmd/api/            inicialização e servidor HTTP
  cmd/seed/           administrador e dados de demonstração
  internal/auth/      JWT, tokens e limites de tentativas
  internal/domain/    regras de OS, orçamento e validações
  internal/registry/  clientes, veículos, serviços e estoque
  internal/workorders/ casos de uso, persistência e métricas de OS
  internal/platform/  conexão e utilitários HTTP
  internal/server/    configuração, rotas, saúde e Swagger
  migrations/         esquema SQL versionado
  tests/              integração HTTP com PostgreSQL
docs/                 contrato, arquitetura, DDD e evidências
scripts/              configuração e execução local
```

**Por que PostgreSQL.** O ponto crítico do domínio é a baixa de estoque no início do serviço: a OS e todos os itens precisam mudar juntos ou não mudar. PostgreSQL oferece transações ACID, bloqueio de linhas (`SELECT ... FOR UPDATE`) e restrições no próprio banco, como saldo não negativo e documento, placa e SKU únicos. Com isso, concorrência e rollback são garantidos pelo banco, não só pela aplicação. O JSONB permite gravar o agregado de OS inteiro, com itens, orçamentos e histórico, sem abrir mão dessas garantias. Detalhes em [Decisões de arquitetura](docs/decisions/0001-fundamentos.md).

Cadastros e o agregado de OS usam JSONB, com colunas geradas, chaves estrangeiras, unicidade e restrições de integridade nos vínculos principais. Movimentos de estoque são registros separados. Essa escolha simplifica a persistência do agregado e seus snapshots no MVP, com o custo de regravar documentos e consultar arrays JSONB nos indicadores. A baixa usa transação e bloqueios para impedir saldo negativo ou consumo duplicado.

## Documentação e entrega acadêmica

- [Especificação e regras de negócio](docs/especificacao.md)
- [Arquitetura e diagramas](docs/arquitetura.md)
- [Decisões de arquitetura](docs/decisions/0001-fundamentos.md)
- [Linguagem ubíqua](docs/ddd/linguagem-ubiqua.md) e [modelo de domínio](docs/ddd/dominio.md)
- [Event Storming no Miro](https://miro.com/app/board/uXjVETuWRiI=/)
- [Event Storming](docs/ddd/event-storming.md)
- [C4 até o nível 3 e exportações](docs/ddd/c4.md)
- [Contrato OpenAPI](docs/api/openapi.yaml)
- [Requisitos e evidências da entrega](docs/entrega-fase-1.md)
- [Vídeo da apresentação](entrega/video/README.md)
- [Diagramas C4 em PDF](docs/ddd/diagramas.pdf)

O modelo DDD é uma construção conceitual do projeto. A entrega identifica **Grupo 1 — Lucas Velôso — RM 377934**; Discord: **Velos0**. O desenvolvimento teve apoio de IA da OpenAI/Codex (GPT-6) e da Anthropic (Claude), com responsabilidade acadêmica do integrante.

O [repositório é público](https://github.com/oveloso/tech-challenge-oficina). A `main` está protegida: mudanças por PR, checks `backend` e `compose-smoke` obrigatórios, regras aplicadas também a administradores e bloqueio de force push e exclusão. Como o grupo possui um integrante, não há aprovação adicional obrigatória. O histórico da implementação e seus checks estão no [PR #1](https://github.com/oveloso/tech-challenge-oficina/pull/1).

O PDF de entrega reúne identificação, links e o commit completo da versão avaliada; o link público do YouTube será adicionado após a gravação. O escopo técnico é uma API executada localmente por HTTP em loopback, demonstrada pelo Swagger; não inclui implantação pública. HTTPS seria necessário para uma futura exposição em rede.
