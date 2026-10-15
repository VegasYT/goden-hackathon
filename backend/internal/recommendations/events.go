package recommendations

import (
	"time"

	"github.com/google/uuid"

	"goden/backend/internal/platform/eventstore"
)

func init() {
	eventstore.Register[HotOfferChanged]()
}

// HotOffersStream - поток ручных настроек горячих предложений точки.
func HotOffersStream(pointID uuid.UUID) string { return "hot-offers-" + pointID.String() }

// HotOfferMode - режим блюда в горячих предложениях точки.
type HotOfferMode string

const (
	// HotOfferPinned - блюдо добавлено вручную и идёт первым, пока его можно приготовить.
	HotOfferPinned HotOfferMode = "pinned"
	// HotOfferHidden - блюдо не показывается в горячих предложениях точки до конца дня.
	HotOfferHidden HotOfferMode = "hidden"
	// HotOfferAuto - решает расчёт.
	HotOfferAuto HotOfferMode = "auto"
)

func (m *HotOfferMode) UnmarshalText(text []byte) error {
	return eventstore.ParseEnum(m, text, "hot offer mode", HotOfferPinned, HotOfferHidden, HotOfferAuto)
}

// HotOfferChanged - режим блюда в горячих предложениях точки после изменения.
type HotOfferChanged struct {
	PointID uuid.UUID    `json:"point_id"`
	DishID  uuid.UUID    `json:"dish_id"`
	Mode    HotOfferMode `json:"mode"`
	// Comment - комментарий для официанта, есть только у pinned.
	Comment string `json:"comment,omitempty"`
	// HiddenUntil - конец дня точки, до которого действует скрытие; есть только у hidden.
	HiddenUntil *time.Time `json:"hidden_until,omitempty"`
}

func (HotOfferChanged) EventType() string  { return "HotOfferChanged" }
func (HotOfferChanged) SchemaVersion() int { return 1 }
