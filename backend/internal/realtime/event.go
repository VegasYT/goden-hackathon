// Package realtime преобразует интеграционные события в сигналы обновления
// и раздаёт их подключённым SSE-клиентам.
package realtime

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/google/uuid"

	platformkafka "goden/backend/internal/platform/kafka"
)

// Category называет REST-представление, которое клиенту нужно перечитать.
type Category string

const (
	// CategoryCatalog - продукты, блюда и рецептуры компании, блюда точки.
	CategoryCatalog Category = "catalog"
	// CategoryDeliveries - список, сводка и карточка поставки.
	CategoryDeliveries Category = "deliveries"
	CategoryExpiry     Category = "expiry"
	// CategoryOrders - история заказов точки: принятый и отклонённый заказ.
	CategoryOrders          Category = "orders"
	CategoryRecommendations Category = "recommendations"
	CategoryStock           Category = "stock"
	CategoryStopList        Category = "stop-list"
)

// stockCategories - что перечитать после изменения остатка. Поставок здесь нет:
// иначе их перечитывал бы каждый заказ.
var stockCategories = []Category{CategoryExpiry, CategoryRecommendations, CategoryStock, CategoryStopList}

var allCategories = []Category{
	CategoryCatalog,
	CategoryDeliveries,
	CategoryExpiry,
	CategoryOrders,
	CategoryRecommendations,
	CategoryStock,
	CategoryStopList,
}

// Update - служебный сигнал без бизнес-данных исходного события.
type Update struct {
	ID         uuid.UUID  `json:"id"`
	OccurredAt time.Time  `json:"occurred_at"`
	Categories []Category `json:"categories"`
	// Адресация: сигнал события склада уходит подписчикам точки, сигнал события каталога -
	// подписчикам всех точек компании. В ответ клиенту эти поля не попадают.
	PointID   uuid.UUID `json:"-"`
	CompanyID uuid.UUID `json:"-"`
}

// Scope - на что подписан клиент: точка и компания, которой она принадлежит (ADR-013).
type Scope struct {
	PointID   uuid.UUID
	CompanyID uuid.UUID
}

// receives сообщает, адресован ли сигнал подписчику. Сигнал без точки и компании не получает никто.
func (s Scope) receives(u Update) bool {
	return (u.PointID != uuid.Nil && u.PointID == s.PointID) || (u.CompanyID != uuid.Nil && u.CompanyID == s.CompanyID)
}

// FromEnvelope возвращает сигнал только для событий, меняющих поддерживаемые представления.
func FromEnvelope(event platformkafka.Envelope) (Update, bool) {
	var categories []Category
	switch event.EventType {
	// события каталога несут компанию, события склада и меню точки - точку
	case "ProductCreated":
		categories = []Category{CategoryCatalog, CategoryRecommendations, CategoryStock}
	case "DishCreated":
		categories = []Category{CategoryCatalog, CategoryStopList}
	case "RecipeChanged", "DishAvailabilityChanged",
		"ProductArchived", "ProductRestored", "DishArchived", "DishRestored":
		categories = []Category{CategoryCatalog, CategoryRecommendations, CategoryStopList}
	// название продукта и блюда видно в других разделах: их тоже нужно перечитать
	case "ProductChanged":
		categories = []Category{
			CategoryCatalog, CategoryDeliveries, CategoryOrders, CategoryRecommendations, CategoryStock, CategoryStopList,
		}
	case "DishRenamed":
		categories = []Category{CategoryCatalog, CategoryOrders, CategoryRecommendations, CategoryStopList}
	case "BatchReceived", "StockConsumed", "StockReturned", "StockAdjusted", "StockWrittenOff":
		categories = slices.Clone(stockCategories)
	case "DeliveryRegistered", "DeliveryApproved", "DeliveryApprovalCorrected",
		"QuarantinedDeliveryItemAccepted", "QuarantinedDeliveryItemRejected":
		categories = []Category{CategoryDeliveries}
	case "BatchExpiryWarningIssued":
		categories = []Category{CategoryExpiry}
	// годный остаток уменьшился без складского события: меню и рекомендации перечитываются
	case "BatchExpired":
		categories = slices.Clone(stockCategories)
	case "HotOfferChanged":
		categories = []Category{CategoryRecommendations}
	case "OrderAccepted", "OrderRejected", "OrderCancelled":
		// складские категории принятому и отменённому заказу дают его события StockConsumed и StockReturned
		categories = []Category{CategoryOrders}
	default:
		return Update{}, false
	}

	var target struct {
		PointID   uuid.UUID `json:"point_id"`
		CompanyID uuid.UUID `json:"company_id"`
	}
	// нечитаемая полезная нагрузка оставляет сигнал без адресата: он никому не уйдёт
	_ = json.Unmarshal(event.Payload, &target)
	return Update{
		ID: event.EventID, OccurredAt: event.OccurredAt, Categories: categories,
		PointID: target.PointID, CompanyID: target.CompanyID,
	}, true
}
