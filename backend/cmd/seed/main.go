package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"oficina/internal/domain"
	"oficina/internal/auth"
	"oficina/internal/platform"
	"oficina/internal/registry"
	"oficina/internal/workorders"
	"oficina/migrations"
	"os"
	"strings"
	"time"
)

func main() {
	if e := run(); e != nil {
		slog.Error("seed failed", "reason", e.Error())
		os.Exit(1)
	}
}
func run() error {
	username, password := strings.TrimSpace(os.Getenv("ADMIN_USERNAME")), os.Getenv("ADMIN_PASSWORD")
	if len(username) < 1 || len(username) > 100 || len(password) < 12 || len(password) > 256 {
		return errors.New("informe ADMIN_USERNAME (1–100 caracteres) e ADMIN_PASSWORD (12–256 bytes)")
	}
	if os.Getenv("DATABASE_URL") == "" {
		return errors.New("DATABASE_URL é obrigatória")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	db, e := platform.Open(ctx, os.Getenv("DATABASE_URL"))
	if e != nil {
		return errors.New("não foi possível conectar ao banco")
	}
	defer db.Close()
	if e = migrations.Apply(ctx, db); e != nil {
		return errors.New("não foi possível aplicar migrações")
	}
	hash, e := auth.PasswordHash(password)
	if e != nil {
		return e
	}
	_, e = db.Exec(ctx, "INSERT INTO administrators(id,username,password_hash) VALUES($1,$2,$3) ON CONFLICT(username) DO NOTHING", uuid.NewString(), username, string(hash))
	if e != nil {
		return errors.New("não foi possível gravar administrador")
	}
	if os.Getenv("DEMO_DATA") == "true" {
		if e = demo(ctx, db); e != nil {
			return e
		}
	}
	fmt.Println("Seed concluído.")
	return nil
}
func demo(ctx context.Context, db *pgxpool.Pool) error {
	var exists bool
	if e := db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM customers WHERE document='52998224725')").Scan(&exists); e != nil {
		return e
	}
	if exists {
		return nil
	}
	r := &registry.Service{DB: db}
	makeRecord := func(kind registry.Kind, payload string) (platform.Record, error) {
		data, e := platform.Decode([]byte(payload))
		if e != nil {
			return nil, e
		}
		return r.Write(ctx, kind, "", data, true)
	}
	ids := func(kind registry.Kind, payloads ...string) ([]string, error) {
		out := []string{}
		for _, payload := range payloads {
			record, e := makeRecord(kind, payload)
			if e != nil {
				return nil, e
			}
			out = append(out, platform.Text(record, "id"))
		}
		return out, nil
	}
	customers, e := ids(registry.CustomerKind,
		`{"name":"Cliente Demonstração","documentType":"CPF","document":"52998224725","email":"cliente@example.com","phone":null}`,
		`{"name":"Ana Demonstração","documentType":"CPF","document":"11144477735","email":"ana@example.com","phone":null}`,
		`{"name":"Bruno Demonstração","documentType":"CPF","document":"93541134780","email":"bruno@example.com","phone":null}`)
	if e != nil {
		return e
	}
	vehicles, e := ids(registry.VehicleKind,
		fmt.Sprintf(`{"customerId":%q,"plate":"DEM1A23","brand":"Marca Demo","model":"Hatch de demonstração","year":2024}`, customers[0]),
		fmt.Sprintf(`{"customerId":%q,"plate":"ANA2B34","brand":"Marca Demo","model":"Sedã de demonstração","year":2021}`, customers[1]),
		fmt.Sprintf(`{"customerId":%q,"plate":"BRU3C45","brand":"Marca Demo","model":"Picape de demonstração","year":2019}`, customers[2]))
	if e != nil {
		return e
	}
	services, e := ids(registry.CatalogServiceKind,
		`{"name":"Revisão de freios","description":"Serviço sintético para demonstrar o fluxo","priceCents":15000}`,
		`{"name":"Troca de óleo","description":"Serviço sintético para demonstrar o fluxo","priceCents":9000}`,
		`{"name":"Alinhamento e balanceamento","description":"Serviço sintético para demonstrar o fluxo","priceCents":12000}`)
	if e != nil {
		return e
	}
	// O disco tem saldo 1 para demonstrar o 409 de estoque insuficiente.
	items, e := ids(registry.InventoryItemKind,
		`{"sku":"DEMO-PASTILHA","name":"Pastilha de demonstração","type":"PART","priceCents":8900,"initialQuantity":20}`,
		`{"sku":"DEMO-FILTRO","name":"Filtro de óleo de demonstração","type":"PART","priceCents":4500,"initialQuantity":8}`,
		`{"sku":"DEMO-OLEO","name":"Óleo 5W30 de demonstração (litro)","type":"SUPPLY","priceCents":5200,"initialQuantity":30}`,
		`{"sku":"DEMO-DISCO","name":"Disco de freio de demonstração","type":"PART","priceCents":21000,"initialQuantity":1}`)
	if e != nil {
		return e
	}
	w := &workorders.Service{DB: db, TokenTTL: time.Hour}
	part := func(item string, quantity int64) workorders.PartSelection {
		return workorders.PartSelection{InventoryItemID: item, Quantity: quantity}
	}
	history := []struct {
		customer, vehicle, problem string
		selection                  workorders.Selection
		daysAgo                    int
		duration                   time.Duration
	}{
		{customers[1], vehicles[1], "Troca de óleo no prazo — demonstração", workorders.Selection{ServiceIDs: []string{services[1]}, Parts: []workorders.PartSelection{part(items[1], 1), part(items[2], 4)}}, 6, 45 * time.Minute},
		{customers[2], vehicles[2], "Freio chiando — demonstração", workorders.Selection{ServiceIDs: []string{services[0]}, Parts: []workorders.PartSelection{part(items[0], 2)}}, 4, 2 * time.Hour},
		{customers[1], vehicles[1], "Volante puxando para a direita — demonstração", workorders.Selection{ServiceIDs: []string{services[2]}, Parts: []workorders.PartSelection{}}, 2, 80 * time.Minute},
	}
	for _, h := range history {
		if e = delivered(ctx, w, h.customer, h.vehicle, h.problem, h.selection, time.Now().UTC().AddDate(0, 0, -h.daysAgo), h.duration); e != nil {
			return e
		}
	}
	if _, e = w.Create(ctx, customers[0], vehicles[0], "Ruído ao frear — demonstração", workorders.Selection{ServiceIDs: []string{services[0]}, Parts: []workorders.PartSelection{part(items[0], 2)}}); e != nil {
		return e
	}
	_, e = w.Create(ctx, customers[2], vehicles[2], "Disco de freio empenado — demonstração", workorders.Selection{ServiceIDs: []string{services[0]}, Parts: []workorders.PartSelection{part(items[3], 2)}})
	return e
}

// delivered percorre o fluxo real da OS até a entrega e depois desloca os horários
// para o passado, para que as métricas de tempo médio tenham dados.
func delivered(ctx context.Context, w *workorders.Service, customer, vehicle, problem string, selection workorders.Selection, start time.Time, duration time.Duration) error {
	o, e := w.Create(ctx, customer, vehicle, problem, selection)
	if e != nil {
		return e
	}
	step := func(action, serviceID string) error {
		result, e := w.Execute(ctx, o.ID, o.Version, action, serviceID, workorders.Selection{})
		if e != nil {
			return e
		}
		if next, ok := result.(*domain.Order); ok {
			o = next
		} else {
			o.Version++
		}
		return nil
	}
	for _, action := range []string{"diagnosis/start", "quotes"} {
		if e = step(action, ""); e != nil {
			return e
		}
	}
	access, e := w.Execute(ctx, o.ID, o.Version, "customer-access", "", workorders.Selection{})
	if e != nil {
		return e
	}
	hash := sha256.Sum256([]byte(access.(map[string]any)["token"].(string)))
	if o, e = w.Decide(ctx, hash[:], o.CurrentQuote.ID, "APPROVED"); e != nil {
		return e
	}
	for _, s := range o.Services {
		if e = step("services/start", s.ID); e != nil {
			return e
		}
		if e = step("services/complete", s.ID); e != nil {
			return e
		}
	}
	for _, action := range []string{"complete", "deliver"} {
		if e = step(action, ""); e != nil {
			return e
		}
	}
	at := func(d time.Duration) *time.Time {
		t := start.Add(d)
		return &t
	}
	started, completed := 70*time.Minute, 70*time.Minute+duration
	times := map[string]time.Duration{"ORDER_RECEIVED": 0, "DIAGNOSIS_STARTED": 10 * time.Minute, "QUOTE_PUBLISHED": 30 * time.Minute, "QUOTE_APPROVED": time.Hour, "SERVICE_STARTED": started, "SERVICE_COMPLETED": completed, "ORDER_COMPLETED": completed + 10*time.Minute, "ORDER_DELIVERED": completed + 2*time.Hour}
	o.CreatedAt = start
	o.ExecutionStartedAt, o.StockConsumedAt = at(started), at(started)
	o.CompletedAt, o.DeliveredAt = at(times["ORDER_COMPLETED"]), at(times["ORDER_DELIVERED"])
	for i := range o.Services {
		o.Services[i].StartedAt, o.Services[i].CompletedAt = at(started), at(completed)
	}
	for i := range o.History {
		o.History[i].OccurredAt = *at(times[o.History[i].Type])
	}
	for i := range o.Quotes {
		o.Quotes[i].PublishedAt, o.Quotes[i].DecidedAt = *at(times["QUOTE_PUBLISHED"]), at(times["QUOTE_APPROVED"])
	}
	if o.CurrentQuote != nil {
		o.CurrentQuote.PublishedAt, o.CurrentQuote.DecidedAt = *at(times["QUOTE_PUBLISHED"]), at(times["QUOTE_APPROVED"])
	}
	return workorders.Save(ctx, w.DB, o, false)
}
