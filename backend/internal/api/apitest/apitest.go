//go:build integration

// Package apitest поднимает HTTP API на тестовой базе для интеграционных тестов эндпоинтов.
package apitest

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"goden/backend/internal/api"
	"goden/backend/internal/api/oas"
	"goden/backend/internal/generator"
	"goden/backend/internal/platform/auth"
	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/postgres/pgtest"
	"goden/backend/internal/platform/projection"
	"goden/backend/internal/readmodel"
	"goden/backend/internal/realtime"
)

// Origin - адрес, которому тестовый API разрешает кросс-доменные запросы.
const Origin = "http://localhost:5173"

// Server - запущенный API с клиентом и доступом к базе и часам.
type Server struct {
	// Client - сгенерированный клиент: им тесты ходят в API по контракту.
	// Он действует от имени пользователя dev со всеми правами.
	Client *oas.Client
	// URL - адрес сервера без префикса API, для сырых запросов и health-эндпоинтов.
	URL   string
	Pool  *pgxpool.Pool
	Clock *clock.Fake
	// Token - токен пользователя dev со всеми правами, для сырых запросов.
	Token string
	// CompanyID - компания пользователя dev: в ней тесты создают каталог.
	CompanyID uuid.UUID
	// PointID - точка этой компании: на ней тесты ведут склад и оформляют заказы.
	PointID uuid.UUID
	// Realtime позволяет интеграционным тестам посылать сигнал в SSE без Kafka.
	Realtime *realtime.Hub

	tokens  *tokens
	engine  *projection.Engine
	metrics *prometheus.Registry
}

// Sync применяет к проекциям все записанные события. Списки строятся асинхронно:
// тест вызывает Sync после записи и перед чтением списка. Фоновых проекторов в обвязке нет.
func (s Server) Sync(t testing.TB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, p := range readmodel.Projections() {
		require.NoError(t, s.engine.CatchUp(ctx, p), p.Name)
	}
}

// ProjectionMetrics возвращает реестр метрик проекций тестового сервера.
func (s Server) ProjectionMetrics() *prometheus.Registry { return s.metrics }

// New запускает API тем же конструктором, что и cmd/api, на отдельной базе теста.
// Токены проверяет подделка: настоящую проверку токенов Keycloak покрывают тесты пакета auth.
func New(t testing.TB) Server {
	return NewWithOptions(t, nil)
}

// NewWithOptions запускает API и позволяет тесту добавить инфраструктурные опции,
// которым нужен созданный для теста пул базы данных.
func NewWithOptions(t testing.TB, configure func(testing.TB, *pgxpool.Pool, *api.Options)) Server {
	t.Helper()
	pool := pgtest.NewDB(t)
	clk := clock.NewFake(time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC))
	issued := &tokens{principals: map[string]auth.Principal{}}
	hub := realtime.NewHub()

	options := api.Options{Verifier: issued, CORSOrigins: []string{Origin}, Realtime: hub}
	if configure != nil {
		configure(t, pool, &options)
	}
	handler, err := api.New(pool, clk, slog.New(slog.DiscardHandler), options)
	require.NoError(t, err)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	dev, err := auth.Disabled().Verify(context.Background(), "")
	require.NoError(t, err)
	metrics := prometheus.NewRegistry()
	s := Server{
		URL: server.URL, Pool: pool, Clock: clk, Token: issued.issue(dev), Realtime: hub, tokens: issued,
		engine: projection.NewEngine(pool, clk, slog.New(slog.DiscardHandler), metrics), metrics: metrics,
	}
	s.Client = s.client(t, s.Token)
	company, err := s.Client.CreateCompany(context.Background(), &oas.NewCompany{Name: "Тест"}, oas.CreateCompanyParams{})
	require.NoError(t, err)
	s.CompanyID = company.ID
	point, err := s.Client.CreatePoint(context.Background(), &oas.NewPoint{Name: "Точка"}, oas.CreatePointParams{CompanyId: company.ID})
	require.NoError(t, err)
	s.PointID = point.ID
	return s
}

// TokenFor выпускает токен клиента с перечисленными правами. Личность клиента следует из автора:
// у одного автора она одна во всех его токенах.
func (s Server) TokenFor(actor eventstore.Actor, permissions ...string) string {
	p := auth.NewPrincipal(actor, permissions...)
	p.User = auth.User{ID: "id-" + actor.ID, Email: actor.ID + "@goden.local", Name: actor.ID}
	return s.tokens.issue(p)
}

// Login возвращает клиент пользователя name без прав в токене: на точке и в компании их
// определяют владение и назначение. Первое обращение уже выполнено, поэтому пользователя
// можно назначить на точку по почте <name>@goden.local.
func (s Server) Login(t testing.TB, name string) *oas.Client {
	t.Helper()
	client := s.As(t, eventstore.Actor{Type: eventstore.ActorUser, ID: name})
	_, err := client.GetMe(context.Background())
	require.NoError(t, err)
	return client
}

// As возвращает клиент, действующий от имени actor с перечисленными правами.
func (s Server) As(t testing.TB, actor eventstore.Actor, permissions ...string) *oas.Client {
	t.Helper()
	return s.client(t, s.TokenFor(actor, permissions...))
}

// WithToken возвращает клиент с готовым токеном, например ключом поставщика: такой токен
// проверяет не подделка, а сам API.
func (s Server) WithToken(t testing.TB, token string) *oas.Client {
	t.Helper()
	return s.client(t, token)
}

// Security возвращает источник токена для клиента, созданного тестом самостоятельно.
func (s Server) Security() oas.SecuritySource {
	return generator.Security(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: s.Token}))
}

func (s Server) client(t testing.TB, token string) *oas.Client {
	t.Helper()
	source := generator.Security(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}))
	client, err := oas.NewClient(s.URL+api.Prefix, source)
	require.NoError(t, err)
	return client
}

// tokens - подделка проверки: токен действителен, если тест его выпустил.
type tokens struct {
	mu         sync.Mutex
	principals map[string]auth.Principal
}

func (v *tokens) issue(p auth.Principal) string {
	v.mu.Lock()
	defer v.mu.Unlock()
	token := fmt.Sprintf("test-token-%d", len(v.principals)+1)
	v.principals[token] = p
	return token
}

func (v *tokens) Verify(_ context.Context, token string) (auth.Principal, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	p, ok := v.principals[token]
	if !ok {
		return auth.Principal{}, fmt.Errorf("%w: unknown test token", auth.ErrInvalidToken)
	}
	return p, nil
}

// Hire назначает автора на точку пользователя dev с ролью role и возвращает его токен.
// Права на точке определяет назначение, а не токен: так тест получает клиента с правами роли.
func (s Server) Hire(t testing.TB, actor eventstore.Actor, role oas.Role) string {
	t.Helper()
	token := s.TokenFor(actor)
	ctx := context.Background()
	// первое обращение запоминает пользователя: без него назначение по почте его не найдёт
	_, err := s.client(t, token).GetMe(ctx)
	require.NoError(t, err)
	_, err = s.Client.AssignStaff(ctx, &oas.NewStaff{Email: actor.ID + "@goden.local", Role: role}, oas.AssignStaffParams{PointId: s.PointID})
	require.NoError(t, err)
	return token
}

// Staff возвращает клиент сотрудника name точки пользователя dev с ролью role.
func (s Server) Staff(t testing.TB, name string, role oas.Role) *oas.Client {
	t.Helper()
	return s.client(t, s.Hire(t, eventstore.Actor{Type: eventstore.ActorUser, ID: name}, role))
}

// Metric читает /metrics сервера и возвращает сумму значений серий метрики name с перечисленными
// метками (пары "имя", "значение"). Для гистограммы name - с суффиксом _count. Серий нет - ноль.
func (s Server) Metric(t testing.TB, name string, labels ...string) float64 {
	t.Helper()
	total := 0.0
	for _, value := range s.series(t, name, labels...) {
		total += value
	}
	return total
}

// MetricSeries возвращает число серий метрики name с перечисленными метками.
func (s Server) MetricSeries(t testing.TB, name string, labels ...string) int {
	t.Helper()
	return len(s.series(t, name, labels...))
}

func (s Server) series(t testing.TB, name string, labels ...string) []float64 {
	t.Helper()
	resp, err := http.Get(s.URL + "/metrics")
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var values []float64
	scanner := bufio.NewScanner(resp.Body)
lines:
	for scanner.Scan() {
		line := scanner.Text()
		rest, found := strings.CutPrefix(line, name)
		if !found || (!strings.HasPrefix(rest, "{") && !strings.HasPrefix(rest, " ")) {
			continue
		}
		for i := 0; i < len(labels); i += 2 {
			if !strings.Contains(rest, labels[i]+"=\""+labels[i+1]+"\"") {
				continue lines
			}
		}
		value, err := strconv.ParseFloat(rest[strings.LastIndex(rest, " ")+1:], 64)
		require.NoError(t, err, line)
		values = append(values, value)
	}
	require.NoError(t, scanner.Err())
	return values
}
