# Design

## Context

Мотивация и границы - в proposal.md. Что уже есть в коде:

- `internal/catalog/events.go`: события `ProductCreated`, `DishCreated`, `RecipeChanged`, тип `Unit`, функции `ProductStream` и `DishStream`, эталонные JSON в `testdata/`.
- `command.Run` с транзакцией, идемпотентностью и повтором при конфликте версий; `Tx.Append` заполняет конверт события.
- `pgtest.NewDB` для интеграционных тестов; правило `depguard`, которое запрещает другим модулям импортировать `catalog/db`.
- sqlc v1.31.1 проверен на Windows, в проекте ещё не использовался.

## Goals / Non-Goals

**Goals:**

- Каталог становится образцом доменного модуля: по нему пишутся поставки и заказы.
- Проверки существования продуктов и уникальности названий не зависят от асинхронных проекций.
- Будущий HTTP-слой вызывает готовые методы и не содержит правил.

**Non-Goals:**

- HTTP-эндпоинты, пагинация и фильтры списков: вместе с каркасом API. Списки каталога малы, функции чтения возвращают их целиком.
- Перестроение таблиц каталога из событий. Запись в таблицы вынесена в функции применения событий, чтобы перестроение позже свелось к их вызову в цикле; сам replay относится к задаче A9.
- Чтение каталога внутри чужой транзакции. Понадобится заказам (A4) для сверки набора блокировок; сигнатура определяется там, когда будет виден настоящий вызов.

## Decisions

### 1. Устройство модуля

По правилам `backend/AGENTS.md`:

| Файл | Содержимое |
| --- | --- |
| `events.go` | уже есть, не меняется |
| `product.go` | типы `Product`, `NewProduct`, чистая проверка входных данных |
| `dish.go` | типы `Dish`, `Ingredient`, чистая проверка названия и состава рецептуры |
| `service.go` | `Service` с командами и чтением |
| `queries.sql` | запросы для sqlc |
| `db/` | сгенерированный код, не редактируется |

```go
type Service struct{ /* runner *command.Runner, pool *pgxpool.Pool */ }
func NewService(runner *command.Runner, pool *pgxpool.Pool) *Service

func (s *Service) CreateProduct(ctx, actor eventstore.Actor, key string, in NewProduct) (Product, error)
func (s *Service) CreateDish(ctx, actor eventstore.Actor, key string, name string) (Dish, error)
func (s *Service) ChangeRecipe(ctx, actor eventstore.Actor, key string, dishID uuid.UUID, ingredients []Ingredient) (Dish, error)

func (s *Service) Product(ctx, id uuid.UUID) (Product, error)
func (s *Service) Products(ctx) ([]Product, error)
func (s *Service) Dish(ctx, id uuid.UUID) (Dish, error)
func (s *Service) Dishes(ctx) ([]Dish, error)

var ErrNotFound, ErrNameTaken error
type ValidationError struct{ Field, Message string }
type UnknownProductError struct{ ProductID uuid.UUID }
```

`key` - ключ идемпотентности, пустая строка отключает её. Команды возвращают созданный или изменённый объект целиком: это и ответ будущего эндпоинта, и результат, который `command.Run` сохраняет для повторов.

### 2. Синхронные таблицы

```sql
CREATE TABLE catalog_products (
    id              uuid PRIMARY KEY,
    name            text NOT NULL,
    unit            text NOT NULL CHECK (unit IN ('g', 'ml', 'pcs')),
    category        text NOT NULL,
    shelf_life_days integer CHECK (shelf_life_days > 0)
);
-- name_key text NOT NULL, CONSTRAINT catalog_products_name_key UNIQUE (name_key)

CREATE TABLE catalog_dishes (
    id   uuid PRIMARY KEY,
    name text NOT NULL
);
-- name_key text NOT NULL, CONSTRAINT catalog_dishes_name_key UNIQUE (name_key)

CREATE TABLE catalog_recipe_ingredients (
    dish_id    uuid   NOT NULL REFERENCES catalog_dishes (id),
    product_id uuid   NOT NULL REFERENCES catalog_products (id),
    quantity   bigint NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (dish_id, product_id)
);
```

Таблицы обновляются в транзакции команды, поэтому чтение сразу после команды видит результат, а проверки внутри следующей команды опираются на актуальные данные. Это тот же приём, что и таблица остатков партий в ADR-002: write model, а не источник истины.

Уникальность названия проверяется по столбцу `name_key`: название в нижнем регистре, вычисленное в приложении. Индекс по `lower(name)` зависел бы от локали базы и мог не сработать для кириллицы.

Ограничения в базе дублируют проверки кода намеренно: они ловят ошибку реализации, а уникальный индекс закрывает гонку двух одновременных созданий с одним названием.

Префикс `catalog_` в именах таблиц показывает владельца: чужие модули эти таблицы не трогают.

### 3. Ход команд

Все три команды идут через `command.Run` без `Locks`: складских потоков каталог не касается.

**CreateProduct:** проверка входных данных до транзакции; идентификатор - новый UUIDv7; внутри транзакции проверка, что название свободно, затем `tx.Append(ProductStream(id), 0, ProductCreated{...})` и вставка строки.

**CreateDish:** так же, событие `DishCreated`.

**ChangeRecipe:** проверка состава до транзакции (не пусто, нет повторов, количество положительное); внутри транзакции чтение блюда (нет - `ErrNotFound`), проверка существования всех продуктов одним запросом (нет - `UnknownProductError`), чтение версии потока блюда, `tx.Append(DishStream(id), version, RecipeChanged{...})`, замена строк ингредиентов.

Нарушение уникального индекса названия при гонке (код `23505` на индексе названия) преобразуется в `ErrNameTaken`. Одновременное изменение рецептуры одного блюда даёт конфликт версий у второй команды; `command.Run` повторяет её, и она проходит на свежей версии.

Название нормализуется обрезкой пробелов по краям перед проверкой и записью; регистр сохраняется как введён, уникальность проверяется без учёта регистра.

### 4. Запись в таблицы через применение событий

Таблицы изменяют только две-три небольшие функции вида `applyProductCreated(ctx, q, e ProductCreated)`. Команда вызывает такую функцию сразу после `tx.Append` с тем же событием. Так содержимое таблиц по построению является функцией от событий, и будущее перестроение не потребует второй реализации.

### 5. sqlc

`backend/sqlc.yaml` со списком пакетов; первый пакет - `internal/catalog`:

- схема берётся из каталога `migrations/`: sqlc понимает разметку goose, отдельная копия схемы не нужна;
- `sql_package: pgx/v5`, результат в `internal/catalog/db`;
- переопределение типа: `uuid` в `github.com/google/uuid.UUID`, чтобы сгенерированный код и события использовали один тип идентификатора.

Запуск: `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate` с закреплённой версией, как у линтера. Директива `tool` в `go.mod` не используется: sqlc добавил бы в граф зависимостей модуля парсеры нескольких СУБД. Это уточняет фразу в `backend/AGENTS.md`; файл обновляется. Сгенерированный код коммитится.

*Альтернатива:* писать запросы каталога вручную на pgx, как в `eventstore`. Отклонена: каталог - образец для остальных модулей, а в них запросов будет много; способ работы с sqlc лучше отладить на самом простом модуле.

### 6. Ошибки

- `ErrNotFound`, `ErrNameTaken` - sentinel-значения для `errors.Is`.
- `ValidationError` и `UnknownProductError` - типы для `errors.As`: HTTP-слою нужны поле и идентификатор, чтобы собрать ответ `application/problem+json`.

Ошибка валидации возвращается до начала транзакции и не занимает ключ идемпотентности.

## Risks / Trade-offs

- [В каталоге событий нет изменения и удаления продукта и блюда] → опечатку в названии нельзя исправить. Принято для текущего объёма: добавление события требует согласования каталога с разработчиком B. При необходимости вводится отдельным изменением.
- [Чтение блюд со всеми рецептурами одним списком] → два запроса и сборка в памяти. Для десятков блюд этого достаточно; пагинация появится вместе с HTTP.
- [Блюдо может долго оставаться без рецептуры] → такое блюдо нельзя заказать. Правило относится к заказам и стоп-листу (A4, B3), каталог его не запрещает: создание блюда и задание рецептуры - разные команды с разными событиями.
- [Внешние ключи на продукты мешают будущему удалению продукта] → удаления нет; ключ защищает от рецептуры с несуществующим продуктом при ошибке в коде.
- [Эндпоинты отложены] → каталог нельзя проверить руками через HTTP до появления каркаса API. Смягчение: все сценарии спецификации покрыты интеграционными тестами сервиса.
