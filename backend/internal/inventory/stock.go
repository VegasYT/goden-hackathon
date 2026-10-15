package inventory

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"goden/backend/internal/catalog"
	"goden/backend/internal/platform/eventstore"
)

// expiredAt сообщает, просрочена ли партия в момент now.
// Партия, срок которой истекает ровно в этот момент, уже просрочена.
func (b Batch) expiredAt(now time.Time) bool {
	return !b.ExpiresAt.After(now)
}

// WriteOff - ручное списание части или всего остатка партии.
type WriteOff struct {
	BatchID  uuid.UUID
	Quantity int64
	Reason   WriteOffReason
	Comment  string
}

// Adjustment - ручная корректировка остатка партии. Delta со знаком.
type Adjustment struct {
	BatchID uuid.UUID
	Delta   int64
	Reason  string
	Comment string
}

// Need - потребность в продукте в его единице измерения.
type Need struct {
	ProductID uuid.UUID
	Quantity  int64
}

// Shortage - нехватка продукта: сколько нужно и сколько доступно.
type Shortage struct {
	ProductID uuid.UUID
	Required  int64
	Available int64
}

// planConsumption набирает потребность из партий продукта по FEFO.
// batches должны идти в порядке FEFO: сначала ближайший срок годности.
//
// Возвращает доступный остаток и разбивку по партиям. Если доступного остатка
// не хватает, разбивка пуста: списание идёт целиком или не идёт вовсе.
func planConsumption(batches []Batch, need int64, now time.Time) (taken []BatchQuantity, available int64) {
	var usable []Batch
	for _, batch := range batches {
		if batch.Quantity > 0 && !batch.expiredAt(now) {
			usable = append(usable, batch)
			available += batch.Quantity
		}
	}
	if available < need {
		return nil, available
	}

	for _, batch := range usable {
		if need == 0 {
			break
		}
		amount := min(batch.Quantity, need)
		taken = append(taken, BatchQuantity{BatchID: batch.ID, Quantity: amount})
		need -= amount
	}
	return taken, available
}

// planWriteOff проверяет ручное списание и формирует событие.
func planWriteOff(b Batch, in WriteOff, actor eventstore.Actor, now time.Time) (StockWrittenOff, error) {
	comment := strings.TrimSpace(in.Comment)
	switch {
	case in.Quantity <= 0:
		return StockWrittenOff{}, invalid("quantity", "must be positive")
	case in.Quantity > b.Quantity:
		return StockWrittenOff{}, invalid("quantity", fmt.Sprintf("exceeds the batch quantity %d", b.Quantity))
	case !slices.Contains(writeOffReasons, in.Reason):
		return StockWrittenOff{}, invalid("reason", "unknown write-off reason")
	case comment == "":
		return StockWrittenOff{}, invalid("comment", "is required")
	// годная партия в потерях от просрочки исказила бы аналитику списаний
	case in.Reason == WriteOffExpired && !b.expiredAt(now):
		return StockWrittenOff{}, invalid("reason", "batch is not expired yet")
	}
	return StockWrittenOff{
		PointID: b.PointID, ProductID: b.ProductID, BatchID: b.ID, Quantity: in.Quantity,
		Reason: in.Reason, Comment: comment, WrittenOffBy: actor,
	}, nil
}

// planExpiredNotice отмечает просрочку партии с остатком. Возвращает false, если отмечать нечего:
// партия годна, пуста или уже отмечена.
func planExpiredNotice(b Batch, now time.Time) (BatchExpired, bool) {
	if b.ExpiredNoticedAt != nil || b.Quantity == 0 || !b.expiredAt(now) {
		return BatchExpired{}, false
	}
	return BatchExpired{
		PointID: b.PointID, ProductID: b.ProductID, BatchID: b.ID, RemainingQuantity: b.Quantity,
		ExpiresAt: eventstore.At(b.ExpiresAt),
	}, true
}

// planAdjustment проверяет корректировку остатка и формирует событие.
func planAdjustment(b Batch, in Adjustment, actor eventstore.Actor) (StockAdjusted, error) {
	reason := strings.TrimSpace(in.Reason)
	switch {
	case in.Delta == 0:
		return StockAdjusted{}, invalid("delta", "must not be zero")
	case reason == "":
		return StockAdjusted{}, invalid("reason", "is required")
	// граница проверяется до сложения: остаток партии не больше MaxQuantity, поэтому сумма не переполняется
	case in.Delta > catalog.MaxQuantity || in.Delta < -catalog.MaxQuantity:
		return StockAdjusted{}, invalid("delta", fmt.Sprintf("must be between -%d and %d", catalog.MaxQuantity, catalog.MaxQuantity))
	case b.Quantity+in.Delta < 0:
		return StockAdjusted{}, invalid("delta", fmt.Sprintf("exceeds the batch quantity %d", b.Quantity))
	case b.Quantity+in.Delta > catalog.MaxQuantity:
		return StockAdjusted{}, invalid("delta", fmt.Sprintf("batch quantity must not exceed %d", catalog.MaxQuantity))
	}
	return StockAdjusted{
		PointID: b.PointID, ProductID: b.ProductID, BatchID: b.ID, Delta: in.Delta,
		Reason: reason, Comment: strings.TrimSpace(in.Comment), AdjustedBy: actor,
	}, nil
}

// planExpiryWarning формирует предупреждение, если до конца срока годности осталось
// не больше period. Возвращает false, если предупреждать не о чем: партия пуста,
// уже просрочена, до срока далеко или предупреждение уже выдано.
func planExpiryWarning(b Batch, period time.Duration, now time.Time) (BatchExpiryWarningIssued, bool) {
	if b.ExpiryWarnedAt != nil || b.Quantity == 0 || b.expiredAt(now) || b.ExpiresAt.Sub(now) > period {
		return BatchExpiryWarningIssued{}, false
	}
	return BatchExpiryWarningIssued{
		PointID: b.PointID, ProductID: b.ProductID, BatchID: b.ID, RemainingQuantity: b.Quantity,
		ExpiresAt: eventstore.At(b.ExpiresAt), WarningPeriodSeconds: int64(period / time.Second),
	}, true
}
