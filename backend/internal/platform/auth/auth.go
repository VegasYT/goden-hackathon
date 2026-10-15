// Package auth проверяет access token Keycloak и определяет, кто обращается к API
// и какие у него права (ADR-006).
package auth

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/eventstore"
)

// ErrInvalidToken - токен не прошёл проверку. Причина есть в обёрнутой ошибке;
// клиенту она не сообщается.
var ErrInvalidToken = errors.New("auth: invalid token")

// User - личность клиента из токена. ID не меняется при смене почты и имени:
// по нему пользователя знают назначения на точки (ADR-013).
type User struct {
	ID    string
	Email string
	Name  string
}

type userKey struct{}

// WithUser кладёт личность клиента в контекст запроса.
func WithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, userKey{}, user)
}

// UserFrom возвращает личность клиента запроса.
func UserFrom(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(userKey{}).(User)
	return user, ok && user.ID != ""
}

// Principal - аутентифицированный клиент API.
type Principal struct {
	// Actor - автор, от имени которого выполняются команды.
	Actor eventstore.Actor
	// User - личность клиента. Права на точке и в компании определяются по ней, а не по токену.
	User        User
	permissions []string
	all         bool
}

// NewPrincipal создаёт клиента с перечисленными правами.
func NewPrincipal(actor eventstore.Actor, permissions ...string) Principal {
	return Principal{Actor: actor, permissions: permissions}
}

// Has сообщает, есть ли у клиента право.
func (p Principal) Has(permission string) bool {
	return p.all || slices.Contains(p.permissions, permission)
}

// Verifier проверяет токен и возвращает его владельца.
type Verifier interface {
	Verify(ctx context.Context, token string) (Principal, error)
}

// DevActor - автор всех запросов в режиме без авторизации.
var DevActor = eventstore.Actor{Type: eventstore.ActorUser, ID: "dev"}

// DevUser - личность всех запросов в режиме без авторизации.
var DevUser = User{ID: "00000000-0000-0000-0000-000000000001", Email: "dev@goden.local", Name: "dev"}

type disabled struct{}

// Disabled возвращает проверку для режима без авторизации: любой запрос выполняется
// от имени DevActor. Права из токена у него все, а права на точке и в компании он получает
// как обычный пользователь DevUser. Только для локальной разработки.
func Disabled() Verifier { return disabled{} }

func (disabled) Verify(context.Context, string) (Principal, error) {
	return Principal{Actor: DevActor, User: DevUser, all: true}, nil
}

// claims - утверждения access token Keycloak, которые читает backend.
type claims struct {
	Subject           string `json:"sub"`
	Email             string `json:"email"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
	// ClientID есть только в токене сервисной учётной записи.
	ClientID       string `json:"client_id"`
	ResourceAccess map[string]struct {
		Roles []string `json:"roles"`
	} `json:"resource_access"`
}

// principal собирает клиента из утверждений. Права - client roles клиента audience.
func (c claims) principal(audience string) (Principal, error) {
	actor := eventstore.Actor{Type: eventstore.ActorUser, ID: c.PreferredUsername}
	if c.ClientID != "" {
		actor = eventstore.Actor{Type: eventstore.ActorService, ID: c.ClientID}
	}
	if actor.ID == "" {
		return Principal{}, fmt.Errorf("%w: token names neither a user nor a client", ErrInvalidToken)
	}
	// без sub клиента не с чем сопоставить: владение и назначения ведутся по этому идентификатору
	if c.Subject == "" {
		return Principal{}, fmt.Errorf("%w: token has no subject", ErrInvalidToken)
	}
	p := NewPrincipal(actor, c.ResourceAccess[audience].Roles...)
	p.User = User{ID: c.Subject, Email: c.Email, Name: c.Name}
	return p, nil
}

type oidcVerifier struct {
	verifier *oidc.IDTokenVerifier
	audience string
}

// NewOIDC возвращает проверку токенов издателя issuer (адрес realm Keycloak): подпись,
// издатель, получатель audience и срок действия. Конфигурация и ключи издателя
// запрашиваются по сети, ключи кешируются.
func NewOIDC(ctx context.Context, issuer, audience string, clk clock.Clock) (Verifier, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("auth: discover issuer %s: %w", issuer, err)
	}
	return &oidcVerifier{
		verifier: provider.Verifier(&oidc.Config{ClientID: audience, Now: clk.Now}),
		audience: audience,
	}, nil
}

func (v *oidcVerifier) Verify(ctx context.Context, token string) (Principal, error) {
	verified, err := v.verifier.Verify(ctx, token)
	if err != nil {
		return Principal{}, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	var c claims
	if err := verified.Claims(&c); err != nil {
		return Principal{}, fmt.Errorf("%w: claims: %w", ErrInvalidToken, err)
	}
	return c.principal(v.audience)
}

// ClientCredentials возвращает источник токенов сервисной учётной записи у издателя issuer.
// Источник кеширует токен и получает новый перед истечением срока. ctx задаёт время жизни
// источника, а не одного запроса.
func ClientCredentials(ctx context.Context, issuer, clientID, clientSecret string) (oauth2.TokenSource, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("auth: discover issuer %s: %w", issuer, err)
	}
	config := clientcredentials.Config{
		ClientID: clientID, ClientSecret: clientSecret, TokenURL: provider.Endpoint().TokenURL,
	}
	return config.TokenSource(ctx), nil
}
