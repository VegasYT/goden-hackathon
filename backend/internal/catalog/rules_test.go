package catalog

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func days(n int) *int { return &n }

func TestNewProductNormalized(t *testing.T) {
	valid := NewProduct{Name: "Сливки 33%", Unit: UnitMilliliter, Category: "Молочные продукты"}

	tests := []struct {
		name      string
		change    func(p *NewProduct)
		wantField string // пусто - данные корректны
	}{
		{"valid without shelf life", func(*NewProduct) {}, ""},
		{"valid with shelf life", func(p *NewProduct) { p.ShelfLifeDays = days(5) }, ""},
		{"empty name", func(p *NewProduct) { p.Name = "" }, "name"},
		{"name of spaces", func(p *NewProduct) { p.Name = "   " }, "name"},
		{"unknown unit", func(p *NewProduct) { p.Unit = "kg" }, "unit"},
		{"empty unit", func(p *NewProduct) { p.Unit = "" }, "unit"},
		{"zero shelf life", func(p *NewProduct) { p.ShelfLifeDays = days(0) }, "shelf_life_days"},
		{"negative shelf life", func(p *NewProduct) { p.ShelfLifeDays = days(-1) }, "shelf_life_days"},
		{"shelf life at the limit", func(p *NewProduct) { p.ShelfLifeDays = days(MaxShelfLifeDays) }, ""},
		// срок больше 106751 дня переполнял длительность при проверке поставки
		{"shelf life above the limit", func(p *NewProduct) { p.ShelfLifeDays = days(MaxShelfLifeDays + 1) }, "shelf_life_days"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := valid
			tt.change(&in)

			_, err := in.normalized()

			if tt.wantField == "" {
				require.NoError(t, err)
				return
			}
			var validation *ValidationError
			require.ErrorAs(t, err, &validation)
			require.Equal(t, tt.wantField, validation.Field)
		})
	}
}

func TestNewProductNormalizedTrimsSpaces(t *testing.T) {
	got, err := NewProduct{Name: "  Сливки 33% ", Unit: UnitMilliliter, Category: " Молочные "}.normalized()

	require.NoError(t, err)
	require.Equal(t, "Сливки 33%", got.Name)
	require.Equal(t, "Молочные", got.Category)
	require.Equal(t, "сливки 33%", nameKey(got.Name))
}

func TestNormalizedDishName(t *testing.T) {
	name, err := normalizedDishName("  Паста карбонара ")
	require.NoError(t, err)
	require.Equal(t, "Паста карбонара", name)

	_, err = normalizedDishName("  ")
	var validation *ValidationError
	require.ErrorAs(t, err, &validation)
	require.Equal(t, "name", validation.Field)
}

func TestValidateRecipe(t *testing.T) {
	cream, bacon := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())

	tests := []struct {
		name        string
		ingredients []Ingredient
		valid       bool
	}{
		{"valid", []Ingredient{{ProductID: cream, Quantity: 80}, {ProductID: bacon, Quantity: 60}}, true},
		{"empty", nil, false},
		{"product listed twice", []Ingredient{{ProductID: cream, Quantity: 80}, {ProductID: cream, Quantity: 20}}, false},
		{"zero quantity", []Ingredient{{ProductID: cream, Quantity: 0}}, false},
		{"negative quantity", []Ingredient{{ProductID: cream, Quantity: -5}}, false},
		{"quantity at the limit", []Ingredient{{ProductID: cream, Quantity: MaxQuantity}}, true},
		{"quantity above the limit", []Ingredient{{ProductID: cream, Quantity: MaxQuantity + 1}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRecipe(tt.ingredients)

			if tt.valid {
				require.NoError(t, err)
				return
			}
			var validation *ValidationError
			require.ErrorAs(t, err, &validation)
			require.Equal(t, "ingredients", validation.Field)
		})
	}
}

func TestProductChangeKeepsFieldsNotSent(t *testing.T) {
	shelfLife, newShelfLife := 5, 7
	product := Product{Name: "Сливки 3%", Unit: UnitMilliliter, Category: "Молочные", ShelfLifeDays: &shelfLife}
	name := "Сливки 33%"

	renamed := ProductChange{Name: &name}.applied(product)
	require.Equal(t, NewProduct{Name: name, Unit: UnitMilliliter, Category: "Молочные", ShelfLifeDays: &shelfLife}, renamed)
	require.False(t, renamed.sameAs(product))

	require.Nil(t, ProductChange{ClearShelfLife: true}.applied(product).ShelfLifeDays)
	require.Equal(t, &newShelfLife, ProductChange{ShelfLifeDays: &newShelfLife}.applied(product).ShelfLifeDays)
	require.True(t, ProductChange{}.applied(product).sameAs(product), "an empty change changes nothing")
}

func TestNamePartEscapesPatternSigns(t *testing.T) {
	require.Equal(t, `филе`, namePart("  ФИЛЕ "))
	require.Equal(t, `100\%`, namePart("100%"))
	require.Equal(t, `a\_b\\c`, namePart(`a_b\c`))
}
