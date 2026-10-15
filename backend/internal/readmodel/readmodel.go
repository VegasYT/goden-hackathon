// Package readmodel перечисляет асинхронные проекции системы. Список один для cmd/worker,
// который их выполняет, и для тестов: новая проекция добавляется сюда и больше нигде.
package readmodel

import (
	"goden/backend/internal/analytics"
	"goden/backend/internal/inventory"
	"goden/backend/internal/orders"
	"goden/backend/internal/platform/projection"
	// проекции декодируют все события, поэтому процесс обязан знать каждый тип
	_ "goden/backend/internal/recommendations"
)

// Projections возвращает все проекции системы.
func Projections() []projection.Projection {
	return []projection.Projection{
		analytics.Projection(),
		inventory.DeliveriesProjection(),
		orders.HistoryProjection(),
	}
}
