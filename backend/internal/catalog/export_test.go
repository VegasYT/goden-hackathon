package catalog

import (
	"context"
	"fmt"
	"reflect"

	"github.com/jackc/pgx/v5"

	"goden/backend/internal/catalog/db"
	"goden/backend/internal/platform/eventstore"
)

// Тестовая опора: перестроение таблиц каталога из Event Store. В сборку процессов не попадает.

// Replayers - что делает с таблицами каталога каждое событие модуля. Событие модуля,
// которого здесь нет, перестроение считает ошибкой.
var Replayers = map[string]func(ctx context.Context, q *db.Queries, p eventstore.Payload) error{
	"ProductCreated": func(ctx context.Context, q *db.Queries, p eventstore.Payload) error {
		return applyProductCreated(ctx, q, p.(ProductCreated))
	},
	"DishCreated": func(ctx context.Context, q *db.Queries, p eventstore.Payload) error {
		return applyDishCreated(ctx, q, p.(DishCreated))
	},
	"RecipeChanged": func(ctx context.Context, q *db.Queries, p eventstore.Payload) error {
		return applyRecipeChanged(ctx, q, p.(RecipeChanged))
	},
	"DishAvailabilityChanged": func(ctx context.Context, q *db.Queries, p eventstore.Payload) error {
		return applyDishAvailabilityChanged(ctx, q, p.(DishAvailabilityChanged))
	},
	"ProductChanged": func(ctx context.Context, q *db.Queries, p eventstore.Payload) error {
		return applyProductChanged(ctx, q, p.(ProductChanged))
	},
	"ProductArchived": func(ctx context.Context, q *db.Queries, p eventstore.Payload) error {
		return applyProductArchived(ctx, q, p.(ProductArchived).ProductID, true)
	},
	"ProductRestored": func(ctx context.Context, q *db.Queries, p eventstore.Payload) error {
		return applyProductArchived(ctx, q, p.(ProductRestored).ProductID, false)
	},
	"DishRenamed": func(ctx context.Context, q *db.Queries, p eventstore.Payload) error {
		return applyDishRenamed(ctx, q, p.(DishRenamed))
	},
	"DishArchived": func(ctx context.Context, q *db.Queries, p eventstore.Payload) error {
		return applyDishArchived(ctx, q, p.(DishArchived).DishID, true)
	},
	"DishRestored": func(ctx context.Context, q *db.Queries, p eventstore.Payload) error {
		return applyDishArchived(ctx, q, p.(DishRestored).DishID, false)
	},
}

// OwnsEvent сообщает, объявлено ли событие в этом модуле.
func OwnsEvent(p eventstore.Payload) bool {
	return reflect.TypeOf(p).PkgPath() == reflect.TypeOf(ProductCreated{}).PkgPath()
}

// Rebuild очищает таблицы каталога и собирает их заново из всех событий в общем порядке чтения,
// теми же функциями apply*, которыми их меняют команды.
func Rebuild(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `DELETE FROM catalog_disabled_dishes; DELETE FROM catalog_recipe_ingredients; DELETE FROM catalog_dishes; DELETE FROM catalog_products`)
	if err != nil {
		return err
	}
	q := db.New(tx)
	var after eventstore.Checkpoint
	for {
		events, err := eventstore.ReadAll(ctx, tx, after, 500)
		if err != nil {
			return err
		}
		if len(events) == 0 {
			return nil
		}
		for _, e := range events {
			after = e.Checkpoint()
			p, err := eventstore.Decode(e.Type, e.SchemaVersion, e.Payload)
			if err != nil {
				return err
			}
			if !OwnsEvent(p) {
				continue
			}
			replay, known := Replayers[e.Type]
			if !known {
				return fmt.Errorf("catalog: rebuild does not know event %s", e.Type)
			}
			if err := replay(ctx, q, p); err != nil {
				return fmt.Errorf("catalog: replay %s at position %d: %w", e.Type, e.Position, err)
			}
		}
	}
}
