//go:build integration

package catalog_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"goden/backend/internal/catalog"
	"goden/backend/internal/platform/eventstore"
)

// catalogSnapshot возвращает строки всех таблиц каталога как JSON в порядке ключей.
func catalogSnapshot(t *testing.T, f fixture, q eventstore.Querier) []string {
	t.Helper()
	tables := map[string]string{
		"catalog_products":           "t.id",
		"catalog_dishes":             "t.id",
		"catalog_recipe_ingredients": "t.dish_id, t.product_id",
		"catalog_disabled_dishes":    "t.point_id, t.dish_id",
	}
	var snapshots []string
	for _, table := range []string{"catalog_products", "catalog_dishes", "catalog_recipe_ingredients", "catalog_disabled_dishes"} {
		rows, err := q.Query(f.ctx, `SELECT coalesce(json_agg(row_to_json(t) ORDER BY `+tables[table]+`)::text, '[]') FROM `+table+` t`)
		require.NoError(t, err)
		require.True(t, rows.Next())
		var snapshot string
		require.NoError(t, rows.Scan(&snapshot))
		rows.Close()
		snapshots = append(snapshots, snapshot)
	}
	return snapshots
}

func TestRebuildKnowsEveryEventOfTheModule(t *testing.T) {
	var own int
	for _, info := range eventstore.RegisteredTypes() {
		payload, err := eventstore.Decode(info.Type, info.SchemaVersion, []byte(`{}`))
		if err != nil || !catalog.OwnsEvent(payload) {
			continue
		}
		own++
		require.Contains(t, catalog.Replayers, info.Type, "a new event of the module must be added to the rebuild")
	}
	require.Equal(t, len(catalog.Replayers), own)
}

func TestCatalogTablesReplayFromEvents(t *testing.T) {
	f := setup(t)
	cream, bacon, eggs := f.product(t, "Сливки", catalog.UnitMilliliter), f.product(t, "Бекон", catalog.UnitGram), f.product(t, "Яйцо", catalog.UnitPiece)
	pasta, omelette := f.dish(t, "Паста"), f.dish(t, "Омлет")
	f.dish(t, "Черновик без рецептуры")

	// рецептура одного блюда меняется несколько раз: состав растёт, сокращается, меняются количества
	for _, ingredients := range [][]catalog.Ingredient{
		{{ProductID: cream.ID, Quantity: 100}},
		{{ProductID: cream.ID, Quantity: 80}, {ProductID: bacon.ID, Quantity: 60}, {ProductID: eggs.ID, Quantity: 1}},
		{{ProductID: bacon.ID, Quantity: 50}, {ProductID: cream.ID, Quantity: 90}},
	} {
		_, err := f.svc.ChangeRecipe(f.ctx, manager, "", pasta.ID, ingredients)
		require.NoError(t, err)
	}
	_, err := f.svc.ChangeRecipe(f.ctx, manager, "", omelette.ID, []catalog.Ingredient{{ProductID: eggs.ID, Quantity: 3}})
	require.NoError(t, err)

	// блюда выключают и включают на точках: в таблице остаются только выключенные
	a, b := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	for _, change := range []struct {
		point, dish uuid.UUID
		enabled     bool
	}{{a, pasta.ID, false}, {a, omelette.ID, false}, {b, pasta.ID, false}, {a, omelette.ID, true}} {
		_, err := f.svc.SetDishOnPoint(f.ctx, manager, "", companyID, change.point, change.dish, change.enabled)
		require.NoError(t, err)
	}

	// продукт меняют, отправляют в архив и восстанавливают; блюда переименовывают и архивируют
	name, shelfLife := "Сливки 33%", 7
	_, err = f.svc.ChangeProduct(f.ctx, manager, "", cream.ID, catalog.ProductChange{Name: &name, ShelfLifeDays: &shelfLife})
	require.NoError(t, err)
	spare := f.product(t, "Запасной продукт", catalog.UnitGram)
	steps := []func() error{
		func() error { _, err := f.svc.ArchiveProduct(f.ctx, manager, "", spare.ID); return err },
		func() error { _, err := f.svc.RestoreProduct(f.ctx, manager, "", spare.ID); return err },
		func() error { _, err := f.svc.ArchiveProduct(f.ctx, manager, "", spare.ID); return err },
		func() error {
			_, err := f.svc.RenameDish(f.ctx, manager, "", omelette.ID, "Омлет с молоком")
			return err
		},
		func() error { _, err := f.svc.ArchiveDish(f.ctx, manager, "", pasta.ID); return err },
		func() error { _, err := f.svc.RestoreDish(f.ctx, manager, "", pasta.ID); return err },
		func() error { _, err := f.svc.ArchiveDish(f.ctx, manager, "", omelette.ID); return err },
	}
	for _, step := range steps {
		require.NoError(t, step())
	}

	live := catalogSnapshot(t, f, f.pool)
	tx, err := f.pool.BeginTx(f.ctx, pgx.TxOptions{})
	require.NoError(t, err)
	defer tx.Rollback(f.ctx) //nolint:errcheck
	require.NoError(t, catalog.Rebuild(f.ctx, tx))
	rebuilt := catalogSnapshot(t, f, tx)

	require.Len(t, live, 4)
	for i := range live {
		require.NotEqual(t, "[]", live[i])
		require.JSONEq(t, live[i], rebuilt[i], "catalog tables are a function of the events")
	}
}

// Рецептуру одного блюда меняют одновременно, каждый запрос со своим ключом идемпотентности.
// С ключом транзакция получает номер до чтения версии потока: без блокировки потока блюда
// более поздняя версия могла оказаться в общем порядке чтения раньше, и перестроение
// каталога давало не ту рецептуру.
func TestConcurrentRecipeChangesKeepStreamOrder(t *testing.T) {
	f := setup(t)
	cream := f.product(t, "Сливки", catalog.UnitMilliliter)
	pasta := f.dish(t, "Паста")

	const changes = 12
	var g errgroup.Group
	for i := range changes {
		g.Go(func() error {
			_, err := f.svc.ChangeRecipe(f.ctx, manager, fmt.Sprintf("recipe-%d", i), pasta.ID,
				[]catalog.Ingredient{{ProductID: cream.ID, Quantity: int64(10 + i)}})
			return err
		})
	}
	require.NoError(t, g.Wait())

	var versions []int64
	require.Eventually(t, func() bool {
		all, err := eventstore.ReadAll(f.ctx, f.pool, eventstore.Checkpoint{}, 1000)
		require.NoError(t, err)
		versions = versions[:0]
		for _, event := range all {
			if event.StreamID == catalog.DishStream(pasta.ID) {
				versions = append(versions, event.Version)
			}
		}
		return len(versions) == changes+1
	}, 20*time.Second, 20*time.Millisecond)
	require.IsIncreasing(t, versions, "events of one dish are read in the order of stream versions")

	live := catalogSnapshot(t, f, f.pool)
	tx, err := f.pool.BeginTx(f.ctx, pgx.TxOptions{})
	require.NoError(t, err)
	defer tx.Rollback(f.ctx) //nolint:errcheck
	require.NoError(t, catalog.Rebuild(f.ctx, tx))
	require.Equal(t, live, catalogSnapshot(t, f, tx), "the rebuilt recipe matches the current one")
}
