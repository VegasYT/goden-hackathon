-- +goose Up

-- Ручные настройки горячих предложений точки: блюдо добавлено вручную или скрыто.
-- Режим auto строки не имеет, скрытие действует до hidden_until (конец дня точки), потом решает расчёт. Таблица записи модуля рекомендаций, перестраивается из HotOfferChanged.
CREATE TABLE recommendations_hot_offers (
    point_id    uuid        NOT NULL,
    dish_id     uuid        NOT NULL,
    mode        text        NOT NULL CHECK (mode IN ('pinned', 'hidden')),
    comment     text        NOT NULL DEFAULT '',
    set_by_type text        NOT NULL,
    set_by_id   text        NOT NULL,
    set_at      timestamptz NOT NULL,
    hidden_until timestamptz,
    PRIMARY KEY (point_id, dish_id)
);

-- +goose Down

DROP TABLE recommendations_hot_offers;
