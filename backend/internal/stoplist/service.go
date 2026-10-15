package stoplist

import (
	"context"

	"github.com/google/uuid"

	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
	"goden/backend/internal/organization"
)

// каталог и остатки читаются страницами этого размера до конца
const readPage = 200

// Service считает меню и стоп-лист. Своих таблиц у модуля нет: блюда даёт каталог,
// доступные остатки - склад.
type Service struct {
	dishes *catalog.Service
	stock  *inventory.Service
}

func NewService(dishes *catalog.Service, stock *inventory.Service) *Service {
	return &Service{dishes: dishes, stock: stock}
}

// Menu возвращает страницу блюд точки с доступностью в алфавитном порядке названий и общее число блюд:
// блюда компании точки, которые на ней продаются, по остаткам этой точки.
// С onlyStopped остаются только блюда, которые нельзя приготовить: это стоп-лист.
//
// упрощение: на каждый запрос читаются все блюда и все остатки, фильтр и пагинация - в памяти.
// Для десятков блюд это несколько запросов; при тысячах нужен кеш с инвалидацией по событиям.
func (s *Service) Menu(ctx context.Context, point organization.Point, onlyStopped bool, page catalog.Page) ([]Availability, int64, error) {
	stock, err := s.stockByProduct(ctx, point)
	if err != nil {
		return nil, 0, err
	}

	var menu []Availability
	for offset := int32(0); ; offset += readPage {
		dishes, total, err := s.dishes.DishesOnPoint(ctx, point.CompanyID, point.ID, catalog.Page{Limit: readPage, Offset: offset})
		if err != nil {
			return nil, 0, err
		}
		for _, dish := range dishes {
			if a := availability(dish, stock); !onlyStopped || !a.Available() {
				menu = append(menu, a)
			}
		}
		if int64(offset)+readPage >= total {
			break
		}
	}

	total := int64(len(menu))
	from := min(int64(page.Offset), total)
	to := min(from+int64(page.Limit), total)
	return menu[from:to], total, nil
}

func (s *Service) stockByProduct(ctx context.Context, point organization.Point) (map[uuid.UUID]inventory.StockLevel, error) {
	stock := map[uuid.UUID]inventory.StockLevel{}
	for offset := int32(0); ; offset += readPage {
		levels, total, err := s.stock.Stock(ctx, point, catalog.Page{Limit: readPage, Offset: offset})
		if err != nil {
			return nil, err
		}
		for _, level := range levels {
			stock[level.Product.ID] = level
		}
		if int64(offset)+readPage >= total {
			return stock, nil
		}
	}
}
