// Package migrations встраивает SQL-миграции в исполняемые файлы.
//
// Новая миграция - файл <NNNNN>_<имя>.sql с секциями "-- +goose Up" и "-- +goose Down",
// где NNNNN - следующий по порядку номер из пяти цифр. Номера идут подряд без пропусков
// и повторов: это проверяет тест пакета.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
