package realtime

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	platformkafka "goden/backend/internal/platform/kafka"
)

func TestFromEnvelope(t *testing.T) {
	tests := map[string][]Category{
		"ProductCreated":          {CategoryCatalog, CategoryRecommendations, CategoryStock},
		"DishCreated":             {CategoryCatalog, CategoryStopList},
		"RecipeChanged":           {CategoryCatalog, CategoryRecommendations, CategoryStopList},
		"DishAvailabilityChanged": {CategoryCatalog, CategoryRecommendations, CategoryStopList},
		"ProductChanged": {
			CategoryCatalog, CategoryDeliveries, CategoryOrders, CategoryRecommendations, CategoryStock, CategoryStopList,
		},
		"ProductArchived":          {CategoryCatalog, CategoryRecommendations, CategoryStopList},
		"ProductRestored":          {CategoryCatalog, CategoryRecommendations, CategoryStopList},
		"DishRenamed":              {CategoryCatalog, CategoryOrders, CategoryRecommendations, CategoryStopList},
		"DishArchived":             {CategoryCatalog, CategoryRecommendations, CategoryStopList},
		"DishRestored":             {CategoryCatalog, CategoryRecommendations, CategoryStopList},
		"BatchReceived":            stockCategories,
		"StockConsumed":            stockCategories,
		"StockAdjusted":            stockCategories,
		"StockWrittenOff":          stockCategories,
		"StockReturned":            stockCategories,
		"BatchExpiryWarningIssued": {CategoryExpiry},
		"BatchExpired":             {CategoryExpiry, CategoryRecommendations, CategoryStock, CategoryStopList},
		// складские категории принятому заказу дают его события StockConsumed
		"OrderAccepted":   {CategoryOrders},
		"HotOfferChanged": {CategoryRecommendations},
		"OrderRejected":   {CategoryOrders},
		"OrderCancelled":  {CategoryOrders},
		// складские события поставок не будят: иначе их перечитывал бы каждый заказ
		"DeliveryRegistered":              {CategoryDeliveries},
		"DeliveryApproved":                {CategoryDeliveries},
		"DeliveryApprovalCorrected":       {CategoryDeliveries},
		"QuarantinedDeliveryItemAccepted": {CategoryDeliveries},
		"QuarantinedDeliveryItemRejected": {CategoryDeliveries},
	}
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	for eventType, expected := range tests {
		t.Run(eventType, func(t *testing.T) {
			id := uuid.Must(uuid.NewV7())
			update, ok := FromEnvelope(platformkafka.Envelope{
				EventID: id, EventType: eventType, OccurredAt: now,
				Payload: json.RawMessage(`{"point_id":"` + scope.PointID.String() + `","company_id":"` + scope.CompanyID.String() + `"}`),
			})
			require.True(t, ok)
			// адресат берётся из события; в JSON сигнала он не попадает
			require.Equal(t, Update{
				ID: id, OccurredAt: now, Categories: expected, PointID: scope.PointID, CompanyID: scope.CompanyID,
			}, update)
			raw, err := json.Marshal(update)
			require.NoError(t, err)
			require.NotContains(t, string(raw), scope.PointID.String())
			require.NotContains(t, string(raw), scope.CompanyID.String())
			require.True(t, slicesAreSorted(update.Categories))
		})
	}
}

func TestFromEnvelopeIgnoresEventsWithoutInvalidation(t *testing.T) {
	for _, eventType := range []string{"CompanyOwnerChanged", "UnknownEvent"} {
		_, ok := FromEnvelope(platformkafka.Envelope{EventType: eventType})
		require.False(t, ok, eventType)
	}
}

func slicesAreSorted(categories []Category) bool {
	for i := 1; i < len(categories); i++ {
		if categories[i-1] > categories[i] {
			return false
		}
	}
	return true
}
