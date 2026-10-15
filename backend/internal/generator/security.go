package generator

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"

	"goden/backend/internal/api/oas"
)

// Security передаёт клиенту API токен из source. Источник сам кеширует и обновляет токен.
// С nil клиент отправляет токен-заглушку: его принимает только API в режиме без авторизации.
func Security(source oauth2.TokenSource) oas.SecuritySource {
	return bearer{source: source}
}

type bearer struct {
	source oauth2.TokenSource
}

func (b bearer) BearerAuth(context.Context, oas.OperationName) (oas.BearerAuth, error) {
	if b.source == nil {
		return oas.BearerAuth{Token: "anonymous"}, nil
	}
	token, err := b.source.Token()
	if err != nil {
		return oas.BearerAuth{}, fmt.Errorf("generator: access token: %w", err)
	}
	return oas.BearerAuth{Token: token.AccessToken}, nil
}
