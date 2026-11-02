$ErrorActionPreference = 'Continue'
$projectRoot = Split-Path -Parent $PSScriptRoot
$network = 'oficina-testes'
$database = 'oficina-testes-db'
docker network create $network | Out-Null
try {
    docker run -d --rm --name $database --network $network -e POSTGRES_USER=oficina -e POSTGRES_DB=oficina_test -e POSTGRES_HOST_AUTH_METHOD=trust postgres:18-alpine | Out-Null
    $ready = $false
    for ($i = 0; $i -lt 30 -and -not $ready; $i++) {
        Start-Sleep -Seconds 1
        docker exec $database pg_isready -U oficina -d oficina_test 2>$null | Out-Null
        $ready = $LASTEXITCODE -eq 0
    }
    if (-not $ready) { throw 'PostgreSQL de teste não ficou pronto.' }
    docker run --rm --network $network -v "${projectRoot}\backend:/src" -w /src `
        -e "TEST_DATABASE_URL=postgres://oficina@${database}:5432/oficina_test?sslmode=disable" golang:1.27.3 `
        sh -c 'go mod download 2>/dev/null && go test -count=1 -coverpkg=./... -coverprofile=/tmp/coverage.out ./... && go tool cover -func=/tmp/coverage.out | tail -1 && go test -count=1 -cover ./internal/domain'
    if ($LASTEXITCODE -ne 0) { throw 'Falha nos testes.' }
} finally {
    docker stop $database 2>$null | Out-Null
    docker network rm $network 2>$null | Out-Null
}
