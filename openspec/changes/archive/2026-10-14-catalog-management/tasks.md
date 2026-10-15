# Tasks

Часть репо: backend. Команды запускаются из `backend/`. Линтер — `GOTOOLCHAIN=go1.27.2 go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...`, генерация — команды ogen и sqlc из `backend/AGENTS.md`.

## 1. Контракт

- [x] 1.1 В `api/openapi.yaml`:
  - параметры списков продуктов (`q`, `category`, `status`, `sort`) и блюд (`q`, `status`, `recipe`);
  - поля `archived`, `dishes`, `unit_locked` у `Product`, `used_in` у продукта по идентификатору;
  - схема ответа `DishIngredient` (`product_id`, `quantity`, `product_name`, `unit`, `product_archived`), `archived` у `Dish`;
  - необязательные `ingredients` в `NewDish`;
  - операции `PATCH` продукта и блюда, `archive` и `restore` обоих, `GET /companies/{companyId}/product-categories`;
  - значение `catalog` в `RealtimeCategory`.

  Перегенерировать ogen. Проверка: `go build ./...`. Каждая новая операция есть в `operations` тестов `internal/api` (`TestEveryOperationHasARequest`).

## 2. Домен каталога

- [x] 2.1 События `ProductChanged`, `ProductArchived`, `ProductRestored`, `DishRenamed`, `DishArchived`, `DishRestored` с `company_id`:
  - реестр типов;
  - эталоны `estest.Golden`;
  - `apply*` в таблицы записи и `Replayers`;
  - миграция `archived` у продуктов и блюд.

  Проверка: `go test ./internal/catalog/...`, `TestRebuildKnowsEveryEventOfTheModule`, `go test ./migrations/`.
- [x] 2.2 Команды:
  - `ChangeProduct` — изменение без отличий не пишет событие, единица по правилу «неиспользованного продукта»;
  - `ArchiveProduct` и `RestoreProduct` — `product-in-use`, если продукт в рецептуре неархивного блюда;
  - `RenameDish`;
  - `ArchiveDish` и `RestoreDish` — `archived-ingredients` при восстановлении;
  - `CreateDish` с необязательной рецептурой, общая проверка рецептуры с `ChangeRecipe`;
  - рецептура отклоняет архивный продукт;
  - блокировка потока через `Spec.Locks`, отказ в отключённой компании.

  Проверка: unit-тесты чистых решений и интеграционные тесты команд по сценариям спецификации `catalog`.
- [x] 2.3 Интерфейс `catalog.ProductUsage`, его реализация в `inventory` (позиции поставок любых статусов и партии) и подключение в `cmd/api`. Проверка: интеграционный тест — единица меняется у нового продукта и не меняется после регистрации непринятой поставки.
- [x] 2.4 Чтения:
  - фильтры и поиск списков с экранированием `%` и `_`, сортировка `category` (без категории в конце);
  - `dishes`, `used_in`, `unit_locked`;
  - категории компании;
  - ингредиенты с данными продукта.

  Проверка: интеграционные тесты списков через `apitest`: поиск по части названия, категория, архив, блюда без рецептуры, страница.

## 3. Другие модули

- [x] 3.1 Архивный продукт:
  - регистрация поставки через `ActiveProductOf` отвечает 422, приёмка и исправление уже зарегистрированной поставки работают;
  - `StockView` показывает архивный продукт, только пока у него есть партии;
  - рекомендации поставок его пропускают.

  Проверка: интеграционные тесты `inventory` и `recommendations` по сценариям «Архив продукта».
- [x] 3.2 Архивное блюдо:
  - `DishesOnPoint` и список блюд точки его не возвращают;
  - `DisabledOnPoint` считает его не продающимся, заказ получает 422;
  - выключение на точке переживает восстановление;
  - проекция `analytics` обрабатывает `DishArchived` и `DishRestored`, `Version` увеличена.

  Проверка: интеграционные тесты `stoplist`, `orders` и `analytics` по сценариям «Архив блюда», `TestMenuAgreesWithOrders`.
- [x] 3.3 Realtime: категория `catalog` у всех событий каталога, дополнительные категории изменения продукта и блюда по спецификации `realtime-updates`, `sync` перечисляет `catalog`. Проверка: табличный тест `FromEnvelope` в `internal/realtime`.

## 4. Ошибки

- [x] 4.1 Тип `NameTakenError{Archived}` в `catalog` и `organization`, деталь поля `email`/`unknown` у `ErrUnknownUser`. `problemFor` кладёт `errors[]` при прежних статусах и `type`, у новых конфликтов свои `type`: `product-in-use`, `unit-locked`, `archived-ingredients`. Проверка: тесты `internal/api` — 409 с полем `name` и `code` у продукта, точки и компании, 422 с полем `email` у назначения и передачи компании.

## 5. Документация и итог

- [x] 5.1 Обновить `backend/AGENTS.md`:
  - архив вместо удаления в каталоге;
  - `ProductUsage`;
  - новые события и категории realtime;
  - фраза «пока в каталоге нет переименования» в разделе «Устройство слоя API» становится описанием того, что названия подставляются при ответе текущими.

  Проверка: упомянутые пути и имена существуют.
- [x] 5.2 Прогнать сборку, `go test -tags=integration ./...` и линтер. Проверка: всё проходит.
