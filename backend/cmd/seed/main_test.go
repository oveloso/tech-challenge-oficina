package main

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"oficina/internal/platform"
)

func TestDemoSeed(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL não configurada: informe uma conexão PostgreSQL de teste para executar o seed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	server, err := platform.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal("não foi possível conectar ao PostgreSQL de teste; detalhes de conexão omitidos")
	}
	t.Cleanup(server.Close)
	name := "oficina_seed_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = server.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal("não foi possível criar o banco temporário do seed")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := server.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)"); err != nil {
			t.Error("não foi possível remover o banco temporário do seed")
		}
	})
	target, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal("TEST_DATABASE_URL inválida")
	}
	target.Path = "/" + name
	t.Setenv("DATABASE_URL", target.String())
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", uuid.NewString())
	t.Setenv("DEMO_DATA", "true")
	for range 2 {
		if err = run(); err != nil {
			t.Fatalf("seed falhou: %v", err)
		}
	}

	db, err := platform.Open(ctx, target.String())
	if err != nil {
		t.Fatal("não foi possível conectar ao banco do seed")
	}
	defer db.Close()
	var orders, delivered int64
	var mean *float64
	err = db.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE data->>'status'='DELIVERED'),
 avg(extract(epoch FROM ((data->>'completedAt')::timestamptz-(data->>'executionStartedAt')::timestamptz)))::float8
 FROM service_orders`).Scan(&orders, &delivered, &mean)
	if err != nil {
		t.Fatal("não foi possível consultar as OS do seed")
	}
	if orders != 5 || delivered != 3 || mean == nil || *mean <= 0 {
		t.Fatalf("seed inesperado: %d OS, %d entregues", orders, delivered)
	}
	var disc int64
	if err = db.QueryRow(ctx, "SELECT (data->>'balance')::bigint FROM inventory_items WHERE sku='DEMO-DISCO'").Scan(&disc); err != nil || disc != 1 {
		t.Fatal("disco de demonstração deve ter saldo 1")
	}
}
