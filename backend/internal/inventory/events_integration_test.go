//go:build integration

package inventory_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"goden/backend/internal/inventory"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/eventstore/estest"
	"goden/backend/internal/platform/postgres/pgtest"
)

// Образец для тестов проекций: события кладутся в Event Store помощником, без команд.
func TestEventsRoundTripThroughEventStore(t *testing.T) {
	ctx, pool := context.Background(), pgtest.NewDB(t)
	stream := inventory.StockStream(point.ID, productID)
	received := inventory.BatchReceived{
		PointID: point.ID, BatchID: batchID, ProductID: productID, DeliveryID: deliveryID, ItemID: itemID,
		Quantity: 1500, ExpiresAt: expiresAt, ReceivedAt: approvedAt, Source: inventory.BatchFromApproval,
	}
	consumed := inventory.StockConsumed{
		PointID: point.ID, ProductID: productID, OrderID: orderID,
		Batches: []inventory.BatchQuantity{{BatchID: batchID, Quantity: 200}},
	}

	estest.Append(t, pool, stream, received)
	estest.AppendAt(t, pool, stream, approvedAt.Time, consumed) // время расхода задаётся явно

	events, err := eventstore.ReadStream(ctx, pool, stream, 1)
	require.NoError(t, err)
	require.Len(t, events, 2)

	var decoded []eventstore.Payload
	for _, e := range events {
		p, err := eventstore.Decode(e.Type, e.SchemaVersion, e.Payload)
		require.NoError(t, err)
		decoded = append(decoded, p)
	}
	require.Equal(t, []eventstore.Payload{received, consumed}, decoded)
	require.Equal(t, approvedAt.Time, events[1].OccurredAt)
}
