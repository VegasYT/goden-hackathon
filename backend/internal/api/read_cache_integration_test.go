//go:build integration

package api_test

import (
	"context"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"goden/backend/internal/api"
	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
	"goden/backend/internal/platform/config"
	"goden/backend/internal/platform/dockertest"
	platformredis "goden/backend/internal/platform/redis"
)

func startRedis(t *testing.T) (string, testcontainers.Container) {
	t.Helper()
	dockertest.Setup()
	container, err := testcontainers.Run(context.Background(), "redis:8.8.3-alpine",
		testcontainers.WithExposedPorts("6379/tcp"),
		testcontainers.WithWaitStrategy(wait.ForLog("Ready to accept connections").WithStartupTimeout(time.Minute)))
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, container)
	endpoint, err := container.PortEndpoint(context.Background(), "6379/tcp", "redis")
	require.NoError(t, err)
	return endpoint, container
}

func newCachedServer(t *testing.T) (apitest.Server, testcontainers.Container) {
	t.Helper()
	endpoint, container := startRedis(t)
	server := apitest.NewWithOptions(t, func(t testing.TB, pool *pgxpool.Pool, options *api.Options) {
		cache, err := platformredis.New(config.Redis{
			URL: endpoint, CacheTTL: time.Minute, Timeout: 100 * time.Millisecond,
		}, slog.New(slog.DiscardHandler), pool)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, cache.Close()) })
		options.ReadCache = cache
	})
	return server, container
}

func TestReadCache(t *testing.T) {
	s, redisContainer := newCachedServer(t)
	cream := create(t, s, "Сливки 33%")
	registered := registerDelivery(t, s, deliveryItem(cream, 100, oct(20)))
	s.Sync(t)

	_, err := s.Client.ListDeliveries(ctx, oas.ListDeliveriesParams{PointId: s.PointID})
	require.NoError(t, err)
	_, err = s.Client.ListDeliveries(ctx, oas.ListDeliveriesParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, 1.0, s.Metric(t, "goden_read_cache_requests_total", "result", "miss"))
	require.Equal(t, 1.0, s.Metric(t, "goden_read_cache_requests_total", "result", "hit"))

	// Параметры запроса входят в ключ.
	_, err = s.Client.ListDeliveries(ctx, oas.ListDeliveriesParams{PointId: s.PointID, Limit: oas.NewOptInt32(1)})
	require.NoError(t, err)
	require.Equal(t, 2.0, s.Metric(t, "goden_read_cache_requests_total", "result", "miss"))

	// После продвижения проекции прежний ключ не используется.
	registerDelivery(t, s, deliveryItem(cream, 50, oct(21)))
	s.Sync(t)
	page, err := s.Client.ListDeliveries(ctx, oas.ListDeliveriesParams{PointId: s.PointID})
	require.NoError(t, err)
	require.EqualValues(t, 2, page.Total)
	require.Equal(t, 3.0, s.Metric(t, "goden_read_cache_requests_total", "result", "miss"))

	dish := dishWith(t, s, "Соус", per(cream, 10))
	batchID := supply(t, s, cream, 100, oct(22))
	_, err = s.Client.PlaceOrder(ctx, order(portions(dish, 1)), oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	s.Sync(t)
	_, err = s.Client.ListOrders(ctx, oas.ListOrdersParams{PointId: s.PointID})
	require.NoError(t, err)
	_, err = s.Client.ListOrders(ctx, oas.ListOrdersParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, 2.0, s.Metric(t, "goden_read_cache_requests_total", "result", "hit"))
	_, err = s.Client.ListOrders(ctx, oas.ListOrdersParams{PointId: s.PointID, Status: oas.NewOptListOrdersStatus(oas.ListOrdersStatusAccepted)})
	require.NoError(t, err)
	ordersMisses := s.Metric(t, "goden_read_cache_requests_total", "result", "miss")
	_, err = s.Client.PlaceOrder(ctx, order(portions(dish, 1)), oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	s.Sync(t)
	ordersPage, err := s.Client.ListOrders(ctx, oas.ListOrdersParams{PointId: s.PointID})
	require.NoError(t, err)
	require.EqualValues(t, 2, ordersPage.Total)
	require.Equal(t, ordersMisses+1, s.Metric(t, "goden_read_cache_requests_total", "result", "miss"))

	from, to := s.Clock.Now().Add(-time.Hour), s.Clock.Now().Add(time.Hour)
	_, err = s.Client.GetWriteOffStatistics(ctx, oas.GetWriteOffStatisticsParams{PointId: s.PointID, From: oas.NewOptDateTime(from), To: oas.NewOptDateTime(to)})
	require.NoError(t, err)
	_, err = s.Client.GetWriteOffStatistics(ctx, oas.GetWriteOffStatisticsParams{PointId: s.PointID, From: oas.NewOptDateTime(from), To: oas.NewOptDateTime(to)})
	require.NoError(t, err)
	_, err = s.Client.GetShortageStatistics(ctx, oas.GetShortageStatisticsParams{PointId: s.PointID, From: oas.NewOptDateTime(from), To: oas.NewOptDateTime(to)})
	require.NoError(t, err)
	_, err = s.Client.GetShortageStatistics(ctx, oas.GetShortageStatisticsParams{PointId: s.PointID, From: oas.NewOptDateTime(from), To: oas.NewOptDateTime(to)})
	require.NoError(t, err)
	require.Equal(t, 4.0, s.Metric(t, "goden_read_cache_requests_total", "result", "hit"))
	_, err = s.Client.GetWriteOffStatistics(ctx, oas.GetWriteOffStatisticsParams{PointId: s.PointID, From: oas.NewOptDateTime(from.Add(-time.Hour)), To: oas.NewOptDateTime(to)})
	require.NoError(t, err)
	analyticsMisses := s.Metric(t, "goden_read_cache_requests_total", "result", "miss")
	_, err = s.Client.WriteOffStock(ctx, &oas.WriteOff{
		Quantity: 5, Reason: oas.WriteOffReasonPhysicalDamage, Comment: "тест кеша",
	}, oas.WriteOffStockParams{PointId: s.PointID, BatchId: batchID})
	require.NoError(t, err)
	s.Sync(t)
	writeOffs, err := s.Client.GetWriteOffStatistics(ctx, oas.GetWriteOffStatisticsParams{PointId: s.PointID, From: oas.NewOptDateTime(from), To: oas.NewOptDateTime(to)})
	require.NoError(t, err)
	require.Len(t, writeOffs.Items, 1)
	require.Equal(t, analyticsMisses+1, s.Metric(t, "goden_read_cache_requests_total", "result", "miss"))

	// Критичные и зависящие от времени представления не проходят через кеш.
	before := s.Metric(t, "goden_read_cache_requests_total")
	_, err = s.Client.GetMenu(ctx, oas.GetMenuParams{PointId: s.PointID})
	require.NoError(t, err)
	_, err = s.Client.GetStopList(ctx, oas.GetStopListParams{PointId: s.PointID})
	require.NoError(t, err)
	_, err = s.Client.GetStock(ctx, oas.GetStockParams{PointId: s.PointID})
	require.NoError(t, err)
	_, err = s.Client.ListProductBatches(ctx, oas.ListProductBatchesParams{PointId: s.PointID, ProductId: cream.ID})
	require.NoError(t, err)
	_, err = s.Client.ListDishRecommendations(ctx, oas.ListDishRecommendationsParams{PointId: s.PointID})
	require.NoError(t, err)
	_, err = s.Client.ListDeliveryRecommendations(ctx, oas.ListDeliveryRecommendationsParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, before, s.Metric(t, "goden_read_cache_requests_total"))

	// Redis перестаёт отвечать: тот же запрос читается из PostgreSQL, readiness остаётся успешным.
	require.NoError(t, redisContainer.Stop(context.Background(), nil))
	page, err = s.Client.ListDeliveries(ctx, oas.ListDeliveriesParams{PointId: s.PointID})
	require.NoError(t, err)
	require.EqualValues(t, 3, page.Total)
	require.GreaterOrEqual(t, s.Metric(t, "goden_read_cache_requests_total", "result", "error"), 1.0)
	response, err := http.Get(s.URL + "/readyz")
	require.NoError(t, err)
	defer response.Body.Close() //nolint:errcheck
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.NotEqual(t, registered.ID, page.Items[0].ID, "latest delivery remains first after Redis failure")
}
