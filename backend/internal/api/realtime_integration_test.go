//go:build integration

package api_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tckafka "github.com/testcontainers/testcontainers-go/modules/kafka"

	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
	"goden/backend/internal/platform/dockertest"
	platformkafka "goden/backend/internal/platform/kafka"
	"goden/backend/internal/platform/observability"
	"goden/backend/internal/realtime"
)

func TestRealtimeAuthenticationAndReconnect(t *testing.T) {
	s := apitest.New(t)

	resp, body := raw(t, s, http.MethodGet, pointPath(s)+"/events", "", map[string]string{"Authorization": ""})
	requireRawProblem(t, resp, body, http.StatusUnauthorized)
	require.Equal(t, "Bearer", resp.Header.Get("WWW-Authenticate"))

	resp, first := rawSSE(t, s, map[string]string{"Origin": apitest.Origin})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	require.Equal(t, apitest.Origin, resp.Header.Get("Access-Control-Allow-Origin"))
	require.Contains(t, string(first), "event: sync\n")
	require.NotEmpty(t, resp.Header.Get("X-Correlation-Id"))

	resp, second := rawSSE(t, s, map[string]string{"Last-Event-ID": uuid.Must(uuid.NewV7()).String()})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, string(second), "event: sync\n")
}

func TestRealtimeDeliversUpdate(t *testing.T) {
	s := apitest.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL+pointPath(s)+"/events", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+s.Token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck
	reader := bufio.NewReader(resp.Body)
	require.Contains(t, string(readSSEEvent(t, reader)), "event: sync\n")

	update := realtime.Update{
		ID: uuid.Must(uuid.NewV7()), OccurredAt: s.Clock.Now(), Categories: []realtime.Category{realtime.CategoryStock}, PointID: s.PointID,
	}
	s.Realtime.Publish(update)
	received := string(readSSEEvent(t, reader))
	require.Contains(t, received, "id: "+update.ID.String()+"\n")
	require.Contains(t, received, `"categories":["stock"]`)
}

func TestRealtimeOrderThroughKafka(t *testing.T) {
	dockertest.Setup()
	containerCtx, containerCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer containerCancel()
	container, err := tckafka.Run(containerCtx, "confluentinc/confluent-local:7.5.0")
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, container)
	brokers, err := container.Brokers(containerCtx)
	require.NoError(t, err)
	cfg := platformkafka.Config{Brokers: brokers, TopicPrefix: "realtime-test"}

	s := apitest.New(t)
	cream := create(t, s, "Сливки для realtime")
	dish := createDish(t, s, "Соус realtime")
	_, err = s.Client.ChangeRecipe(ctx, recipe(per(cream, 100)), oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: dish.ID})
	require.NoError(t, err)
	supply(t, s, cream, 250, oct(20))

	producer, err := platformkafka.NewProducer(cfg, "realtime-integration-publisher")
	require.NoError(t, err)
	t.Cleanup(producer.Close)
	publisher := platformkafka.NewPublisher(s.Pool, producer, s.Clock, slog.New(slog.DiscardHandler),
		observability.NewRegistry(), cfg.TopicPrefix)
	publishRealtimeBatch(t, publisher)

	reader, err := realtime.NewReader(cfg, s.Realtime, slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	readerCtx, stopReader := context.WithCancel(context.Background())
	readerDone := make(chan error, 1)
	go func() { readerDone <- reader.Run(readerCtx) }()
	t.Cleanup(func() {
		stopReader()
		require.NoError(t, <-readerDone)
		reader.Close()
	})

	// Маркер подтверждает, что reader уже начал читать с конца, прежде чем тест создаст заказ.
	warmupResp, warmupReader, closeWarmup := openSSE(t, s)
	defer closeWarmup()
	require.Equal(t, http.StatusOK, warmupResp.StatusCode)
	require.Contains(t, string(readSSEEvent(t, warmupReader)), "event: sync\n")
	waitForRealtimeReader(t, producer, cfg.TopicPrefix, warmupReader, s.Clock.Now(), s.PointID)
	closeWarmup()

	firstResp, firstReader, closeFirst := openSSE(t, s)
	defer closeFirst()
	secondResp, secondReader, closeSecond := openSSE(t, s)
	defer closeSecond()
	require.Equal(t, http.StatusOK, firstResp.StatusCode)
	require.Equal(t, http.StatusOK, secondResp.StatusCode)
	require.Contains(t, string(readSSEEvent(t, firstReader)), "event: sync\n")
	require.Contains(t, string(readSSEEvent(t, secondReader)), "event: sync\n")

	placed, err := s.Client.PlaceOrder(ctx, order(portions(dish, 2)), oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, oas.OrderStatusAccepted, placed.Status)
	publishRealtimeBatch(t, publisher)

	// событие заказа и складские события идут разными топиками: склеятся ли они в один сигнал,
	// зависит от времени доставки, поэтому сверяются категории всех сигналов заказа
	expected := []realtime.Category{
		realtime.CategoryExpiry, realtime.CategoryOrders, realtime.CategoryRecommendations,
		realtime.CategoryStock, realtime.CategoryStopList,
	}
	first := realtimeCategoriesUntil(t, firstReader, expected)
	// оба клиента API получают сигналы заказа
	realtimeCategoriesUntil(t, secondReader, expected)
	closeFirst()
	closeSecond()

	resp, reconnected := rawSSE(t, s, map[string]string{"Last-Event-ID": first.ID.String()})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, string(reconnected), "event: sync\n")
}

// realtimeCategoriesUntil читает сигналы, пока их категории вместе не составят expected,
// и возвращает последний сигнал.
func realtimeCategoriesUntil(t *testing.T, reader *bufio.Reader, expected []realtime.Category) realtime.Update {
	t.Helper()
	seen := map[realtime.Category]bool{}
	for range len(expected) {
		update := decodeRealtimeUpdate(t, readSSEEvent(t, reader))
		for _, category := range update.Categories {
			seen[category] = true
		}
		if len(seen) == len(expected) {
			for _, category := range expected {
				require.True(t, seen[category], category)
			}
			return update
		}
	}
	require.Failf(t, "signals do not cover the expected categories", "seen %v, expected %v", seen, expected)
	return realtime.Update{}
}

type sseRead struct {
	event []byte
	err   error
}

func publishRealtimeBatch(t *testing.T, publisher *platformkafka.Publisher) {
	t.Helper()
	var lastErr error
	require.Eventually(t, func() bool {
		_, lastErr = publisher.PublishBatch(context.Background())
		return lastErr == nil
	}, 30*time.Second, 200*time.Millisecond, "Kafka topics did not become writable: %v", lastErr)
}

func waitForRealtimeReader(t *testing.T, producer platformkafka.Producer, prefix string,
	reader *bufio.Reader, occurredAt time.Time, pointID uuid.UUID,
) {
	t.Helper()
	received := make(chan sseRead, 1)
	go func() {
		event, err := readSSEEventRaw(reader)
		received <- sseRead{event: event, err: err}
	}()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(20 * time.Second)
	defer timeout.Stop()

	for {
		marker := platformkafka.Envelope{
			EnvelopeVersion: platformkafka.EnvelopeVersion, EventID: uuid.Must(uuid.NewV7()),
			EventType: "StockAdjusted", OccurredAt: occurredAt,
			// сигнал без точки никому не уходит: маркер адресован точке клиента
			Payload: json.RawMessage(`{"point_id":"` + pointID.String() + `"}`),
		}
		raw, err := json.Marshal(marker)
		require.NoError(t, err)
		require.NoError(t, producer.Produce(context.Background(), platformkafka.Message{
			Topic: platformkafka.FullTopic(prefix, platformkafka.StockTopic), Value: raw,
		}))
		select {
		case result := <-received:
			require.NoError(t, result.err)
			require.Contains(t, string(result.event), "event: update\n")
			return
		case <-ticker.C:
		case <-timeout.C:
			t.Fatal("realtime reader did not consume the Kafka marker")
		}
	}
}

func openSSE(t *testing.T, s apitest.Server) (*http.Response, *bufio.Reader, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL+pointPath(s)+"/events", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+s.Token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	closeResponse := func() {
		_ = resp.Body.Close()
		cancel()
	}
	return resp, bufio.NewReader(resp.Body), closeResponse
}

func decodeRealtimeUpdate(t *testing.T, event []byte) realtime.Update {
	t.Helper()
	for _, line := range strings.Split(string(event), "\n") {
		if data, ok := strings.CutPrefix(line, "data: "); ok {
			var update realtime.Update
			require.NoError(t, json.Unmarshal([]byte(data), &update))
			return update
		}
	}
	t.Fatalf("SSE update has no data: %s", event)
	return realtime.Update{}
}

func rawSSE(t *testing.T, s apitest.Server, headers map[string]string) (*http.Response, []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL+pointPath(s)+"/events", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+s.Token)
	for name, value := range headers {
		if value == "" {
			req.Header.Del(name)
		} else {
			req.Header.Set(name, value)
		}
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close() //nolint:errcheck
		body, readErr := io.ReadAll(resp.Body)
		require.NoError(t, readErr)
		return resp, body
	}
	body := readSSEEvent(t, bufio.NewReader(resp.Body))
	require.NoError(t, resp.Body.Close())
	cancel()
	return resp, body
}

func readSSEEvent(t *testing.T, reader *bufio.Reader) []byte {
	t.Helper()
	event, err := readSSEEventRaw(reader)
	require.NoError(t, err)
	return event
}

func readSSEEventRaw(reader *bufio.Reader) ([]byte, error) {
	var out strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		out.WriteString(line)
		if line == "\n" {
			return []byte(out.String()), nil
		}
	}
}

// openStream открывает поток точки и дочитывает до sync: после него клиент подписан.
func openStream(t *testing.T, s apitest.Server, token string, pointID uuid.UUID) *bufio.Reader {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL+"/api/v1/points/"+pointID.String()+"/events", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { resp.Body.Close() }) //nolint:errcheck
	require.Equal(t, http.StatusOK, resp.StatusCode)
	reader := bufio.NewReader(resp.Body)
	require.Contains(t, string(readSSEEvent(t, reader)), "event: sync"+"\n")
	return reader
}

// Поток точки получает сигналы своей точки и каталога своей компании и не получает чужих.
func TestStreamIsScopedToThePoint(t *testing.T) {
	s := apitest.New(t)
	other, err := s.Client.CreatePoint(ctx, &oas.NewPoint{Name: "Вторая"}, oas.CreatePointParams{CompanyId: s.CompanyID})
	require.NoError(t, err)
	stream := openStream(t, s, s.Token, s.PointID)
	sibling := openStream(t, s, s.Token, other.ID)

	signal := func(pointID, companyID uuid.UUID) realtime.Update {
		update := realtime.Update{
			ID: uuid.Must(uuid.NewV7()), OccurredAt: s.Clock.Now(), Categories: []realtime.Category{realtime.CategoryStock},
			PointID: pointID, CompanyID: companyID,
		}
		s.Realtime.Publish(update)
		return update
	}
	// порядок публикации: чужие сигналы раньше своего, поэтому свой - первый, что дойдёт
	signal(other.ID, uuid.Nil)                               // соседняя точка
	signal(uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())) // чужая компания
	own := signal(s.PointID, uuid.Nil)

	require.Equal(t, own.ID, decodeRealtimeUpdate(t, readSSEEvent(t, stream)).ID)
	// следующий сигнал публикуется после чтения первого: сигналы, пришедшие подряд, склеиваются
	catalog := signal(uuid.Nil, s.CompanyID)
	received := readSSEEvent(t, stream)
	require.Equal(t, catalog.ID, decodeRealtimeUpdate(t, received).ID)
	require.NotContains(t, string(received), s.CompanyID.String(), "the signal carries no addressing")

	// соседняя точка той же компании: свой сигнал склада и общий сигнал каталога
	require.NotEqual(t, own.ID, decodeRealtimeUpdate(t, readSSEEvent(t, sibling)).ID)
	require.Equal(t, catalog.ID, decodeRealtimeUpdate(t, readSSEEvent(t, sibling)).ID)
}

func TestStreamRequiresAccessToThePoint(t *testing.T) {
	s := apitest.New(t)
	path := pointPath(s) + "/events"

	// сотрудник точки подписывается, посторонний - нет, даже со всеми правами в токене
	openStream(t, s, s.Hire(t, user("cook"), oas.RoleCook), s.PointID)
	stranger := s.TokenFor(user("stranger"), allPermissions...)
	resp, body := raw(t, s, http.MethodGet, path, "", bearer(stranger))
	problem := requireRawProblem(t, resp, body, http.StatusForbidden)
	require.Contains(t, problem.Detail, "point.view")

	resp, body = raw(t, s, http.MethodGet, "/api/v1/points/"+uuid.Must(uuid.NewV7()).String()+"/events", "", nil)
	requireRawProblem(t, resp, body, http.StatusNotFound)
	resp, body = raw(t, s, http.MethodGet, "/api/v1/points/42/events", "", nil)
	requireRawProblem(t, resp, body, http.StatusBadRequest)
	// прежнего общего потока нет
	resp, body = raw(t, s, http.MethodGet, "/api/v1/events", "", nil)
	requireRawProblem(t, resp, body, http.StatusNotFound)
}
