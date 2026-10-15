package organization

import (
	"context"
	"fmt"
	"reflect"

	"github.com/jackc/pgx/v5"

	"goden/backend/internal/organization/db"
	"goden/backend/internal/platform/eventstore"
)

// Тестовая опора: перестроение таблиц организации из Event Store. В сборку процессов не попадает.

// Appliers - что делает с таблицами организации каждое событие модуля.
var Appliers = appliers

// OwnsEvent сообщает, объявлено ли событие в этом модуле.
func OwnsEvent(p eventstore.Payload) bool {
	return reflect.TypeOf(p).PkgPath() == reflect.TypeOf(CompanyCreated{}).PkgPath()
}

// Rebuild очищает таблицы организации и собирает их заново из всех событий в общем порядке
// чтения, теми же функциями, которыми их меняют команды. Пользователи - не события и остаются.
func Rebuild(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `DELETE FROM organization_assignments; DELETE FROM organization_points; DELETE FROM organization_companies`)
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
			apply, known := appliers[e.Type]
			if !known {
				return fmt.Errorf("organization: rebuild does not know event %s", e.Type)
			}
			if err := apply(ctx, q, p); err != nil {
				return fmt.Errorf("organization: replay %s at position %d: %w", e.Type, e.Position, err)
			}
		}
	}
}
