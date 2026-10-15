//go:build integration

package pgtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"goden/backend/internal/platform/config"
	"goden/backend/internal/platform/dockertest"
	"goden/backend/internal/platform/postgres"
)

const (
	image      = "postgres:18-alpine"
	templateDB = "goden_template"
)

var (
	once     sync.Once
	adminURL string // подключение к служебной базе postgres в контейнере
	startErr error
)

// NewDB возвращает пул к отдельной базе с применёнными миграциями.
// База создаётся для одного теста и удаляется после него.
//
// Тесты одного пакета делят один контейнер. Не запускай через t.Parallel тесты,
// которые читают события через eventstore.ReadAll: незавершённая транзакция
// в любой базе контейнера задерживает такое чтение.
func NewDB(t testing.TB) *pgxpool.Pool {
	t.Helper()
	once.Do(start)
	require.NoError(t, startErr, "start postgres container")

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	name := "t_" + randomHex()
	admin, err := pgx.Connect(ctx, adminURL)
	require.NoError(t, err)
	defer admin.Close(ctx) //nolint:errcheck

	_, err = admin.Exec(ctx, "CREATE DATABASE "+name+" TEMPLATE "+templateDB)
	require.NoError(t, err)

	pool, err := postgres.NewPool(ctx, config.Postgres{URL: withDatabase(adminURL, name), MaxConns: 10})
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Close()
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		admin, err := pgx.Connect(ctx, adminURL)
		if err != nil {
			return
		}
		defer admin.Close(ctx)                                        //nolint:errcheck
		_, _ = admin.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)") // контейнер всё равно удаляется после тестов
	})
	return pool
}

// start запускает контейнер и готовит шаблонную базу с миграциями.
// Контейнер удаляет testcontainers после завершения процесса тестов.
func start() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	dockertest.Setup()

	container, err := tcpostgres.Run(ctx, image,
		tcpostgres.WithDatabase("postgres"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		startErr = err
		return
	}
	if adminURL, startErr = container.ConnectionString(ctx, "sslmode=disable"); startErr != nil {
		return
	}

	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		startErr = err
		return
	}
	defer admin.Close(ctx) //nolint:errcheck
	if _, startErr = admin.Exec(ctx, "CREATE DATABASE "+templateDB); startErr != nil {
		return
	}

	// пул закрывается до первого CREATE DATABASE ... TEMPLATE: к шаблону не должно быть подключений
	pool, err := postgres.NewPool(ctx, config.Postgres{URL: withDatabase(adminURL, templateDB), MaxConns: 2})
	if err != nil {
		startErr = err
		return
	}
	defer pool.Close()
	_, startErr = postgres.Migrate(ctx, pool)
}

func withDatabase(connURL, name string) string {
	u, err := url.Parse(connURL)
	if err != nil {
		panic(err) // адрес выдаёт testcontainers, он всегда корректен
	}
	u.Path = "/" + name
	return u.String()
}

func randomHex() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
