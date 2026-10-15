//go:build integration

package api_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
)

func stockOf(t *testing.T, s apitest.Server, product *oas.Product) oas.StockLevel {
	t.Helper()
	page, err := s.Client.GetStock(ctx, oas.GetStockParams{PointId: s.PointID})
	require.NoError(t, err)
	for _, level := range page.Items {
		if level.Product.ID == product.ID {
			return level
		}
	}
	t.Fatalf("product %s is not in the stock overview", product.Name)
	return oas.StockLevel{}
}

func TestStockOverview(t *testing.T) {
	s := apitest.New(t)
	cream, bacon, salt := create(t, s, "Сливки 33%"), create(t, s, "Бекон"), create(t, s, "Соль")
	dish := createDish(t, s, "Соус")
	_, err := s.Client.ChangeRecipe(ctx, recipe(per(cream, 1)), oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: dish.ID})
	require.NoError(t, err)
	supply(t, s, cream, 60, oct(12))
	supply(t, s, cream, 40, oct(20))

	t.Run("several batches", func(t *testing.T) {
		level := stockOf(t, s, cream)
		require.Equal(t, *cream, level.Product)
		require.EqualValues(t, 100, level.AvailableQuantity)
		require.Zero(t, level.ExpiredQuantity)
		require.EqualValues(t, 2, level.Batches)
		require.Equal(t, oct(12), level.NearestExpiresAt.Value)
	})
	t.Run("product without batches", func(t *testing.T) {
		level := stockOf(t, s, salt)
		require.Zero(t, level.AvailableQuantity)
		require.Zero(t, level.ExpiredQuantity)
		require.Zero(t, level.Batches)
		require.False(t, level.NearestExpiresAt.Set)
	})
	t.Run("quarantined and rejected quantity is not stock", func(t *testing.T) {
		registered := registerDelivery(t, s, deliveryItem(bacon, 100, oct(20)))
		_, err := s.Client.ApproveDelivery(ctx, &oas.DeliveryApproval{Items: []oas.ItemDecision{{
			ItemID: registered.Items[0].ID, AcceptedQuantity: 60, RejectedQuantity: 10, QuarantinedQuantity: 30,
			RejectReason: oas.NewOptWriteOffReason(oas.WriteOffReasonPhysicalDamage), QuarantineReason: oas.NewOptWriteOffReason(oas.WriteOffReasonStorageViolation),
			Comment: oas.NewOptString("c"),
		}}}, oas.ApproveDeliveryParams{PointId: s.PointID, DeliveryId: registered.ID})
		require.NoError(t, err)
		require.EqualValues(t, 60, stockOf(t, s, bacon).AvailableQuantity)
	})
	t.Run("right after an order, without any projection", func(t *testing.T) {
		placed, err := s.Client.PlaceOrder(ctx, order(portions(dish, 60)), oas.PlaceOrderParams{PointId: s.PointID})
		require.NoError(t, err)
		require.Equal(t, oas.OrderStatusAccepted, placed.Status)

		level := stockOf(t, s, cream)
		require.EqualValues(t, 40, level.AvailableQuantity)
		require.EqualValues(t, 1, level.Batches, "a batch consumed to zero is not counted")
		require.Equal(t, oct(20), level.NearestExpiresAt.Value)
	})
	t.Run("pagination by product name", func(t *testing.T) {
		page, err := s.Client.GetStock(ctx, oas.GetStockParams{PointId: s.PointID, Limit: oas.NewOptInt32(2)})
		require.NoError(t, err)
		require.EqualValues(t, 3, page.Total)
		require.Equal(t, []string{"Бекон", "Сливки 33%"}, []string{page.Items[0].Product.Name, page.Items[1].Product.Name})

		next, err := s.Client.GetStock(ctx, oas.GetStockParams{PointId: s.PointID, Limit: oas.NewOptInt32(2), Offset: oas.NewOptInt32(2)})
		require.NoError(t, err)
		require.Len(t, next.Items, 1)
		require.Equal(t, "Соль", next.Items[0].Product.Name)
	})
	t.Run("expired batch", func(t *testing.T) {
		supply(t, s, cream, 25, oct(18))
		s.Clock.Set(oct(18)) // срок партии до 18 октября истёк, до 20 октября - нет
		level := stockOf(t, s, cream)
		require.EqualValues(t, 40, level.AvailableQuantity)
		require.EqualValues(t, 25, level.ExpiredQuantity)
		require.EqualValues(t, 2, level.Batches)
		require.Equal(t, oct(20), level.NearestExpiresAt.Value, "the nearest date of a batch that is still good")

		// то же правило, что у заказа: просроченное недоступно
		rejected, err := s.Client.PlaceOrder(ctx, order(portions(dish, 41)), oas.PlaceOrderParams{PointId: s.PointID})
		require.NoError(t, err)
		require.Equal(t, int64(40), rejected.Shortages[0].AvailableQuantity)
	})
}

func TestDeliveryHistory(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	registered := registerDelivery(t, s, deliveryItem(cream, 100, oct(20)))
	params := oas.GetDeliveryHistoryParams{PointId: s.PointID, DeliveryId: registered.ID}
	itemID := registered.Items[0].ID
	registeredAt := s.Clock.Now()

	history, err := s.Client.GetDeliveryHistory(ctx, params)
	require.NoError(t, err)
	require.Len(t, history.Items, 1)
	require.Equal(t, oas.DeliveryHistoryEntryTypeRegistered, history.Items[0].Type)
	require.Equal(t, *registered, history.Items[0].Delivery)

	// приёмку и дальнейшие действия выполняет другой пользователь
	approver := s.Staff(t, "approver", oas.RoleApprover)
	s.Clock.Advance(time.Hour)
	_, err = approver.ApproveDelivery(ctx, &oas.DeliveryApproval{Items: []oas.ItemDecision{{
		ItemID: itemID, AcceptedQuantity: 60, RejectedQuantity: 10, QuarantinedQuantity: 30,
		RejectReason: oas.NewOptWriteOffReason(oas.WriteOffReasonPhysicalDamage), QuarantineReason: oas.NewOptWriteOffReason(oas.WriteOffReasonStorageViolation),
		Comment: oas.NewOptString("c"),
	}}}, oas.ApproveDeliveryParams{PointId: s.PointID, DeliveryId: registered.ID})
	require.NoError(t, err)
	s.Clock.Advance(time.Hour)
	_, err = approver.ResolveQuarantine(ctx, &oas.QuarantineResolution{AcceptedQuantity: 30},
		oas.ResolveQuarantineParams{PointId: s.PointID, DeliveryId: registered.ID, ItemId: itemID})
	require.NoError(t, err)
	s.Clock.Advance(time.Hour)
	_, err = approver.CorrectDeliveryApproval(ctx, &oas.ApprovalCorrection{
		Reason: "ошибка пересчёта",
		Items:  []oas.ItemCorrection{{ItemID: itemID, AcceptedQuantity: 95, RejectedQuantity: 5}},
	}, oas.CorrectDeliveryApprovalParams{PointId: s.PointID, DeliveryId: registered.ID})
	require.NoError(t, err)

	history, err = s.Client.GetDeliveryHistory(ctx, params)
	require.NoError(t, err)
	require.Len(t, history.Items, 4)

	types := make([]oas.DeliveryHistoryEntryType, len(history.Items))
	distributions := make([][]int64, len(history.Items))
	for i, entry := range history.Items {
		types[i] = entry.Type
		distributions[i] = itemDistribution(entry.Delivery.Items[0])
	}
	require.Equal(t, []oas.DeliveryHistoryEntryType{
		oas.DeliveryHistoryEntryTypeRegistered, oas.DeliveryHistoryEntryTypeApproved,
		oas.DeliveryHistoryEntryTypeQuarantineAccepted, oas.DeliveryHistoryEntryTypeCorrected,
	}, types)
	require.Equal(t, [][]int64{{0, 0, 0}, {60, 10, 30}, {90, 10, 0}, {95, 5, 0}}, distributions,
		"the original approval stays visible after the correction")

	require.Equal(t, registeredAt, history.Items[0].OccurredAt)
	require.Equal(t, registeredAt.Add(3*time.Hour), history.Items[3].OccurredAt)
	require.Equal(t, oas.Actor{Type: oas.ActorTypeUser, ID: "dev", Name: oas.NewOptString("dev")}, history.Items[0].Actor)
	require.Equal(t, oas.Actor{Type: oas.ActorTypeUser, ID: "approver", Name: oas.NewOptString("approver")}, history.Items[1].Actor)
	require.False(t, history.Items[1].Reason.Set)
	require.Equal(t, "ошибка пересчёта", history.Items[3].Reason.Value)
	require.Len(t, history.Items[1].Delivery.Items[0].BatchIds, 1)
	require.Len(t, history.Items[3].Delivery.Items[0].BatchIds, 3)

	read, err := s.Client.GetDelivery(ctx, oas.GetDeliveryParams{PointId: s.PointID, DeliveryId: registered.ID})
	require.NoError(t, err)
	require.Equal(t, *read, history.Items[3].Delivery, "the last entry is the current state")

	_, err = s.Client.GetDeliveryHistory(ctx, oas.GetDeliveryHistoryParams{PointId: s.PointID, DeliveryId: uuid.Must(uuid.NewV7())})
	requireProblem(t, err, http.StatusNotFound)
}

func TestProductMovements(t *testing.T) {
	s := apitest.New(t)
	cream, salt := create(t, s, "Сливки 33%"), create(t, s, "Соль")
	dish := createDish(t, s, "Соус")
	_, err := s.Client.ChangeRecipe(ctx, recipe(per(cream, 1)), oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: dish.ID})
	require.NoError(t, err)

	early := supply(t, s, cream, 20, oct(12))
	late := supply(t, s, cream, 100, oct(20))
	s.Clock.Advance(time.Hour)
	waiter := s.Staff(t, "waiter", oas.RoleWaiter)
	placed, err := waiter.PlaceOrder(ctx, order(portions(dish, 30)), oas.PlaceOrderParams{PointId: s.PointID}) // 20 из ранней партии и 10 из поздней
	require.NoError(t, err)
	s.Clock.Advance(time.Hour)
	manager := s.Staff(t, "manager", oas.RoleManager)
	_, err = manager.WriteOffStock(ctx, &oas.WriteOff{
		Quantity: 20, Reason: oas.WriteOffReasonPhysicalDamage, Comment: "разбили",
	}, oas.WriteOffStockParams{PointId: s.PointID, BatchId: late})
	require.NoError(t, err)
	_, err = manager.AdjustStock(ctx, &oas.Adjustment{Delta: 5, Reason: "inventory_count"}, oas.AdjustStockParams{PointId: s.PointID, BatchId: late})
	require.NoError(t, err)

	page, err := s.Client.ListProductMovements(ctx, oas.ListProductMovementsParams{PointId: s.PointID, ProductId: cream.ID})
	require.NoError(t, err)
	require.EqualValues(t, 5, page.Total)
	types := make([]oas.MovementType, len(page.Items))
	for i, movement := range page.Items {
		types[i] = movement.Type
	}
	require.Equal(t, []oas.MovementType{
		oas.MovementTypeAdjusted, oas.MovementTypeWrittenOff, oas.MovementTypeConsumed,
		oas.MovementTypeReceived, oas.MovementTypeReceived,
	}, types, "latest first")

	adjusted, writtenOff, consumed, received := page.Items[0], page.Items[1], page.Items[2], page.Items[3]
	require.Equal(t, []oas.BatchChange{{BatchID: late, DeliveryNumber: "ПН-000123", Delta: 5}}, adjusted.Changes)
	require.Equal(t, "inventory_count", adjusted.Reason.Value)
	require.Equal(t, []oas.BatchChange{{BatchID: late, DeliveryNumber: "ПН-000123", Delta: -20}}, writtenOff.Changes)
	require.Equal(t, "physical_damage", writtenOff.Reason.Value)
	require.Equal(t, "разбили", writtenOff.Comment.Value)
	require.Equal(t, oas.Actor{Type: oas.ActorTypeUser, ID: "manager", Name: oas.NewOptString("manager")}, writtenOff.Actor)

	require.Equal(t, []oas.BatchChange{{BatchID: early, DeliveryNumber: "ПН-000123", Delta: -20}, {BatchID: late, DeliveryNumber: "ПН-000123", Delta: -10}}, consumed.Changes,
		"one movement for an order that took two batches")
	require.Equal(t, placed.ID, consumed.OrderID.Value)
	require.Equal(t, oas.Actor{Type: oas.ActorTypeUser, ID: "waiter", Name: oas.NewOptString("waiter")}, consumed.Actor)
	require.Equal(t, placed.PlacedAt.Value, consumed.OccurredAt)

	require.Equal(t, []oas.BatchChange{{BatchID: late, DeliveryNumber: "ПН-000123", Delta: 100}}, received.Changes)
	require.True(t, received.DeliveryID.Set)
	require.Equal(t, oct(20), received.ExpiresAt.Value)

	// сумма изменений по партии равна её остатку
	sums := map[uuid.UUID]int64{}
	for _, movement := range page.Items {
		for _, change := range movement.Changes {
			sums[change.BatchID] += change.Delta
		}
	}
	batches, err := s.Client.ListProductBatches(ctx, oas.ListProductBatchesParams{PointId: s.PointID, ProductId: cream.ID})
	require.NoError(t, err)
	for _, batch := range batches.Items {
		require.Equal(t, batch.Quantity, sums[batch.ID], "batch %s", batch.ID)
	}
	require.Equal(t, map[uuid.UUID]int64{early: 0, late: 75}, sums)

	t.Run("pagination", func(t *testing.T) {
		second, err := s.Client.ListProductMovements(ctx, oas.ListProductMovementsParams{
			PointId:   s.PointID,
			ProductId: cream.ID, Limit: oas.NewOptInt32(2), Offset: oas.NewOptInt32(2),
		})
		require.NoError(t, err)
		require.EqualValues(t, 5, second.Total)
		require.Equal(t, []oas.Movement{page.Items[2], page.Items[3]}, second.Items)

		beyond, err := s.Client.ListProductMovements(ctx, oas.ListProductMovementsParams{PointId: s.PointID, ProductId: cream.ID, Offset: oas.NewOptInt32(50)})
		require.NoError(t, err)
		require.Empty(t, beyond.Items)
		require.EqualValues(t, 5, beyond.Total)
	})
	t.Run("product without movements", func(t *testing.T) {
		empty, err := s.Client.ListProductMovements(ctx, oas.ListProductMovementsParams{PointId: s.PointID, ProductId: salt.ID})
		require.NoError(t, err)
		require.Empty(t, empty.Items)
		require.Zero(t, empty.Total)
	})
	t.Run("unknown product", func(t *testing.T) {
		_, err := s.Client.ListProductMovements(ctx, oas.ListProductMovementsParams{PointId: s.PointID, ProductId: uuid.Must(uuid.NewV7())})
		requireProblem(t, err, http.StatusNotFound)
	})
}

func TestExpiringBatches(t *testing.T) {
	s := apitest.New(t)
	cream, bacon := create(t, s, "Сливки 33%"), create(t, s, "Бекон")
	dish := createDish(t, s, "Соус")
	_, err := s.Client.ChangeRecipe(ctx, recipe(per(bacon, 1)), oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: dish.ID})
	require.NoError(t, err)
	expired := supply(t, s, cream, 40, oct(12))
	soon := supply(t, s, cream, 60, oct(14))
	later := supply(t, s, cream, 80, oct(23))
	empty := supply(t, s, bacon, 10, oct(13))
	s.Clock.Set(oct(12).Add(14 * time.Hour)) // первая партия просрочена, вторая истекает через 34 часа

	ids := func(page *oas.ExpiringBatchPage) []uuid.UUID {
		out := make([]uuid.UUID, len(page.Items))
		for i, item := range page.Items {
			out[i] = item.Batch.ID
		}
		return out
	}

	page, err := s.Client.ListExpiringBatches(ctx, oas.ListExpiringBatchesParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{expired, empty, soon}, ids(page), "the nearest expiry first, within the default 72 hours")
	require.EqualValues(t, 3, page.Total)
	require.True(t, page.Items[0].Expired)
	require.False(t, page.Items[2].Expired)
	require.Equal(t, *cream, page.Items[0].Product)
	require.Equal(t, *bacon, page.Items[1].Product)
	require.EqualValues(t, 40, page.Items[0].Batch.Quantity)

	// израсходованная до нуля партия из списка уходит
	placed, err := s.Client.PlaceOrder(ctx, order(portions(dish, 10)), oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, oas.OrderStatusAccepted, placed.Status)
	page, err = s.Client.ListExpiringBatches(ctx, oas.ListExpiringBatchesParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{expired, soon}, ids(page))

	// списанная просрочка тоже
	_, err = s.Client.WriteOffStock(ctx, &oas.WriteOff{
		Quantity: 40, Reason: oas.WriteOffReasonExpired, Comment: "срок истёк",
	}, oas.WriteOffStockParams{PointId: s.PointID, BatchId: expired})
	require.NoError(t, err)
	page, err = s.Client.ListExpiringBatches(ctx, oas.ListExpiringBatchesParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{soon}, ids(page))

	// горизонт запроса
	wide, err := s.Client.ListExpiringBatches(ctx, oas.ListExpiringBatchesParams{PointId: s.PointID, WithinHours: oas.NewOptInt32(300)})
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{soon, later}, ids(wide))
	narrow, err := s.Client.ListExpiringBatches(ctx, oas.ListExpiringBatchesParams{PointId: s.PointID, WithinHours: oas.NewOptInt32(24)})
	require.NoError(t, err)
	require.Empty(t, narrow.Items)
	require.Zero(t, narrow.Total)

	paged, err := s.Client.ListExpiringBatches(ctx, oas.ListExpiringBatchesParams{
		PointId:     s.PointID,
		WithinHours: oas.NewOptInt32(300), Limit: oas.NewOptInt32(1), Offset: oas.NewOptInt32(1),
	})
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{later}, ids(paged))
	require.EqualValues(t, 2, paged.Total)

	for _, query := range []string{"?within_hours=0", "?within_hours=-5", "?within_hours=abc"} {
		resp, body := raw(t, s, http.MethodGet, pointPath(s)+"/batches/expiring"+query, "", nil)
		requireRawProblem(t, resp, body, http.StatusBadRequest)
	}

	// адрес /batches/expiring не мешает чтению партии по идентификатору
	read, err := s.Client.GetBatch(ctx, oas.GetBatchParams{PointId: s.PointID, BatchId: soon})
	require.NoError(t, err)
	require.Equal(t, soon, read.ID)
}

// Список партий отдаётся страницами: у одного продукта партий со временем сотни, и без
// пагинации ответ рос без предела.
func TestProductBatchesArePaged(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	sauce := dishWith(t, s, "Соус", per(cream, 100))
	first := supply(t, s, cream, 100, oct(12))
	supply(t, s, cream, 300, oct(15))
	supply(t, s, cream, 300, oct(20))
	placed, err := s.Client.PlaceOrder(ctx, order(portions(sauce, 1)), oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, oas.OrderStatusAccepted, placed.Status, "the order uses up the first batch")

	all, err := s.Client.ListProductBatches(ctx, oas.ListProductBatchesParams{PointId: s.PointID, ProductId: cream.ID})
	require.NoError(t, err)
	require.Len(t, all.Items, 3)
	require.EqualValues(t, 3, all.Total)
	require.Equal(t, first, all.Items[0].ID, "the empty batch is still listed by default")

	page, err := s.Client.ListProductBatches(ctx, oas.ListProductBatchesParams{
		PointId:   s.PointID,
		ProductId: cream.ID, Limit: oas.NewOptInt32(2), Offset: oas.NewOptInt32(2),
	})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.EqualValues(t, 3, page.Total)
	require.Equal(t, all.Items[2].ID, page.Items[0].ID)

	stocked, err := s.Client.ListProductBatches(ctx, oas.ListProductBatchesParams{PointId: s.PointID, ProductId: cream.ID, InStock: oas.NewOptBool(true)})
	require.NoError(t, err)
	require.Len(t, stocked.Items, 2)
	require.EqualValues(t, 2, stocked.Total)
	for _, batch := range stocked.Items {
		require.Positive(t, batch.Quantity)
	}

	beyond, err := s.Client.ListProductBatches(ctx, oas.ListProductBatchesParams{PointId: s.PointID, ProductId: cream.ID, Offset: oas.NewOptInt32(10)})
	require.NoError(t, err)
	require.Empty(t, beyond.Items)
	require.EqualValues(t, 3, beyond.Total)

	// неизвестный продукт - 404, как у движений; раньше был пустой список
	_, err = s.Client.ListProductBatches(ctx, oas.ListProductBatchesParams{PointId: s.PointID, ProductId: uuid.Must(uuid.NewV7())})
	requireProblem(t, err, http.StatusNotFound)
}

func TestBatchesCarryDeliveryNumber(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	// принимает поставку с номером целиком и возвращает партию
	supplyNumbered := func(number string, quantity int64, expiresAt time.Time) uuid.UUID {
		t.Helper()
		registered, err := s.Client.RegisterDelivery(ctx, &oas.NewDelivery{
			Number: number, DeliveredAt: s.Clock.Now(), Items: []oas.NewDeliveryItem{deliveryItem(cream, quantity, expiresAt)},
		}, oas.RegisterDeliveryParams{PointId: s.PointID})
		require.NoError(t, err)
		approved, err := s.Client.ApproveDelivery(ctx, acceptAllItems(registered),
			oas.ApproveDeliveryParams{PointId: s.PointID, DeliveryId: registered.ID})
		require.NoError(t, err)
		return approved.Items[0].BatchIds[0]
	}
	first := supplyNumbered("Н-1", 20, oct(12))
	second := supplyNumbered("Н-2", 100, oct(20))
	numbers := map[uuid.UUID]string{first: "Н-1", second: "Н-2"}

	// проекции не применялись: номер не зависит от асинхронного списка поставок
	list, err := s.Client.ListProductBatches(ctx, oas.ListProductBatchesParams{PointId: s.PointID, ProductId: cream.ID})
	require.NoError(t, err)
	require.Len(t, list.Items, 2)
	for _, batch := range list.Items {
		require.Equal(t, numbers[batch.ID], batch.DeliveryNumber)
	}
	one, err := s.Client.GetBatch(ctx, oas.GetBatchParams{PointId: s.PointID, BatchId: second})
	require.NoError(t, err)
	require.Equal(t, "Н-2", one.DeliveryNumber)
	expiring, err := s.Client.ListExpiringBatches(ctx, oas.ListExpiringBatchesParams{PointId: s.PointID, WithinHours: oas.NewOptInt32(8760)})
	require.NoError(t, err)
	require.Len(t, expiring.Items, 2)
	for _, item := range expiring.Items {
		require.Equal(t, numbers[item.Batch.ID], item.Batch.DeliveryNumber)
	}

	adjusted, err := s.Client.AdjustStock(ctx, &oas.Adjustment{Delta: -5, Reason: "inventory_count"},
		oas.AdjustStockParams{PointId: s.PointID, BatchId: second})
	require.NoError(t, err)
	require.Equal(t, "Н-2", adjusted.DeliveryNumber)
	writtenOff, err := s.Client.WriteOffStock(ctx, &oas.WriteOff{Quantity: 20, Reason: oas.WriteOffReasonOther, Comment: "бой"},
		oas.WriteOffStockParams{PointId: s.PointID, BatchId: first})
	require.NoError(t, err)
	require.Equal(t, "Н-1", writtenOff.DeliveryNumber)
	require.Zero(t, writtenOff.Quantity)

	// первая партия пуста, но её движения называют поставку
	page, err := s.Client.ListProductMovements(ctx, oas.ListProductMovementsParams{PointId: s.PointID, ProductId: cream.ID})
	require.NoError(t, err)
	require.Len(t, page.Items, 4)
	for _, movement := range page.Items {
		for _, change := range movement.Changes {
			require.Equal(t, numbers[change.BatchID], change.DeliveryNumber, movement.Type)
		}
	}
}

func TestStockFiltersAndSummary(t *testing.T) {
	s := apitest.New(t)
	inCategory := func(name, category string) *oas.Product {
		t.Helper()
		product, err := s.Client.CreateProduct(ctx, &oas.NewProduct{Name: name, Unit: oas.UnitG, Category: oas.NewOptString(category)},
			oas.CreateProductParams{CompanyId: s.CompanyID})
		require.NoError(t, err)
		return product
	}
	cream := inCategory("Сливки 33%", "Молочные")
	butter := inCategory("Сливочное масло", "Молочные")
	inCategory("Соль", "Бакалея")
	percent := inCategory("%-ный уксус", "Бакалея")
	supply(t, s, cream, 40, oct(12))
	supply(t, s, cream, 60, oct(20))
	supply(t, s, butter, 500, oct(13))
	supply(t, s, percent, 5, oct(30))
	s.Clock.Set(oct(12).Add(6 * time.Hour)) // партия сливок просрочена, масло истекает через 18 часов

	names := func(params oas.GetStockParams) ([]string, int64) {
		t.Helper()
		params.PointId = s.PointID
		page, err := s.Client.GetStock(ctx, params)
		require.NoError(t, err)
		out := make([]string, len(page.Items))
		for i, level := range page.Items {
			out[i] = level.Product.Name
		}
		return out, page.Total
	}
	list := func(params oas.GetStockParams) []string {
		t.Helper()
		out, _ := names(params)
		return out
	}

	t.Run("search by the start of the name", func(t *testing.T) {
		found, total := names(oas.GetStockParams{Q: oas.NewOptString("СЛИВ")})
		require.Equal(t, []string{"Сливки 33%", "Сливочное масло"}, found)
		require.EqualValues(t, 2, total)
		require.Equal(t, []string{"%-ный уксус"}, list(oas.GetStockParams{Q: oas.NewOptString("%")}), "the pattern sign is literal")
		require.Empty(t, list(oas.GetStockParams{Q: oas.NewOptString("масло")}), "not the middle of the name")
	})
	t.Run("category", func(t *testing.T) {
		require.Equal(t, []string{"%-ный уксус", "Соль"}, list(oas.GetStockParams{Category: oas.NewOptString("Бакалея")}))
		found, total := names(oas.GetStockParams{Category: oas.NewOptString("Мясо")})
		require.Empty(t, found)
		require.Zero(t, total)
	})
	t.Run("status", func(t *testing.T) {
		status := func(v oas.StockStatus) []string { return list(oas.GetStockParams{Status: oas.NewOptStockStatus(v)}) }
		require.Equal(t, []string{"%-ный уксус", "Сливки 33%", "Сливочное масло"}, status(oas.StockStatusInStock))
		require.Equal(t, []string{"Сливочное масло"}, status(oas.StockStatusExpiring))
		require.Equal(t, []string{"Сливки 33%"}, status(oas.StockStatusExpired))
		require.Equal(t, []string{"Соль"}, status(oas.StockStatusOutOfStock))
	})
	t.Run("sorting", func(t *testing.T) {
		sorted := func(v oas.GetStockSort) []string { return list(oas.GetStockParams{Sort: oas.NewOptGetStockSort(v)}) }
		require.Equal(t, []string{"Соль", "Сливочное масло", "Сливки 33%", "%-ный уксус"}, sorted(oas.GetStockSortMinusName))
		require.Equal(t, []string{"Сливочное масло", "Сливки 33%", "%-ный уксус", "Соль"}, sorted(oas.GetStockSortMinusAvailable))
		require.Equal(t, []string{"Сливочное масло", "Сливки 33%", "%-ный уксус", "Соль"}, sorted(oas.GetStockSortNearestExpiry))
		require.Equal(t, []string{"%-ный уксус", "Сливки 33%", "Сливочное масло", "Соль"}, sorted(oas.GetStockSortMinusNearestExpiry),
			"a product without a date is last in both directions")
	})
	t.Run("page after the filter", func(t *testing.T) {
		found, total := names(oas.GetStockParams{
			Status: oas.NewOptStockStatus(oas.StockStatusInStock), Limit: oas.NewOptInt32(2), Offset: oas.NewOptInt32(2),
		})
		require.Equal(t, []string{"Сливочное масло"}, found)
		require.EqualValues(t, 3, total)
	})
	t.Run("unknown sort", func(t *testing.T) {
		resp, data := raw(t, s, http.MethodGet, pointPath(s)+"/stock?sort=price", "", nil)
		requireRawProblem(t, resp, data, http.StatusBadRequest)
	})
	t.Run("summary", func(t *testing.T) {
		summary, err := s.Client.GetStockSummary(ctx, oas.GetStockSummaryParams{PointId: s.PointID})
		require.NoError(t, err)
		require.Equal(t, oas.StockSummary{Total: 4, InStock: 3, Expiring: 1, Expired: 1, OutOfStock: 1}, *summary)

		dairy, err := s.Client.GetStockSummary(ctx, oas.GetStockSummaryParams{
			PointId: s.PointID, Q: oas.NewOptString("слив"), Category: oas.NewOptString("Молочные"),
		})
		require.NoError(t, err)
		require.Equal(t, oas.StockSummary{Total: 2, InStock: 2, Expiring: 1, Expired: 1}, *dairy)
	})
	t.Run("summary needs inventory.view", func(t *testing.T) {
		waiter := s.Staff(t, "waiter", oas.RoleWaiter)
		_, err := waiter.GetStockSummary(ctx, oas.GetStockSummaryParams{PointId: s.PointID})
		requireProblem(t, err, http.StatusForbidden)
	})
}
