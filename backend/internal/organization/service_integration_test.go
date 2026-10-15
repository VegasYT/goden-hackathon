//go:build integration

package organization_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"goden/backend/internal/organization"
	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/command"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/postgres/pgtest"
)

var actor = eventstore.Actor{Type: eventstore.ActorUser, ID: "tester"}

type fixture struct {
	ctx  context.Context
	pool *pgxpool.Pool
	svc  *organization.Service
}

func setup(t *testing.T) fixture {
	pool := pgtest.NewDB(t)
	clk := clock.NewFake(time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC))
	runner := command.NewRunner(pool, clk, slog.New(slog.DiscardHandler))
	return fixture{context.Background(), pool, organization.NewService(runner, pool)}
}

// user записывает пользователя так же, как это делает его первое обращение к системе.
func (f fixture) user(t *testing.T, name string) organization.User {
	t.Helper()
	u := organization.User{ID: uuid.NewString(), Email: name + "@goden.local", Name: name}
	require.NoError(t, f.svc.Remember(f.ctx, u))
	return u
}

func (f fixture) company(t *testing.T, owner organization.User, name string) organization.Company {
	t.Helper()
	c, err := f.svc.CreateCompany(f.ctx, actor, "", owner.ID, name)
	require.NoError(t, err)
	return c
}

func (f fixture) point(t *testing.T, company organization.Company, name string) organization.Point {
	t.Helper()
	p, err := f.svc.CreatePoint(f.ctx, actor, "", company.ID, name)
	require.NoError(t, err)
	return p
}

func (f fixture) assign(t *testing.T, by organization.User, point organization.Point, who organization.User, role organization.Role) {
	t.Helper()
	_, err := f.svc.Assign(f.ctx, actor, "", by.ID, point.ID, who.Email, role)
	require.NoError(t, err)
}

func (f fixture) pointPermissions(t *testing.T, user organization.User, point organization.Point) []string {
	t.Helper()
	permissions, err := f.svc.PointPermissions(f.ctx, user.ID, point.ID)
	require.NoError(t, err)
	return permissions
}

func TestCompaniesAndPoints(t *testing.T) {
	f := setup(t)
	owner := f.user(t, "owner")

	pizza := f.company(t, owner, "  Пиццерии ")
	require.Equal(t, organization.Company{ID: pizza.ID, Name: "Пиццерии", OwnerID: owner.ID, Active: true}, pizza)
	sushi := f.company(t, owner, "Суши")

	_, err := f.svc.CreateCompany(f.ctx, actor, "", owner.ID, "   ")
	var validation *organization.ValidationError
	require.ErrorAs(t, err, &validation)

	lenina := f.point(t, pizza, "Точка на Ленина")
	_, err = f.svc.CreatePoint(f.ctx, actor, "", pizza.ID, "точка НА ленина")
	require.ErrorIs(t, err, organization.ErrNameTaken, "the name is unique within the company, case-insensitively")
	f.point(t, sushi, "Точка на Ленина")

	mira := f.point(t, pizza, "Точка на Мира")
	_, err = f.svc.RenamePoint(f.ctx, actor, "", mira.ID, "Точка на Ленина")
	require.ErrorIs(t, err, organization.ErrNameTaken)
	renamed, err := f.svc.RenamePoint(f.ctx, actor, "", mira.ID, "Точка у вокзала")
	require.NoError(t, err)
	require.Equal(t, "Точка у вокзала", renamed.Name)

	points, err := f.svc.Points(f.ctx, pizza.ID)
	require.NoError(t, err)
	require.Equal(t, []organization.Point{lenina, renamed}, points)

	company, err := f.svc.RenameCompany(f.ctx, actor, "", pizza.ID, "Пиццерии Севера")
	require.NoError(t, err)
	require.Equal(t, "Пиццерии Севера", company.Name)

	unknown := uuid.Must(uuid.NewV7())
	_, err = f.svc.CreatePoint(f.ctx, actor, "", unknown, "Точка")
	require.ErrorIs(t, err, organization.ErrNotFound)
	_, err = f.svc.Company(f.ctx, unknown)
	require.ErrorIs(t, err, organization.ErrNotFound)
	_, err = f.svc.RenamePoint(f.ctx, actor, "", unknown, "Точка")
	require.ErrorIs(t, err, organization.ErrNotFound)
}

func TestIdempotentCommands(t *testing.T) {
	f := setup(t)
	owner := f.user(t, "owner")

	first, err := f.svc.CreateCompany(f.ctx, actor, "key-1", owner.ID, "Пиццерии")
	require.NoError(t, err)
	second, err := f.svc.CreateCompany(f.ctx, actor, "key-1", owner.ID, "Пиццерии")
	require.NoError(t, err)
	require.Equal(t, first, second)

	profile, err := f.svc.Profile(f.ctx, owner.ID)
	require.NoError(t, err)
	require.Len(t, profile.Companies, 1)

	_, err = f.svc.CreateCompany(f.ctx, actor, "key-1", owner.ID, "Суши")
	require.ErrorIs(t, err, command.ErrKeyReused)
}

func TestStaffAssignments(t *testing.T) {
	f := setup(t)
	owner, manager, cook, stranger := f.user(t, "owner"), f.user(t, "manager"), f.user(t, "cook"), f.user(t, "stranger")
	company := f.company(t, owner, "Пиццерии")
	a, b := f.point(t, company, "А"), f.point(t, company, "Б")

	// владелец назначает управляющего, почта - без учёта регистра и пробелов
	staff, err := f.svc.Assign(f.ctx, actor, "", owner.ID, a.ID, "  Manager@Goden.Local ", organization.RoleManager)
	require.NoError(t, err)
	require.Equal(t, organization.Staff{UserID: manager.ID, Email: manager.Email, Name: manager.Name, Role: organization.RoleManager}, staff)

	_, err = f.svc.Assign(f.ctx, actor, "", owner.ID, a.ID, manager.Email, organization.RoleCook)
	require.ErrorIs(t, err, organization.ErrAlreadyAssigned, "the role is changed by a separate command")
	_, err = f.svc.Assign(f.ctx, actor, "", owner.ID, a.ID, "nobody@goden.local", organization.RoleCook)
	require.ErrorIs(t, err, organization.ErrUnknownUser)
	_, err = f.svc.Assign(f.ctx, actor, "", owner.ID, a.ID, cook.Email, "admin")
	var validation *organization.ValidationError
	require.ErrorAs(t, err, &validation)

	// управляющий распоряжается на своей точке всеми ролями, кроме управляющего
	f.assign(t, manager, a, cook, organization.RoleCook)
	_, err = f.svc.Assign(f.ctx, actor, "", manager.ID, a.ID, stranger.Email, organization.RoleManager)
	require.ErrorIs(t, err, organization.ErrForbidden)
	_, err = f.svc.ChangeRole(f.ctx, actor, "", manager.ID, a.ID, cook.ID, organization.RoleManager)
	require.ErrorIs(t, err, organization.ErrForbidden)
	require.ErrorIs(t, f.svc.Remove(f.ctx, actor, "", manager.ID, a.ID, manager.ID), organization.ErrForbidden)
	_, err = f.svc.Assign(f.ctx, actor, "", manager.ID, b.ID, stranger.Email, organization.RoleWaiter)
	require.ErrorIs(t, err, organization.ErrForbidden, "a manager of one point has no say on another")
	_, err = f.svc.Assign(f.ctx, actor, "", cook.ID, a.ID, stranger.Email, organization.RoleWaiter)
	require.ErrorIs(t, err, organization.ErrForbidden)

	changed, err := f.svc.ChangeRole(f.ctx, actor, "", manager.ID, a.ID, cook.ID, organization.RoleWaiter)
	require.NoError(t, err)
	require.Equal(t, organization.RoleWaiter, changed.Role)
	require.Equal(t, cook.Email, changed.Email)

	// один сотрудник - разные роли на разных точках
	f.assign(t, owner, b, cook, organization.RoleManager)
	list, err := f.svc.Staff(f.ctx, a.ID)
	require.NoError(t, err)
	require.ElementsMatch(t, []organization.Staff{
		{UserID: manager.ID, Email: manager.Email, Name: manager.Name, Role: organization.RoleManager},
		{UserID: cook.ID, Email: cook.Email, Name: cook.Name, Role: organization.RoleWaiter},
	}, list)

	require.NoError(t, f.svc.Remove(f.ctx, actor, "", manager.ID, a.ID, cook.ID))
	require.ErrorIs(t, f.svc.Remove(f.ctx, actor, "", owner.ID, a.ID, cook.ID), organization.ErrNotFound)
	_, err = f.svc.ChangeRole(f.ctx, actor, "", owner.ID, a.ID, cook.ID, organization.RoleCook)
	require.ErrorIs(t, err, organization.ErrNotFound)
	require.NoError(t, f.svc.Remove(f.ctx, actor, "", owner.ID, a.ID, manager.ID))
}

func TestPermissionsFollowOwnershipAndAssignments(t *testing.T) {
	f := setup(t)
	owner, cook, stranger := f.user(t, "owner"), f.user(t, "cook"), f.user(t, "stranger")
	company := f.company(t, owner, "Пиццерии")
	a, b := f.point(t, company, "А"), f.point(t, company, "Б")
	f.assign(t, owner, a, cook, organization.RoleCook)

	require.Contains(t, f.pointPermissions(t, owner, a), organization.PermissionCompanyManage, "the owner needs no assignment")
	require.ElementsMatch(t, []string{"point.view", "staff.view", "kitchen.view", "catalog.view"}, f.pointPermissions(t, cook, a))
	require.Empty(t, f.pointPermissions(t, cook, b), "a neighbouring point of the same company is closed")
	require.Empty(t, f.pointPermissions(t, stranger, a))

	inCompany := func(u organization.User) []string {
		permissions, err := f.svc.CompanyPermissions(f.ctx, u.ID, company.ID)
		require.NoError(t, err)
		return permissions
	}
	require.Contains(t, inCompany(owner), organization.PermissionCompanyManage)
	require.ElementsMatch(t, []string{organization.PermissionCompanyView, "catalog.view"}, inCompany(cook), "staff sees the company and reads its catalog")
	require.Empty(t, inCompany(stranger))

	unknown := uuid.Must(uuid.NewV7())
	_, err := f.svc.PointPermissions(f.ctx, owner.ID, unknown)
	require.ErrorIs(t, err, organization.ErrNotFound)
	_, err = f.svc.CompanyPermissions(f.ctx, owner.ID, unknown)
	require.ErrorIs(t, err, organization.ErrNotFound)

	// снятое назначение перестаёт действовать сразу
	require.NoError(t, f.svc.Remove(f.ctx, actor, "", owner.ID, a.ID, cook.ID))
	require.Empty(t, f.pointPermissions(t, cook, a))
	require.Empty(t, inCompany(cook))
}

func TestDisabledPointAndCompany(t *testing.T) {
	f := setup(t)
	owner, cook, waiter := f.user(t, "owner"), f.user(t, "cook"), f.user(t, "waiter")
	company := f.company(t, owner, "Пиццерии")
	a, b := f.point(t, company, "А"), f.point(t, company, "Б")
	f.assign(t, owner, a, cook, organization.RoleCook)
	f.assign(t, owner, b, cook, organization.RoleCook)

	disabled, err := f.svc.DisablePoint(f.ctx, actor, "", a.ID)
	require.NoError(t, err)
	require.False(t, disabled.Active)

	// отключённая точка не меняется, но читается, и владелец сохраняет права
	_, err = f.svc.Assign(f.ctx, actor, "", owner.ID, a.ID, waiter.Email, organization.RoleWaiter)
	require.ErrorIs(t, err, organization.ErrDisabled)
	_, err = f.svc.RenamePoint(f.ctx, actor, "", a.ID, "В")
	require.ErrorIs(t, err, organization.ErrDisabled)
	_, err = f.svc.DisablePoint(f.ctx, actor, "", a.ID)
	require.ErrorIs(t, err, organization.ErrDisabled)
	staff, err := f.svc.Staff(f.ctx, a.ID)
	require.NoError(t, err)
	require.Len(t, staff, 1)
	require.NotEmpty(t, f.pointPermissions(t, owner, a))
	require.Empty(t, f.pointPermissions(t, cook, a), "staff loses the permissions on a disabled point")
	require.NotEmpty(t, f.pointPermissions(t, cook, b))

	profile, err := f.svc.Profile(f.ctx, cook.ID)
	require.NoError(t, err)
	require.Len(t, profile.Points, 1, "staff does not see a disabled point")
	require.Equal(t, b.ID, profile.Points[0].Point.ID)

	_, err = f.svc.DisableCompany(f.ctx, actor, "", company.ID)
	require.NoError(t, err)
	_, err = f.svc.CreatePoint(f.ctx, actor, "", company.ID, "Г")
	require.ErrorIs(t, err, organization.ErrDisabled)
	_, err = f.svc.RenameCompany(f.ctx, actor, "", company.ID, "Другое")
	require.ErrorIs(t, err, organization.ErrDisabled)
	_, err = f.svc.Assign(f.ctx, actor, "", owner.ID, b.ID, waiter.Email, organization.RoleWaiter)
	require.ErrorIs(t, err, organization.ErrDisabled)
	require.Empty(t, f.pointPermissions(t, cook, b))

	profile, err = f.svc.Profile(f.ctx, owner.ID)
	require.NoError(t, err)
	require.Len(t, profile.Points, 2, "the owner still sees the points of a disabled company")
	require.False(t, profile.Companies[0].Active)
}

func TestProfile(t *testing.T) {
	f := setup(t)
	owner, other, newcomer := f.user(t, "owner"), f.user(t, "other"), f.user(t, "newcomer")

	profile, err := f.svc.Profile(f.ctx, newcomer.ID)
	require.NoError(t, err)
	require.Empty(t, profile.Companies)
	require.Empty(t, profile.Points)

	own := f.company(t, owner, "Пиццерии")
	a, b := f.point(t, own, "А"), f.point(t, own, "Б")
	foreign := f.company(t, other, "Суши")
	c := f.point(t, foreign, "В")
	f.assign(t, other, c, owner, organization.RoleCook)
	// владелец, назначенный на свою точку, остаётся владельцем
	f.assign(t, owner, a, owner, organization.RoleWaiter)

	profile, err = f.svc.Profile(f.ctx, owner.ID)
	require.NoError(t, err)
	require.Equal(t, []organization.Company{own}, profile.Companies)
	require.Len(t, profile.Points, 3)
	byID := map[uuid.UUID]organization.Membership{}
	for _, m := range profile.Points {
		byID[m.Point.ID] = m
	}
	for _, point := range []organization.Point{a, b} {
		require.True(t, byID[point.ID].Owner)
		require.Empty(t, byID[point.ID].Role)
		require.Equal(t, "Пиццерии", byID[point.ID].CompanyName)
		require.Contains(t, byID[point.ID].Permissions, organization.PermissionCompanyManage)
	}
	require.False(t, byID[c.ID].Owner)
	require.Equal(t, organization.RoleCook, byID[c.ID].Role)
	require.Equal(t, "Суши", byID[c.ID].CompanyName)
	require.NotContains(t, byID[c.ID].Permissions, organization.PermissionStaffManage)
}

// Почта принадлежит одному пользователю: сменивший её находится по новой, а прежний адрес
// может занять другой человек.
func TestUsersAreFoundByCurrentEmail(t *testing.T) {
	f := setup(t)
	owner := f.user(t, "owner")
	point := f.point(t, f.company(t, owner, "Пиццерии"), "А")

	anna := organization.User{ID: uuid.NewString(), Email: "anna@goden.local", Name: "Анна"}
	require.NoError(t, f.svc.Remember(f.ctx, anna))
	anna.Email, anna.Name = "anna.new@goden.local", "Анна Иванова"
	require.NoError(t, f.svc.Remember(f.ctx, anna))

	_, err := f.svc.Assign(f.ctx, actor, "", owner.ID, point.ID, "anna@goden.local", organization.RoleCook)
	require.ErrorIs(t, err, organization.ErrUnknownUser)
	staff, err := f.svc.Assign(f.ctx, actor, "", owner.ID, point.ID, "anna.new@goden.local", organization.RoleCook)
	require.NoError(t, err)
	require.Equal(t, organization.Staff{UserID: anna.ID, Email: anna.Email, Name: "Анна Иванова", Role: organization.RoleCook}, staff)

	// адрес занял другой пользователь, прежний владелец адреса ещё не обращался с новым
	boris := organization.User{ID: uuid.NewString(), Email: "ANNA.new@goden.local", Name: "Борис"}
	require.NoError(t, f.svc.Remember(f.ctx, boris))
	list, err := f.svc.Staff(f.ctx, point.ID)
	require.NoError(t, err)
	require.Equal(t, []organization.Staff{{UserID: anna.ID, Email: "", Name: "Анна Иванова", Role: organization.RoleCook}}, list)

	// сервисная учётная запись без почты не мешает другой такой же
	require.NoError(t, f.svc.Remember(f.ctx, organization.User{ID: uuid.NewString(), Name: "service-1"}))
	require.NoError(t, f.svc.Remember(f.ctx, organization.User{ID: uuid.NewString(), Name: "service-2"}))
}

// Одного пользователя назначают на точку одновременно: назначение получается одно.
func TestConcurrentAssignmentsDoNotDuplicate(t *testing.T) {
	f := setup(t)
	owner, cook := f.user(t, "owner"), f.user(t, "cook")
	point := f.point(t, f.company(t, owner, "Пиццерии"), "А")

	var g errgroup.Group
	results := make([]error, 8)
	for i := range results {
		g.Go(func() error {
			_, results[i] = f.svc.Assign(f.ctx, actor, "", owner.ID, point.ID, cook.Email, organization.RoleCook)
			return nil
		})
	}
	require.NoError(t, g.Wait())

	var assigned int
	for _, err := range results {
		if err == nil {
			assigned++
			continue
		}
		require.ErrorIs(t, err, organization.ErrAlreadyAssigned)
	}
	require.Equal(t, 1, assigned)
}

func snapshot(t *testing.T, f fixture, q eventstore.Querier) []string {
	t.Helper()
	tables := []struct{ name, order string }{
		{"organization_companies", "t.id"}, {"organization_points", "t.id"}, {"organization_assignments", "t.point_id, t.user_id"},
	}
	var snapshots []string
	for _, table := range tables {
		rows, err := q.Query(f.ctx, `SELECT coalesce(json_agg(row_to_json(t) ORDER BY `+table.order+`)::text, '[]') FROM `+table.name+` t`)
		require.NoError(t, err)
		require.True(t, rows.Next())
		var s string
		require.NoError(t, rows.Scan(&s))
		rows.Close()
		snapshots = append(snapshots, s)
	}
	return snapshots
}

func TestRebuildKnowsEveryEventOfTheModule(t *testing.T) {
	var own int
	for _, info := range eventstore.RegisteredTypes() {
		payload, err := eventstore.Decode(info.Type, info.SchemaVersion, []byte(`{}`))
		if err != nil || !organization.OwnsEvent(payload) {
			continue
		}
		own++
		require.Contains(t, organization.Appliers, info.Type, "a new event of the module must be added to appliers")
	}
	require.Equal(t, len(organization.Appliers), own)
}

func TestOrganizationTablesReplayFromEvents(t *testing.T) {
	f := setup(t)
	owner, manager, cook := f.user(t, "owner"), f.user(t, "manager"), f.user(t, "cook")
	company := f.company(t, owner, "Пиццерии")
	a, b, c := f.point(t, company, "А"), f.point(t, company, "Б"), f.point(t, company, "В")
	f.assign(t, owner, a, manager, organization.RoleManager)
	f.assign(t, manager, a, cook, organization.RoleCook)
	f.assign(t, owner, b, cook, organization.RoleWaiter)
	_, err := f.svc.ChangeRole(f.ctx, actor, "", owner.ID, a.ID, cook.ID, organization.RoleApprover)
	require.NoError(t, err)
	require.NoError(t, f.svc.Remove(f.ctx, actor, "", owner.ID, b.ID, cook.ID))
	_, err = f.svc.RenamePoint(f.ctx, actor, "", b.ID, "Б у вокзала")
	require.NoError(t, err)
	_, err = f.svc.DisablePoint(f.ctx, actor, "", c.ID)
	require.NoError(t, err)
	_, err = f.svc.RenameCompany(f.ctx, actor, "", company.ID, "Пиццерии Севера")
	require.NoError(t, err)
	// чат задан точке А, а точке Б задан и снят
	chat := int64(-1001)
	_, err = f.svc.SetTelegramChat(f.ctx, actor, "", a.ID, &chat)
	require.NoError(t, err)
	_, err = f.svc.SetTelegramChat(f.ctx, actor, "", b.ID, &chat)
	require.NoError(t, err)
	_, err = f.svc.SetTelegramChat(f.ctx, actor, "", b.ID, nil)
	require.NoError(t, err)
	_, err = f.svc.TransferCompany(f.ctx, actor, "", owner.ID, f.company(t, owner, "Переданная").ID, manager.Email)
	require.NoError(t, err)
	closed := f.company(t, owner, "Закрытая")
	_, err = f.svc.DisableCompany(f.ctx, actor, "", closed.ID)
	require.NoError(t, err)

	live := snapshot(t, f, f.pool)
	tx, err := f.pool.BeginTx(f.ctx, pgx.TxOptions{})
	require.NoError(t, err)
	defer tx.Rollback(f.ctx) //nolint:errcheck
	require.NoError(t, organization.Rebuild(f.ctx, tx))
	rebuilt := snapshot(t, f, tx)

	require.Len(t, live, 3)
	for i := range live {
		require.NotEqual(t, "[]", live[i])
		require.JSONEq(t, live[i], rebuilt[i], "organization tables are a function of the events")
	}
}

func TestPointTelegramChat(t *testing.T) {
	f := setup(t)
	owner := f.user(t, "owner")
	point := f.point(t, f.company(t, owner, "Пиццерии"), "А")
	require.Nil(t, point.TelegramChatID)

	chat := int64(-1001234567890)
	updated, err := f.svc.SetTelegramChat(f.ctx, actor, "", point.ID, &chat)
	require.NoError(t, err)
	require.Equal(t, &chat, updated.TelegramChatID)
	read, err := f.svc.Point(f.ctx, point.ID)
	require.NoError(t, err)
	require.Equal(t, updated, read)

	cleared, err := f.svc.SetTelegramChat(f.ctx, actor, "", point.ID, nil)
	require.NoError(t, err)
	require.Nil(t, cleared.TelegramChatID)

	zero := int64(0)
	_, err = f.svc.SetTelegramChat(f.ctx, actor, "", point.ID, &zero)
	var validation *organization.ValidationError
	require.ErrorAs(t, err, &validation)
	require.Equal(t, "chat_id", validation.Field)

	_, err = f.svc.DisablePoint(f.ctx, actor, "", point.ID)
	require.NoError(t, err)
	_, err = f.svc.SetTelegramChat(f.ctx, actor, "", point.ID, &chat)
	require.ErrorIs(t, err, organization.ErrDisabled)
}

func TestCompanyTransfer(t *testing.T) {
	f := setup(t)
	owner, heir, cook := f.user(t, "owner"), f.user(t, "heir"), f.user(t, "cook")
	company := f.company(t, owner, "Пиццерии")
	a, b := f.point(t, company, "А"), f.point(t, company, "Б")
	f.assign(t, owner, a, heir, organization.RoleCook)
	f.assign(t, owner, a, cook, organization.RoleCook)

	_, err := f.svc.TransferCompany(f.ctx, actor, "", heir.ID, company.ID, owner.Email)
	require.ErrorIs(t, err, organization.ErrForbidden, "only the owner transfers")
	_, err = f.svc.TransferCompany(f.ctx, actor, "", owner.ID, company.ID, "nobody@goden.local")
	require.ErrorIs(t, err, organization.ErrUnknownUser)
	_, err = f.svc.TransferCompany(f.ctx, actor, "", owner.ID, company.ID, "  ")
	var validation *organization.ValidationError
	require.ErrorAs(t, err, &validation)

	same, err := f.svc.TransferCompany(f.ctx, actor, "", owner.ID, company.ID, owner.Email)
	require.NoError(t, err)
	require.Equal(t, company, same)
	version, err := eventstore.StreamVersion(f.ctx, f.pool, organization.CompanyStream(company.ID))
	require.NoError(t, err)
	require.EqualValues(t, 1, version, "a transfer to oneself writes nothing")

	moved, err := f.svc.TransferCompany(f.ctx, actor, "", owner.ID, company.ID, " HEIR@goden.local ")
	require.NoError(t, err)
	require.Equal(t, heir.ID, moved.OwnerID)

	// новый владелец получает всё на обеих точках, хотя назначен поваром на одну
	require.Contains(t, f.pointPermissions(t, heir, b), organization.PermissionCompanyManage)
	require.Contains(t, f.pointPermissions(t, heir, a), organization.PermissionCompanyManage)
	require.Empty(t, f.pointPermissions(t, owner, a), "the former owner keeps nothing")
	permissions, err := f.svc.CompanyPermissions(f.ctx, owner.ID, company.ID)
	require.NoError(t, err)
	require.Empty(t, permissions)
	profile, err := f.svc.Profile(f.ctx, owner.ID)
	require.NoError(t, err)
	require.Empty(t, profile.Companies)
	require.Empty(t, profile.Points)
	profile, err = f.svc.Profile(f.ctx, heir.ID)
	require.NoError(t, err)
	require.Len(t, profile.Points, 2)
	require.NotEmpty(t, f.pointPermissions(t, cook, a), "staff assignments survive the transfer")

	// прежний владелец больше не распоряжается компанией и её управляющими
	_, err = f.svc.TransferCompany(f.ctx, actor, "", owner.ID, company.ID, owner.Email)
	require.ErrorIs(t, err, organization.ErrForbidden)
	_, err = f.svc.Assign(f.ctx, actor, "", owner.ID, b.ID, cook.Email, organization.RoleManager)
	require.ErrorIs(t, err, organization.ErrForbidden)

	_, err = f.svc.DisableCompany(f.ctx, actor, "", company.ID)
	require.NoError(t, err)
	_, err = f.svc.TransferCompany(f.ctx, actor, "", heir.ID, company.ID, owner.Email)
	require.ErrorIs(t, err, organization.ErrDisabled)
}
