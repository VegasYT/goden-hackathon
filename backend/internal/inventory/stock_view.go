package inventory

import (
	"cmp"
	"slices"
	"time"
)

// Статус и порядок остатков: чистые функции, которыми чтение остатков фильтрует и сортирует продукты.

// StockStatus - статус остатка продукта. Статусы не исключают друг друга: продукт с годной
// и просроченной партией и в наличии, и просрочен.
type StockStatus string

const (
	StockInStock    StockStatus = "in_stock"
	StockExpiring   StockStatus = "expiring"
	StockExpired    StockStatus = "expired"
	StockOutOfStock StockStatus = "out_of_stock"
)

// StockStatuses - все статусы, в порядке сводки.
var StockStatuses = []StockStatus{StockInStock, StockExpiring, StockExpired, StockOutOfStock}

// stockExpiringWithin - за сколько до ближайшего срока годной партии продукт считается истекающим.
const stockExpiringWithin = 24 * time.Hour

// Is сообщает, есть ли у остатка статус на момент now. Остаток должен быть посчитан на тот же момент.
func (l StockLevel) Is(status StockStatus, now time.Time) bool {
	switch status {
	case StockInStock:
		return l.Available > 0
	case StockExpiring:
		return l.NearestExpiresAt != nil && !l.NearestExpiresAt.After(now.Add(stockExpiringWithin))
	case StockExpired:
		return l.Expired > 0
	case StockOutOfStock:
		return l.Available == 0
	}
	return false
}

// StockSort - поле сортировки остатков.
type StockSort string

const (
	StockByName          StockSort = "name"
	StockByNearestExpiry StockSort = "nearest_expiry"
	StockByAvailable     StockSort = "available"
)

// StockQuery - условия чтения остатков. Пустые значения не фильтруют, порядок по умолчанию - по названию.
type StockQuery struct {
	// NamePrefix оставляет продукты, название которых начинается с этого текста, без учёта регистра.
	NamePrefix string
	Category   string
	Status     StockStatus
	Sort       StockSort
	Descending bool
}

// StockSummary - число продуктов по статусам. Статусы пересекаются: сумма может превышать Total.
type StockSummary struct {
	Total    int64
	ByStatus map[StockStatus]int64
}

// sortStock упорядочивает остатки, пришедшие в порядке названий. Сортировка устойчива: продукты
// с равным значением остаются по названию. Продукты без ближайшего срока при сортировке по сроку
// идут последними в обоих направлениях.
func sortStock(levels []StockLevel, by StockSort, descending bool) {
	direction := 1
	if descending {
		direction = -1
	}
	switch by {
	case StockByAvailable:
		slices.SortStableFunc(levels, func(a, b StockLevel) int { return direction * cmp.Compare(a.Available, b.Available) })
	case StockByNearestExpiry:
		slices.SortStableFunc(levels, func(a, b StockLevel) int {
			switch {
			case a.NearestExpiresAt == nil || b.NearestExpiresAt == nil:
				// без срока - в конец, независимо от направления
				return cmp.Compare(boolRank(a.NearestExpiresAt == nil), boolRank(b.NearestExpiresAt == nil))
			default:
				return direction * a.NearestExpiresAt.Compare(*b.NearestExpiresAt)
			}
		})
	default:
		if descending {
			slices.Reverse(levels)
		}
	}
}

func boolRank(v bool) int {
	if v {
		return 1
	}
	return 0
}
