# Verificações — 02/11/2026

Ambiente: Go 1.27.3 (imagem `golang:1.27.3`) e PostgreSQL 18 (imagem `postgres:18-alpine`), em contêineres descartáveis. A mesma verificação roda na CI a cada PR.

| Verificação | Resultado |
|---|---|
| `gofmt`, `go vet` e `go build` | Aprovados |
| Testes unitários e de integração com PostgreSQL, com `-race` | Aprovados; [saída](testes.txt) |
| Cobertura global com `-coverpkg=./...` | **88,4%**; [resultado por função](cobertura.txt) |
| Cobertura de `internal/domain` | **99,0%** |
| `govulncheck` 1.9.0 (dependências) | Zero vulnerabilidades alcançáveis; [relatório](govulncheck.txt) |
| `gosec` 2.30.0 (código) | Zero achados em 20 arquivos e 2.618 linhas; [relatório](gosec.txt) |
| [CI 41526702477](https://github.com/oveloso/tech-challenge-oficina/actions/runs/41526702477) | Jobs `backend` e `compose-smoke` aprovados |

Para reproduzir sem Go instalado, apenas com Docker:

```powershell
./scripts/test-docker.ps1
```

Com Go no PATH, `.env` configurado e o banco de testes criado, `./scripts/run.ps1 -Action test` executa `go test -count=1 -coverpkg=./... -coverprofile=coverage.out ./...`. Sem `TEST_DATABASE_URL`, a integração é ignorada e a cobertura será menor.

O [relatório do govulncheck](govulncheck.txt) registra um aviso em `golang.org/x/crypto`, módulo usado para bcrypt, em um pacote que a aplicação não importa. Foram encontrados zero problemas nos símbolos alcançáveis e nos pacotes importados. Conforme orientação do professor, não é necessário zerar os achados para apresentar o relatório. O resultado não substitui revisão de segurança nem garante ausência de vulnerabilidades.

O job `compose-smoke` constrói as imagens, sobe PostgreSQL, seed e API pelo Docker Compose, confere `/health`, `/docs` e `/openapi.yaml` e autentica o administrador criado pelo seed.

## Documentação e entrega

O Swagger é servido pela própria API em `/docs` e carrega os assets da versão 5.33.1 pela CDN. A especificação em `/openapi.yaml` e as operações da API são locais.

A documentação DDD reúne o [Event Storming no Miro](https://miro.com/app/board/uXjVETuWRiI=/?share_link_id=192620924232), o [C4 níveis 1, 2 e 3](../ddd/diagramas.pdf) e a [linguagem ubíqua](../ddd/linguagem-ubiqua.md). O nível 3 detalha o início do primeiro serviço com baixa transacional de estoque.

A identificação é **Grupo 1 — Lucas Velôso — RM 377934**. Discord: **Velos0**. O [vídeo](../../entrega/video/README.md) é gravado com a narração do integrante; o link público do YouTube será adicionado após a gravação.

O repositório público e a proteção da `main` estão configurados: PR obrigatório, checks `backend`/`compose-smoke`, aplicação a administradores, sem force push nem exclusão. O PDF de entrega registra a branch e o commit completo da versão avaliada. O escopo verificado é a API local por HTTP, sem implantação pública.
