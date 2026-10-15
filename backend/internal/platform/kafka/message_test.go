package kafka

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTopicForIntegrationEvents(t *testing.T) {
	tests := map[string]string{
		"ProductCreated":                  CatalogTopic,
		"DishCreated":                     CatalogTopic,
		"RecipeChanged":                   CatalogTopic,
		"DishAvailabilityChanged":         CatalogTopic,
		"HotOfferChanged":                 CatalogTopic,
		"ProductChanged":                  CatalogTopic,
		"ProductArchived":                 CatalogTopic,
		"ProductRestored":                 CatalogTopic,
		"DishRenamed":                     CatalogTopic,
		"DishArchived":                    CatalogTopic,
		"DishRestored":                    CatalogTopic,
		"DeliveryRegistered":              DeliveriesTopic,
		"DeliveryApproved":                DeliveriesTopic,
		"DeliveryApprovalCorrected":       DeliveriesTopic,
		"QuarantinedDeliveryItemAccepted": DeliveriesTopic,
		"QuarantinedDeliveryItemRejected": DeliveriesTopic,
		"BatchReceived":                   StockTopic,
		"StockConsumed":                   StockTopic,
		"StockAdjusted":                   StockTopic,
		"StockWrittenOff":                 StockTopic,
		"StockReturned":                   StockTopic,
		"OrderAccepted":                   OrdersTopic,
		"OrderRejected":                   OrdersTopic,
		"OrderCancelled":                  OrdersTopic,
		"BatchExpiryWarningIssued":        ExpiryTopic,
		"BatchExpired":                    ExpiryTopic,
	}
	for eventType, expected := range tests {
		t.Run(eventType, func(t *testing.T) {
			topic, ok := topicFor(eventType)
			require.True(t, ok)
			require.Equal(t, expected, topic)
		})
	}

	_, ok := topicFor("UnknownEvent")
	require.False(t, ok, "unsupported events still advance the publisher checkpoint")
}

func TestFullTopic(t *testing.T) {
	require.Equal(t, "goden.stock.v1", fullTopic("goden", StockTopic))
	require.Equal(t, "goden.stock.v1", fullTopic("goden.", StockTopic))
	require.Equal(t, "stock.v1", fullTopic("", StockTopic))
	require.Equal(t, "goden.catalog.v1", fullTopic("goden", CatalogTopic))
}
