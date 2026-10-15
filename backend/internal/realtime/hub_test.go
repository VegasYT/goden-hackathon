package realtime

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// scope - точка, на которую подписаны клиенты в тестах пакета.
var scope = Scope{PointID: uuid.Must(uuid.NewV7()), CompanyID: uuid.Must(uuid.NewV7())}

func TestHubPublishesToEverySubscriber(t *testing.T) {
	hub := newHub(2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first := hub.Subscribe(ctx, scope)
	second := hub.Subscribe(ctx, scope)
	update := Update{ID: uuid.Must(uuid.NewV7()), PointID: scope.PointID, Categories: []Category{CategoryStock}}

	hub.Publish(update)

	require.Equal(t, update, <-first)
	require.Equal(t, update, <-second)
}

func TestHubDisconnectsOnlySlowSubscriber(t *testing.T) {
	hub := newHub(1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	slow := hub.Subscribe(ctx, scope)
	fast := hub.Subscribe(ctx, scope)
	first := Update{ID: uuid.Must(uuid.NewV7()), PointID: scope.PointID}
	second := Update{ID: uuid.Must(uuid.NewV7()), PointID: scope.PointID}

	hub.Publish(first)
	require.Equal(t, first, <-fast)
	hub.Publish(second)

	require.Equal(t, second, <-fast)
	require.Equal(t, first, <-slow)
	_, open := <-slow
	require.False(t, open)
}

func TestHubClosesSubscriptionWithContext(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	updates := hub.Subscribe(ctx, scope)
	cancel()

	_, open := <-updates
	require.False(t, open)
}

// При остановке сервера hub закрывает подписки: обработчик потока видит закрытый канал
// и возвращается, а новые подписки закрыты сразу.
func TestHubCloseEndsSubscriptions(t *testing.T) {
	hub := newHub(2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	existing := hub.Subscribe(ctx, scope)

	hub.Close()

	_, open := <-existing
	require.False(t, open, "an existing subscription is closed")
	_, open = <-hub.Subscribe(ctx, scope)
	require.False(t, open, "a new subscription is closed at once")
	hub.Publish(Update{ID: uuid.Must(uuid.NewV7()), PointID: scope.PointID}) // публикация после закрытия не паникует
	hub.Close()                                                              // повторное закрытие тоже
}

// Сигнал получает подписчик точки события или, для каталога, любой точки компании события.
func TestHubDeliversByPointAndCompany(t *testing.T) {
	hub := newHub(4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sibling := Scope{PointID: uuid.Must(uuid.NewV7()), CompanyID: scope.CompanyID}
	stranger := Scope{PointID: uuid.Must(uuid.NewV7()), CompanyID: uuid.Must(uuid.NewV7())}
	own, sameCompany, other := hub.Subscribe(ctx, scope), hub.Subscribe(ctx, sibling), hub.Subscribe(ctx, stranger)

	stock := Update{ID: uuid.Must(uuid.NewV7()), PointID: scope.PointID}
	catalog := Update{ID: uuid.Must(uuid.NewV7()), CompanyID: scope.CompanyID}
	hub.Publish(stock)
	hub.Publish(catalog)
	hub.Publish(Update{ID: uuid.Must(uuid.NewV7())}) // без адресата: никому

	require.Equal(t, stock, <-own)
	require.Equal(t, catalog, <-own)
	require.Equal(t, catalog, <-sameCompany, "the sibling point gets only the catalog signal")
	require.Empty(t, own)
	require.Empty(t, sameCompany)
	require.Empty(t, other)
}
