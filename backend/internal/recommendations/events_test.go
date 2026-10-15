package recommendations_test

import (
	"testing"

	"github.com/google/uuid"

	"goden/backend/internal/platform/eventstore/estest"
	"goden/backend/internal/recommendations"
)

func TestEventContract(t *testing.T) {
	estest.Golden(t, recommendations.HotOfferChanged{
		PointID: uuid.MustParse("0198c0de-0000-7000-8000-000000000012"),
		DishID:  uuid.MustParse("0198c0de-0000-7000-8000-000000000002"),
		Mode:    recommendations.HotOfferPinned, Comment: "Хорошо идёт к белому вину",
	})
}
