// Package contract_test проверяет каталог доменных событий целиком, по всем модулям сразу.
package contract_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"goden/backend/internal/platform/eventstore"
	// события регистрируются в пакетах модулей; каталог берётся через readmodel, как в worker:
	// проекции декодируют каждое событие, и тип, которого worker не знает, остановил бы их
	_ "goden/backend/internal/readmodel"
)

// Новый тип события или новая версия схемы добавляются сюда осознанно:
// каталог - контракт между записью и чтением.
func TestEventCatalog(t *testing.T) {
	want := []string{
		"BatchExpired",
		"BatchExpiryWarningIssued",
		"BatchReceived",
		"CompanyCreated",
		"CompanyDisabled",
		"CompanyOwnerChanged",
		"CompanyRenamed",
		"DeliveryApprovalCorrected",
		"DeliveryApproved",
		"DeliveryRegistered",
		"DishArchived",
		"DishAvailabilityChanged",
		"DishCreated",
		"DishRenamed",
		"DishRestored",
		"HotOfferChanged",
		"OrderAccepted",
		"OrderCancelled",
		"OrderRejected",
		"PointCreated",
		"PointDisabled",
		"PointRenamed",
		"PointTelegramChatChanged",
		"ProductArchived",
		"ProductChanged",
		"ProductCreated",
		"ProductRestored",
		"QuarantinedDeliveryItemAccepted",
		"QuarantinedDeliveryItemRejected",
		"RecipeChanged",
		"StaffAssigned",
		"StaffRemoved",
		"StaffRoleChanged",
		"StockAdjusted",
		"StockConsumed",
		"StockReturned",
		"StockWrittenOff",
	}

	var got []string
	for _, info := range eventstore.RegisteredTypes() {
		require.Equal(t, 1, info.SchemaVersion, info.Type)
		got = append(got, info.Type)
	}

	require.Equal(t, want, got)
}
