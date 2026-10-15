package stoplist

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
)

func TestAvailability(t *testing.T) {
	cream, bacon, eggs := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	level := func(id uuid.UUID, available, expired int64) inventory.StockLevel {
		return inventory.StockLevel{Product: catalog.Product{ID: id}, Available: available, Expired: expired}
	}
	per := func(id uuid.UUID, quantity int64) catalog.Ingredient {
		return catalog.Ingredient{ProductID: id, Quantity: quantity}
	}
	limiting := func(id uuid.UUID, required, available, expired int64) LimitingIngredient {
		return LimitingIngredient{Product: catalog.Product{ID: id}, Required: required, Available: available, Expired: expired}
	}

	tests := []struct {
		name         string
		ingredients  []catalog.Ingredient
		stock        []inventory.StockLevel
		wantPortions int64
		wantReason   Reason
		wantLimiting []LimitingIngredient
	}{
		{
			name:         "the scarcest ingredient limits the portions",
			ingredients:  []catalog.Ingredient{per(cream, 100), per(bacon, 50)},
			stock:        []inventory.StockLevel{level(cream, 350, 0), level(bacon, 500, 0)},
			wantPortions: 3, wantLimiting: []LimitingIngredient{limiting(cream, 100, 350, 0)},
		},
		{
			name:         "portions are rounded down",
			ingredients:  []catalog.Ingredient{per(cream, 100)},
			stock:        []inventory.StockLevel{level(cream, 199, 0)},
			wantPortions: 1, wantLimiting: []LimitingIngredient{limiting(cream, 100, 199, 0)},
		},
		{
			name:         "exactly one portion",
			ingredients:  []catalog.Ingredient{per(cream, 100)},
			stock:        []inventory.StockLevel{level(cream, 100, 0)},
			wantPortions: 1, wantLimiting: []LimitingIngredient{limiting(cream, 100, 100, 0)},
		},
		{
			name:        "not enough for a portion",
			ingredients: []catalog.Ingredient{per(cream, 100), per(bacon, 60)},
			stock:       []inventory.StockLevel{level(cream, 1000, 0), level(bacon, 40, 0)},
			wantReason:  InsufficientStock, wantLimiting: []LimitingIngredient{limiting(bacon, 60, 40, 0)},
		},
		{
			name:         "two products are missing: both are listed",
			ingredients:  []catalog.Ingredient{per(cream, 100), per(bacon, 60), per(eggs, 1)},
			stock:        []inventory.StockLevel{level(cream, 30, 0), level(bacon, 40, 0), level(eggs, 10, 0)},
			wantReason:   InsufficientStock,
			wantLimiting: []LimitingIngredient{limiting(cream, 100, 30, 0), limiting(bacon, 60, 40, 0)},
		},
		{
			name:        "the product was never delivered",
			ingredients: []catalog.Ingredient{per(cream, 100), per(bacon, 60)},
			stock:       []inventory.StockLevel{level(cream, 1000, 0)},
			wantReason:  InsufficientStock, wantLimiting: []LimitingIngredient{limiting(bacon, 60, 0, 0)},
		},
		{
			name:        "expired stock is not available",
			ingredients: []catalog.Ingredient{per(cream, 100)},
			stock:       []inventory.StockLevel{level(cream, 0, 500)},
			wantReason:  InsufficientStock, wantLimiting: []LimitingIngredient{limiting(cream, 100, 0, 500)},
		},
		{
			name:         "several ingredients limit equally",
			ingredients:  []catalog.Ingredient{per(cream, 100), per(bacon, 50), per(eggs, 1)},
			stock:        []inventory.StockLevel{level(cream, 250, 0), level(bacon, 100, 0), level(eggs, 30, 0)},
			wantPortions: 2,
			wantLimiting: []LimitingIngredient{limiting(cream, 100, 250, 0), limiting(bacon, 50, 100, 0)},
		},
		{name: "no recipe", wantReason: NoRecipe},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stock := make(map[uuid.UUID]inventory.StockLevel, len(tt.stock))
			for _, l := range tt.stock {
				stock[l.Product.ID] = l
			}
			dish := catalog.Dish{ID: uuid.Must(uuid.NewV7()), Name: "Блюдо", Ingredients: tt.ingredients}

			got := availability(dish, stock)

			require.Equal(t, dish, got.Dish)
			require.Equal(t, tt.wantPortions, got.Portions)
			require.Equal(t, tt.wantPortions > 0, got.Available())
			require.Equal(t, tt.wantReason, got.Reason)
			require.Equal(t, tt.wantLimiting, got.Limiting)
			// вся рецептура в её порядке, а не только ограничивающие продукты
			require.Len(t, got.Ingredients, len(tt.ingredients))
			for i, ingredient := range tt.ingredients {
				on := stock[ingredient.ProductID]
				require.Equal(t, limiting(ingredient.ProductID, ingredient.Quantity, on.Available, on.Expired), got.Ingredients[i])
			}
		})
	}
}
