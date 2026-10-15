package migrations_test

import (
	"fmt"
	"io/fs"
	"regexp"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"goden/backend/migrations"
)

var fileName = regexp.MustCompile(`^(\d{5})_[a-z0-9_]+\.sql$`)

// Две ветки могут создать миграцию с одним номером: имена файлов разные, git конфликта
// не покажет, а goose упадёт только при запуске. Тест ловит это сразу после слияния.
func TestMigrationsAreNumberedConsecutively(t *testing.T) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	require.NoError(t, err)
	require.NotEmpty(t, entries)

	// ReadDir отдаёт файлы по алфавиту, для номеров из пяти цифр это порядок номеров
	for i, entry := range entries {
		match := fileName.FindStringSubmatch(entry.Name())
		require.NotNil(t, match, "%s: name must look like 00001_snake_case.sql", entry.Name())

		number, err := strconv.Atoi(match[1])
		require.NoError(t, err)
		require.Equal(t, i+1, number,
			"%s: expected number %s; numbers go without gaps and repeats, renumber the migration of the branch merged later",
			entry.Name(), fmt.Sprintf("%05d", i+1))
	}
}
