package orders

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/catalog"
	"goden/backend/internal/inventory"
)

func newID() uuid.UUID { return uuid.Must(uuid.NewV7()) }

func TestValidateItems(t *testing.T) {
	pasta, soup := newID(), newID()

	tests := []struct {
		name  string
		items []OrderItem
		valid bool
	}{
		{"valid", []OrderItem{{pasta, 2}, {soup, 1}}, true},
		{"empty order", nil, false},
		{"zero portions", []OrderItem{{pasta, 0}}, false},
		{"negative portions", []OrderItem{{pasta, -1}}, false},
		{"portions at the limit", []OrderItem{{pasta, MaxPortions}}, true},
		{"portions above the limit", []OrderItem{{pasta, MaxPortions + 1}}, false},
		// 200 единиц на порцию при таком числе порций переполняли потребность до 184
		{"portions that overflowed the need", []OrderItem{{pasta, 92233720368547759}}, false},
		{"dish listed twice", []OrderItem{{pasta, 1}, {pasta, 2}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateItems(tt.items)

			if tt.valid {
				require.NoError(t, err)
				return
			}
			var validation *ValidationError
			require.ErrorAs(t, err, &validation)
		})
	}
}

func TestRequirements(t *testing.T) {
	milk, ham := newID(), newID()
	pasta := catalog.Dish{ID: newID(), Ingredients: []catalog.Ingredient{{ProductID: milk, Quantity: 80}, {ProductID: ham, Quantity: 60}}}
	soup := catalog.Dish{ID: newID(), Ingredients: []catalog.Ingredient{{ProductID: milk, Quantity: 50}}}
	draft := catalog.Dish{ID: newID(), Ingredients: []catalog.Ingredient{}}
	dishes := map[uuid.UUID]catalog.Dish{pasta.ID: pasta, soup.ID: soup, draft.ID: draft}

	t.Run("product used by two dishes is summed", func(t *testing.T) {
		needs, err := requirements(dishes, []OrderItem{{pasta.ID, 2}, {soup.ID, 1}})

		require.NoError(t, err)
		require.ElementsMatch(t, []inventory.Need{
			{ProductID: milk, Quantity: 2*80 + 1*50},
			{ProductID: ham, Quantity: 2 * 60},
		}, needs)
	})

	t.Run("result is ordered by product", func(t *testing.T) {
		needs, err := requirements(dishes, []OrderItem{{pasta.ID, 1}})

		require.NoError(t, err)
		require.Len(t, needs, 2)
		require.Less(t, needs[0].ProductID.String(), needs[1].ProductID.String())
	})

	t.Run("dish without recipe", func(t *testing.T) {
		_, err := requirements(dishes, []OrderItem{{pasta.ID, 1}, {draft.ID, 1}})

		var noRecipe *DishWithoutRecipeError
		require.ErrorAs(t, err, &noRecipe)
		require.Equal(t, draft.ID, noRecipe.DishID)
	})
}

func TestCancelComment(t *testing.T) {
	tests := []struct {
		name, comment, want string
		invalid             bool
	}{
		{name: "trimmed", comment: "  гость ушёл\n", want: "гость ушёл"},
		{name: "blank", comment: "   ", want: ""},
		{name: "limit counts characters", comment: strings.Repeat("я", MaxCancelCommentLength), want: strings.Repeat("я", MaxCancelCommentLength)},
		{name: "too long", comment: strings.Repeat("я", MaxCancelCommentLength+1), invalid: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cancelComment(tt.comment)
			if tt.invalid {
				var invalid *ValidationError
				require.ErrorAs(t, err, &invalid)
				require.Equal(t, "comment", invalid.Field)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
