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

func oct(day int) time.Time { return time.Date(2026, 10, day, 0, 0, 0, 0, time.UTC) }

func deliveryItem(product *oas.Product, quantity int64, expiresAt time.Time) oas.NewDeliveryItem {
	return oas.NewDeliveryItem{ProductID: product.ID, DeclaredQuantity: quantity, ExpiresAt: expiresAt}
}

func registerDelivery(t *testing.T, s apitest.Server, items ...oas.NewDeliveryItem) *oas.Delivery {
	t.Helper()
	delivery, err := s.Client.RegisterDelivery(ctx, &oas.NewDelivery{
		Number: "ПН-000123", DeliveredAt: s.Clock.Now(), Items: items,
	}, oas.RegisterDeliveryParams{PointId: s.PointID})
	require.NoError(t, err)
	return delivery
}

func acceptAllItems(d *oas.Delivery) *oas.DeliveryApproval {
	approval := &oas.DeliveryApproval{}
	for _, item := range d.Items {
		approval.Items = append(approval.Items, oas.ItemDecision{ItemID: item.ID, AcceptedQuantity: item.DeclaredQuantity})
	}
	return approval
}

// supply принимает поставку одного продукта целиком и возвращает идентификатор партии.
func supply(t *testing.T, s apitest.Server, product *oas.Product, quantity int64, expiresAt time.Time) uuid.UUID {
	t.Helper()
	registered := registerDelivery(t, s, deliveryItem(product, quantity, expiresAt))
	approved, err := s.Client.ApproveDelivery(ctx, acceptAllItems(registered), oas.ApproveDeliveryParams{PointId: s.PointID, DeliveryId: registered.ID})
	require.NoError(t, err)
	return approved.Items[0].BatchIds[0]
}

func quantities(t *testing.T, s apitest.Server, product *oas.Product) []int64 {
	t.Helper()
	batches, err := s.Client.ListProductBatches(ctx, oas.ListProductBatchesParams{PointId: s.PointID, ProductId: product.ID})
	require.NoError(t, err)
	out := make([]int64, len(batches.Items))
	for i, batch := range batches.Items {
		out[i] = batch.Quantity
	}
	return out
}

func itemDistribution(item oas.DeliveryItem) []int64 {
	return []int64{item.AcceptedQuantity, item.RejectedQuantity, item.QuarantinedQuantity}
}

// Поставки.

func TestDeliveryThroughAPI(t *testing.T) {
	s := apitest.New(t)
	cream, bacon := create(t, s, "Сливки 33%"), create(t, s, "Бекон")
	made := oct(9)

	registered, err := s.Client.RegisterDelivery(ctx, &oas.NewDelivery{
		Number: "ПН-000123", DeliveredAt: s.Clock.Now(),
		Items: []oas.NewDeliveryItem{
			{ProductID: cream.ID, DeclaredQuantity: 10, ExpiresAt: oct(15), ManufacturedAt: oas.NewOptDateTime(made)},
			deliveryItem(bacon, 500, oct(20)),
		},
	}, oas.RegisterDeliveryParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, oas.DeliveryStatusRegistered, registered.Status)
	require.Len(t, registered.Items, 2)
	require.False(t, registered.ApprovedAt.Set)
	require.Empty(t, registered.Items[0].BatchIds)
	require.Empty(t, quantities(t, s, cream), "registration does not change stock")

	// приёмка: частично с карантином и отбраковкой
	approved, err := s.Client.ApproveDelivery(ctx, &oas.DeliveryApproval{Items: []oas.ItemDecision{
		{
			ItemID: registered.Items[0].ID, AcceptedQuantity: 6, RejectedQuantity: 1, QuarantinedQuantity: 3,
			RejectReason: oas.NewOptWriteOffReason(oas.WriteOffReasonPhysicalDamage), QuarantineReason: oas.NewOptWriteOffReason(oas.WriteOffReasonStorageViolation),
			Comment: oas.NewOptString("одна вскрыта, три тёплые"),
		},
		{ItemID: registered.Items[1].ID, AcceptedQuantity: 500},
	}}, oas.ApproveDeliveryParams{PointId: s.PointID, DeliveryId: registered.ID})
	require.NoError(t, err)
	require.Equal(t, oas.DeliveryStatusApproved, approved.Status)
	require.Equal(t, oas.Actor{Type: oas.ActorTypeUser, ID: "dev", Name: oas.NewOptString("dev")}, approved.ApprovedBy.Value)
	require.Equal(t, s.Clock.Now(), approved.ApprovedAt.Value)
	require.Equal(t, []int64{6, 1, 3}, itemDistribution(approved.Items[0]))
	require.Len(t, approved.Items[0].BatchIds, 1)
	require.Equal(t, "physical_damage", approved.Items[0].RejectReason.Value)
	require.Equal(t, []int64{6}, quantities(t, s, cream))

	// решение по карантину: 2 принято, 1 отклонено
	resolved, err := s.Client.ResolveQuarantine(ctx, &oas.QuarantineResolution{
		AcceptedQuantity: 2, RejectedQuantity: 1,
		RejectReason: oas.NewOptWriteOffReason(oas.WriteOffReasonStorageViolation), Comment: oas.NewOptString("одна выше нормы"),
	}, oas.ResolveQuarantineParams{PointId: s.PointID, DeliveryId: registered.ID, ItemId: registered.Items[0].ID})
	require.NoError(t, err)
	require.Equal(t, []int64{8, 2, 0}, itemDistribution(resolved.Items[0]))
	require.Len(t, resolved.Items[0].BatchIds, 2)
	require.Equal(t, []int64{6, 2}, quantities(t, s, cream))

	// корректировка: отклонённая единица оказалась годной
	corrected, err := s.Client.CorrectDeliveryApproval(ctx, &oas.ApprovalCorrection{
		Reason: "ошибка пересчёта",
		Items:  []oas.ItemCorrection{{ItemID: registered.Items[0].ID, AcceptedQuantity: 9, RejectedQuantity: 1}},
	}, oas.CorrectDeliveryApprovalParams{PointId: s.PointID, DeliveryId: registered.ID})
	require.NoError(t, err)
	require.Equal(t, []int64{9, 1, 0}, itemDistribution(corrected.Items[0]))
	require.Len(t, corrected.Items[0].BatchIds, 3)
	require.Equal(t, []int64{6, 2, 1}, quantities(t, s, cream))

	read, err := s.Client.GetDelivery(ctx, oas.GetDeliveryParams{PointId: s.PointID, DeliveryId: registered.ID})
	require.NoError(t, err)
	require.Equal(t, corrected, read)
}

func TestDeliveryErrors(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	registered := registerDelivery(t, s, deliveryItem(cream, 10, oct(15)))
	itemID := registered.Items[0].ID
	unknown := uuid.Must(uuid.NewV7())
	deliveryPath := pointPath(s) + "/deliveries/" + registered.ID.String()

	t.Run("item without expires_at", func(t *testing.T) {
		body := `{"number":"ПН-1","delivered_at":"2026-10-10T09:00:00Z","items":[{"product_id":"` + cream.ID.String() + `","declared_quantity":5}]}`
		resp, data := raw(t, s, http.MethodPost, pointPath(s)+"/deliveries", body, nil)
		requireRawProblem(t, resp, data, http.StatusBadRequest)
	})
	t.Run("no items", func(t *testing.T) {
		resp, data := raw(t, s, http.MethodPost, pointPath(s)+"/deliveries", `{"number":"ПН-1","delivered_at":"2026-10-10T09:00:00Z","items":[]}`, nil)
		requireRawProblem(t, resp, data, http.StatusBadRequest)
	})
	t.Run("unknown product", func(t *testing.T) {
		_, err := s.Client.RegisterDelivery(ctx, &oas.NewDelivery{
			Number: "ПН-1", DeliveredAt: s.Clock.Now(),
			Items: []oas.NewDeliveryItem{{ProductID: unknown, DeclaredQuantity: 1, ExpiresAt: oct(15)}},
		}, oas.RegisterDeliveryParams{PointId: s.PointID})
		problem := requireProblem(t, err, http.StatusUnprocessableEntity)
		require.Equal(t, "unknown-reference", problem.Type)
	})
	t.Run("correction before approval", func(t *testing.T) {
		_, err := s.Client.CorrectDeliveryApproval(ctx, &oas.ApprovalCorrection{
			Reason: "r", Items: []oas.ItemCorrection{{ItemID: itemID, AcceptedQuantity: 10}},
		}, oas.CorrectDeliveryApprovalParams{PointId: s.PointID, DeliveryId: registered.ID})
		requireProblem(t, err, http.StatusConflict)
	})
	t.Run("quarantine resolution before approval", func(t *testing.T) {
		_, err := s.Client.ResolveQuarantine(ctx, &oas.QuarantineResolution{AcceptedQuantity: 1},
			oas.ResolveQuarantineParams{PointId: s.PointID, DeliveryId: registered.ID, ItemId: itemID})
		requireProblem(t, err, http.StatusConflict)
	})
	t.Run("sum does not match", func(t *testing.T) {
		_, err := s.Client.ApproveDelivery(ctx, &oas.DeliveryApproval{Items: []oas.ItemDecision{
			{ItemID: itemID, AcceptedQuantity: 6, RejectedQuantity: 3, RejectReason: oas.NewOptWriteOffReason(oas.WriteOffReasonOther), Comment: oas.NewOptString("c")},
		}}, oas.ApproveDeliveryParams{PointId: s.PointID, DeliveryId: registered.ID})
		problem := requireProblem(t, err, http.StatusUnprocessableEntity)
		require.Len(t, problem.Errors, 1)
		require.Contains(t, problem.Errors[0].Field, itemID.String(), "the error names the item")
	})
	t.Run("negative quantity is rejected by the contract", func(t *testing.T) {
		body := `{"items":[{"item_id":"` + itemID.String() + `","accepted_quantity":11,"rejected_quantity":-1,"quarantined_quantity":0}]}`
		resp, data := raw(t, s, http.MethodPost, deliveryPath+"/approval", body, nil)
		requireRawProblem(t, resp, data, http.StatusBadRequest)
	})
	t.Run("unknown delivery", func(t *testing.T) {
		_, err := s.Client.GetDelivery(ctx, oas.GetDeliveryParams{PointId: s.PointID, DeliveryId: unknown})
		requireProblem(t, err, http.StatusNotFound)
		_, err = s.Client.ApproveDelivery(ctx, acceptAllItems(registered), oas.ApproveDeliveryParams{PointId: s.PointID, DeliveryId: unknown})
		requireProblem(t, err, http.StatusNotFound)
	})

	// после приёмки: повторная приёмка и закрытый карантин
	_, err := s.Client.ApproveDelivery(ctx, acceptAllItems(registered), oas.ApproveDeliveryParams{PointId: s.PointID, DeliveryId: registered.ID})
	require.NoError(t, err)

	t.Run("second approval", func(t *testing.T) {
		_, err := s.Client.ApproveDelivery(ctx, acceptAllItems(registered), oas.ApproveDeliveryParams{PointId: s.PointID, DeliveryId: registered.ID})
		problem := requireProblem(t, err, http.StatusConflict)
		require.Equal(t, "delivery-already-approved", problem.Type)
	})
	t.Run("quarantine is closed", func(t *testing.T) {
		_, err := s.Client.ResolveQuarantine(ctx, &oas.QuarantineResolution{AcceptedQuantity: 1},
			oas.ResolveQuarantineParams{PointId: s.PointID, DeliveryId: registered.ID, ItemId: itemID})
		requireProblem(t, err, http.StatusConflict)
	})
	require.Equal(t, []int64{10}, quantities(t, s, cream))
}

func TestApproveDeliveryIdempotencyKey(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	registered := registerDelivery(t, s, deliveryItem(cream, 10, oct(15)))
	params := oas.ApproveDeliveryParams{PointId: s.PointID, DeliveryId: registered.ID, IdempotencyKey: oas.NewOptString("approval-1")}

	first, err := s.Client.ApproveDelivery(ctx, acceptAllItems(registered), params)
	require.NoError(t, err)
	second, err := s.Client.ApproveDelivery(ctx, acceptAllItems(registered), params)
	require.NoError(t, err)

	require.Equal(t, first, second)
	require.Equal(t, []int64{10}, quantities(t, s, cream))
}

// Склад.

func TestStockThroughAPI(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	late := supply(t, s, cream, 1000, oct(20))
	early := supply(t, s, cream, 100, oct(12))
	supply(t, s, cream, 100, oct(15))

	batches, err := s.Client.ListProductBatches(ctx, oas.ListProductBatchesParams{PointId: s.PointID, ProductId: cream.ID})
	require.NoError(t, err)
	require.Len(t, batches.Items, 3)
	require.Equal(t, []time.Time{oct(12), oct(15), oct(20)},
		[]time.Time{batches.Items[0].ExpiresAt, batches.Items[1].ExpiresAt, batches.Items[2].ExpiresAt}, "FEFO order")
	require.Equal(t, early, batches.Items[0].ID)
	require.Equal(t, int64(100), batches.Items[0].ReceivedQuantity)

	writtenOff, err := s.Client.WriteOffStock(ctx, &oas.WriteOff{
		Quantity: 300, Reason: oas.WriteOffReasonStorageViolation, Comment: "вне холодильника",
	}, oas.WriteOffStockParams{PointId: s.PointID, BatchId: late})
	require.NoError(t, err)
	require.Equal(t, int64(700), writtenOff.Quantity)
	require.Equal(t, int64(1000), writtenOff.ReceivedQuantity)

	adjusted, err := s.Client.AdjustStock(ctx, &oas.Adjustment{Delta: -50, Reason: "inventory_count"}, oas.AdjustStockParams{PointId: s.PointID, BatchId: late})
	require.NoError(t, err)
	require.Equal(t, int64(650), adjusted.Quantity)

	read, err := s.Client.GetBatch(ctx, oas.GetBatchParams{PointId: s.PointID, BatchId: late})
	require.NoError(t, err)
	require.Equal(t, adjusted, read)

	empty, err := s.Client.ListProductBatches(ctx, oas.ListProductBatchesParams{PointId: s.PointID, ProductId: create(t, s, "Соль").ID})
	require.NoError(t, err)
	require.Empty(t, empty.Items)
}

func TestStockErrors(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	batchID := supply(t, s, cream, 100, oct(15))
	unknown := uuid.Must(uuid.NewV7())
	batchPath := pointPath(s) + "/batches/" + batchID.String()

	t.Run("more than the batch has", func(t *testing.T) {
		_, err := s.Client.WriteOffStock(ctx, &oas.WriteOff{
			Quantity: 150, Reason: oas.WriteOffReasonOther, Comment: "c",
		}, oas.WriteOffStockParams{PointId: s.PointID, BatchId: batchID})
		problem := requireProblem(t, err, http.StatusUnprocessableEntity)
		require.Equal(t, "quantity", problem.Errors[0].Field)
	})
	t.Run("blank comment", func(t *testing.T) {
		_, err := s.Client.WriteOffStock(ctx, &oas.WriteOff{Quantity: 10, Reason: oas.WriteOffReasonOther, Comment: "  "},
			oas.WriteOffStockParams{PointId: s.PointID, BatchId: batchID})
		problem := requireProblem(t, err, http.StatusUnprocessableEntity)
		require.Equal(t, "comment", problem.Errors[0].Field)
	})
	t.Run("no comment field", func(t *testing.T) {
		resp, data := raw(t, s, http.MethodPost, batchPath+"/write-offs", `{"quantity":1,"reason":"other"}`, nil)
		requireRawProblem(t, resp, data, http.StatusBadRequest)
	})
	t.Run("reason expired for a good batch", func(t *testing.T) {
		_, err := s.Client.WriteOffStock(ctx, &oas.WriteOff{
			Quantity: 10, Reason: oas.WriteOffReasonExpired, Comment: "c",
		}, oas.WriteOffStockParams{PointId: s.PointID, BatchId: batchID})
		requireProblem(t, err, http.StatusUnprocessableEntity)
	})
	t.Run("unknown reason", func(t *testing.T) {
		resp, data := raw(t, s, http.MethodPost, batchPath+"/write-offs", `{"quantity":1,"reason":"lost","comment":"c"}`, nil)
		requireRawProblem(t, resp, data, http.StatusBadRequest)
	})
	t.Run("zero quantity", func(t *testing.T) {
		resp, data := raw(t, s, http.MethodPost, batchPath+"/write-offs", `{"quantity":0,"reason":"other","comment":"c"}`, nil)
		requireRawProblem(t, resp, data, http.StatusBadRequest)
	})
	t.Run("adjustment below zero", func(t *testing.T) {
		_, err := s.Client.AdjustStock(ctx, &oas.Adjustment{Delta: -150, Reason: "inventory_count"}, oas.AdjustStockParams{PointId: s.PointID, BatchId: batchID})
		problem := requireProblem(t, err, http.StatusUnprocessableEntity)
		require.Equal(t, "delta", problem.Errors[0].Field)
	})
	t.Run("zero adjustment", func(t *testing.T) {
		_, err := s.Client.AdjustStock(ctx, &oas.Adjustment{Reason: "inventory_count"}, oas.AdjustStockParams{PointId: s.PointID, BatchId: batchID})
		requireProblem(t, err, http.StatusUnprocessableEntity)
	})
	t.Run("unknown batch", func(t *testing.T) {
		_, err := s.Client.GetBatch(ctx, oas.GetBatchParams{PointId: s.PointID, BatchId: unknown})
		requireProblem(t, err, http.StatusNotFound)
		_, err = s.Client.WriteOffStock(ctx, &oas.WriteOff{Quantity: 1, Reason: oas.WriteOffReasonOther, Comment: "c"},
			oas.WriteOffStockParams{PointId: s.PointID, BatchId: unknown})
		requireProblem(t, err, http.StatusNotFound)
		_, err = s.Client.AdjustStock(ctx, &oas.Adjustment{Delta: 1, Reason: "r"}, oas.AdjustStockParams{PointId: s.PointID, BatchId: unknown})
		requireProblem(t, err, http.StatusNotFound)
	})

	require.Equal(t, []int64{100}, quantities(t, s, cream))
}

// Заказы.

func order(items ...oas.OrderItem) *oas.NewOrder { return &oas.NewOrder{Items: items} }

func portions(dish *oas.Dish, n int64) oas.OrderItem {
	return oas.OrderItem{DishID: dish.ID, Portions: n}
}

func TestOrdersThroughAPI(t *testing.T) {
	s := apitest.New(t)
	cream, bacon := create(t, s, "Сливки 33%"), create(t, s, "Бекон")
	pasta := createDish(t, s, "Паста карбонара")
	_, err := s.Client.ChangeRecipe(ctx, recipe(per(cream, 80), per(bacon, 60)), oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: pasta.ID})
	require.NoError(t, err)
	supply(t, s, cream, 1000, oct(20))
	supply(t, s, bacon, 100, oct(20))

	accepted, err := s.Client.PlaceOrder(ctx, order(portions(pasta, 1)), oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, oas.OrderStatusAccepted, accepted.Status)
	require.Empty(t, accepted.Shortages)
	require.Equal(t, []int64{920}, quantities(t, s, cream))
	require.Equal(t, []int64{40}, quantities(t, s, bacon))

	read, err := s.Client.GetOrder(ctx, oas.GetOrderParams{PointId: s.PointID, OrderId: accepted.ID})
	require.NoError(t, err)
	require.Equal(t, accepted, read)

	// бекона осталось 40, на порцию нужно 60: отказ - это результат, а не ошибка
	rejected, err := s.Client.PlaceOrder(ctx, order(portions(pasta, 1)), oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, oas.OrderStatusRejected, rejected.Status)
	require.Equal(t, []oas.Shortage{{ProductID: bacon.ID, Product: oas.NewOptProduct(*bacon), RequiredQuantity: 60, AvailableQuantity: 40}}, rejected.Shortages)
	require.Equal(t, []int64{920}, quantities(t, s, cream), "a rejected order does not change stock")
	require.Equal(t, []int64{40}, quantities(t, s, bacon))

	read, err = s.Client.GetOrder(ctx, oas.GetOrderParams{PointId: s.PointID, OrderId: rejected.ID})
	require.NoError(t, err)
	require.Equal(t, rejected, read)

	// статус ответа одинаков для принятого и отклонённого заказа
	body := `{"items":[{"dish_id":"` + pasta.ID.String() + `","portions":1}]}`
	resp, data := raw(t, s, http.MethodPost, pointPath(s)+"/orders", body, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode, string(data))
	require.Contains(t, string(data), `"status":"rejected"`)
}

func TestOrderErrors(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	pasta := createDish(t, s, "Паста")
	_, err := s.Client.ChangeRecipe(ctx, recipe(per(cream, 80)), oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: pasta.ID})
	require.NoError(t, err)
	draft := createDish(t, s, "Черновик")
	unknown := uuid.Must(uuid.NewV7())

	t.Run("unknown dish", func(t *testing.T) {
		_, err := s.Client.PlaceOrder(ctx, order(oas.OrderItem{DishID: unknown, Portions: 1}), oas.PlaceOrderParams{PointId: s.PointID})
		problem := requireProblem(t, err, http.StatusUnprocessableEntity)
		require.Equal(t, "unknown-reference", problem.Type)
		require.Contains(t, problem.Detail.Value, unknown.String())
	})
	t.Run("dish without recipe", func(t *testing.T) {
		_, err := s.Client.PlaceOrder(ctx, order(portions(draft, 1)), oas.PlaceOrderParams{PointId: s.PointID})
		requireProblem(t, err, http.StatusUnprocessableEntity)
	})
	t.Run("dish listed twice", func(t *testing.T) {
		_, err := s.Client.PlaceOrder(ctx, order(portions(pasta, 1), portions(pasta, 2)), oas.PlaceOrderParams{PointId: s.PointID})
		problem := requireProblem(t, err, http.StatusUnprocessableEntity)
		require.Equal(t, "validation-error", problem.Type)
	})
	for name, body := range map[string]string{
		"empty order":   `{"items":[]}`,
		"zero portions": `{"items":[{"dish_id":"` + pasta.ID.String() + `","portions":0}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			resp, data := raw(t, s, http.MethodPost, pointPath(s)+"/orders", body, nil)
			requireRawProblem(t, resp, data, http.StatusBadRequest)
		})
	}
	t.Run("unknown order", func(t *testing.T) {
		_, err := s.Client.GetOrder(ctx, oas.GetOrderParams{PointId: s.PointID, OrderId: unknown})
		requireProblem(t, err, http.StatusNotFound)
	})
}

func TestPlaceOrderIdempotencyKey(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	pasta := createDish(t, s, "Паста")
	_, err := s.Client.ChangeRecipe(ctx, recipe(per(cream, 80)), oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: pasta.ID})
	require.NoError(t, err)
	supply(t, s, cream, 1000, oct(20))
	params := oas.PlaceOrderParams{PointId: s.PointID, IdempotencyKey: oas.NewOptString("order-1")}

	first, err := s.Client.PlaceOrder(ctx, order(portions(pasta, 1)), params)
	require.NoError(t, err)
	second, err := s.Client.PlaceOrder(ctx, order(portions(pasta, 1)), params)
	require.NoError(t, err)

	require.Equal(t, first, second)
	require.Equal(t, []int64{920}, quantities(t, s, cream), "consumed once")
}

// Сквозной сценарий целиком через API: от каталога и поставки до заказов и стоп-листа.
// Он же показывает последовательность вызовов для генератора.
func TestScenarioFromCatalogToOrders(t *testing.T) {
	s := apitest.New(t)

	// каталог
	cream, bacon := create(t, s, "Сливки 33%"), create(t, s, "Бекон")
	pasta := createDish(t, s, "Паста карбонара")
	_, err := s.Client.ChangeRecipe(ctx, recipe(per(cream, 100), per(bacon, 50)), oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: pasta.ID})
	require.NoError(t, err)

	// поставка: сливки двумя позициями с разным сроком, бекон частично в карантин
	registered := registerDelivery(t, s,
		deliveryItem(cream, 300, oct(20)), deliveryItem(cream, 200, oct(12)), deliveryItem(bacon, 400, oct(18)))
	require.Empty(t, quantities(t, s, cream), "nothing is in stock before approval")

	approved, err := s.Client.ApproveDelivery(ctx, &oas.DeliveryApproval{Items: []oas.ItemDecision{
		{ItemID: registered.Items[0].ID, AcceptedQuantity: 300},
		{ItemID: registered.Items[1].ID, AcceptedQuantity: 200},
		{
			ItemID: registered.Items[2].ID, AcceptedQuantity: 100, QuarantinedQuantity: 300,
			QuarantineReason: oas.NewOptWriteOffReason(oas.WriteOffReasonStorageViolation), Comment: oas.NewOptString("проверить температуру"),
		},
	}}, oas.ApproveDeliveryParams{PointId: s.PointID, DeliveryId: registered.ID})
	require.NoError(t, err)
	require.Equal(t, []int64{200, 300}, quantities(t, s, cream), "FEFO: Oct 12 before Oct 20")
	require.Equal(t, []int64{100}, quantities(t, s, bacon), "quarantined quantity is not in stock")

	// заказ на 3 порции: сливок 300 из двух партий по FEFO, бекона 150 при доступных 100 - отказ
	rejected, err := s.Client.PlaceOrder(ctx, order(portions(pasta, 3)), oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, oas.OrderStatusRejected, rejected.Status)
	require.Equal(t, []oas.Shortage{{ProductID: bacon.ID, Product: oas.NewOptProduct(*bacon), RequiredQuantity: 150, AvailableQuantity: 100}}, rejected.Shortages)
	require.Equal(t, []int64{200, 300}, quantities(t, s, cream))

	// карантин бекона: 250 принято, 50 отклонено
	_, err = s.Client.ResolveQuarantine(ctx, &oas.QuarantineResolution{
		AcceptedQuantity: 250, RejectedQuantity: 50,
		RejectReason: oas.NewOptWriteOffReason(oas.WriteOffReasonStorageViolation), Comment: oas.NewOptString("часть выше нормы"),
	}, oas.ResolveQuarantineParams{PointId: s.PointID, DeliveryId: approved.ID, ItemId: approved.Items[2].ID})
	require.NoError(t, err)
	require.Equal(t, []int64{100, 250}, quantities(t, s, bacon))

	// тот же заказ теперь проходит: сливки списаны из партии с ближним сроком, затем из следующей
	accepted, err := s.Client.PlaceOrder(ctx, order(portions(pasta, 3)), oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, oas.OrderStatusAccepted, accepted.Status)
	require.Equal(t, []int64{0, 200}, quantities(t, s, cream), "200 from the Oct 12 batch, 100 from the Oct 20 batch")
	require.Equal(t, []int64{0, 200}, quantities(t, s, bacon))

	// списание испорченного и корректировка после пересчёта
	batches, err := s.Client.ListProductBatches(ctx, oas.ListProductBatchesParams{PointId: s.PointID, ProductId: cream.ID})
	require.NoError(t, err)
	remaining := batches.Items[1].ID
	_, err = s.Client.WriteOffStock(ctx, &oas.WriteOff{
		Quantity: 50, Reason: oas.WriteOffReasonPhysicalDamage, Comment: "разбили бутылку",
	}, oas.WriteOffStockParams{PointId: s.PointID, BatchId: remaining})
	require.NoError(t, err)
	_, err = s.Client.AdjustStock(ctx, &oas.Adjustment{Delta: 10, Reason: "inventory_count"}, oas.AdjustStockParams{PointId: s.PointID, BatchId: remaining})
	require.NoError(t, err)
	require.Equal(t, []int64{0, 160}, quantities(t, s, cream))

	// на одну порцию хватает, на две уже нет
	last, err := s.Client.PlaceOrder(ctx, order(portions(pasta, 2)), oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, oas.OrderStatusRejected, last.Status)
	require.Equal(t, []oas.Shortage{{ProductID: cream.ID, Product: oas.NewOptProduct(*cream), RequiredQuantity: 200, AvailableQuantity: 160}}, last.Shortages)
}

// Количество, которое не принято на склад или выбыло с него, заказом израсходовать нельзя.
func TestUnavailableQuantityCannotBeOrdered(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	dish := createDish(t, s, "Соус")
	_, err := s.Client.ChangeRecipe(ctx, recipe(per(cream, 1)), oas.ChangeRecipeParams{CompanyId: s.CompanyID, DishId: dish.ID})
	require.NoError(t, err)

	// rejectedWith оформляет заказ на need единиц и ждёт отказа с числом "доступно"
	rejectedWith := func(need, available int64) {
		t.Helper()
		before := quantities(t, s, cream)
		placed, err := s.Client.PlaceOrder(ctx, order(portions(dish, need)), oas.PlaceOrderParams{PointId: s.PointID})
		require.NoError(t, err)
		require.Equal(t, oas.OrderStatusRejected, placed.Status)
		require.Equal(t, []oas.Shortage{{ProductID: cream.ID, Product: oas.NewOptProduct(*cream), RequiredQuantity: need, AvailableQuantity: available}}, placed.Shortages)
		require.Equal(t, before, quantities(t, s, cream), "a rejected order changes nothing")
	}

	// из 100: принято 60, отклонено 10, в карантине 30
	registered := registerDelivery(t, s, deliveryItem(cream, 100, oct(15)))
	itemID := registered.Items[0].ID
	_, err = s.Client.ApproveDelivery(ctx, &oas.DeliveryApproval{Items: []oas.ItemDecision{{
		ItemID: itemID, AcceptedQuantity: 60, RejectedQuantity: 10, QuarantinedQuantity: 30,
		RejectReason: oas.NewOptWriteOffReason(oas.WriteOffReasonPhysicalDamage), QuarantineReason: oas.NewOptWriteOffReason(oas.WriteOffReasonStorageViolation),
		Comment: oas.NewOptString("c"),
	}}}, oas.ApproveDeliveryParams{PointId: s.PointID, DeliveryId: registered.ID})
	require.NoError(t, err)
	rejectedWith(70, 60)

	// карантин отклонён целиком
	_, err = s.Client.ResolveQuarantine(ctx, &oas.QuarantineResolution{
		RejectedQuantity: 30, RejectReason: oas.NewOptWriteOffReason(oas.WriteOffReasonStorageViolation), Comment: oas.NewOptString("c"),
	}, oas.ResolveQuarantineParams{PointId: s.PointID, DeliveryId: registered.ID, ItemId: itemID})
	require.NoError(t, err)
	rejectedWith(70, 60)

	// списано 20
	batches, err := s.Client.ListProductBatches(ctx, oas.ListProductBatchesParams{PointId: s.PointID, ProductId: cream.ID})
	require.NoError(t, err)
	_, err = s.Client.WriteOffStock(ctx, &oas.WriteOff{
		Quantity: 20, Reason: oas.WriteOffReasonPhysicalDamage, Comment: "c",
	}, oas.WriteOffStockParams{PointId: s.PointID, BatchId: batches.Items[0].ID})
	require.NoError(t, err)
	rejectedWith(50, 40)

	// ровно доступное количество заказать можно
	accepted, err := s.Client.PlaceOrder(ctx, order(portions(dish, 10)), oas.PlaceOrderParams{PointId: s.PointID})
	require.NoError(t, err)
	require.Equal(t, oas.OrderStatusAccepted, accepted.Status)

	// срок годности партии истёк: остаток 30 есть, но недоступен
	s.Clock.Set(oct(15))
	rejectedWith(1, 0)
	require.Equal(t, []int64{30}, quantities(t, s, cream))
}
