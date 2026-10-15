package inventory

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory/db"
	"goden/backend/internal/organization"
	"goden/backend/internal/platform/command"
	"goden/backend/internal/platform/eventstore"
)

// Чтения, которым не нужны проекции: остатки берутся из таблицы партий и потому не отстают
// от записи, истории - прямо из потоков событий.

// StockLevel - остаток продукта на складе на момент запроса.
type StockLevel struct {
	Product catalog.Product
	// Available - сумма остатков партий с неистёкшим сроком годности.
	Available int64
	// Expired - сумма остатков партий, срок годности которых истёк.
	Expired int64
	// Batches - число партий с ненулевым остатком.
	Batches int64
	// NearestExpiresAt - ближайший срок среди непросроченных партий с остатком.
	NearestExpiresAt *time.Time
}

// Stock возвращает страницу остатков точки: по строке на продукт каталога её компании в порядке названий,
// включая продукты без партий, и общее число продуктов.
//
// Продукты и их порядок даёт каталог: таблицы каталога модуль inventory не читает. Просрочка
// определяется тем же правилом и по тем же часам, что в командах.
func (s *Service) Stock(ctx context.Context, point organization.Point, page catalog.Page) ([]StockLevel, int64, error) {
	products, total, err := s.products.Products(ctx, point.CompanyID, page)
	if err != nil {
		return nil, 0, err
	}
	levels, err := s.levels(ctx, point.ID, products, s.runner.Now())
	return levels, total, err
}

// levels возвращает остатки продуктов на точке на момент now, в порядке products.
func (s *Service) levels(ctx context.Context, pointID uuid.UUID, products []catalog.Product, now time.Time) ([]StockLevel, error) {
	ids := make([]uuid.UUID, len(products))
	for i, product := range products {
		ids[i] = product.ID
	}
	rows, err := db.New(s.pool).StockLevels(ctx, db.StockLevelsParams{PointID: pointID, Now: now, ProductIds: ids})
	if err != nil {
		return nil, fmt.Errorf("inventory: stock levels: %w", err)
	}
	byProduct := make(map[uuid.UUID]db.StockLevelsRow, len(rows))
	for _, row := range rows {
		byProduct[row.ProductID] = row
	}

	levels := make([]StockLevel, len(products))
	for i, product := range products {
		row := byProduct[product.ID] // у продукта без партий строки нет: количества нулевые
		levels[i] = StockLevel{Product: product, Available: row.AvailableQuantity, Expired: row.ExpiredQuantity, Batches: row.Batches}
		if nearest, ok := row.NearestExpiresAt.(time.Time); ok {
			nearest = nearest.UTC()
			levels[i].NearestExpiresAt = &nearest
		}
	}
	return levels, nil
}

// StockView возвращает страницу остатков точки под условия query и число подходящих продуктов.
//
// упрощение: читает все продукты компании под условия названия и категории, фильтрует и сортирует
// в памяти - статус и количество зависят от партий, а таблицы каталога модуль не читает.
// На каталоге в сотни продуктов это дёшево; при росте нужна проекция остатков по продуктам.
func (s *Service) StockView(ctx context.Context, point organization.Point, query StockQuery, page catalog.Page) ([]StockLevel, int64, error) {
	levels, now, err := s.matchingLevels(ctx, point, query)
	if err != nil {
		return nil, 0, err
	}
	if query.Status != "" {
		levels = slices.DeleteFunc(levels, func(l StockLevel) bool { return !l.Is(query.Status, now) })
	}
	sortStock(levels, query.Sort, query.Descending)
	total := int64(len(levels))
	from := min(int(page.Offset), len(levels))
	return levels[from:min(from+int(page.Limit), len(levels))], total, nil
}

// StockSummary считает продукты точки по статусам с учётом названия и категории из query.
// Статус из query не учитывается.
func (s *Service) StockSummary(ctx context.Context, point organization.Point, query StockQuery) (StockSummary, error) {
	levels, now, err := s.matchingLevels(ctx, point, query)
	if err != nil {
		return StockSummary{}, err
	}
	summary := StockSummary{Total: int64(len(levels)), ByStatus: make(map[StockStatus]int64, len(StockStatuses))}
	for _, status := range StockStatuses {
		summary.ByStatus[status] = 0
		for _, level := range levels {
			if level.Is(status, now) {
				summary.ByStatus[status]++
			}
		}
	}
	return summary, nil
}

// matchingLevels возвращает остатки всех продуктов под название и категорию и момент, на который они посчитаны.
func (s *Service) matchingLevels(ctx context.Context, point organization.Point, query StockQuery) ([]StockLevel, time.Time, error) {
	products, err := s.products.ProductsMatching(ctx, point.CompanyID, query.NamePrefix, query.Category)
	if err != nil {
		return nil, time.Time{}, err
	}
	now := s.runner.Now()
	levels, err := s.levels(ctx, point.ID, products, now)
	// архивный продукт виден на складе, только пока у него есть партии
	levels = slices.DeleteFunc(levels, func(l StockLevel) bool { return l.Product.Archived && l.Batches == 0 })
	return levels, now, err
}

// notReturned отсекает расход, который вернули при отмене заказа.
const notReturned = `
		AND NOT EXISTS (
			SELECT 1 FROM events r
			WHERE r.event_type = 'StockReturned' AND r.payload->>'order_id' = events.payload->>'order_id'
			  AND r.payload->>'product_id' = events.payload->>'product_id')`

// Consumption возвращает расход продуктов точки под заказы начиная с момента since: сумму по событиям
// StockConsumed. Списания и корректировки в расход не входят: это не спрос. Расход отменённого заказа
// (продукт вернули событием StockReturned) тоже не входит.
//
// упрощение: запрос просматривает все события периода. На учебном объёме это дёшево; при росте
// нужна проекция расхода по часам, она же пригодится аналитике.
func (s *Service) Consumption(ctx context.Context, pointID uuid.UUID, since time.Time) (map[uuid.UUID]int64, time.Time, error) {
	// время самого раннего расхода нужно, чтобы делить расход на время, за которое есть данные
	var first *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT min(occurred_at) FROM events
		WHERE event_type = 'StockConsumed' AND occurred_at >= $1 AND payload->>'point_id' = $2`+notReturned,
		since, pointID.String()).Scan(&first)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("inventory: first consumption: %w", err)
	}
	if first == nil {
		return map[uuid.UUID]int64{}, time.Time{}, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT (payload->>'product_id')::uuid, sum((batch->>'quantity')::bigint)::bigint
		FROM events, jsonb_array_elements(payload->'batches') AS batch
		WHERE event_type = 'StockConsumed' AND occurred_at >= $1 AND payload->>'point_id' = $2`+notReturned+`
		GROUP BY 1`, since, pointID.String())
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("inventory: consumption: %w", err)
	}
	defer rows.Close()
	consumed := map[uuid.UUID]int64{}
	for rows.Next() {
		var (
			productID uuid.UUID
			quantity  int64
		)
		if err := rows.Scan(&productID, &quantity); err != nil {
			return nil, time.Time{}, fmt.Errorf("inventory: consumption: %w", err)
		}
		consumed[productID] = quantity
	}
	return consumed, first.UTC(), rows.Err()
}

// HistoryEntryType - вид события в истории поставки.
type HistoryEntryType string

const (
	HistoryRegistered         HistoryEntryType = "registered"
	HistoryApproved           HistoryEntryType = "approved"
	HistoryQuarantineAccepted HistoryEntryType = "quarantine_accepted"
	HistoryQuarantineRejected HistoryEntryType = "quarantine_rejected"
	HistoryCorrected          HistoryEntryType = "corrected"
)

// HistoryEntry - запись истории поставки: событие и состояние поставки сразу после него.
type HistoryEntry struct {
	Type       HistoryEntryType
	OccurredAt time.Time
	Actor      eventstore.Actor
	// Reason - причина корректировки приёмки.
	Reason string
	// RejectReason и Comment - причина отклонения и комментарий решения по карантину.
	RejectReason string
	Comment      string
	Delivery     Delivery
}

// DeliveryHistory возвращает историю поставки: по записи на каждое событие её потока в порядке
// появления. Состояние в записи - та же свёртка, что при чтении поставки, остановленная на этом
// событии: исходная приёмка остаётся видна и после корректировок.
func (s *Service) DeliveryHistory(ctx context.Context, id uuid.UUID) ([]HistoryEntry, error) {
	stored, err := eventstore.ReadStream(ctx, s.pool, DeliveryStream(id), 1)
	if err != nil {
		return nil, err
	}
	if len(stored) == 0 {
		return nil, fmt.Errorf("delivery %s: %w", id, ErrNotFound)
	}

	payloads := make([]eventstore.Payload, len(stored))
	entries := make([]HistoryEntry, len(stored))
	for i, e := range stored {
		if payloads[i], err = eventstore.Decode(e.Type, e.SchemaVersion, e.Payload); err != nil {
			return nil, err
		}
		entry := HistoryEntry{OccurredAt: e.OccurredAt.UTC()}
		switch p := payloads[i].(type) {
		case DeliveryRegistered:
			entry.Type = HistoryRegistered
		case DeliveryApproved:
			entry.Type = HistoryApproved
		case QuarantinedDeliveryItemAccepted:
			entry.Type, entry.Comment = HistoryQuarantineAccepted, p.Comment
		case QuarantinedDeliveryItemRejected:
			entry.Type, entry.RejectReason, entry.Comment = HistoryQuarantineRejected, p.Reason, p.Comment
		case DeliveryApprovalCorrected:
			entry.Type, entry.Reason = HistoryCorrected, p.Reason
		default:
			return nil, fmt.Errorf("inventory: delivery history does not know event %s", e.Type)
		}
		if entry.Actor, err = command.ActorOf(e); err != nil {
			return nil, err
		}
		if entry.Delivery, err = foldDelivery(payloads[:i+1]); err != nil {
			return nil, err
		}
		entries[i] = entry
	}
	return entries, nil
}

// MovementType - вид движения по продукту.
type MovementType string

const (
	MovementReceived      MovementType = "received"
	MovementConsumed      MovementType = "consumed"
	MovementWrittenOff    MovementType = "written_off"
	MovementAdjusted      MovementType = "adjusted"
	MovementExpiryWarning MovementType = "expiry_warning"
	MovementExpired       MovementType = "expired"
	// MovementReturned - продукты вернулись в партии при отмене заказа
	MovementReturned MovementType = "returned"
)

// BatchChange - изменение остатка партии со знаком.
type BatchChange struct {
	BatchID uuid.UUID
	Delta   int64
}

// Movement - движение по продукту: одно событие его складского потока.
type Movement struct {
	ID         uuid.UUID
	Type       MovementType
	OccurredAt time.Time
	Actor      eventstore.Actor
	// Changes - изменения остатка по партиям; у расхода по заказу их может быть несколько.
	Changes    []BatchChange
	OrderID    *uuid.UUID
	DeliveryID *uuid.UUID
	Reason     string
	Comment    string
	// ExpiresAt - срок годности партии, у получения и предупреждения о сроке.
	ExpiresAt *time.Time
}

// Movements возвращает страницу движений по продукту на точке от последних к ранним и их общее число.
// Размер страницы считается в движениях, то есть в событиях складского потока.
func (s *Service) Movements(ctx context.Context, point organization.Point, productID uuid.UUID, page catalog.Page) ([]Movement, int64, error) {
	// продукт без поставок - пустой список, продукт не из компании точки - ошибка
	if _, err := s.products.ProductOf(ctx, point.CompanyID, productID); err != nil {
		return nil, 0, err
	}
	stream := StockStream(point.ID, productID)
	total, err := eventstore.StreamVersion(ctx, s.pool, stream)
	if err != nil {
		return nil, 0, err
	}
	stored, err := eventstore.ReadStreamBackward(ctx, s.pool, stream, total-int64(page.Offset)+1, int(page.Limit))
	if err != nil {
		return nil, 0, err
	}
	movements := make([]Movement, len(stored))
	for i, e := range stored {
		if movements[i], err = movementFromEvent(e); err != nil {
			return nil, 0, err
		}
	}
	return movements, total, nil
}

// movementFromEvent переводит событие складского потока в движение. Неизвестное событие - ошибка:
// новое складское событие должно получить вид движения.
func movementFromEvent(e eventstore.Event) (Movement, error) {
	payload, err := eventstore.Decode(e.Type, e.SchemaVersion, e.Payload)
	if err != nil {
		return Movement{}, err
	}
	m := Movement{ID: e.ID, OccurredAt: e.OccurredAt.UTC()}
	if m.Actor, err = command.ActorOf(e); err != nil {
		return Movement{}, err
	}
	switch p := payload.(type) {
	case BatchReceived:
		expiresAt := p.ExpiresAt.Time
		m.Type, m.DeliveryID, m.ExpiresAt = MovementReceived, &p.DeliveryID, &expiresAt
		m.Changes = []BatchChange{{BatchID: p.BatchID, Delta: p.Quantity}}
	case StockConsumed:
		m.Type, m.OrderID = MovementConsumed, &p.OrderID
		for _, taken := range p.Batches {
			m.Changes = append(m.Changes, BatchChange{BatchID: taken.BatchID, Delta: -taken.Quantity})
		}
	case StockReturned:
		m.Type, m.OrderID = MovementReturned, &p.OrderID
		for _, returned := range p.Batches {
			m.Changes = append(m.Changes, BatchChange{BatchID: returned.BatchID, Delta: returned.Quantity})
		}
	case StockWrittenOff:
		m.Type, m.Reason, m.Comment = MovementWrittenOff, string(p.Reason), p.Comment
		m.Changes = []BatchChange{{BatchID: p.BatchID, Delta: -p.Quantity}}
	case StockAdjusted:
		m.Type, m.Reason, m.Comment = MovementAdjusted, p.Reason, p.Comment
		m.Changes = []BatchChange{{BatchID: p.BatchID, Delta: p.Delta}}
	case BatchExpiryWarningIssued:
		expiresAt := p.ExpiresAt.Time
		// остаток не меняется; партия указана нулевым изменением
		m.Type, m.ExpiresAt = MovementExpiryWarning, &expiresAt
		m.Changes = []BatchChange{{BatchID: p.BatchID}}
	case BatchExpired:
		expiresAt := p.ExpiresAt.Time
		m.Type, m.ExpiresAt = MovementExpired, &expiresAt
		m.Changes = []BatchChange{{BatchID: p.BatchID}}
	default:
		return Movement{}, fmt.Errorf("inventory: movements do not know event %s", e.Type)
	}
	return m, nil
}
