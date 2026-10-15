package recommendations

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	_ "time/tzdata" // часовой пояс точки не зависит от базы поясов в контейнере

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"goden/backend/internal/platform/command"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/recommendations/db"
	"goden/backend/internal/stoplist"
)

// MaxCommentLength - самый длинный комментарий к ручному предложению.
const MaxCommentLength = 500

// HotOfferSetting - режим блюда в горячих предложениях точки, кто и когда его задал.
type HotOfferSetting struct {
	DishID  uuid.UUID        `json:"dish_id"`
	Mode    HotOfferMode     `json:"mode"`
	Comment string           `json:"comment"`
	SetBy   eventstore.Actor `json:"set_by"`
	SetAt   time.Time        `json:"set_at"`
	// HiddenUntil - до какого момента действует скрытие.
	HiddenUntil *time.Time `json:"hidden_until,omitempty"`
}

// SetHotOffer задаёт блюду режим горячего предложения на точке. Точку и принадлежность блюда
// её компании проверяет вызывающий. Скрытие действует до конца текущего дня в часовом поясе timeZone
// (имя IANA, нужно только для hidden). Повтор действующего режима с тем же комментарием события не пишет.
func (s *Service) SetHotOffer(
	ctx context.Context, actor eventstore.Actor, key string, pointID, dishID uuid.UUID, mode HotOfferMode, comment, timeZone string,
) (HotOfferSetting, error) {
	comment = strings.TrimSpace(comment)
	var location *time.Location
	switch {
	case mode != HotOfferPinned && mode != HotOfferHidden && mode != HotOfferAuto:
		return HotOfferSetting{}, &ValidationError{Field: "mode", Message: "must be one of pinned, hidden, auto"}
	case len([]rune(comment)) > MaxCommentLength:
		return HotOfferSetting{}, &ValidationError{Field: "comment", Message: fmt.Sprintf("must not exceed %d characters", MaxCommentLength)}
	case mode == HotOfferHidden:
		var err error
		// пустое имя и Local дали бы часовой пояс сервера, а не точки
		if location, err = time.LoadLocation(timeZone); err != nil || timeZone == "" || timeZone == "Local" {
			return HotOfferSetting{}, &ValidationError{Field: "time_zone", Message: "must be a known IANA time zone"}
		}
		fallthrough
	case mode != HotOfferPinned:
		comment = "" // комментарий нужен официанту только у добавленного вручную блюда
	}

	stream := HotOffersStream(pointID)
	spec := command.Spec{
		Name: "SetHotOffer", Actor: actor, IdempotencyKey: key, Locks: []string{stream},
		Request: struct {
			PointID, DishID uuid.UUID
			Mode            HotOfferMode
			Comment         string
			TimeZone        string
		}{pointID, dishID, mode, comment, timeZone},
	}
	return command.Run(ctx, s.runner, spec, func(ctx context.Context, tx *command.Tx) (HotOfferSetting, error) {
		q := db.New(tx)
		current, err := q.GetHotOffer(ctx, db.GetHotOfferParams{PointID: pointID, DishID: dishID})
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			current = db.GetHotOfferRow{Mode: string(HotOfferAuto)}
		case err != nil:
			return HotOfferSetting{}, fmt.Errorf("recommendations: read hot offer: %w", err)
		}
		now := tx.Now().Truncate(time.Microsecond)
		if current.HiddenUntil != nil && !current.HiddenUntil.After(now) {
			current = db.GetHotOfferRow{Mode: string(HotOfferAuto)} // скрытие истекло
		}
		setting := HotOfferSetting{DishID: dishID, Mode: mode, Comment: comment, SetBy: actor, SetAt: now}
		if location != nil {
			y, m, d := now.In(location).Date()
			until := time.Date(y, m, d+1, 0, 0, 0, 0, location).UTC()
			setting.HiddenUntil = &until
		}
		// повторное скрытие в тот же день ничего не меняет
		if HotOfferMode(current.Mode) == mode && current.Comment == comment && equalTimes(current.HiddenUntil, setting.HiddenUntil) {
			return setting, nil
		}

		version, err := eventstore.StreamVersion(ctx, tx, stream)
		if err != nil {
			return HotOfferSetting{}, err
		}
		event := HotOfferChanged{PointID: pointID, DishID: dishID, Mode: mode, Comment: comment, HiddenUntil: setting.HiddenUntil}
		if _, err := tx.Append(ctx, stream, version, event); err != nil {
			return HotOfferSetting{}, err
		}
		if err := applyHotOfferChanged(ctx, q, event, actor, setting.SetAt); err != nil {
			return HotOfferSetting{}, err
		}
		return setting, nil
	})
}

// HotOfferSettings возвращает блюда точки, добавленные вручную или скрытые сейчас, в порядке изменения.
func (s *Service) HotOfferSettings(ctx context.Context, pointID uuid.UUID) ([]HotOfferSetting, error) {
	rows, err := db.New(s.pool).ListHotOffers(ctx, db.ListHotOffersParams{PointID: pointID, Now: s.now()})
	if err != nil {
		return nil, fmt.Errorf("recommendations: list hot offers: %w", err)
	}
	settings := make([]HotOfferSetting, len(rows))
	for i, row := range rows {
		settings[i] = HotOfferSetting{
			DishID: row.DishID, Mode: HotOfferMode(row.Mode), Comment: row.Comment,
			SetBy: eventstore.Actor{Type: eventstore.ActorType(row.SetByType), ID: row.SetByID}, SetAt: row.SetAt.UTC(),
			HiddenUntil: utc(row.HiddenUntil),
		}
	}
	return settings, nil
}

// applyHotOfferChanged - единственное изменение таблицы ручных настроек: им же её перестраивают из событий.
func applyHotOfferChanged(ctx context.Context, q *db.Queries, e HotOfferChanged, actor eventstore.Actor, at time.Time) error {
	var err error
	if e.Mode == HotOfferAuto {
		err = q.DeleteHotOffer(ctx, db.DeleteHotOfferParams{PointID: e.PointID, DishID: e.DishID})
	} else {
		err = q.SaveHotOffer(ctx, db.SaveHotOfferParams{
			PointID: e.PointID, DishID: e.DishID, Mode: string(e.Mode), Comment: e.Comment,
			SetByType: string(actor.Type), SetByID: actor.ID, SetAt: at, HiddenUntil: e.HiddenUntil,
		})
	}
	if err != nil {
		return fmt.Errorf("recommendations: save hot offer: %w", err)
	}
	return nil
}

func equalTimes(a, b *time.Time) bool {
	return a == nil && b == nil || a != nil && b != nil && a.Equal(*b)
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// withSettings накладывает ручные настройки на расчёт: скрытые блюда убираются, добавленные
// вручную и доступные идут первыми в порядке добавления, с причинами расчёта, если они есть.
// menu - доступность блюд точки, settings - в порядке изменения.
func withSettings(offers []DishOffer, menu []stoplist.Availability, settings []HotOfferSetting) []DishOffer {
	modes := make(map[uuid.UUID]HotOfferSetting, len(settings))
	for _, setting := range settings {
		modes[setting.DishID] = setting
	}
	computed := make(map[uuid.UUID]DishOffer, len(offers))
	for _, offer := range offers {
		computed[offer.Dish.ID] = offer
	}
	available := make(map[uuid.UUID]stoplist.Availability, len(menu))
	for _, a := range menu {
		if a.Available() {
			available[a.Dish.ID] = a
		}
	}

	var result []DishOffer
	for _, setting := range settings {
		a, ok := available[setting.DishID]
		if setting.Mode != HotOfferPinned || !ok {
			continue // недоступное блюдо ждёт, пока его снова можно будет приготовить
		}
		offer, ok := computed[setting.DishID]
		if !ok {
			offer = DishOffer{Dish: a.Dish, Portions: a.Portions}
		}
		manual := setting
		offer.Manual = &manual
		result = append(result, offer)
	}
	for _, offer := range offers {
		if _, set := modes[offer.Dish.ID]; !set {
			result = append(result, offer)
		}
	}
	return slices.Clip(result)
}
