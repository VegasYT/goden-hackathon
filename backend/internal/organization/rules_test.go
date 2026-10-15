package organization

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMayManage(t *testing.T) {
	tests := []struct {
		name   string
		owner  bool
		caller Role
		roles  []Role
		ok     bool
	}{
		{name: "owner assigns a manager", owner: true, roles: []Role{RoleManager}, ok: true},
		{name: "owner removes a manager", owner: true, roles: []Role{RoleManager}, ok: true},
		{name: "manager assigns a cook", caller: RoleManager, roles: []Role{RoleCook}, ok: true},
		{name: "manager turns a waiter into an approver", caller: RoleManager, roles: []Role{RoleWaiter, RoleApprover}, ok: true},
		{name: "manager assigns a manager", caller: RoleManager, roles: []Role{RoleManager}},
		{name: "manager promotes a cook to manager", caller: RoleManager, roles: []Role{RoleCook, RoleManager}},
		{name: "manager demotes a manager", caller: RoleManager, roles: []Role{RoleManager, RoleCook}},
		{name: "cook assigns a waiter", caller: RoleCook, roles: []Role{RoleWaiter}},
		{name: "outsider assigns a waiter", roles: []Role{RoleWaiter}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mayManage(tt.owner, tt.caller, tt.roles...)
			if tt.ok {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, ErrForbidden)
		})
	}
}

func TestPermissions(t *testing.T) {
	owner := permissions(true, "")
	for role, granted := range rolePermissions {
		require.NoError(t, validRole(role))
		require.Subset(t, owner, granted, "the owner can do everything the %s can", role)
		// каталог компании меняет только владелец, точки и компанию - тоже
		for _, ownerOnly := range []string{"products.manage", "dishes.manage", "generator.manage", PermissionCompanyManage} {
			require.NotContains(t, granted, ownerOnly, role)
		}
		require.Contains(t, granted, PermissionPointView, role)
	}
	require.Contains(t, owner, PermissionCompanyManage)
	require.Contains(t, permissions(false, RoleManager), PermissionStaffManage)
	require.NotContains(t, permissions(false, RoleCook), PermissionStaffManage)
	require.Empty(t, permissions(false, ""), "no assignment, no permissions")
	require.Empty(t, permissions(false, "admin"))

	// вызывающий получает копию: изменить права роли через результат нельзя
	permissions(false, RoleCook)[0] = "changed"
	require.NotContains(t, rolePermissions[RoleCook], "changed")
}

func TestValidation(t *testing.T) {
	name, err := normalizedName("  Точка на Ленина ")
	require.NoError(t, err)
	require.Equal(t, "Точка на Ленина", name)

	var validation *ValidationError
	_, err = normalizedName("   ")
	require.ErrorAs(t, err, &validation)
	require.Equal(t, "name", validation.Field)

	require.ErrorAs(t, validRole("admin"), &validation)
	require.Equal(t, "role", validation.Field)

	require.Equal(t, "boss@goden.local", key(" Boss@Goden.Local "))
}
