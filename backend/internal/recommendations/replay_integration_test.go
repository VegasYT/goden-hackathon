//go:build integration

package recommendations_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/command"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/postgres/pgtest"
	"goden/backend/internal/recommendations"
)

func TestRebuildKnowsEveryEventOfTheModule(t *testing.T) {
	var own int
	for _, info := range eventstore.RegisteredTypes() {
		payload, err := eventstore.Decode(info.Type, info.SchemaVersion, []byte(`{}`))
		if err != nil || !recommendations.OwnsEvent(payload) {
			continue
		}
		own++
		require.Contains(t, recommendations.Replayers, info.Type, "a new event of the module must be added to the rebuild")
	}
	require.Equal(t, len(recommendations.Replayers), own)
}

func TestHotOffersReplayFromEvents(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	clk := clock.NewFake(time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC))
	svc := recommendations.NewService(command.NewRunner(pool, clk, slog.New(slog.DiscardHandler)), pool, nil, nil)
	manager := eventstore.Actor{Type: eventstore.ActorUser, ID: "manager"}

	a, b := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	pasta, soup, salad := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	for _, change := range []struct {
		point, dish uuid.UUID
		mode        recommendations.HotOfferMode
		comment     string
	}{
		{a, pasta, recommendations.HotOfferPinned, "к вину"},
		{a, soup, recommendations.HotOfferHidden, ""},
		{b, pasta, recommendations.HotOfferHidden, ""},
		{a, salad, recommendations.HotOfferPinned, ""},
		{a, pasta, recommendations.HotOfferPinned, "к белому вину"},
		{a, salad, recommendations.HotOfferAuto, ""},
	} {
		clk.Advance(time.Minute)
		_, err := svc.SetHotOffer(ctx, manager, "", change.point, change.dish, change.mode, change.comment, "Europe/Moscow")
		require.NoError(t, err)
	}

	snapshot := func(q interface {
		QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	},
	) string {
		var s string
		require.NoError(t, q.QueryRow(ctx, `SELECT coalesce(json_agg(row_to_json(t) ORDER BY point_id, dish_id)::text, '[]')
			FROM recommendations_hot_offers t`).Scan(&s))
		return s
	}
	live := snapshot(pool)
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	require.NoError(t, err)
	defer tx.Rollback(ctx) //nolint:errcheck
	require.NoError(t, recommendations.Rebuild(ctx, tx))
	require.JSONEq(t, live, snapshot(tx))

	settings, err := svc.HotOfferSettings(ctx, a)
	require.NoError(t, err)
	require.Len(t, settings, 2, "auto removes the setting")
	require.Equal(t, soup, settings[0].DishID)
	require.Equal(t, "к белому вину", settings[1].Comment)
	require.Equal(t, manager, settings[1].SetBy)
}

func TestRepeatedHotOfferWritesNoEvent(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	clk := clock.NewFake(time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC))
	svc := recommendations.NewService(command.NewRunner(pool, clk, slog.New(slog.DiscardHandler)), pool, nil, nil)
	manager := eventstore.Actor{Type: eventstore.ActorUser, ID: "manager"}
	point, dish := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())

	for _, mode := range []recommendations.HotOfferMode{
		recommendations.HotOfferAuto, recommendations.HotOfferHidden, recommendations.HotOfferHidden,
	} {
		_, err := svc.SetHotOffer(ctx, manager, "", point, dish, mode, "", "UTC")
		require.NoError(t, err)
	}
	version, err := eventstore.StreamVersion(ctx, pool, recommendations.HotOffersStream(point))
	require.NoError(t, err)
	require.Equal(t, int64(1), version, "auto without a setting and a repeated hidden write nothing")
}
