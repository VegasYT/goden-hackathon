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

func numbered(t *testing.T, s apitest.Server, number string, items ...oas.NewDeliveryItem) *oas.Delivery {
	t.Helper()
	delivery, err := s.Client.RegisterDelivery(ctx, &oas.NewDelivery{Number: number, DeliveredAt: s.Clock.Now(), Items: items},
		oas.RegisterDeliveryParams{PointId: s.PointID})
	require.NoError(t, err)
	return delivery
}

func quarantine(item oas.DeliveryItem, accepted, quarantined int64) oas.ItemDecision {
	return oas.ItemDecision{
		ItemID: item.ID, AcceptedQuantity: accepted, QuarantinedQuantity: quarantined,
		QuarantineReason: oas.NewOptWriteOffReason(oas.WriteOffReasonStorageViolation), Comment: oas.NewOptString("проверить температуру"),
	}
}

// Данные экранов приёмки: то, что позиция и история несут сверх количеств.
func TestDeliveryScreen(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	manager := s.Staff(t, "manager", oas.RoleManager)
	registered := registerDelivery(t, s, deliveryItem(cream, 100, oct(20)))
	at := oas.ApproveDeliveryParams{PointId: s.PointID, DeliveryId: registered.ID}
	itemAt := oas.ResolveQuarantineParams{PointId: s.PointID, DeliveryId: registered.ID, ItemId: registered.Items[0].ID}
	resolution := &oas.QuarantineResolution{
		AcceptedQuantity: 20, RejectedQuantity: 10,
		RejectReason: oas.NewOptWriteOffReason(oas.WriteOffReasonQualityIssue), Comment: oas.NewOptString("одна коробка подтекла"),
	}
	managerActor := oas.Actor{Type: oas.ActorTypeUser, ID: "manager", Name: oas.NewOptString("manager")}

	_, err := manager.ResolveQuarantine(ctx, resolution, itemAt)
	require.Equal(t, "delivery-not-approved", requireProblem(t, err, http.StatusConflict).Type)

	// управляющий принимает поставку наравне с приёмщиком
	approvedAt := s.Clock.Now()
	approved, err := manager.ApproveDelivery(ctx, &oas.DeliveryApproval{Items: []oas.ItemDecision{quarantine(registered.Items[0], 70, 30)}}, at)
	require.NoError(t, err)
	require.Equal(t, managerActor, approved.ApprovedBy.Value)
	item := approved.Items[0]
	require.Equal(t, cream.Category, item.ProductCategory.Value)
	require.Equal(t, approvedAt, item.QuarantinedAt.Value)
	require.Equal(t, oas.ItemDistribution{AcceptedQuantity: 70, QuarantinedQuantity: 30}, item.Approved.Value)
	require.EqualValues(t, 70, item.InStockQuantity)
	require.Empty(t, item.QuarantineResolutions)
	require.False(t, approved.CorrectedAt.Set)

	s.Clock.Advance(time.Hour)
	decidedAt := s.Clock.Now()
	resolved, err := manager.ResolveQuarantine(ctx, resolution, itemAt)
	require.NoError(t, err)
	item = resolved.Items[0]
	require.Equal(t, []oas.QuarantineDecision{{
		DecidedAt: decidedAt, DecidedBy: managerActor, AcceptedQuantity: 20, RejectedQuantity: 10,
		RejectReason: oas.NewOptString("quality_issue"), Comment: oas.NewOptString("одна коробка подтекла"),
	}}, item.QuarantineResolutions)
	require.False(t, item.QuarantinedAt.Set)
	require.EqualValues(t, 90, item.InStockQuantity)

	_, err = manager.ResolveQuarantine(ctx, resolution, itemAt)
	require.Equal(t, "quarantine-closed", requireProblem(t, err, http.StatusConflict).Type)

	history, err := s.Client.GetDeliveryHistory(ctx, oas.GetDeliveryHistoryParams{PointId: s.PointID, DeliveryId: registered.ID})
	require.NoError(t, err)
	require.Len(t, history.Items, 4)
	accepted, rejected := history.Items[2], history.Items[3]
	require.Equal(t, oas.DeliveryHistoryEntryTypeQuarantineAccepted, accepted.Type)
	require.Equal(t, "одна коробка подтекла", accepted.Comment.Value)
	require.False(t, accepted.RejectReason.Set)
	require.Equal(t, "quality_issue", rejected.RejectReason.Value)
	require.Equal(t, "одна коробка подтекла", rejected.Comment.Value)
	require.Equal(t, managerActor, rejected.Actor)

	// остаток по позиции - текущий: списание уменьшает его без событий в потоке поставки
	_, err = s.Client.WriteOffStock(ctx, &oas.WriteOff{Quantity: 5, Reason: oas.WriteOffReasonPhysicalDamage, Comment: "разбили"},
		oas.WriteOffStockParams{PointId: s.PointID, BatchId: item.BatchIds[0]})
	require.NoError(t, err)
	read, err := s.Client.GetDelivery(ctx, oas.GetDeliveryParams{PointId: s.PointID, DeliveryId: registered.ID})
	require.NoError(t, err)
	require.EqualValues(t, 85, read.Items[0].InStockQuantity)

	// уменьшить принятое можно не больше остатка: предел приходит числом
	correctAt := oas.CorrectDeliveryApprovalParams{PointId: s.PointID, DeliveryId: registered.ID}
	_, err = manager.CorrectDeliveryApproval(ctx, &oas.ApprovalCorrection{
		Reason: "пересчёт", Items: []oas.ItemCorrection{{ItemID: item.ID, RejectedQuantity: 100}},
	}, correctAt)
	problem := requireProblem(t, err, http.StatusUnprocessableEntity)
	require.Len(t, problem.Errors, 1)
	require.Equal(t, "stock-consumed", problem.Errors[0].Code.Value)
	require.EqualValues(t, 85, problem.Errors[0].Limit.Value)

	s.Clock.Advance(time.Hour)
	corrected, err := manager.CorrectDeliveryApproval(ctx, &oas.ApprovalCorrection{
		Reason: "пересчёт", Items: []oas.ItemCorrection{{ItemID: item.ID, AcceptedQuantity: 80, RejectedQuantity: 15, QuarantinedQuantity: 5}},
	}, correctAt)
	require.NoError(t, err)
	require.Equal(t, s.Clock.Now(), corrected.CorrectedAt.Value)
	require.Equal(t, s.Clock.Now(), corrected.Items[0].QuarantinedAt.Value, "the correction opened the quarantine")
	require.Equal(t, oas.ItemDistribution{AcceptedQuantity: 70, QuarantinedQuantity: 30}, corrected.Items[0].Approved.Value)

	// список отдаёт то же, что чтение по идентификатору
	s.Sync(t)
	page := listDeliveries(t, s, oas.ListDeliveriesParams{PointId: s.PointID})
	require.Equal(t, *corrected, page.Items[0])
}

func TestDeliveryFieldErrorCodes(t *testing.T) {
	s := apitest.New(t)
	cream := create(t, s, "Сливки 33%")
	registered := registerDelivery(t, s, deliveryItem(cream, 100, s.Clock.Now().Add(time.Hour)))
	at := oas.ApproveDeliveryParams{PointId: s.PointID, DeliveryId: registered.ID}

	// свободный текст причины больше не принимается
	body := `{"items":[{"item_id":"` + registered.Items[0].ID.String() +
		`","accepted_quantity":0,"rejected_quantity":100,"quarantined_quantity":0,"reject_reason":"damaged","comment":"c"}]}`
	resp, respBody := raw(t, s, http.MethodPost, pointPath(s)+"/deliveries/"+registered.ID.String()+"/approval", body, nil)
	requireRawProblem(t, resp, respBody, http.StatusBadRequest)

	// срок истёк, пока заполнялась приёмка
	s.Clock.Advance(2 * time.Hour)
	_, err := s.Client.ApproveDelivery(ctx, acceptAllItems(registered), at)
	problem := requireProblem(t, err, http.StatusUnprocessableEntity)
	require.Len(t, problem.Errors, 1)
	require.Equal(t, "expired", problem.Errors[0].Code.Value)
	require.False(t, problem.Errors[0].Limit.Set)
	require.Contains(t, problem.Errors[0].Field, registered.Items[0].ID.String())

	// у ошибки без различимого вида кода нет
	_, err = s.Client.ApproveDelivery(ctx, &oas.DeliveryApproval{Items: []oas.ItemDecision{{ItemID: registered.Items[0].ID, AcceptedQuantity: 1}}}, at)
	problem = requireProblem(t, err, http.StatusUnprocessableEntity)
	require.False(t, problem.Errors[0].Code.Set)
}

func TestDeliverySearchSortAndSummary(t *testing.T) {
	s := apitest.New(t)
	cream, milk := create(t, s, "Сливки 33%"), create(t, s, "Молоко")
	summary := func() *oas.DeliverySummary {
		t.Helper()
		s.Sync(t)
		out, err := s.Client.GetDeliveriesSummary(ctx, oas.GetDeliveriesSummaryParams{PointId: s.PointID})
		require.NoError(t, err)
		return out
	}
	require.Equal(t, &oas.DeliverySummary{}, summary(), "a point without deliveries")

	first := numbered(t, s, "ПН-100%", deliveryItem(cream, 100, oct(20)), deliveryItem(milk, 100, oct(20)))
	s.Clock.Advance(time.Hour)
	second := numbered(t, s, "пн-100a", deliveryItem(cream, 100, oct(20)))
	s.Clock.Advance(time.Hour)
	third := numbered(t, s, "INV-7", deliveryItem(cream, 100, oct(20)))

	// две позиции одной поставки в карантине: сводка считает позиции
	_, err := s.Client.ApproveDelivery(ctx, &oas.DeliveryApproval{Items: []oas.ItemDecision{
		quarantine(first.Items[0], 70, 30), quarantine(first.Items[1], 0, 100),
	}}, oas.ApproveDeliveryParams{PointId: s.PointID, DeliveryId: first.ID})
	require.NoError(t, err)
	require.Equal(t, &oas.DeliverySummary{Total: 3, Registered: 2, Approved: 1, OpenQuarantineItems: 2}, summary())

	find := func(number string) []uuid.UUID {
		t.Helper()
		return deliveryIDs(listDeliveries(t, s, oas.ListDeliveriesParams{PointId: s.PointID, Number: oas.NewOptString(number)}))
	}
	require.Equal(t, []uuid.UUID{second.ID, first.ID}, find("Пн-100"), "the beginning of the number, case-insensitively")
	require.Equal(t, []uuid.UUID{first.ID}, find("ПН-100%"), "a percent sign is a character, not a pattern")
	require.Empty(t, find("100"), "only the beginning matches")
	require.Equal(t, []uuid.UUID{third.ID}, find(" inv "))

	oldest := listDeliveries(t, s, oas.ListDeliveriesParams{
		PointId: s.PointID, Sort: oas.NewOptListDeliveriesSort(oas.ListDeliveriesSortDeliveredAt), Limit: oas.NewOptInt32(2),
	})
	require.Equal(t, []uuid.UUID{first.ID, second.ID}, deliveryIDs(oldest))
	require.EqualValues(t, 3, oldest.Total)
	latest := listDeliveries(t, s, oas.ListDeliveriesParams{PointId: s.PointID, Sort: oas.NewOptListDeliveriesSort(oas.ListDeliveriesSortMinusDeliveredAt)})
	require.Equal(t, []uuid.UUID{third.ID, second.ID, first.ID}, deliveryIDs(latest))

	// сводка чужой точки пуста, сотрудник без права её не читает
	other := newNetwork(t, s)
	foreign, err := other.owner.GetDeliveriesSummary(ctx, oas.GetDeliveriesSummaryParams{PointId: other.a.ID})
	require.NoError(t, err)
	require.Equal(t, &oas.DeliverySummary{}, foreign)
	_, err = s.Staff(t, "cook", oas.RoleCook).GetDeliveriesSummary(ctx, oas.GetDeliveriesSummaryParams{PointId: s.PointID})
	requireForbidden(t, err, "deliveries.view")
}
