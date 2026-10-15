//go:build integration

package demo_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"goden/backend/internal/demo"
	"goden/backend/internal/generator"
	"goden/backend/internal/organization"
	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/command"
	"goden/backend/internal/platform/postgres/pgtest"
)

func TestSeedCreatesTheDemoCompanyOnce(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	clk := clock.NewFake(time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC))
	org := organization.NewService(command.NewRunner(pool, clk, slog.New(slog.DiscardHandler)), pool)
	gen := generator.NewService(pool, clk)
	events := func() (n int) {
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM events`).Scan(&n))
		return n
	}
	require.Zero(t, events(), "nothing exists until the demo company is requested")

	const chat = int64(-1001234567890)
	require.NoError(t, demo.Seed(ctx, org, gen, chat))

	// владелец видит компанию и обе точки
	owner, err := org.Profile(ctx, demo.Owner.ID)
	require.NoError(t, err)
	require.Len(t, owner.Companies, 1)
	require.Equal(t, demo.CompanyName, owner.Companies[0].Name)
	require.Len(t, owner.Points, 2)
	var first, second organization.Point
	for _, m := range owner.Points {
		require.True(t, m.Owner)
		if m.Point.Name == demo.FirstPoint {
			first = m.Point
		} else {
			second = m.Point
		}
	}
	require.Equal(t, demo.SecondPoint, second.Name)

	// каждый тестовый пользователь - сотрудник первой точки в своей роли
	for _, member := range demo.Staff {
		profile, err := org.Profile(ctx, member.User.ID)
		require.NoError(t, err)
		require.Empty(t, profile.Companies, member.User.Name)
		require.Len(t, profile.Points, 1, member.User.Name)
		require.Equal(t, first.ID, profile.Points[0].Point.ID)
		require.Equal(t, member.Role, profile.Points[0].Role)
	}

	// чат и генератор - только на первой точке
	point, err := org.Point(ctx, first.ID)
	require.NoError(t, err)
	require.EqualValues(t, chat, *point.TelegramChatID)
	running, err := gen.RunningPoints(ctx)
	require.NoError(t, err)
	require.Len(t, running, 1)
	require.Equal(t, first.ID, running[0].PointID)
	require.Equal(t, owner.Companies[0].ID, running[0].CompanyID)

	// владелец остановил генератор и снял сотрудника: повторный запуск этого не отменяет
	stopped, err := gen.Settings(ctx, first.ID)
	require.NoError(t, err)
	stopped.Running = false
	_, err = gen.Update(ctx, first, stopped)
	require.NoError(t, err)
	require.NoError(t, org.Remove(ctx, demo.Actor, "", demo.Owner.ID, first.ID, demo.Staff[3].User.ID))
	before := events()

	clk.Advance(time.Hour)
	require.NoError(t, demo.Seed(ctx, org, gen, chat))
	require.Equal(t, before, events(), "the second run adds nothing")
	running, err = gen.RunningPoints(ctx)
	require.NoError(t, err)
	require.Empty(t, running)
	waiter, err := org.Profile(ctx, demo.Staff[3].User.ID)
	require.NoError(t, err)
	require.Empty(t, waiter.Points)
}

func TestSeedWithoutAChat(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	clk := clock.NewFake(time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC))
	org := organization.NewService(command.NewRunner(pool, clk, slog.New(slog.DiscardHandler)), pool)

	require.NoError(t, demo.Seed(ctx, org, generator.NewService(pool, clk), 0))
	owner, err := org.Profile(ctx, demo.Owner.ID)
	require.NoError(t, err)
	for _, m := range owner.Points {
		point, err := org.Point(ctx, m.Point.ID)
		require.NoError(t, err)
		require.Nil(t, point.TelegramChatID)
	}
}

// Чат из настройки окружения задаётся только точке без чата: чат, который владелец задал
// в приложении, повторный запуск не перезаписывает, а другой чат в настройке не ломает запуск.
func TestSeedKeepsTheChatOfTheOwner(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	clk := clock.NewFake(time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC))
	org := organization.NewService(command.NewRunner(pool, clk, slog.New(slog.DiscardHandler)), pool)
	gen := generator.NewService(pool, clk)
	firstPoint := func() organization.Point {
		owner, err := org.Profile(ctx, demo.Owner.ID)
		require.NoError(t, err)
		for _, m := range owner.Points {
			if m.Point.Name == demo.FirstPoint {
				point, err := org.Point(ctx, m.Point.ID)
				require.NoError(t, err)
				return point
			}
		}
		t.Fatal("the first point is missing")
		return organization.Point{}
	}

	require.NoError(t, demo.Seed(ctx, org, gen, -1001))
	require.EqualValues(t, -1001, *firstPoint().TelegramChatID)

	// владелец задал свой чат: настройка окружения с другим чатом его не меняет
	own := int64(-2002)
	_, err := org.SetTelegramChat(ctx, demo.Actor, "", firstPoint().ID, &own)
	require.NoError(t, err)
	require.NoError(t, demo.Seed(ctx, org, gen, -3003))
	require.EqualValues(t, own, *firstPoint().TelegramChatID)

	// владелец снял чат: прежняя настройка его не возвращает
	_, err = org.SetTelegramChat(ctx, demo.Actor, "", firstPoint().ID, nil)
	require.NoError(t, err)
	require.NoError(t, demo.Seed(ctx, org, gen, -1001))
	require.Nil(t, firstPoint().TelegramChatID)
}

// Тестовый пользователь уже входил: его имя из Keycloak повторный запуск не заменяет логином.
func TestSeedKeepsKnownUsers(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewDB(t)
	clk := clock.NewFake(time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC))
	org := organization.NewService(command.NewRunner(pool, clk, slog.New(slog.DiscardHandler)), pool)
	gen := generator.NewService(pool, clk)
	require.NoError(t, demo.Seed(ctx, org, gen, 0))

	full := demo.Owner
	full.Name = "Администратор Тестовый"
	require.NoError(t, org.Remember(ctx, full))
	require.NoError(t, demo.Seed(ctx, org, gen, 0))

	var name string
	require.NoError(t, pool.QueryRow(ctx, `SELECT name FROM organization_users WHERE id = $1`, demo.Owner.ID).Scan(&name))
	require.Equal(t, full.Name, name)
}
