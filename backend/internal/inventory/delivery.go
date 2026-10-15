package inventory

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"goden/backend/internal/catalog"
	"goden/backend/internal/platform/eventstore"
)

var (
	// ErrNotFound - поставки с таким идентификатором нет.
	ErrNotFound = errors.New("inventory: not found")
	// ErrAlreadyApproved - поставка уже принята, приёмка выполняется один раз.
	ErrAlreadyApproved = errors.New("inventory: delivery is already approved")
	// ErrNotApproved - операция доступна только для принятой поставки.
	ErrNotApproved = errors.New("inventory: delivery is not approved yet")
	// ErrQuarantineClosed - у позиции нет карантина, по которому можно принять решение.
	ErrQuarantineClosed = errors.New("inventory: item has no open quarantine")
)

// ValidationError - некорректное поле входных данных команды.
type ValidationError struct {
	Field   string
	Message string
	// Code - вид ошибки, если клиенту нужно отличать его без разбора текста.
	Code string
	// Limit - предел для CodeStockConsumed: на сколько можно уменьшить принятое количество.
	Limit *int64
}

const (
	// CodeExpired - срок годности позиции истёк к моменту действия.
	CodeExpired = "expired"
	// CodeStockConsumed - принятое количество нельзя уменьшить ниже уже израсходованного.
	CodeStockConsumed = "stock-consumed"
)

// FieldDetail возвращает вид ошибки и предел для слоя API; пустой вид - различать нечего.
func (e *ValidationError) FieldDetail() (code string, limit *int64) {
	return e.Code, e.Limit
}

func expiredField(field, message string) error {
	return &ValidationError{Field: field, Message: message, Code: CodeExpired}
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("inventory: %s: %s", e.Field, e.Message)
}

// InvalidField возвращает некорректное поле и сообщение: по этому методу
// слой API распознаёт ошибку валидации, не зная о типах модулей.
func (e *ValidationError) InvalidField() (field, message string) {
	return e.Field, e.Message
}

func invalid(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}

type DeliveryStatus string

const (
	DeliveryRegisteredStatus DeliveryStatus = "registered"
	DeliveryApprovedStatus   DeliveryStatus = "approved"
)

// Delivery - состояние поставки, собранное из событий её потока.
type Delivery struct {
	ID          uuid.UUID         `json:"id"`
	PointID     uuid.UUID         `json:"point_id"`
	Number      string            `json:"number"`
	DeliveredAt time.Time         `json:"delivered_at"`
	Status      DeliveryStatus    `json:"status"`
	ApprovedAt  *time.Time        `json:"approved_at,omitempty"`
	ApprovedBy  *eventstore.Actor `json:"approved_by,omitempty"`
	// время последнего исправления приёмки
	CorrectedAt *time.Time          `json:"corrected_at,omitempty"`
	Items       []DeliveryItemState `json:"items"`
}

// Distribution - распределение заявленного количества позиции.
type Distribution struct {
	AcceptedQuantity    int64 `json:"accepted_quantity"`
	RejectedQuantity    int64 `json:"rejected_quantity"`
	QuarantinedQuantity int64 `json:"quarantined_quantity"`
}

// QuarantineResolved - принятое решение по карантину позиции: одна команда ResolveQuarantine.
type QuarantineResolved struct {
	DecidedAt        time.Time        `json:"decided_at"`
	DecidedBy        eventstore.Actor `json:"decided_by"`
	AcceptedQuantity int64            `json:"accepted_quantity"`
	RejectedQuantity int64            `json:"rejected_quantity"`
	RejectReason     string           `json:"reject_reason,omitempty"`
	Comment          string           `json:"comment,omitempty"`
}

// DeliveryItemState - позиция поставки. После приёмки всегда выполняется
// заявлено = принято + отклонено + карантин.
type DeliveryItemState struct {
	ID               uuid.UUID  `json:"id"`
	ProductID        uuid.UUID  `json:"product_id"`
	DeclaredQuantity int64      `json:"declared_quantity"`
	ExpiresAt        time.Time  `json:"expires_at"`
	ManufacturedAt   *time.Time `json:"manufactured_at,omitempty"`

	AcceptedQuantity int64 `json:"accepted_quantity"`
	RejectedQuantity int64 `json:"rejected_quantity"`
	// карантин, по которому ещё нет решения
	QuarantinedQuantity int64 `json:"quarantined_quantity"`
	// просрочена ли позиция на момент приёмки
	Expired bool `json:"expired"`
	// партии позиции в порядке создания: из приёмки, из карантина, из корректировок
	BatchIDs []uuid.UUID `json:"batch_ids,omitempty"`
	// причины и комментарий из исходной приёмки
	RejectReason     string `json:"reject_reason,omitempty"`
	QuarantineReason string `json:"quarantine_reason,omitempty"`
	Comment          string `json:"comment,omitempty"`
	// когда открыт текущий карантин: приёмкой или исправлением, которое его назначило
	QuarantinedAt *time.Time `json:"quarantined_at,omitempty"`
	// решения по карантину в порядке принятия
	Resolutions []QuarantineResolved `json:"quarantine_resolutions,omitempty"`
	// распределение на момент приёмки: исправления его не меняют
	Approved *Distribution `json:"approved,omitempty"`
}

// resolved добавляет решение по карантину. Принятая и отклонённая части одного решения -
// два события с одним временем: они склеиваются в одну запись.
func (item *DeliveryItemState) resolved(at time.Time, by eventstore.Actor, change func(r *QuarantineResolved)) {
	item.Resolutions = slices.Clone(item.Resolutions)
	if n := len(item.Resolutions); n == 0 || !item.Resolutions[n-1].DecidedAt.Equal(at) {
		item.Resolutions = append(item.Resolutions, QuarantineResolved{DecidedAt: at, DecidedBy: by})
	}
	change(&item.Resolutions[len(item.Resolutions)-1])
	if item.QuarantinedQuantity == 0 {
		item.QuarantinedAt = nil
	}
}

// item возвращает позицию поставки по идентификатору.
func (d Delivery) item(id uuid.UUID) (DeliveryItemState, bool) {
	for _, item := range d.Items {
		if item.ID == id {
			return item, true
		}
	}
	return DeliveryItemState{}, false
}

// withItem возвращает поставку с изменённой позицией. Срезы копируются:
// apply не должна менять прежнее состояние.
func (d Delivery) withItem(id uuid.UUID, change func(item *DeliveryItemState)) (Delivery, error) {
	items := slices.Clone(d.Items)
	for i := range items {
		if items[i].ID == id {
			items[i].BatchIDs = slices.Clone(items[i].BatchIDs)
			change(&items[i])
			d.Items = items
			return d, nil
		}
	}
	return Delivery{}, fmt.Errorf("inventory: delivery %s has no item %s", d.ID, id)
}

// NewDelivery - данные для регистрации поставки.
type NewDelivery struct {
	// PointID - точка, на которую пришла поставка; заполняет команда регистрации.
	PointID     uuid.UUID
	Number      string
	DeliveredAt time.Time
	Items       []NewDeliveryItem
}

type NewDeliveryItem struct {
	ProductID        uuid.UUID
	DeclaredQuantity int64
	ExpiresAt        time.Time
	ManufacturedAt   *time.Time
}

// ItemDecision - решение по позиции при приёмке. Причина и комментарий обязательны
// для отклонённого и карантинного количества.
type ItemDecision struct {
	ItemID              uuid.UUID
	AcceptedQuantity    int64
	RejectedQuantity    int64
	QuarantinedQuantity int64
	RejectReason        string
	QuarantineReason    string
	Comment             string
}

// planRegistration проверяет данные поставки и формирует событие регистрации.
// products - продукты позиций из каталога.
func planRegistration(
	deliveryID uuid.UUID, in NewDelivery, actor eventstore.Actor, products map[uuid.UUID]catalog.Product,
) (DeliveryRegistered, error) {
	number := strings.TrimSpace(in.Number)
	switch {
	case number == "":
		return DeliveryRegistered{}, invalid("number", "must not be empty")
	case in.DeliveredAt.IsZero():
		return DeliveryRegistered{}, invalid("delivered_at", "is required")
	case len(in.Items) == 0:
		return DeliveryRegistered{}, invalid("items", "must not be empty")
	}

	event := DeliveryRegistered{
		PointID: in.PointID, DeliveryID: deliveryID, Number: number, DeliveredAt: eventstore.At(in.DeliveredAt),
		RegisteredBy: actor, Items: make([]DeliveryItem, len(in.Items)),
	}
	for i, item := range in.Items {
		field := func(name string) string { return fmt.Sprintf("items[%d].%s", i, name) }
		product, known := products[item.ProductID]
		switch {
		case !known:
			return DeliveryRegistered{}, &catalog.UnknownProductError{ProductID: item.ProductID}
		case item.DeclaredQuantity <= 0:
			return DeliveryRegistered{}, invalid(field("declared_quantity"), "must be positive")
		case item.DeclaredQuantity > catalog.MaxQuantity:
			return DeliveryRegistered{}, invalid(field("declared_quantity"), fmt.Sprintf("must not exceed %d", catalog.MaxQuantity))
		case item.ExpiresAt.IsZero():
			return DeliveryRegistered{}, invalid(field("expires_at"), "is required")
		}

		out := DeliveryItem{
			ItemID: uuid.Must(uuid.NewV7()), ProductID: item.ProductID,
			DeclaredQuantity: item.DeclaredQuantity, ExpiresAt: eventstore.At(item.ExpiresAt),
		}
		if item.ManufacturedAt != nil {
			made := *item.ManufacturedAt
			if made.After(item.ExpiresAt) {
				return DeliveryRegistered{}, invalid(field("manufactured_at"), "must not be after expires_at")
			}
			// срок годности не может быть длиннее срока хранения продукта
			if product.ShelfLifeDays != nil {
				shelfLife := time.Duration(*product.ShelfLifeDays) * 24 * time.Hour
				if item.ExpiresAt.Sub(made) > shelfLife {
					return DeliveryRegistered{}, invalid(field("expires_at"),
						fmt.Sprintf("exceeds the product shelf life of %d days", *product.ShelfLifeDays))
				}
			}
			at := eventstore.At(made)
			out.ManufacturedAt = &at
		}
		event.Items[i] = out
	}
	return event, nil
}

// planApproval проверяет решения по позициям и формирует снимок приёмки
// и события партий для принятого количества. now - время приёмки.
func (d Delivery) planApproval(
	decisions []ItemDecision, actor eventstore.Actor, now time.Time,
) (DeliveryApproved, []BatchReceived, error) {
	if d.Status != DeliveryRegisteredStatus {
		return DeliveryApproved{}, nil, fmt.Errorf("delivery %s: %w", d.ID, ErrAlreadyApproved)
	}

	byItem := make(map[uuid.UUID]ItemDecision, len(decisions))
	for _, decision := range decisions {
		if _, twice := byItem[decision.ItemID]; twice {
			return DeliveryApproved{}, nil, invalid("items", fmt.Sprintf("item %s is decided twice", decision.ItemID))
		}
		byItem[decision.ItemID] = decision
	}

	approved := DeliveryApproved{
		PointID: d.PointID, DeliveryID: d.ID, DeliveredAt: eventstore.At(d.DeliveredAt), ApprovedAt: eventstore.At(now),
		ApprovedBy: actor, Items: make([]ApprovedItem, len(d.Items)),
	}
	var batches []BatchReceived
	for i, item := range d.Items {
		decision, decided := byItem[item.ID]
		if !decided {
			return DeliveryApproved{}, nil, invalid("items", fmt.Sprintf("item %s has no decision", item.ID))
		}
		delete(byItem, item.ID)

		// просрочка определяется в момент приёмки, а не регистрации
		expired := !item.ExpiresAt.After(now)
		if err := checkDecision(item, decision, expired); err != nil {
			return DeliveryApproved{}, nil, err
		}

		out := ApprovedItem{
			ItemID: item.ID, ProductID: item.ProductID, DeclaredQuantity: item.DeclaredQuantity,
			AcceptedQuantity: decision.AcceptedQuantity, RejectedQuantity: decision.RejectedQuantity,
			QuarantinedQuantity: decision.QuarantinedQuantity,
			ExpiresAt:           eventstore.At(item.ExpiresAt), Expired: expired,
		}
		if decision.RejectedQuantity > 0 {
			out.RejectReason = strings.TrimSpace(decision.RejectReason)
		}
		if decision.QuarantinedQuantity > 0 {
			out.QuarantineReason = strings.TrimSpace(decision.QuarantineReason)
		}
		if decision.RejectedQuantity > 0 || decision.QuarantinedQuantity > 0 {
			out.Comment = strings.TrimSpace(decision.Comment)
		}
		if decision.AcceptedQuantity > 0 {
			batchID := uuid.Must(uuid.NewV7())
			out.BatchID = &batchID
			batches = append(batches, BatchReceived{
				PointID: d.PointID, BatchID: batchID, ProductID: item.ProductID, DeliveryID: d.ID, ItemID: item.ID,
				Quantity: decision.AcceptedQuantity, ExpiresAt: eventstore.At(item.ExpiresAt),
				ReceivedAt: eventstore.At(now), Source: BatchFromApproval,
			})
		}
		approved.Items[i] = out
	}

	for itemID := range byItem {
		return DeliveryApproved{}, nil, invalid("items", fmt.Sprintf("item %s is not part of the delivery", itemID))
	}
	return approved, batches, nil
}

func checkDecision(item DeliveryItemState, decision ItemDecision, expired bool) error {
	field := func(name string) string { return fmt.Sprintf("items[%s].%s", item.ID, name) }
	accepted, rejected, quarantined := decision.AcceptedQuantity, decision.RejectedQuantity, decision.QuarantinedQuantity
	switch {
	case accepted < 0 || rejected < 0 || quarantined < 0:
		return invalid(field("quantity"), "quantities must not be negative")
	// каждая часть не больше заявленного: иначе сумма трёх int64 переполняется и совпадает с ним
	case accepted > item.DeclaredQuantity || rejected > item.DeclaredQuantity || quarantined > item.DeclaredQuantity:
		return invalid(field("quantity"), fmt.Sprintf("each quantity must not exceed declared quantity %d", item.DeclaredQuantity))
	case accepted+rejected+quarantined != item.DeclaredQuantity:
		return invalid(field("quantity"),
			fmt.Sprintf("accepted + rejected + quarantined must equal declared quantity %d", item.DeclaredQuantity))
	case expired && accepted > 0:
		return expiredField(field("accepted_quantity"), "item is expired and cannot be accepted")
	case rejected > 0 && strings.TrimSpace(decision.RejectReason) == "":
		return invalid(field("reject_reason"), "is required for rejected quantity")
	case quarantined > 0 && strings.TrimSpace(decision.QuarantineReason) == "":
		return invalid(field("quarantine_reason"), "is required for quarantined quantity")
	case (rejected > 0 || quarantined > 0) && strings.TrimSpace(decision.Comment) == "":
		return invalid(field("comment"), "is required for rejected or quarantined quantity")
	}
	return nil
}

// foldDelivery собирает состояние поставки из событий её потока.
func foldDelivery(events []eventstore.Payload) (Delivery, error) {
	var d Delivery
	for _, event := range events {
		var err error
		if d, err = d.apply(event); err != nil {
			return Delivery{}, err
		}
	}
	return d, nil
}

func (d Delivery) apply(event eventstore.Payload) (Delivery, error) {
	switch e := event.(type) {
	case DeliveryRegistered:
		d = Delivery{
			ID: e.DeliveryID, PointID: e.PointID, Number: e.Number, DeliveredAt: e.DeliveredAt.Time,
			Status: DeliveryRegisteredStatus, Items: make([]DeliveryItemState, len(e.Items)),
		}
		for i, item := range e.Items {
			d.Items[i] = DeliveryItemState{
				ID: item.ItemID, ProductID: item.ProductID,
				DeclaredQuantity: item.DeclaredQuantity, ExpiresAt: item.ExpiresAt.Time,
			}
			if item.ManufacturedAt != nil {
				made := item.ManufacturedAt.Time
				d.Items[i].ManufacturedAt = &made
			}
		}
	case DeliveryApproved:
		approvedAt, approvedBy := e.ApprovedAt.Time, e.ApprovedBy
		d.Status, d.ApprovedAt, d.ApprovedBy = DeliveryApprovedStatus, &approvedAt, &approvedBy
		// события не меняют срез прежнего состояния: копия нужна, чтобы apply оставалась чистой
		items := make([]DeliveryItemState, len(d.Items))
		copy(items, d.Items)
		byID := make(map[uuid.UUID]ApprovedItem, len(e.Items))
		for _, item := range e.Items {
			byID[item.ItemID] = item
		}
		for i := range items {
			a := byID[items[i].ID]
			items[i].AcceptedQuantity = a.AcceptedQuantity
			items[i].RejectedQuantity = a.RejectedQuantity
			items[i].QuarantinedQuantity = a.QuarantinedQuantity
			items[i].Expired = a.Expired
			if a.BatchID != nil {
				items[i].BatchIDs = []uuid.UUID{*a.BatchID}
			}
			items[i].RejectReason = a.RejectReason
			items[i].QuarantineReason = a.QuarantineReason
			items[i].Comment = a.Comment
			items[i].Approved = &Distribution{
				AcceptedQuantity: a.AcceptedQuantity, RejectedQuantity: a.RejectedQuantity, QuarantinedQuantity: a.QuarantinedQuantity,
			}
			if a.QuarantinedQuantity > 0 {
				items[i].QuarantinedAt = &approvedAt
			}
		}
		d.Items = items
	case QuarantinedDeliveryItemAccepted:
		return d.withItem(e.ItemID, func(item *DeliveryItemState) {
			item.AcceptedQuantity += e.Quantity
			item.QuarantinedQuantity -= e.Quantity
			item.BatchIDs = append(item.BatchIDs, e.BatchID)
			item.resolved(e.DecidedAt.Time, e.DecidedBy, func(r *QuarantineResolved) {
				r.AcceptedQuantity, r.Comment = e.Quantity, e.Comment
			})
		})
	case QuarantinedDeliveryItemRejected:
		return d.withItem(e.ItemID, func(item *DeliveryItemState) {
			item.RejectedQuantity += e.Quantity
			item.QuarantinedQuantity -= e.Quantity
			item.resolved(e.DecidedAt.Time, e.DecidedBy, func(r *QuarantineResolved) {
				r.RejectedQuantity, r.RejectReason, r.Comment = e.Quantity, e.Reason, e.Comment
			})
		})
	case DeliveryApprovalCorrected:
		correctedAt := e.CorrectedAt.Time
		d.CorrectedAt = &correctedAt
		for _, corrected := range e.Items {
			var err error
			d, err = d.withItem(corrected.ItemID, func(item *DeliveryItemState) {
				// карантин открывает исправление, которое назначило его позиции без карантина
				switch {
				case corrected.QuarantinedQuantity == 0:
					item.QuarantinedAt = nil
				case item.QuarantinedQuantity == 0:
					item.QuarantinedAt = &correctedAt
				}
				item.AcceptedQuantity = corrected.AcceptedQuantity
				item.RejectedQuantity = corrected.RejectedQuantity
				item.QuarantinedQuantity = corrected.QuarantinedQuantity
				if corrected.BatchID != nil {
					item.BatchIDs = append(item.BatchIDs, *corrected.BatchID)
				}
			})
			if err != nil {
				return Delivery{}, err
			}
		}
	default:
		// молча пропущенное событие означало бы неверные количества
		return Delivery{}, fmt.Errorf("inventory: delivery stream has unsupported event %s", event.EventType())
	}
	return d, nil
}
