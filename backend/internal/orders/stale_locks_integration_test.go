//go:build integration

package orders

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
	"goden/backend/internal/organization"
	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/command"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/postgres/pgtest"
)

// companyID - компания, в которой тест создаёт каталог.
var companyID = uuid.Must(uuid.NewV7())

// point - точка, на которой тесты пакета ведут склад; её компания - companyID.
var point = organization.Point{ID: uuid.Must(uuid.NewV7()), CompanyID: companyID, Active: true}

// Рецептура изменилась после того, как заказ прочитал её и выбрал блокировки.
// Попытка со старым набором блокировок не должна списать продукт без блокировки.
func TestPlaceWithStaleLocks(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	clk := clock.NewFake(time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC))
	runner := command.NewRunner(pool, clk, slog.New(slog.DiscardHandler))
	cat := catalog.NewService(runner, pool)
	stock := inventory.NewService(runner, pool, cat)
	svc := NewService(runner, pool, cat, stock)
	actor := eventstore.Actor{Type: eventstore.ActorUser, ID: "user-1"}

	product := func(name string) catalog.Product {
		p, err := cat.CreateProduct(ctx, actor, "", companyID, catalog.NewProduct{Name: name, Unit: catalog.UnitGram})
		require.NoError(t, err)
		d, err := stock.RegisterDelivery(ctx, actor, "", point, inventory.NewDelivery{
			Number: "ПН-1", DeliveredAt: clk.Now(),
			Items: []inventory.NewDeliveryItem{{ProductID: p.ID, DeclaredQuantity: 1000, ExpiresAt: clk.Now().Add(240 * time.Hour)}},
		})
		require.NoError(t, err)
		_, err = stock.ApproveDelivery(ctx, actor, "", d.ID, []inventory.ItemDecision{{ItemID: d.Items[0].ID, AcceptedQuantity: 1000}})
		require.NoError(t, err)
		return p
	}
	remaining := func(p catalog.Product) int64 {
		batches, err := stock.Batches(ctx, point.ID, p.ID)
		require.NoError(t, err)
		return batches[0].Quantity
	}
	countEvents := func() (n int) {
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM events").Scan(&n))
		return n
	}

	milk, oat := product("Сливки"), product("Овсяное молоко")
	dish, err := cat.CreateDish(ctx, actor, "", companyID, "Паста")
	require.NoError(t, err)
	_, err = cat.ChangeRecipe(ctx, actor, "", dish.ID, []catalog.Ingredient{{ProductID: milk.ID, Quantity: 80}})
	require.NoError(t, err)
	items := []OrderItem{{DishID: dish.ID, Portions: 1}}

	// заказ прочитал рецептуру со сливками...
	staleNeeds := []inventory.Need{{ProductID: milk.ID, Quantity: 80}}
	// ...а до взятия блокировок рецептуру перевели на другой продукт
	_, err = cat.ChangeRecipe(ctx, actor, "", dish.ID, []catalog.Ingredient{{ProductID: oat.ID, Quantity: 70}})
	require.NoError(t, err)
	before := countEvents()

	_, err = svc.place(ctx, actor, "", point, newID(), items, staleNeeds)

	require.ErrorIs(t, err, errRecipesChanged)
	require.Equal(t, before, countEvents(), "the stale attempt wrote nothing")
	require.Equal(t, int64(1000), remaining(milk))
	require.Equal(t, int64(1000), remaining(oat))

	// публичная команда перечитывает рецептуру и списывает по новой
	order, err := svc.PlaceOrder(ctx, actor, "", point, items)
	require.NoError(t, err)
	require.Equal(t, OrderAcceptedStatus, order.Status)
	require.Equal(t, int64(1000), remaining(milk))
	require.Equal(t, int64(930), remaining(oat))
}
