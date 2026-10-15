package catalog_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/catalog"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/eventstore/estest"
)

var (
	productID = uuid.MustParse("0198c0de-0000-7000-8000-000000000001")
	dishID    = uuid.MustParse("0198c0de-0000-7000-8000-000000000002")
	pointID   = uuid.MustParse("0198c0de-0000-7000-8000-000000000012")
	// компания, в которой тесты пакета создают каталог
	companyID = uuid.MustParse("0198c0de-0000-7000-8000-000000000011")
)

func TestEventContract(t *testing.T) {
	shelfLife := 5
	samples := []eventstore.Payload{
		catalog.ProductCreated{
			ProductID: productID, CompanyID: companyID, Name: "Сливки 33%", Unit: catalog.UnitMilliliter,
			Category: "Молочные продукты", ShelfLifeDays: &shelfLife,
		},
		catalog.DishCreated{DishID: dishID, CompanyID: companyID, Name: "Паста карбонара"},
		catalog.DishAvailabilityChanged{PointID: pointID, DishID: dishID, Enabled: false},
		catalog.RecipeChanged{DishID: dishID, CompanyID: companyID, Ingredients: []catalog.RecipeIngredient{
			{ProductID: productID, Quantity: 80},
		}},
		catalog.ProductChanged{
			ProductID: productID, CompanyID: companyID, Name: "Сливки 33%", Unit: catalog.UnitMilliliter,
			Category: "Молочные продукты", ShelfLifeDays: &shelfLife,
		},
		catalog.ProductArchived{ProductID: productID, CompanyID: companyID},
		catalog.ProductRestored{ProductID: productID, CompanyID: companyID},
		catalog.DishRenamed{DishID: dishID, CompanyID: companyID, Name: "Салат Цезарь с курицей"},
		catalog.DishArchived{DishID: dishID, CompanyID: companyID},
		catalog.DishRestored{DishID: dishID, CompanyID: companyID},
	}
	for _, sample := range samples {
		t.Run(sample.EventType(), func(t *testing.T) { estest.Golden(t, sample) })
	}
}

func TestStreamNames(t *testing.T) {
	require.Equal(t, "product-0198c0de-0000-7000-8000-000000000001", catalog.ProductStream(productID))
	require.Equal(t, "dish-0198c0de-0000-7000-8000-000000000002", catalog.DishStream(dishID))
	require.Equal(t, "menu-0198c0de-0000-7000-8000-000000000012", catalog.MenuStream(pointID))
}

func TestUnitIsClosedSet(t *testing.T) {
	_, err := eventstore.Decode("ProductCreated", 1, json.RawMessage(`{"unit":"kg"}`))

	require.ErrorContains(t, err, "kg")
}
