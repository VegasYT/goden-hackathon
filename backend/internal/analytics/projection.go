// Package analytics строит статистику потерь и дефицита и прогноз стоп-листа.
package analytics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
	"goden/backend/internal/orders"
	"goden/backend/internal/organization"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/projection"
)

// Projection нормализует события, нужные отчётам, и восстанавливает интервалы,
// в которые конкретный продукт делал блюдо недоступным.
func Projection() projection.Projection {
	return projection.Projection{
		// версия 2: факты и интервалы стоп-листа ведутся по точкам
		// версия 3: отключение точки и компании закрывает интервалы стоп-листа
		// версия 4: архивное блюдо не продаётся ни на одной точке и закрывает свои интервалы
		// версия 5: расход помнит заказ, отмена заказа удаляет его расход
		Name: "analytics", Version: 5,
		Apply: applyEvent,
		Reset: func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `TRUNCATE analytics_applied_events, analytics_write_offs,
				analytics_order_shortages, analytics_consumption, analytics_batches,
				analytics_dishes, analytics_recipe_ingredients, analytics_stop_intervals,
				analytics_points, analytics_disabled_dishes, analytics_projection_clock`)
			return err
		},
	}
}

func applyEvent(ctx context.Context, tx pgx.Tx, stored eventstore.Event) error {
	applied, err := rememberEvent(ctx, tx, stored.ID)
	if err != nil || !applied {
		return err
	}
	at, err := advanceTime(ctx, tx, stored.OccurredAt.UTC())
	if err != nil {
		return err
	}
	payload, err := eventstore.Decode(stored.Type, stored.SchemaVersion, stored.Payload)
	if err != nil {
		return err
	}

	switch event := payload.(type) {
	case organization.PointCreated:
		_, err = tx.Exec(ctx, `INSERT INTO analytics_points (id, company_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			event.PointID, event.CompanyID)
		if err == nil {
			err = recomputePoint(ctx, tx, event.PointID, at)
		}
	case organization.PointDisabled:
		_, err = tx.Exec(ctx, `UPDATE analytics_points SET active = false WHERE id = $1`, event.PointID)
		if err == nil {
			err = recomputePoint(ctx, tx, event.PointID, at)
		}
	case organization.CompanyDisabled:
		err = disableCompany(ctx, tx, event.CompanyID, at)
	case catalog.DishCreated:
		_, err = tx.Exec(ctx, `INSERT INTO analytics_dishes (id, company_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			event.DishID, event.CompanyID)
	case catalog.DishAvailabilityChanged:
		if event.Enabled {
			_, err = tx.Exec(ctx, `DELETE FROM analytics_disabled_dishes WHERE point_id = $1 AND dish_id = $2`, event.PointID, event.DishID)
		} else {
			_, err = tx.Exec(ctx, `INSERT INTO analytics_disabled_dishes (point_id, dish_id) VALUES ($1, $2)
				ON CONFLICT DO NOTHING`, event.PointID, event.DishID)
		}
		if err == nil {
			err = recomputeDishAt(ctx, tx, event.PointID, event.DishID, at)
		}
	case catalog.DishArchived:
		err = setDishArchived(ctx, tx, event.DishID, true, at)
	case catalog.DishRestored:
		err = setDishArchived(ctx, tx, event.DishID, false, at)
	case catalog.RecipeChanged:
		err = replaceRecipe(ctx, tx, event, at)
	case inventory.BatchReceived:
		_, err = tx.Exec(ctx, `INSERT INTO analytics_batches (id, point_id, product_id, quantity, expires_at)
			VALUES ($1, $2, $3, $4, $5) ON CONFLICT (id) DO NOTHING`,
			event.BatchID, event.PointID, event.ProductID, event.Quantity, event.ExpiresAt.Time)
		if err == nil {
			err = recomputeProduct(ctx, tx, event.PointID, event.ProductID, at)
		}
	case inventory.StockConsumed:
		err = consume(ctx, tx, stored, event, at)
	case inventory.StockReturned:
		err = giveBack(ctx, tx, event, at)
	case inventory.StockAdjusted:
		_, err = tx.Exec(ctx, `UPDATE analytics_batches SET quantity = quantity + $2 WHERE id = $1`, event.BatchID, event.Delta)
		if err == nil {
			err = recomputeProduct(ctx, tx, event.PointID, event.ProductID, at)
		}
	case inventory.StockWrittenOff:
		err = writeOff(ctx, tx, stored, event, at)
	case orders.OrderRejected:
		err = recordShortages(ctx, tx, stored, event)
	}
	if err != nil {
		return fmt.Errorf("analytics: apply %s: %w", stored.Type, err)
	}
	return nil
}

func rememberEvent(ctx context.Context, tx pgx.Tx, id uuid.UUID) (bool, error) {
	var inserted bool
	err := tx.QueryRow(ctx, `INSERT INTO analytics_applied_events (event_id) VALUES ($1)
		ON CONFLICT DO NOTHING RETURNING true`, id).Scan(&inserted)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("analytics: remember event: %w", err)
	}
	return inserted, nil
}

// advanceTime открывает интервалы, вызванные истечением партий между событиями.
// Для события с более ранним временем используется уже достигнутое время проекции.
func advanceTime(ctx context.Context, tx pgx.Tx, eventTime time.Time) (time.Time, error) {
	var advanced time.Time
	err := tx.QueryRow(ctx, `SELECT advanced_at FROM analytics_projection_clock WHERE singleton`).Scan(&advanced)
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = tx.Exec(ctx, `INSERT INTO analytics_projection_clock (advanced_at) VALUES ($1)`, eventTime)
		return eventTime, err
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("analytics: read projection time: %w", err)
	}
	advanced = advanced.UTC()
	if !eventTime.After(advanced) {
		return advanced, nil
	}

	rows, err := tx.Query(ctx, `SELECT DISTINCT point_id, product_id, expires_at
		FROM analytics_batches WHERE quantity > 0 AND expires_at > $1 AND expires_at <= $2
		ORDER BY expires_at, point_id, product_id`, advanced, eventTime)
	if err != nil {
		return time.Time{}, fmt.Errorf("analytics: read expirations: %w", err)
	}
	type expiration struct {
		pointID, productID uuid.UUID
		at                 time.Time
	}
	var expirations []expiration
	for rows.Next() {
		var item expiration
		if err := rows.Scan(&item.pointID, &item.productID, &item.at); err != nil {
			rows.Close()
			return time.Time{}, fmt.Errorf("analytics: read expiration: %w", err)
		}
		expirations = append(expirations, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return time.Time{}, fmt.Errorf("analytics: read expirations: %w", err)
	}
	rows.Close()
	for _, item := range expirations {
		if err := recomputeProduct(ctx, tx, item.pointID, item.productID, item.at.UTC()); err != nil {
			return time.Time{}, err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE analytics_projection_clock SET advanced_at = $1 WHERE singleton`, eventTime)
	return eventTime, err
}

func replaceRecipe(ctx context.Context, tx pgx.Tx, event catalog.RecipeChanged, at time.Time) error {
	if _, err := tx.Exec(ctx, `DELETE FROM analytics_recipe_ingredients WHERE dish_id = $1`, event.DishID); err != nil {
		return err
	}
	for _, ingredient := range event.Ingredients {
		if _, err := tx.Exec(ctx, `INSERT INTO analytics_recipe_ingredients (dish_id, product_id, quantity)
			VALUES ($1, $2, $3)`, event.DishID, ingredient.ProductID, ingredient.Quantity); err != nil {
			return err
		}
	}
	return recomputeDish(ctx, tx, event.DishID, at)
}

func consume(ctx context.Context, tx pgx.Tx, stored eventstore.Event, event inventory.StockConsumed, at time.Time) error {
	var quantity int64
	for _, batch := range event.Batches {
		quantity += batch.Quantity
		if _, err := tx.Exec(ctx, `UPDATE analytics_batches SET quantity = quantity - $2 WHERE id = $1`, batch.BatchID, batch.Quantity); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO analytics_consumption (event_id, point_id, product_id, order_id, quantity, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6)`, stored.ID, event.PointID, event.ProductID, event.OrderID, quantity, stored.OccurredAt.UTC()); err != nil {
		return err
	}
	return recomputeProduct(ctx, tx, event.PointID, event.ProductID, at)
}

// giveBack возвращает продукт отменённого заказа в партии и убирает его расход: отменённый
// заказ спросом не считается, прогноз строится так, как если бы заказа не было.
func giveBack(ctx context.Context, tx pgx.Tx, event inventory.StockReturned, at time.Time) error {
	for _, batch := range event.Batches {
		if _, err := tx.Exec(ctx, `UPDATE analytics_batches SET quantity = quantity + $2 WHERE id = $1`, batch.BatchID, batch.Quantity); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM analytics_consumption WHERE point_id = $1 AND product_id = $2 AND order_id = $3`,
		event.PointID, event.ProductID, event.OrderID); err != nil {
		return err
	}
	return recomputeProduct(ctx, tx, event.PointID, event.ProductID, at)
}

func writeOff(ctx context.Context, tx pgx.Tx, stored eventstore.Event, event inventory.StockWrittenOff, at time.Time) error {
	if _, err := tx.Exec(ctx, `UPDATE analytics_batches SET quantity = quantity - $2 WHERE id = $1`, event.BatchID, event.Quantity); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO analytics_write_offs
		(event_id, point_id, product_id, reason, quantity, occurred_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		stored.ID, event.PointID, event.ProductID, event.Reason, event.Quantity, stored.OccurredAt.UTC()); err != nil {
		return err
	}
	return recomputeProduct(ctx, tx, event.PointID, event.ProductID, at)
}

func recordShortages(ctx context.Context, tx pgx.Tx, stored eventstore.Event, event orders.OrderRejected) error {
	for _, shortage := range event.Shortages {
		if _, err := tx.Exec(ctx, `INSERT INTO analytics_order_shortages (event_id, point_id, product_id, occurred_at)
			VALUES ($1, $2, $3, $4)`, stored.ID, event.PointID, shortage.ProductID, stored.OccurredAt.UTC()); err != nil {
			return err
		}
	}
	return nil
}

// recomputeProduct пересчитывает на точке интервалы блюд, в рецепт которых входит продукт.
func recomputeProduct(ctx context.Context, tx pgx.Tx, pointID, productID uuid.UUID, at time.Time) error {
	rows, err := tx.Query(ctx, `SELECT dish_id FROM analytics_recipe_ingredients WHERE product_id = $1`, productID)
	if err != nil {
		return err
	}
	dishes, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return err
	}
	for _, dishID := range dishes {
		if err := recomputeDishAt(ctx, tx, pointID, dishID, at); err != nil {
			return err
		}
	}
	return nil
}

// recomputeDish пересчитывает интервалы блюда на всех точках его компании: рецептура общая,
// а остатки у каждой точки свои.
func recomputeDish(ctx context.Context, tx pgx.Tx, dishID uuid.UUID, at time.Time) error {
	rows, err := tx.Query(ctx, `SELECT p.id FROM analytics_points p
		JOIN analytics_dishes d ON d.company_id = p.company_id WHERE d.id = $1 ORDER BY p.id`, dishID)
	if err != nil {
		return err
	}
	points, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return err
	}
	for _, pointID := range points {
		if err := recomputeDishAt(ctx, tx, pointID, dishID, at); err != nil {
			return err
		}
	}
	return nil
}

// setDishArchived помечает блюдо архивным или восстановленным и пересчитывает его интервалы
// на всех точках компании: архивное блюдо нигде не продаётся.
func setDishArchived(ctx context.Context, tx pgx.Tx, dishID uuid.UUID, archived bool, at time.Time) error {
	if _, err := tx.Exec(ctx, `UPDATE analytics_dishes SET archived = $2 WHERE id = $1`, dishID, archived); err != nil {
		return err
	}
	return recomputeDish(ctx, tx, dishID, at)
}

// recomputePoint пересчитывает на точке интервалы всех блюд её компании: у новой точки
// остатков нет, и блюда с рецептурой сразу оказываются в стоп-листе.
func recomputePoint(ctx context.Context, tx pgx.Tx, pointID uuid.UUID, at time.Time) error {
	rows, err := tx.Query(ctx, `SELECT d.id FROM analytics_dishes d
		JOIN analytics_points p ON p.company_id = d.company_id WHERE p.id = $1 ORDER BY d.id`, pointID)
	if err != nil {
		return err
	}
	dishes, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return err
	}
	for _, dishID := range dishes {
		if err := recomputeDishAt(ctx, tx, pointID, dishID, at); err != nil {
			return err
		}
	}
	return nil
}

// disableCompany закрывает интервалы стоп-листа всех точек отключённой компании.
func disableCompany(ctx context.Context, tx pgx.Tx, companyID uuid.UUID, at time.Time) error {
	rows, err := tx.Query(ctx, `UPDATE analytics_points SET active = false WHERE company_id = $1 AND active RETURNING id`, companyID)
	if err != nil {
		return err
	}
	points, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return err
	}
	for _, pointID := range points {
		if err := recomputePoint(ctx, tx, pointID, at); err != nil {
			return err
		}
	}
	return nil
}

// short - условие "ингредиента r не хватает на точке $3 в момент $2". Блюдо, выключенное
// на точке или архивное, на ней не продаётся и дефицита не создаёт: его ингредиенты считаются
// достаточными. То же для отключённой точки: на ней не продаётся ничего.
const short = `r.quantity > COALESCE((SELECT sum(b.quantity) FROM analytics_batches b
				WHERE b.point_id = $3 AND b.product_id = r.product_id AND b.expires_at > $2), 0)
			  AND NOT EXISTS (SELECT 1 FROM analytics_disabled_dishes d WHERE d.point_id = $3 AND d.dish_id = r.dish_id)
			  AND NOT EXISTS (SELECT 1 FROM analytics_dishes ad WHERE ad.id = r.dish_id AND ad.archived)
			  AND NOT EXISTS (SELECT 1 FROM analytics_points p WHERE p.id = $3 AND NOT p.active)`

// recomputeDishAt пересчитывает интервалы стоп-листа блюда на одной точке.
func recomputeDishAt(ctx context.Context, tx pgx.Tx, pointID, dishID uuid.UUID, at time.Time) error {
	// Закрываем продукты, которых больше нет в рецепте или которых снова хватает.
	_, err := tx.Exec(ctx, `UPDATE analytics_stop_intervals i
		SET ended_at = GREATEST(i.started_at, $2::timestamptz)
		WHERE i.point_id = $3 AND i.dish_id = $1 AND i.ended_at IS NULL AND NOT EXISTS (
			SELECT 1 FROM analytics_recipe_ingredients r
			WHERE r.dish_id = i.dish_id AND r.product_id = i.product_id
			  AND `+short+`
		)`, dishID, at, pointID)
	if err != nil {
		return err
	}
	// Интервал, который открылся и закрылся в этот же момент, имеет тот же первичный ключ, что и
	// новый: без удаления повторное открытие с тем же временем события падало на ключе, и проекция
	// останавливалась навсегда. Удаляются только интервалы продуктов, которых снова не хватает.
	_, err = tx.Exec(ctx, `DELETE FROM analytics_stop_intervals i
		WHERE i.point_id = $3 AND i.dish_id = $1 AND i.started_at = $2 AND i.ended_at = $2 AND EXISTS (
			SELECT 1 FROM analytics_recipe_ingredients r
			WHERE r.dish_id = i.dish_id AND r.product_id = i.product_id
			  AND `+short+`
		)`, dishID, at, pointID)
	if err != nil {
		return err
	}
	// Открываем отдельный интервал для каждого недостающего ингредиента.
	_, err = tx.Exec(ctx, `INSERT INTO analytics_stop_intervals (point_id, dish_id, product_id, started_at)
		SELECT $3, r.dish_id, r.product_id, $2
		FROM analytics_recipe_ingredients r
		WHERE r.dish_id = $1
		  AND `+short+`
		ON CONFLICT (point_id, dish_id, product_id) WHERE ended_at IS NULL DO NOTHING`, dishID, at, pointID)
	return err
}
