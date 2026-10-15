//go:build integration

package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"goden/backend/internal/platform/auth"
	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/dockertest"
	"goden/backend/internal/platform/eventstore"
)

const (
	// тот же образ и тот же файл realm, что в infra/compose.yaml
	keycloakImage = "quay.io/keycloak/keycloak:26.8.0"
	realmFile     = "../../../../infra/keycloak/goden-realm.json"
	audience      = "goden-api"
)

var ctx = context.Background()

var allPermissions = []string{
	"catalog.view", "deliveries.view", "deliveries.register", "deliveries.approve",
	"orders.view", "orders.accept", "inventory.view", "inventory.edit", "kitchen.view",
	"products.manage", "dishes.manage", "analytics.view", "recommendations.view", "generator.manage",
}

// startKeycloak поднимает Keycloak с realm goden и возвращает его адрес.
func startKeycloak(t *testing.T) string {
	t.Helper()
	dockertest.Setup()
	container, err := testcontainers.Run(ctx, keycloakImage,
		testcontainers.WithCmd("start-dev", "--import-realm"),
		testcontainers.WithEnv(map[string]string{"KC_BOOTSTRAP_ADMIN_USERNAME": "admin", "KC_BOOTSTRAP_ADMIN_PASSWORD": "admin"}),
		testcontainers.WithExposedPorts("8080/tcp"),
		testcontainers.WithFiles(testcontainers.ContainerFile{
			HostFilePath: realmFile, ContainerFilePath: "/opt/keycloak/data/import/goden-realm.json", FileMode: 0o644,
		}),
		// Оба срока заданы явно: по умолчанию и testcontainers, и сама проверка ждут по минуте,
		// а под нагрузкой полного прогона Keycloak стартует дольше.
		testcontainers.WithWaitStrategyAndDeadline(4*time.Minute,
			wait.ForHTTP("/realms/goden").WithPort("8080/tcp").WithStartupTimeout(4*time.Minute)),
	)
	testcontainers.CleanupContainer(t, container)
	require.NoError(t, err)
	endpoint, err := container.PortEndpoint(ctx, "8080/tcp", "http")
	require.NoError(t, err)
	return endpoint
}

// token получает access token у Keycloak.
func token(t *testing.T, keycloak, realm string, form url.Values) string {
	t.Helper()
	resp, err := http.PostForm(keycloak+"/realms/"+realm+"/protocol/openid-connect/token", form)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck
	var body struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error_description"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Equal(t, http.StatusOK, resp.StatusCode, body.Error)
	return body.AccessToken
}

func userToken(t *testing.T, keycloak, username string) string {
	return token(t, keycloak, "goden", url.Values{
		"grant_type": {"password"}, "client_id": {"goden-web"}, "username": {username}, "password": {username},
	})
}

func TestKeycloakTokens(t *testing.T) {
	keycloak := startKeycloak(t)
	issuer := keycloak + "/realms/goden"
	clk := clock.NewFake(time.Now())
	verifier, err := auth.NewOIDC(ctx, issuer, audience, clk)
	require.NoError(t, err)

	requirePermissions := func(t *testing.T, p auth.Principal, want ...string) {
		t.Helper()
		for _, permission := range allPermissions {
			require.Equal(t, strings.Contains(" "+strings.Join(want, " ")+" ", " "+permission+" "), p.Has(permission), permission)
		}
	}

	// Тестовые пользователи и их постоянные идентификаторы из файла realm: по ним пользователя
	// знает демонстрационная компания. Прав в токене у пользователей нет (ADR-013).
	users := map[string]string{
		"admin":    "0a000000-0000-4000-8000-000000000001",
		"manager":  "0a000000-0000-4000-8000-000000000002",
		"approver": "0a000000-0000-4000-8000-000000000003",
		"cook":     "0a000000-0000-4000-8000-000000000004",
		"waiter":   "0a000000-0000-4000-8000-000000000005",
	}
	for username, id := range users {
		t.Run("user "+username, func(t *testing.T) {
			principal, err := verifier.Verify(ctx, userToken(t, keycloak, username))
			require.NoError(t, err)
			require.Equal(t, eventstore.Actor{Type: eventstore.ActorUser, ID: username}, principal.Actor)
			// личность: постоянный идентификатор и почта, по которой сотрудника назначают на точку
			require.Equal(t, id, principal.User.ID)
			require.Equal(t, username+"@goden.local", principal.User.Email)
			require.NotEmpty(t, principal.User.Name)
			requirePermissions(t, principal)
			require.False(t, principal.Has("generator.run"), "users never act as the generator")
			require.False(t, principal.Has("migom.integration"), "users never act as Migom")
		})
	}

	t.Run("generator service account", func(t *testing.T) {
		source, err := auth.ClientCredentials(ctx, issuer, "goden-generator", "goden-generator-dev-secret")
		require.NoError(t, err)
		issued, err := source.Token()
		require.NoError(t, err)

		principal, err := verifier.Verify(ctx, issued.AccessToken)
		require.NoError(t, err)
		require.Equal(t, eventstore.Actor{Type: eventstore.ActorService, ID: "goden-generator"}, principal.Actor)
		requirePermissions(t, principal)
		// право генератора есть только у его сервисной учётной записи
		require.True(t, principal.Has("generator.run"))
	})

	// Мигом ходит в API Goden от имени своего пользователя: тот же пользователь, с правами по владению
	t.Run("user signed in through migom", func(t *testing.T) {
		issued := token(t, keycloak, "goden", url.Values{
			"grant_type": {"password"}, "client_id": {"migom-backend"}, "client_secret": {"migom-backend-dev-secret"},
			"username": {"admin"}, "password": {"admin"},
		})
		principal, err := verifier.Verify(ctx, issued)
		require.NoError(t, err)
		require.Equal(t, eventstore.Actor{Type: eventstore.ActorUser, ID: "admin"}, principal.Actor)
		require.Equal(t, users["admin"], principal.User.ID)
		requirePermissions(t, principal)
		require.False(t, principal.Has("migom.integration"))
	})

	t.Run("migom service account", func(t *testing.T) {
		source, err := auth.ClientCredentials(ctx, issuer, "migom-integration", "migom-integration-dev-secret")
		require.NoError(t, err)
		issued, err := source.Token()
		require.NoError(t, err)

		principal, err := verifier.Verify(ctx, issued.AccessToken)
		require.NoError(t, err)
		require.Equal(t, eventstore.Actor{Type: eventstore.ActorService, ID: "migom-integration"}, principal.Actor)
		requirePermissions(t, principal)
		// права на точке Мигом получает, только где его подключил владелец
		require.True(t, principal.Has("migom.integration"))
		require.False(t, principal.Has("generator.run"))
	})

	t.Run("wrong client secret", func(t *testing.T) {
		source, err := auth.ClientCredentials(ctx, issuer, "goden-generator", "wrong")
		require.NoError(t, err)
		_, err = source.Token()
		require.Error(t, err)
		require.NotContains(t, err.Error(), "wrong", "the secret does not leak into the error")
	})

	valid := userToken(t, keycloak, "approver")
	rejected := map[string]string{
		"garbage":           "not-a-token",
		"empty":             "",
		"broken signature":  valid[:len(valid)-6] + "AAAAAA",
		"other realm":       token(t, keycloak, "master", url.Values{"grant_type": {"password"}, "client_id": {"admin-cli"}, "username": {"admin"}, "password": {"admin"}}),
		"payload of admin":  swapPayload(valid, userToken(t, keycloak, "admin")),
		"signed by nothing": unsigned(valid),
	}
	for name, raw := range rejected {
		t.Run("rejects "+name, func(t *testing.T) {
			_, err := verifier.Verify(ctx, raw)
			require.ErrorIs(t, err, auth.ErrInvalidToken)
		})
	}

	t.Run("rejects a token for another audience", func(t *testing.T) {
		other, err := auth.NewOIDC(ctx, issuer, "another-api", clk)
		require.NoError(t, err)
		_, err = other.Verify(ctx, valid)
		require.ErrorIs(t, err, auth.ErrInvalidToken)
	})

	// последним: часы уходят вперёд, и все выданные токены истекают
	t.Run("rejects an expired token", func(t *testing.T) {
		_, err := verifier.Verify(ctx, valid)
		require.NoError(t, err)
		clk.Advance(time.Hour)
		_, err = verifier.Verify(ctx, valid)
		require.ErrorIs(t, err, auth.ErrInvalidToken)
	})
}

// swapPayload подставляет в токен утверждения другого токена, оставляя подпись прежней.
func swapPayload(token, other string) string {
	parts, otherParts := strings.Split(token, "."), strings.Split(other, ".")
	return parts[0] + "." + otherParts[1] + "." + parts[2]
}

// unsigned объявляет токен неподписанным (alg none) и убирает подпись.
func unsigned(token string) string {
	return "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." + strings.Split(token, ".")[1] + "."
}

func TestNewOIDCFailsWithoutIssuer(t *testing.T) {
	_, err := auth.NewOIDC(ctx, "http://127.0.0.1:1/realms/goden", audience, clock.System{})
	require.Error(t, err)
}
