//go:build integration

package api_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
)

func TestAnalyticsAuthorizationAndReports(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	pasta := dishWith(t, s, "Паста", per(cream, 10))
	batchID := supply(t, s, cream, 100, oct(20))

	accepted, err := s.Client.PlaceOrder(ctx, order(portions(pasta, 5)), oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, oas.OrderStatusAccepted, accepted.Status)
	_, err = s.Client.WriteOffStock(ctx, &oas.WriteOff{
		Quantity: 10, Reason: oas.WriteOffReasonPhysicalDamage, Comment: "повреждение",
	}, oas.WriteOffStockParams{PointId: s.PointID, BatchId: batchID})
	require.NoError(t, err)
	rejected, err := s.Client.PlaceOrder(ctx, order(portions(pasta, 10)), oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, oas.OrderStatusRejected, rejected.Status)

	from := s.Clock.Now().Add(-time.Hour)
	s.Clock.Advance(time.Hour)
	to := s.Clock.Now().Add(time.Hour)
	s.Sync(t)

	writeOffs, err := s.Client.GetWriteOffStatistics(ctx, oas.GetWriteOffStatisticsParams{PointId: s.PointID, From: oas.NewOptDateTime(from), To: oas.NewOptDateTime(to)})
	require.NoError(t, err)
	require.Equal(t, []oas.WriteOffStatistic{{
		Product: *cream, Reason: oas.WriteOffReasonPhysicalDamage, Quantity: 10, Operations: 1,
	}}, writeOffs.Items)

	shortages, err := s.Client.GetShortageStatistics(ctx, oas.GetShortageStatisticsParams{PointId: s.PointID, From: oas.NewOptDateTime(from), To: oas.NewOptDateTime(to)})
	require.NoError(t, err)
	require.Len(t, shortages.Items, 1)
	require.Equal(t, cream.ID, shortages.Items[0].Product.ID)
	require.EqualValues(t, 1, shortages.Items[0].RejectedOrders)

	forecast, err := s.Client.GetStopListForecast(ctx, oas.GetStopListForecastParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Len(t, forecast.Items, 1)
	require.Equal(t, pasta.ID, forecast.Items[0].Dish.ID)
	require.Equal(t, cream.ID, forecast.Items[0].LimitingProduct.ID)
	require.True(t, forecast.Items[0].ExpectedAt.After(s.Clock.Now()))
	// 100 принято, 50 ушло в заказ, 10 списано; 50 израсходовано за час до запроса
	require.EqualValues(t, 40, forecast.Items[0].AvailableQuantity)
	require.EqualValues(t, 10, forecast.Items[0].RequiredQuantity)
	require.InDelta(t, 50, forecast.Items[0].ConsumptionPerHour, 0.01)
	require.Equal(t, oas.StopListForecastCauseConsumption, forecast.Items[0].Cause)

	withoutPermission := s.Staff(t, "cook", oas.RoleCook)
	_, err = withoutPermission.GetWriteOffStatistics(ctx, oas.GetWriteOffStatisticsParams{PointId: s.PointID, From: oas.NewOptDateTime(from), To: oas.NewOptDateTime(to)})
	requireProblem(t, err, http.StatusForbidden)

	_, err = s.Client.GetShortageStatistics(ctx, oas.GetShortageStatisticsParams{PointId: s.PointID, From: oas.NewOptDateTime(to), To: oas.NewOptDateTime(from)})
	problem := requireProblem(t, err, http.StatusBadRequest)
	require.Equal(t, "bad-request", problem.Type)
}

func TestWriteOffDaysFollowTheClientTimeZone(t *testing.T) {
	s := apitest.New(t)
	s.Clock.Set(oct(13).Add(22 * time.Hour))
	tomatoes := create(t, s, "Томаты")
	batchID := supply(t, s, tomatoes, 1000, oct(20))
	for _, quantity := range []int64{300, 200} {
		s.Clock.Advance(15 * time.Minute)
		_, err := s.Client.WriteOffStock(ctx, &oas.WriteOff{
			Quantity: quantity, Reason: oas.WriteOffReasonPhysicalDamage, Comment: "упаковка",
		}, oas.WriteOffStockParams{PointId: s.PointID, BatchId: batchID})
		require.NoError(t, err)
	}
	s.Sync(t)

	period := func(zone string) oas.GetWriteOffDaysParams {
		return oas.GetWriteOffDaysParams{
			PointId: s.PointID, From: oas.NewOptDateTime(oct(13)), To: oas.NewOptDateTime(oct(15)),
			TimeZone: oas.NewOptString(zone),
		}
	}
	// 22:15 и 22:30 UTC - ещё 13 октября по UTC, но уже 14-е по Москве
	for zone, day := range map[string]time.Time{"UTC": oct(13), "Europe/Moscow": oct(14)} {
		days, err := s.Client.GetWriteOffDays(ctx, period(zone))
		require.NoError(t, err)
		require.Equal(t, []oas.WriteOffDay{{
			Day: day, Product: *tomatoes, Reason: oas.WriteOffReasonPhysicalDamage, Quantity: 500, Operations: 2,
		}}, days.Items, zone)
	}

	_, err := s.Client.GetWriteOffDays(ctx, period("Mars/Olympus"))
	problem := requireProblem(t, err, http.StatusBadRequest)
	require.Equal(t, "Некорректный часовой пояс", problem.Title)
}
