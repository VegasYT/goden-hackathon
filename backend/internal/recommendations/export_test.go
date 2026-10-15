package recommendations

import (
	"context"
	"fmt"
	"reflect"

	"github.com/jackc/pgx/v5"

	"goden/backend/internal/platform/command"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/recommendations/db"
)

// Тестовая опора: перестроение таблицы ручных горячих предложений из Event Store.

// Replayers - что делает с таблицей каждое событие модуля. Событие модуля, которого здесь нет,
// перестроение считает ошибкой.
var Replayers = map[string]func(ctx context.Context, q *db.Queries, e eventstore.Event, p eventstore.Payload) error{
	"HotOfferChanged": func(ctx context.Context, q *db.Queries, e eventstore.Event, p eventstore.Payload) error {
		actor, err := command.ActorOf(e)
		if err != nil {
			return err
		}
		return applyHotOfferChanged(ctx, q, p.(HotOfferChanged), actor, e.OccurredAt)
	},
}

// OwnsEvent сообщает, объявлено ли событие в этом модуле.
func OwnsEvent(p eventstore.Payload) bool {
	return reflect.TypeOf(p).PkgPath() == reflect.TypeOf(HotOfferChanged{}).PkgPath()
}

// Rebuild очищает таблицу и собирает её заново из всех событий в общем порядке чтения.
func Rebuild(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `DELETE FROM recommendations_hot_offers`); err != nil {
		return err
	}
	q := db.New(tx)
	var after eventstore.Checkpoint
	for {
		events, err := eventstore.ReadAll(ctx, tx, after, 500)
		if err != nil || len(events) == 0 {
			return err
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
				return fmt.Errorf("recommendations: rebuild does not know event %s", e.Type)
			}
			if err := replay(ctx, q, e, p); err != nil {
				return fmt.Errorf("recommendations: replay %s at position %d: %w", e.Type, e.Position, err)
			}
		}
	}
}
