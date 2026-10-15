-- +goose Up

-- Подключение сервиса доставки Мигом к точке: техническая настройка без событий, как у генератора.
-- Строка на точку; outlet_id - номер заведения в Мигоме, NULL - Мигом отключён. Одно заведение
-- могут указать несколько точек: какая из них его, подтверждает владелец заведения в Мигоме.
CREATE TABLE migom_points (
    point_id   uuid        PRIMARY KEY,
    outlet_id  bigint,
    updated_at timestamptz NOT NULL
);

-- +goose Down
DROP TABLE migom_points;
