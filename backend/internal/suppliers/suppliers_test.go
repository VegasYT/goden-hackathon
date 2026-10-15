package suppliers

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/platform/eventstore"
)

// Ключ узнаётся по префиксу, каждый выпуск новый, а хранится только хеш ключа.
func TestNewKey(t *testing.T) {
	first, firstHash, err := newKey()
	require.NoError(t, err)
	second, _, err := newKey()
	require.NoError(t, err)

	require.True(t, strings.HasPrefix(first, keyPrefix))
	require.NotEqual(t, first, second)
	require.Equal(t, hash(first), firstHash)
}

// Автор поставщика однозначно возвращает поставщика, другие сервисные авторы - нет.
func TestFromActor(t *testing.T) {
	id := uuid.Must(uuid.NewV7())
	got, ok := FromActor(Actor(id))
	require.True(t, ok)
	require.Equal(t, id, got)

	for _, actor := range []eventstore.Actor{
		{Type: eventstore.ActorService, ID: "migom-integration"},
		{Type: eventstore.ActorUser, ID: Actor(id).ID},
		{Type: eventstore.ActorService, ID: "supplier:not-a-uuid"},
	} {
		_, ok := FromActor(actor)
		require.False(t, ok, actor)
	}
}
