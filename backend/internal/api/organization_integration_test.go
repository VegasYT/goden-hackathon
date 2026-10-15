//go:build integration

package api_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
	"goden/backend/internal/platform/auth"
)

// network - компания владельца с двумя точками.
type network struct {
	owner   *oas.Client
	company *oas.Company
	a, b    *oas.Point
}

func newNetwork(t *testing.T, s apitest.Server) network {
	t.Helper()
	n := network{owner: s.Login(t, "owner")}
	var err error
	n.company, err = n.owner.CreateCompany(ctx, &oas.NewCompany{Name: "Пиццерии"}, oas.CreateCompanyParams{})
	require.NoError(t, err)
	n.a, err = n.owner.CreatePoint(ctx, &oas.NewPoint{Name: "Точка на Ленина"}, oas.CreatePointParams{CompanyId: n.company.ID})
	require.NoError(t, err)
	n.b, err = n.owner.CreatePoint(ctx, &oas.NewPoint{Name: "Точка на Мира"}, oas.CreatePointParams{CompanyId: n.company.ID})
	require.NoError(t, err)
	return n
}

// hire входит пользователем name и назначает его на точку от имени by.
func hire(t *testing.T, s apitest.Server, by *oas.Client, point *oas.Point, name string, role oas.Role) (*oas.Client, *oas.Staff) {
	t.Helper()
	client := s.Login(t, name)
	staff, err := by.AssignStaff(ctx, &oas.NewStaff{Email: name + "@goden.local", Role: role}, oas.AssignStaffParams{PointId: point.ID})
	require.NoError(t, err)
	return client, staff
}

func requireForbidden(t *testing.T, err error, permission string) {
	t.Helper()
	problem := requireProblem(t, err, http.StatusForbidden)
	require.Equal(t, "forbidden", problem.Type)
	require.Contains(t, problem.Detail.Value, permission)
}

func TestProfileOfANewUser(t *testing.T) {
	s := apitest.New(t)

	profile, err := s.Login(t, "newcomer").GetMe(ctx)
	require.NoError(t, err)
	require.Equal(t, "newcomer@goden.local", profile.Email)
	require.Equal(t, "newcomer", profile.Name)
	require.NotEmpty(t, profile.ID)
	require.Empty(t, profile.Companies)
	require.Empty(t, profile.Points)
}

func TestOwnerBuildsACompany(t *testing.T) {
	s := apitest.New(t)
	n := newNetwork(t, s)

	require.True(t, n.company.Active)
	require.Equal(t, n.company.ID, n.a.CompanyID)

	// вторая компания того же владельца со своей точкой с тем же названием
	second, err := n.owner.CreateCompany(ctx, &oas.NewCompany{Name: "Суши"}, oas.CreateCompanyParams{})
	require.NoError(t, err)
	_, err = n.owner.CreatePoint(ctx, &oas.NewPoint{Name: "Точка на Ленина"}, oas.CreatePointParams{CompanyId: second.ID})
	require.NoError(t, err)

	_, err = n.owner.CreatePoint(ctx, &oas.NewPoint{Name: "точка на ленина"}, oas.CreatePointParams{CompanyId: n.company.ID})
	require.Equal(t, "conflict", requireProblem(t, err, http.StatusConflict).Type)
	_, err = n.owner.CreateCompany(ctx, &oas.NewCompany{Name: "   "}, oas.CreateCompanyParams{})
	problem := requireProblem(t, err, http.StatusUnprocessableEntity)
	require.Equal(t, "name", problem.Errors[0].Field)

	renamed, err := n.owner.RenameCompany(ctx, &oas.NewCompany{Name: "Пиццерии Севера"}, oas.RenameCompanyParams{CompanyId: n.company.ID})
	require.NoError(t, err)
	require.Equal(t, "Пиццерии Севера", renamed.Name)
	point, err := n.owner.RenamePoint(ctx, &oas.NewPoint{Name: "Точка у вокзала"}, oas.RenamePointParams{PointId: n.b.ID})
	require.NoError(t, err)
	require.Equal(t, "Точка у вокзала", point.Name)

	points, err := n.owner.ListPoints(ctx, oas.ListPointsParams{CompanyId: n.company.ID})
	require.NoError(t, err)
	require.Len(t, points, 2)

	profile, err := n.owner.GetMe(ctx)
	require.NoError(t, err)
	require.Len(t, profile.Companies, 2)
	require.Len(t, profile.Points, 3)
	for _, p := range profile.Points {
		require.True(t, p.Owner)
		require.False(t, p.Role.Set, "the owner has no role on a point")
		require.Contains(t, p.Permissions, "company.manage")
		require.Contains(t, p.Permissions, "deliveries.approve")
	}

	// повтор с тем же ключом возвращает ту же компанию
	params := oas.CreateCompanyParams{IdempotencyKey: oas.NewOptString("company-1")}
	first, err := n.owner.CreateCompany(ctx, &oas.NewCompany{Name: "Бургеры"}, params)
	require.NoError(t, err)
	again, err := n.owner.CreateCompany(ctx, &oas.NewCompany{Name: "Бургеры"}, params)
	require.NoError(t, err)
	require.Equal(t, first, again)
}

func TestStaffIsAssignedByEmail(t *testing.T) {
	s := apitest.New(t)
	n := newNetwork(t, s)

	_, err := n.owner.AssignStaff(ctx, &oas.NewStaff{Email: "ghost@goden.local", Role: oas.RoleCook}, oas.AssignStaffParams{PointId: n.a.ID})
	problem := requireProblem(t, err, http.StatusUnprocessableEntity)
	require.Equal(t, "unknown-reference", problem.Type, "nobody has signed in with this email")

	manager, staff := hire(t, s, n.owner, n.a, "manager", oas.RoleManager)
	require.Equal(t, "manager@goden.local", staff.Email)
	require.Equal(t, oas.RoleManager, staff.Role)

	_, err = n.owner.AssignStaff(ctx, &oas.NewStaff{Email: "manager@goden.local", Role: oas.RoleCook}, oas.AssignStaffParams{PointId: n.a.ID})
	requireProblem(t, err, http.StatusConflict)

	// управляющий распоряжается своей точкой, но не управляющими и не соседней точкой
	cook, cookStaff := hire(t, s, manager, n.a, "cook", oas.RoleCook)
	s.Login(t, "second-manager")
	_, err = manager.AssignStaff(ctx, &oas.NewStaff{Email: "second-manager@goden.local", Role: oas.RoleManager}, oas.AssignStaffParams{PointId: n.a.ID})
	requireProblem(t, err, http.StatusForbidden)
	err = manager.RemoveStaff(ctx, oas.RemoveStaffParams{PointId: n.a.ID, UserId: staff.UserID})
	requireProblem(t, err, http.StatusForbidden)
	_, err = manager.AssignStaff(ctx, &oas.NewStaff{Email: "second-manager@goden.local", Role: oas.RoleWaiter}, oas.AssignStaffParams{PointId: n.b.ID})
	requireForbidden(t, err, "staff.manage")
	_, err = manager.CreatePoint(ctx, &oas.NewPoint{Name: "Ещё одна"}, oas.CreatePointParams{CompanyId: n.company.ID})
	requireForbidden(t, err, "company.manage")

	// один сотрудник - разные роли на разных точках
	_, err = n.owner.AssignStaff(ctx, &oas.NewStaff{Email: "cook@goden.local", Role: oas.RoleManager}, oas.AssignStaffParams{PointId: n.b.ID})
	require.NoError(t, err)
	profile, err := cook.GetMe(ctx)
	require.NoError(t, err)
	require.Empty(t, profile.Companies)
	roles := map[uuid.UUID]oas.Role{}
	for _, p := range profile.Points {
		require.False(t, p.Owner)
		require.Equal(t, "Пиццерии", p.CompanyName)
		roles[p.ID] = p.Role.Value
	}
	require.Equal(t, map[uuid.UUID]oas.Role{n.a.ID: oas.RoleCook, n.b.ID: oas.RoleManager}, roles)

	changed, err := manager.ChangeStaffRole(ctx, &oas.StaffRole{Role: oas.RoleWaiter}, oas.ChangeStaffRoleParams{PointId: n.a.ID, UserId: cookStaff.UserID})
	require.NoError(t, err)
	require.Equal(t, oas.RoleWaiter, changed.Role)

	list, err := cook.ListStaff(ctx, oas.ListStaffParams{PointId: n.a.ID})
	require.NoError(t, err)
	require.Len(t, list, 2)
}

func TestAccessFollowsAssignments(t *testing.T) {
	s := apitest.New(t)
	n := newNetwork(t, s)
	cook, staff := hire(t, s, n.owner, n.a, "cook", oas.RoleCook)
	stranger := s.Login(t, "stranger")

	// сотрудник видит свою точку и её компанию, но не соседнюю точку и не список точек
	_, err := cook.GetPoint(ctx, oas.GetPointParams{PointId: n.a.ID})
	require.NoError(t, err)
	_, err = cook.GetCompany(ctx, oas.GetCompanyParams{CompanyId: n.company.ID})
	require.NoError(t, err)
	_, err = cook.ListStaff(ctx, oas.ListStaffParams{PointId: n.b.ID})
	requireForbidden(t, err, "staff.view")
	_, err = cook.ListPoints(ctx, oas.ListPointsParams{CompanyId: n.company.ID})
	requireForbidden(t, err, "company.manage")
	_, err = cook.AssignStaff(ctx, &oas.NewStaff{Email: "stranger@goden.local", Role: oas.RoleWaiter}, oas.AssignStaffParams{PointId: n.a.ID})
	requireForbidden(t, err, "staff.manage")

	// посторонний не читает и не меняет чужое
	_, err = stranger.GetCompany(ctx, oas.GetCompanyParams{CompanyId: n.company.ID})
	requireForbidden(t, err, "company.view")
	_, err = stranger.GetPoint(ctx, oas.GetPointParams{PointId: n.a.ID})
	requireForbidden(t, err, "point.view")
	_, err = stranger.RenameCompany(ctx, &oas.NewCompany{Name: "Моё"}, oas.RenameCompanyParams{CompanyId: n.company.ID})
	requireForbidden(t, err, "company.manage")
	company, err := n.owner.GetCompany(ctx, oas.GetCompanyParams{CompanyId: n.company.ID})
	require.NoError(t, err)
	require.Equal(t, "Пиццерии", company.Name)

	// точки нет
	_, err = n.owner.GetPoint(ctx, oas.GetPointParams{PointId: uuid.Must(uuid.NewV7())})
	requireProblem(t, err, http.StatusNotFound)

	// назначение снято, а токен сотрудника прежний: права пропадают сразу
	require.NoError(t, n.owner.RemoveStaff(ctx, oas.RemoveStaffParams{PointId: n.a.ID, UserId: staff.UserID}))
	_, err = cook.ListStaff(ctx, oas.ListStaffParams{PointId: n.a.ID})
	requireForbidden(t, err, "staff.view")
	profile, err := cook.GetMe(ctx)
	require.NoError(t, err)
	require.Empty(t, profile.Points)
}

func TestDisabledPointIsReadOnly(t *testing.T) {
	s := apitest.New(t)
	n := newNetwork(t, s)
	cook, _ := hire(t, s, n.owner, n.a, "cook", oas.RoleCook)
	s.Login(t, "waiter")

	disabled, err := n.owner.DisablePoint(ctx, oas.DisablePointParams{PointId: n.a.ID})
	require.NoError(t, err)
	require.False(t, disabled.Active)

	_, err = n.owner.AssignStaff(ctx, &oas.NewStaff{Email: "waiter@goden.local", Role: oas.RoleWaiter}, oas.AssignStaffParams{PointId: n.a.ID})
	require.Equal(t, "conflict", requireProblem(t, err, http.StatusConflict).Type)
	list, err := n.owner.ListStaff(ctx, oas.ListStaffParams{PointId: n.a.ID})
	require.NoError(t, err)
	require.Len(t, list, 1, "the owner still reads a disabled point")
	_, err = cook.ListStaff(ctx, oas.ListStaffParams{PointId: n.a.ID})
	requireForbidden(t, err, "staff.view")

	company, err := n.owner.DisableCompany(ctx, oas.DisableCompanyParams{CompanyId: n.company.ID})
	require.NoError(t, err)
	require.False(t, company.Active)
	_, err = n.owner.CreatePoint(ctx, &oas.NewPoint{Name: "Новая"}, oas.CreatePointParams{CompanyId: n.company.ID})
	requireProblem(t, err, http.StatusConflict)
}

// Режим без проверки токенов отключает только проверку токена: пользователь разработки
// получает права на точке и в компании по тем же правилам, что и остальные.
func TestDevUserIsAnOrdinaryUser(t *testing.T) {
	s := apitest.New(t)
	n := newNetwork(t, s)

	// у клиента dev в токене все права, но чужая компания для него закрыта
	_, err := s.Client.GetCompany(ctx, oas.GetCompanyParams{CompanyId: n.company.ID})
	requireForbidden(t, err, "company.view")
	_, err = s.Client.AssignStaff(ctx, &oas.NewStaff{Email: "owner@goden.local", Role: oas.RoleCook}, oas.AssignStaffParams{PointId: n.a.ID})
	requireForbidden(t, err, "staff.manage")

	own, err := s.Client.CreateCompany(ctx, &oas.NewCompany{Name: "Локальная"}, oas.CreateCompanyParams{})
	require.NoError(t, err)
	require.Equal(t, auth.DevUser.ID, own.OwnerID)
	point, err := s.Client.CreatePoint(ctx, &oas.NewPoint{Name: "Точка"}, oas.CreatePointParams{CompanyId: own.ID})
	require.NoError(t, err)
	_, err = s.Client.AssignStaff(ctx, &oas.NewStaff{Email: "owner@goden.local", Role: oas.RoleCook}, oas.AssignStaffParams{PointId: point.ID})
	require.NoError(t, err)

	profile, err := s.Client.GetMe(ctx)
	require.NoError(t, err)
	require.Equal(t, auth.DevUser.Email, profile.Email)
	require.Len(t, profile.Points, 2, "the point of the test server and the new one")
}

// Чат Telegram точки задают владелец и управляющий; остальные сотрудники его видят, но не меняют.
func TestPointTelegramChatThroughAPI(t *testing.T) {
	s := apitest.New(t)
	n := newNetwork(t, s)
	manager, _ := hire(t, s, n.owner, n.a, "manager", oas.RoleManager)
	cook, _ := hire(t, s, n.owner, n.a, "cook", oas.RoleCook)
	chat := &oas.PointTelegramChat{ChatID: oas.NewOptInt64(-1001234567890)}

	updated, err := manager.SetPointTelegramChat(ctx, chat, oas.SetPointTelegramChatParams{PointId: n.a.ID})
	require.NoError(t, err)
	require.EqualValues(t, -1001234567890, updated.TelegramChatID.Value)
	read, err := cook.GetPoint(ctx, oas.GetPointParams{PointId: n.a.ID})
	require.NoError(t, err)
	require.Equal(t, updated, read)

	_, err = cook.SetPointTelegramChat(ctx, chat, oas.SetPointTelegramChatParams{PointId: n.a.ID})
	requireForbidden(t, err, "notifications.manage")
	_, err = manager.SetPointTelegramChat(ctx, chat, oas.SetPointTelegramChatParams{PointId: n.b.ID})
	requireForbidden(t, err, "notifications.manage")

	// запрос без чата снимает его
	cleared, err := n.owner.SetPointTelegramChat(ctx, &oas.PointTelegramChat{}, oas.SetPointTelegramChatParams{PointId: n.a.ID})
	require.NoError(t, err)
	require.False(t, cleared.TelegramChatID.Set)
	_, err = n.owner.SetPointTelegramChat(ctx, &oas.PointTelegramChat{ChatID: oas.NewOptInt64(0)}, oas.SetPointTelegramChatParams{PointId: n.a.ID})
	requireProblem(t, err, http.StatusUnprocessableEntity)
}

func TestCompanyTransfer(t *testing.T) {
	s := apitest.New(t)
	n := newNetwork(t, s)
	manager, _ := hire(t, s, n.owner, n.a, "manager", oas.RoleManager)
	heir, _ := hire(t, s, n.owner, n.a, "heir", oas.RoleCook)
	params := oas.TransferCompanyParams{CompanyId: n.company.ID}

	_, err := manager.TransferCompany(ctx, &oas.NewOwner{Email: "manager@goden.local"}, params)
	requireForbidden(t, err, "company.manage")
	_, err = n.owner.TransferCompany(ctx, &oas.NewOwner{Email: "nobody@goden.local"}, params)
	require.Equal(t, "unknown-reference", requireProblem(t, err, http.StatusUnprocessableEntity).Type)

	company, err := n.owner.TransferCompany(ctx, &oas.NewOwner{Email: "heir@goden.local"}, params)
	require.NoError(t, err)
	require.NotEqual(t, n.company.OwnerID, company.OwnerID)

	// права меняются сразу, токены прежние
	_, err = n.owner.GetCompany(ctx, oas.GetCompanyParams{CompanyId: n.company.ID})
	requireForbidden(t, err, "company.view")
	_, err = n.owner.GetPoint(ctx, oas.GetPointParams{PointId: n.a.ID})
	requireForbidden(t, err, "point.view")
	_, err = n.owner.TransferCompany(ctx, &oas.NewOwner{Email: "owner@goden.local"}, params)
	requireForbidden(t, err, "company.manage")
	profile, err := n.owner.GetMe(ctx)
	require.NoError(t, err)
	require.Empty(t, profile.Companies)
	require.Empty(t, profile.Points)

	profile, err = heir.GetMe(ctx)
	require.NoError(t, err)
	require.Len(t, profile.Companies, 1)
	require.Len(t, profile.Points, 2)
	for _, p := range profile.Points {
		require.True(t, p.Owner, "the cook of point A now owns both points")
	}
	_, err = heir.CreatePoint(ctx, &oas.NewPoint{Name: "Третья"}, oas.CreatePointParams{CompanyId: n.company.ID})
	require.NoError(t, err)
	_, err = manager.ListStaff(ctx, oas.ListStaffParams{PointId: n.a.ID})
	require.NoError(t, err, "staff keeps working under the new owner")
}
