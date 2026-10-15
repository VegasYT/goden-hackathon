//go:build integration

package pgtest_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"goden/backend/internal/platform/postgres/pgtest"
)

func TestNewDBGivesIsolatedDatabases(t *testing.T) {
	ctx := context.Background()
	first, second := pgtest.NewDB(t), pgtest.NewDB(t)

	_, err := first.Exec(ctx, "CREATE TABLE only_in_first (id int)")
	require.NoError(t, err)

	var exists bool
	require.NoError(t, second.QueryRow(ctx, "SELECT to_regclass('only_in_first') IS NOT NULL").Scan(&exists))
	require.False(t, exists)
}
