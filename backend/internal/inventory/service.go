package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory/db"
	"goden/backend/internal/organization"
	"goden/backend/internal/platform/command"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/observability"
)

// Service выполняет команды склада и читает его данные.
type Service struct {
	runner   *command.Runner
	pool     *pgxpool.Pool
	products *catalog.Service

	writeOffs          *prometheus.CounterVec
	writtenOffQuantity *prometheus.CounterVec
}

func NewService(runner *command.Runner, pool *pgxpool.Pool, products *catalog.Service) *Service {
	registry := runner.Registry()
	s := &Service{
		runner: runner, pool: pool, products: products,
		writeOffs: observability.Register(registry, prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: observability.MetricsNamespace, Name: "stock_write_offs_total",
			Help: "Stock write-offs by reason.",
		}, []string{"reason"})),
		// количество считается в единице продукта, поэтому единица - метка: граммы со штуками не складываются
		writtenOffQuantity: observability.Register(registry, prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: observability.MetricsNamespace, Name: "stock_written_off_quantity_total",
			Help: "Quantity written off by reason, in the unit of the product.",
		}, []string{"reason", "unit"})),
	}
	// Серии причин создаются сразу с нулём: иначе Prometheus не увидит прироста
	// от первого списания по причине, для него серия появилась бы уже со значением 1.
	for _, reason := range writeOffReasons {
		s.writeOffs.WithLabelValues(string(reason))
	}
	return s
}

// countWriteOff учитывает списание в метриках после фиксации команды.
func (s *Service) countWriteOff(ctx context.Context, tx *command.Tx, e StockWrittenOff) {
	tx.OnCommit(func() {
		s.writeOffs.WithLabelValues(string(e.Reason)).Inc()
		product, err := s.products.Product(ctx, e.ProductID)
		if err != nil {
			return // метрика не стоит отказа уже выполненной команды
		}
		s.writtenOffQuantity.WithLabelValues(string(e.Reason), string(product.Unit)).Add(float64(e.Quantity))
	})
}

// Batch - партия продукта на складе.
type Batch struct {
	ID               uuid.UUID `json:"id"`
	PointID          uuid.UUID `json:"point_id"`
	ProductID        uuid.UUID `json:"product_id"`
	DeliveryID       uuid.UUID `json:"delivery_id"`
	ItemID           uuid.UUID `json:"item_id"`
	ReceivedQuantity int64     `json:"received_quantity"`
	// текущий остаток
	Quantity   int64     `json:"quantity"`
	ExpiresAt  time.Time `json:"expires_at"`
	ReceivedAt time.Time `json:"received_at"`
	// когда выдано предупреждение о приближении срока годности
	ExpiryWarnedAt *time.Time `json:"expiry_warned_at,omitempty"`
	// ExpiredNoticedAt - когда партия отмечена просроченной.
	ExpiredNoticedAt *time.Time `json:"expired_noticed_at,omitempty"`
}

// RegisterDelivery регистрирует поставку на точке. Остатки не меняются до приёмки.
// Позиции - только продукты компании точки. key - ключ идемпотентности, пустой отключает её.
func (s *Service) RegisterDelivery(
	ctx context.Context, actor eventstore.Actor, key string, point organization.Point, in NewDelivery,
) (Delivery, error) {
	in.PointID = point.ID
	products := make(map[uuid.UUID]catalog.Product, len(in.Items))
	for _, item := range in.Items {
		if _, loaded := products[item.ProductID]; loaded {
			continue
		}
		// архивный продукт в новую поставку не берётся: ArchivedProductError
		product, err := s.products.ActiveProductOf(ctx, point.CompanyID, item.ProductID)
		if errors.Is(err, catalog.ErrNotFound) {
			return Delivery{}, &catalog.UnknownProductError{ProductID: item.ProductID}
		}
		if err != nil {
			return Delivery{}, err
		}
		products[item.ProductID] = product
	}

	deliveryID := uuid.Must(uuid.NewV7())
	event, err := planRegistration(deliveryID, in, actor, products)
	if err != nil {
		return Delivery{}, err
	}
	delivery, err := foldDelivery([]eventstore.Payload{event})
	if err != nil {
		return Delivery{}, err
	}

	spec := command.Spec{Name: "RegisterDelivery", Actor: actor, IdempotencyKey: key, Request: in}
	return command.Run(ctx, s.runner, spec, func(ctx context.Context, tx *command.Tx) (Delivery, error) {
		if _, err := tx.Append(ctx, DeliveryStream(deliveryID), 0, event); err != nil {
			return Delivery{}, err
		}
		if err := applyDeliveryRegistered(ctx, db.New(tx), event); err != nil {
			return Delivery{}, err
		}
		return delivery, nil
	})
}

// DeliveredProducts отвечает, какие из продуктов уже были в поставках любого статуса: у них
// нельзя менять единицу. Реализует catalog.ProductUsage; партии бывают только у продуктов
// из поставок, поэтому отдельно их проверять не нужно.
func (s *Service) DeliveredProducts(ctx context.Context, q catalog.Querier, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	rows, err := db.New(q).DeliveredProducts(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("inventory: delivered products: %w", err)
	}
	delivered := make(map[uuid.UUID]bool, len(rows))
	for _, id := range rows {
		delivered[id] = true
	}
	return delivered, nil
}

// ApproveDelivery принимает поставку: записывает снимок приёмки и создаёт партии
// для принятого количества. Приёмка выполняется один раз.
func (s *Service) ApproveDelivery(
	ctx context.Context, actor eventstore.Actor, key string, deliveryID uuid.UUID, decisions []ItemDecision,
) (Delivery, error) {
	// Состав позиций после регистрации не меняется, поэтому набор складских потоков
	// можно узнать до транзакции: он не устареет к моменту взятия блокировок.
	registered, _, err := loadDelivery(ctx, s.pool, deliveryID)
	if err != nil {
		return Delivery{}, err
	}
	var locks []string
	for _, item := range registered.Items {
		locks = append(locks, StockStream(registered.PointID, item.ProductID))
	}

	spec := command.Spec{
		Name: "ApproveDelivery", Actor: actor, Locks: locks, IdempotencyKey: key,
		Request: struct {
			DeliveryID uuid.UUID
			Decisions  []ItemDecision
		}{deliveryID, decisions},
	}
	return command.Run(ctx, s.runner, spec, func(ctx context.Context, tx *command.Tx) (Delivery, error) {
		delivery, version, err := loadDelivery(ctx, tx, deliveryID)
		if err != nil {
			return Delivery{}, err
		}
		approved, batches, err := delivery.planApproval(decisions, actor, tx.Now())
		if err != nil {
			return Delivery{}, err
		}

		if err := receiveBatches(ctx, tx, batches); err != nil {
			return Delivery{}, err
		}
		// ожидаемая версия отсекает вторую одновременную приёмку
		if _, err := tx.Append(ctx, DeliveryStream(deliveryID), version, approved); err != nil {
			return Delivery{}, err
		}
		return delivery.apply(approved)
	})
}

// ConsumeForOrder списывает продукты точки под заказ по FEFO в транзакции вызывающей команды.
//
// Списание идёт целиком или не идёт вовсе: если хотя бы одного продукта не хватает,
// возвращается список всех нехваток, а остатки и события не меняются.
// Складские потоки всех продуктов должны быть объявлены в Spec.Locks команды.
func (s *Service) ConsumeForOrder(ctx context.Context, tx *command.Tx, pointID, orderID uuid.UUID, needs []Need) ([]Shortage, error) {
	productIDs := make([]uuid.UUID, len(needs))
	for i, need := range needs {
		if need.Quantity <= 0 {
			return nil, fmt.Errorf("inventory: need of product %s must be positive", need.ProductID)
		}
		if stream := StockStream(pointID, need.ProductID); !tx.Locked(stream) {
			return nil, fmt.Errorf("inventory: stock stream %s is consumed without a lock", stream)
		}
		productIDs[i] = need.ProductID
	}

	q := db.New(tx)
	rows, err := q.ListStockedBatches(ctx, db.ListStockedBatchesParams{PointID: pointID, ProductIds: productIDs})
	if err != nil {
		return nil, fmt.Errorf("inventory: read stock: %w", err)
	}
	byProduct := make(map[uuid.UUID][]Batch, len(needs))
	for _, row := range rows {
		byProduct[row.ProductID] = append(byProduct[row.ProductID], batchFromRow(row))
	}

	// решение по всем продуктам принимается до первой записи
	consumed := make([]StockConsumed, 0, len(needs))
	var shortages []Shortage
	for _, need := range needs {
		taken, available := planConsumption(byProduct[need.ProductID], need.Quantity, tx.Now())
		if taken == nil {
			shortages = append(shortages, Shortage{ProductID: need.ProductID, Required: need.Quantity, Available: available})
			continue
		}
		consumed = append(consumed, StockConsumed{PointID: pointID, ProductID: need.ProductID, OrderID: orderID, Batches: taken})
	}
	if len(shortages) > 0 {
		return shortages, nil
	}

	for _, event := range consumed {
		if err := appendToStock(ctx, tx, pointID, event.ProductID, event); err != nil {
			return nil, err
		}
		if err := applyStockConsumed(ctx, q, event); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// OrderConsumption возвращает расход заказа на точке по партиям: его события StockConsumed.
// События неизменны, поэтому результат можно читать до транзакции отмены и брать по нему блокировки.
func (s *Service) OrderConsumption(ctx context.Context, pointID, orderID uuid.UUID) ([]StockConsumed, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT schema_version, payload FROM events
		WHERE event_type = 'StockConsumed' AND payload->>'order_id' = $1 AND payload->>'point_id' = $2
		ORDER BY position`, orderID.String(), pointID.String())
	if err != nil {
		return nil, fmt.Errorf("inventory: read consumption of order %s: %w", orderID, err)
	}
	defer rows.Close()
	var consumed []StockConsumed
	for rows.Next() {
		var (
			version int
			raw     []byte
		)
		if err := rows.Scan(&version, &raw); err != nil {
			return nil, fmt.Errorf("inventory: read consumption of order %s: %w", orderID, err)
		}
		payload, err := eventstore.Decode("StockConsumed", version, raw)
		if err != nil {
			return nil, err
		}
		consumed = append(consumed, payload.(StockConsumed))
	}
	return consumed, rows.Err()
}

// ReturnForOrder возвращает в партии всё, что взяли под заказ, событиями StockReturned.
// Складские потоки продуктов должны быть заблокированы командой.
func (s *Service) ReturnForOrder(ctx context.Context, tx *command.Tx, actor eventstore.Actor, consumed []StockConsumed) error {
	q := db.New(tx)
	for _, taken := range consumed {
		event := StockReturned{
			PointID: taken.PointID, ProductID: taken.ProductID, OrderID: taken.OrderID,
			Batches: taken.Batches, ReturnedBy: actor,
		}
		if err := appendToStock(ctx, tx, taken.PointID, taken.ProductID, event); err != nil {
			return err
		}
		if err := applyStockReturned(ctx, q, event); err != nil {
			return err
		}
	}
	return nil
}

// WriteOffStock списывает часть или весь остаток партии: порча, повреждение, просрочка.
// key - ключ идемпотентности, пустой отключает её.
func (s *Service) WriteOffStock(ctx context.Context, actor eventstore.Actor, key string, in WriteOff) (Batch, error) {
	spec := command.Spec{Name: "WriteOffStock", Actor: actor, IdempotencyKey: key, Request: in}
	return runOnBatch(ctx, s, spec, in.BatchID, func(ctx context.Context, tx *command.Tx, q *db.Queries, b Batch) (Batch, error) {
		event, err := planWriteOff(b, in, actor, tx.Now())
		if err != nil {
			return Batch{}, err
		}
		if err := appendToStock(ctx, tx, b.PointID, b.ProductID, event); err != nil {
			return Batch{}, err
		}
		if err := applyStockWrittenOff(ctx, q, event); err != nil {
			return Batch{}, err
		}
		s.countWriteOff(ctx, tx, event)
		return readBatch(ctx, q, b.ID)
	})
}

// MarkBatchExpired отмечает просрочку партии с остатком событием BatchExpired, не меняя остаток.
// Предназначена для worker: отметка выдаётся один раз на партию, повторный вызов безопасен.
// Возвращает false, если отметки не было: партия годна, пуста или уже отмечена.
func (s *Service) MarkBatchExpired(ctx context.Context, actor eventstore.Actor, batchID uuid.UUID) (bool, error) {
	spec := command.Spec{Name: "MarkBatchExpired", Actor: actor}
	return runOnBatch(ctx, s, spec, batchID, func(ctx context.Context, tx *command.Tx, q *db.Queries, b Batch) (bool, error) {
		event, due := planExpiredNotice(b, tx.Now())
		if !due {
			return false, nil
		}
		if err := appendToStock(ctx, tx, b.PointID, b.ProductID, event); err != nil {
			return false, err
		}
		return true, applyBatchExpired(ctx, q, event, tx.Now())
	})
}

// AdjustStock исправляет остаток партии на величину со знаком, например после пересчёта склада.
// key - ключ идемпотентности, пустой отключает её.
func (s *Service) AdjustStock(ctx context.Context, actor eventstore.Actor, key string, in Adjustment) (Batch, error) {
	spec := command.Spec{Name: "AdjustStock", Actor: actor, IdempotencyKey: key, Request: in}
	return runOnBatch(ctx, s, spec, in.BatchID, func(ctx context.Context, tx *command.Tx, q *db.Queries, b Batch) (Batch, error) {
		event, err := planAdjustment(b, in, actor)
		if err != nil {
			return Batch{}, err
		}
		if err := appendToStock(ctx, tx, b.PointID, b.ProductID, event); err != nil {
			return Batch{}, err
		}
		if err := applyStockAdjusted(ctx, q, event); err != nil {
			return Batch{}, err
		}
		return readBatch(ctx, q, b.ID)
	})
}

// IssueBatchExpiryWarning выдаёт предупреждение, если до конца срока годности партии
// осталось не больше period. Предназначена для worker: предупреждение выдаётся один раз
// на партию, повторный вызов безопасен. Возвращает false, если предупреждение не выдано.
func (s *Service) IssueBatchExpiryWarning(
	ctx context.Context, actor eventstore.Actor, batchID uuid.UUID, period time.Duration,
) (bool, error) {
	if period <= 0 {
		return false, invalid("period", "must be positive")
	}
	spec := command.Spec{Name: "IssueBatchExpiryWarning", Actor: actor}
	return runOnBatch(ctx, s, spec, batchID, func(ctx context.Context, tx *command.Tx, q *db.Queries, b Batch) (bool, error) {
		event, due := planExpiryWarning(b, period, tx.Now())
		if !due {
			return false, nil
		}
		if err := appendToStock(ctx, tx, b.PointID, b.ProductID, event); err != nil {
			return false, err
		}
		return true, applyBatchExpiryWarningIssued(ctx, q, event, tx.Now())
	})
}

// Batch возвращает партию или ErrNotFound.
func (s *Service) Batch(ctx context.Context, id uuid.UUID) (Batch, error) {
	return readBatch(ctx, db.New(s.pool), id)
}

// BatchAt возвращает партию точки. Партия другой точки - ErrNotFound: по идентификатору
// нельзя прочитать или изменить чужой склад. Точка партии не меняется.
func (s *Service) BatchAt(ctx context.Context, pointID, id uuid.UUID) (Batch, error) {
	batch, err := s.Batch(ctx, id)
	if err == nil && batch.PointID != pointID {
		return Batch{}, fmt.Errorf("batch %s: %w", id, ErrNotFound)
	}
	return batch, err
}

// DeliveryNumbers возвращает номера поставок по их идентификаторам. Номер записан в первом
// событии потока поставки и не меняется; проекция списка асинхронна, поэтому читаются события.
func (s *Service) DeliveryNumbers(ctx context.Context, deliveryIDs []uuid.UUID) (map[uuid.UUID]string, error) {
	streams := make([]string, len(deliveryIDs))
	for i, id := range deliveryIDs {
		streams[i] = DeliveryStream(id)
	}
	events, err := eventstore.ReadFirst(ctx, s.pool, streams)
	if err != nil {
		return nil, err
	}
	numbers := make(map[uuid.UUID]string, len(events))
	for _, e := range events {
		payload, err := eventstore.Decode(e.Type, e.SchemaVersion, e.Payload)
		if err != nil {
			return nil, err
		}
		if registered, ok := payload.(DeliveryRegistered); ok {
			numbers[registered.DeliveryID] = registered.Number
		}
	}
	return numbers, nil
}

// BatchDeliveries возвращает поставки партий по идентификаторам партий, включая пустые партии.
func (s *Service) BatchDeliveries(ctx context.Context, batchIDs []uuid.UUID) (map[uuid.UUID]uuid.UUID, error) {
	if len(batchIDs) == 0 {
		return nil, nil
	}
	rows, err := db.New(s.pool).DeliveriesOfBatches(ctx, batchIDs)
	if err != nil {
		return nil, fmt.Errorf("inventory: deliveries of batches: %w", err)
	}
	deliveries := make(map[uuid.UUID]uuid.UUID, len(rows))
	for _, row := range rows {
		deliveries[row.ID] = row.DeliveryID
	}
	return deliveries, nil
}

// runOnBatch выполняет команду над одной партией под блокировкой складского потока её продукта.
// fn получает партию, перечитанную под блокировкой: решение принимается по актуальным остатку и сроку.
func runOnBatch[T any](
	ctx context.Context, s *Service, spec command.Spec, batchID uuid.UUID,
	fn func(ctx context.Context, tx *command.Tx, q *db.Queries, b Batch) (T, error),
) (T, error) {
	// продукт партии не меняется, поэтому поток для блокировки можно узнать до транзакции
	current, err := s.Batch(ctx, batchID)
	if err != nil {
		var zero T
		return zero, err
	}
	spec.Locks = []string{StockStream(current.PointID, current.ProductID)}

	return command.Run(ctx, s.runner, spec, func(ctx context.Context, tx *command.Tx) (T, error) {
		q := db.New(tx)
		batch, err := readBatch(ctx, q, batchID)
		if err != nil {
			var zero T
			return zero, err
		}
		return fn(ctx, tx, q, batch)
	})
}

func readBatch(ctx context.Context, q *db.Queries, id uuid.UUID) (Batch, error) {
	row, err := q.GetBatch(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Batch{}, fmt.Errorf("batch %s: %w", id, ErrNotFound)
	}
	if err != nil {
		return Batch{}, fmt.Errorf("inventory: read batch: %w", err)
	}
	return batchFromRow(row), nil
}

// ResolveQuarantine принимает решение по карантину позиции: всё карантинное количество
// делится на принятую и отклонённую части. После решения карантин позиции закрыт.
func (s *Service) ResolveQuarantine(
	ctx context.Context, actor eventstore.Actor, key string, deliveryID uuid.UUID, in QuarantineDecision,
) (Delivery, error) {
	spec := command.Spec{
		Name: "ResolveQuarantine", Actor: actor, IdempotencyKey: key,
		Request: struct {
			DeliveryID uuid.UUID
			Decision   QuarantineDecision
		}{deliveryID, in},
	}
	return s.changeApprovedDelivery(ctx, spec, deliveryID, func(ctx context.Context, tx *command.Tx, d Delivery) ([]eventstore.Payload, error) {
		events, batch, err := d.planQuarantineResolution(in, actor, tx.Now())
		if err != nil {
			return nil, err
		}
		if batch != nil {
			if err := receiveBatches(ctx, tx, []BatchReceived{*batch}); err != nil {
				return nil, err
			}
		}
		return events, nil
	})
}

// CorrectDeliveryApproval исправляет приёмку: перераспределяет заявленное количество позиций
// между принятым, отклонённым и карантинным. Исходная приёмка не меняется.
func (s *Service) CorrectDeliveryApproval(
	ctx context.Context, actor eventstore.Actor, key string, deliveryID uuid.UUID, in ApprovalCorrection,
) (Delivery, error) {
	spec := command.Spec{
		Name: "CorrectDeliveryApproval", Actor: actor, IdempotencyKey: key,
		Request: struct {
			DeliveryID uuid.UUID
			Correction ApprovalCorrection
		}{deliveryID, in},
	}
	return s.changeApprovedDelivery(ctx, spec, deliveryID, func(ctx context.Context, tx *command.Tx, d Delivery) ([]eventstore.Payload, error) {
		q := db.New(tx)
		// остатки партий читаются под блокировкой: по ним ограничивается уменьшение принятого
		batches := make(map[uuid.UUID][]Batch, len(in.Items))
		for _, item := range in.Items {
			rows, err := q.ListItemBatches(ctx, item.ItemID)
			if err != nil {
				return nil, fmt.Errorf("inventory: read item batches: %w", err)
			}
			for _, row := range rows {
				batches[item.ItemID] = append(batches[item.ItemID], batchFromRow(row))
			}
		}

		corrected, received, adjusted, err := d.planCorrection(in, batches, actor, tx.Now())
		if err != nil {
			return nil, err
		}
		if err := receiveBatches(ctx, tx, received); err != nil {
			return nil, err
		}
		for _, adjustment := range adjusted {
			if err := appendToStock(ctx, tx, adjustment.PointID, adjustment.ProductID, adjustment); err != nil {
				return nil, err
			}
			if err := applyStockAdjusted(ctx, q, adjustment); err != nil {
				return nil, err
			}
		}
		return []eventstore.Payload{corrected}, nil
	})
}

// changeApprovedDelivery выполняет команду над поставкой под блокировками складских потоков
// всех её продуктов. change меняет склад и возвращает события для потока поставки;
// они записываются с ожидаемой версией, что отсекает одновременное изменение той же поставки.
func (s *Service) changeApprovedDelivery(
	ctx context.Context, spec command.Spec, deliveryID uuid.UUID,
	change func(ctx context.Context, tx *command.Tx, d Delivery) ([]eventstore.Payload, error),
) (Delivery, error) {
	// состав позиций после регистрации не меняется: набор потоков не устареет
	registered, _, err := loadDelivery(ctx, s.pool, deliveryID)
	if err != nil {
		return Delivery{}, err
	}
	for _, item := range registered.Items {
		spec.Locks = append(spec.Locks, StockStream(registered.PointID, item.ProductID))
	}

	return command.Run(ctx, s.runner, spec, func(ctx context.Context, tx *command.Tx) (Delivery, error) {
		delivery, version, err := loadDelivery(ctx, tx, deliveryID)
		if err != nil {
			return Delivery{}, err
		}
		events, err := change(ctx, tx, delivery)
		if err != nil {
			return Delivery{}, err
		}
		if _, err := tx.Append(ctx, DeliveryStream(deliveryID), version, events...); err != nil {
			return Delivery{}, err
		}
		for _, event := range events {
			if delivery, err = delivery.apply(event); err != nil {
				return Delivery{}, err
			}
		}
		return delivery, nil
	})
}

// Delivery возвращает поставку или ErrNotFound.
func (s *Service) Delivery(ctx context.Context, id uuid.UUID) (Delivery, error) {
	delivery, _, err := loadDelivery(ctx, s.pool, id)
	return delivery, err
}

// DeliveryAt возвращает поставку точки. Поставка другой точки - ErrNotFound. Точка поставки не меняется.
func (s *Service) DeliveryAt(ctx context.Context, pointID, id uuid.UUID) (Delivery, error) {
	delivery, err := s.Delivery(ctx, id)
	if err == nil && delivery.PointID != pointID {
		return Delivery{}, fmt.Errorf("delivery %s: %w", id, ErrNotFound)
	}
	return delivery, err
}

// DeliveryFilter - условия списка поставок. Пустые значения не фильтруют.
type DeliveryFilter struct {
	Status DeliveryStatus
	// OpenQuarantine оставляет поставки с позицией в карантине, по которой нет решения.
	OpenQuarantine bool
	// NumberPrefix оставляет поставки, номер которых начинается с этого текста, без учёта регистра.
	NumberPrefix string
	// OldestFirst меняет порядок: от ранней даты поставки к поздней.
	OldestFirst bool
}

// DeliverySummary - числа для вкладок списка поставок точки.
type DeliverySummary struct {
	Total, Registered, Approved int64
	// OpenQuarantineItems - число позиций, а не поставок, с карантином без решения.
	OpenQuarantineItems int64
}

// DeliverySummary считает поставки точки по статусам. Читает ту же проекцию, что и список.
func (s *Service) DeliverySummary(ctx context.Context, pointID uuid.UUID) (DeliverySummary, error) {
	row, err := db.New(s.pool).SummarizeDeliveries(ctx, pointID)
	if err != nil {
		return DeliverySummary{}, fmt.Errorf("inventory: summarize deliveries: %w", err)
	}
	return DeliverySummary{
		Total: row.Total, Registered: row.Registered, Approved: row.Approved, OpenQuarantineItems: row.OpenQuarantineItems,
	}, nil
}

// ItemStock возвращает текущий остаток партий по позициям поставок: тот же, которым
// исправление приёмки ограничивает уменьшение принятого количества.
func (s *Service) ItemStock(ctx context.Context, itemIDs []uuid.UUID) (map[uuid.UUID]int64, error) {
	if len(itemIDs) == 0 {
		return nil, nil
	}
	rows, err := db.New(s.pool).StockOfItems(ctx, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("inventory: stock of items: %w", err)
	}
	stock := make(map[uuid.UUID]int64, len(rows))
	for _, row := range rows {
		stock[row.ItemID] = row.Quantity
	}
	return stock, nil
}

// Deliveries возвращает страницу поставок точки от поздней даты поставки к ранней и общее число поставок
// по фильтру. Список строится асинхронной проекцией и может отставать от записи.
func (s *Service) Deliveries(ctx context.Context, pointID uuid.UUID, filter DeliveryFilter, page catalog.Page) ([]Delivery, int64, error) {
	var status *string
	if filter.Status != "" {
		value := string(filter.Status)
		status = &value
	}
	prefix := strings.ToLower(strings.TrimSpace(filter.NumberPrefix))
	q := db.New(s.pool)
	rows, err := q.ListDeliveries(ctx, db.ListDeliveriesParams{
		PointID: pointID, Status: status, OpenQuarantineOnly: filter.OpenQuarantine,
		NumberPrefix: prefix, OldestFirst: filter.OldestFirst, PageLimit: page.Limit, PageOffset: page.Offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("inventory: list deliveries: %w", err)
	}
	total, err := q.CountDeliveries(ctx, db.CountDeliveriesParams{
		PointID: pointID, Status: status, OpenQuarantineOnly: filter.OpenQuarantine, NumberPrefix: prefix,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("inventory: count deliveries: %w", err)
	}
	deliveries := make([]Delivery, len(rows))
	for i, state := range rows {
		if err := json.Unmarshal(state, &deliveries[i]); err != nil {
			return nil, 0, fmt.Errorf("inventory: decode delivery: %w", err)
		}
	}
	return deliveries, total, nil
}

// Batches возвращает партии продукта на точке в порядке FEFO: сначала с ближайшим сроком годности.
func (s *Service) Batches(ctx context.Context, pointID, productID uuid.UUID) ([]Batch, error) {
	rows, err := db.New(s.pool).ListBatches(ctx, db.ListBatchesParams{PointID: pointID, ProductID: productID})
	if err != nil {
		return nil, fmt.Errorf("inventory: list batches: %w", err)
	}
	batches := make([]Batch, len(rows))
	for i, row := range rows {
		batches[i] = batchFromRow(row)
	}
	return batches, nil
}

// BatchPage возвращает страницу партий продукта на точке в порядке FEFO и их общее число.
// onlyStocked оставляет партии с остатком больше нуля. Продукт не из компании точки - ошибка.
func (s *Service) BatchPage(
	ctx context.Context, point organization.Point, productID uuid.UUID, onlyStocked bool, page catalog.Page,
) ([]Batch, int64, error) {
	// продукт без поставок - пустой список, неизвестный продукт - ошибка
	if _, err := s.products.ProductOf(ctx, point.CompanyID, productID); err != nil {
		return nil, 0, err
	}
	batches, err := s.Batches(ctx, point.ID, productID)
	if err != nil {
		return nil, 0, err
	}
	if onlyStocked {
		batches = slices.DeleteFunc(batches, func(b Batch) bool { return b.Quantity == 0 })
	}
	// упрощение: срез готового списка; при тысячах партий одного продукта перенести LIMIT в запрос
	from := min(int(page.Offset), len(batches))
	return batches[from:min(from+int(page.Limit), len(batches))], int64(len(batches)), nil
}

func batchFromRow(row db.InventoryBatch) Batch {
	b := Batch{
		ID: row.ID, PointID: row.PointID, ProductID: row.ProductID, DeliveryID: row.DeliveryID, ItemID: row.ItemID,
		ReceivedQuantity: row.ReceivedQuantity, Quantity: row.Quantity,
		ExpiresAt: row.ExpiresAt.UTC(), ReceivedAt: row.ReceivedAt.UTC(),
	}
	if row.ExpiryWarnedAt != nil {
		warnedAt := row.ExpiryWarnedAt.UTC()
		b.ExpiryWarnedAt = &warnedAt
	}
	if row.ExpiredNoticedAt != nil {
		noticedAt := row.ExpiredNoticedAt.UTC()
		b.ExpiredNoticedAt = &noticedAt
	}
	return b
}

// loadDelivery собирает состояние поставки из её потока и возвращает версию потока.
func loadDelivery(ctx context.Context, q eventstore.Querier, id uuid.UUID) (Delivery, int64, error) {
	stored, err := eventstore.ReadStream(ctx, q, DeliveryStream(id), 1)
	if err != nil {
		return Delivery{}, 0, err
	}
	if len(stored) == 0 {
		return Delivery{}, 0, fmt.Errorf("delivery %s: %w", id, ErrNotFound)
	}
	events := make([]eventstore.Payload, len(stored))
	for i, e := range stored {
		if events[i], err = eventstore.Decode(e.Type, e.SchemaVersion, e.Payload); err != nil {
			return Delivery{}, 0, err
		}
	}
	delivery, err := foldDelivery(events)
	return delivery, stored[len(stored)-1].Version, err
}

// receiveBatches записывает события партий в складские потоки продуктов и добавляет партии в таблицу остатков.
func receiveBatches(ctx context.Context, tx *command.Tx, batches []BatchReceived) error {
	// все партии одной команды принадлежат одной поставке, а значит одной точке
	byStream := make(map[string][]eventstore.Payload)
	var streams []string
	for _, batch := range batches {
		stream := StockStream(batch.PointID, batch.ProductID)
		if _, seen := byStream[stream]; !seen {
			streams = append(streams, stream)
		}
		byStream[stream] = append(byStream[stream], batch)
	}
	// порядок записи по потокам не влияет на корректность, сортировка делает его повторяемым
	slices.Sort(streams)

	q := db.New(tx)
	for _, stream := range streams {
		if !tx.Locked(stream) {
			return fmt.Errorf("inventory: stock stream %s is written without a lock", stream)
		}
		version, err := eventstore.StreamVersion(ctx, tx, stream)
		if err != nil {
			return err
		}
		if _, err := tx.Append(ctx, stream, version, byStream[stream]...); err != nil {
			return err
		}
	}
	for _, batch := range batches {
		if err := applyBatchReceived(ctx, q, batch); err != nil {
			return err
		}
	}
	return nil
}

// Таблицу остатков меняют только функции apply*: её содержимое - функция от событий
// складских потоков, поэтому перестроение сведётся к вызову этих же функций.

func applyDeliveryRegistered(ctx context.Context, q *db.Queries, e DeliveryRegistered) error {
	ids := make([]uuid.UUID, len(e.Items))
	for i, item := range e.Items {
		ids[i] = item.ProductID
	}
	if err := q.MarkProductsDelivered(ctx, ids); err != nil {
		return fmt.Errorf("inventory: mark delivered products: %w", err)
	}
	return nil
}

func applyBatchReceived(ctx context.Context, q *db.Queries, e BatchReceived) error {
	err := q.InsertBatch(ctx, db.InsertBatchParams{
		ID: e.BatchID, ProductID: e.ProductID, DeliveryID: e.DeliveryID, ItemID: e.ItemID,
		ReceivedQuantity: e.Quantity, ExpiresAt: e.ExpiresAt.Time, ReceivedAt: e.ReceivedAt.Time, PointID: e.PointID,
	})
	if err != nil {
		return fmt.Errorf("inventory: save batch %s: %w", e.BatchID, err)
	}
	return nil
}

func applyStockConsumed(ctx context.Context, q *db.Queries, e StockConsumed) error {
	for _, taken := range e.Batches {
		if err := changeQuantity(ctx, q, taken.BatchID, -taken.Quantity); err != nil {
			return err
		}
	}
	return nil
}

func applyStockReturned(ctx context.Context, q *db.Queries, e StockReturned) error {
	for _, returned := range e.Batches {
		if err := changeQuantity(ctx, q, returned.BatchID, returned.Quantity); err != nil {
			return err
		}
	}
	return nil
}

func applyStockWrittenOff(ctx context.Context, q *db.Queries, e StockWrittenOff) error {
	return changeQuantity(ctx, q, e.BatchID, -e.Quantity)
}

func applyStockAdjusted(ctx context.Context, q *db.Queries, e StockAdjusted) error {
	return changeQuantity(ctx, q, e.BatchID, e.Delta)
}

func applyBatchExpired(ctx context.Context, q *db.Queries, e BatchExpired, noticedAt time.Time) error {
	if err := q.MarkBatchExpiredNoticed(ctx, db.MarkBatchExpiredNoticedParams{ID: e.BatchID, NoticedAt: &noticedAt}); err != nil {
		return fmt.Errorf("inventory: mark batch %s as expired: %w", e.BatchID, err)
	}
	return nil
}

func applyBatchExpiryWarningIssued(ctx context.Context, q *db.Queries, e BatchExpiryWarningIssued, issuedAt time.Time) error {
	if err := q.MarkBatchWarned(ctx, db.MarkBatchWarnedParams{ID: e.BatchID, WarnedAt: &issuedAt}); err != nil {
		return fmt.Errorf("inventory: mark batch %s as warned: %w", e.BatchID, err)
	}
	return nil
}

// changeQuantity меняет остаток партии. CHECK (quantity >= 0) в таблице
// не даст уйти в минус при ошибке расчёта.
func changeQuantity(ctx context.Context, q *db.Queries, batchID uuid.UUID, delta int64) error {
	if err := q.ChangeBatchQuantity(ctx, db.ChangeBatchQuantityParams{ID: batchID, Delta: delta}); err != nil {
		return fmt.Errorf("inventory: change quantity of batch %s: %w", batchID, err)
	}
	return nil
}

// appendToStock добавляет события в складской поток продукта на точке. Поток должен быть под блокировкой команды.
func appendToStock(ctx context.Context, tx *command.Tx, pointID, productID uuid.UUID, events ...eventstore.Payload) error {
	stream := StockStream(pointID, productID)
	if !tx.Locked(stream) {
		return fmt.Errorf("inventory: stock stream %s is written without a lock", stream)
	}
	version, err := eventstore.StreamVersion(ctx, tx, stream)
	if err != nil {
		return err
	}
	_, err = tx.Append(ctx, stream, version, events...)
	return err
}
