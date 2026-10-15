//go:build integration

package organization_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/organization"
)

// В событиях автор записан логином: имя по нему находится только у пользователя, чей логин известен.
func TestNamesByLogin(t *testing.T) {
	f := setup(t)
	anna := organization.User{ID: uuid.NewString(), Email: "anna@goden.local", Name: "Анна Котова", Login: "anna"}
	require.NoError(t, f.svc.Remember(f.ctx, anna))
	f.user(t, "old") // запомнен без логина, как пользователи до появления поля

	names, err := f.svc.Names(f.ctx, []string{"anna", "old", "nobody", ""})
	require.NoError(t, err)
	require.Equal(t, map[string]string{"anna": "Анна Котова"}, names)

	// имя следует за профилем
	anna.Name = "Анна Смирнова"
	require.NoError(t, f.svc.Remember(f.ctx, anna))
	names, err = f.svc.Names(f.ctx, []string{"anna"})
	require.NoError(t, err)
	require.Equal(t, map[string]string{"anna": "Анна Смирнова"}, names)

	names, err = f.svc.Names(f.ctx, nil)
	require.NoError(t, err)
	require.Empty(t, names)
}
