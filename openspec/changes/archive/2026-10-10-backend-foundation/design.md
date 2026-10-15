# Design

## Context

Мотивация и границы изменения описаны в proposal.md. Здесь только то, что определяет способ реализации.

- В `backend/` нет ни одного Go-файла, есть каркас каталогов с `.gitkeep`, `AGENTS.md` со стеком и правилами и `plan.md` с разделением на потоки A и B.
- Хранение и чтение событий заданы ADR-002: таблица событий с `position` и `txid xid8`, уникальность версии потока, чтение ниже `pg_snapshot_xmin` в порядке `(txid, position)`.
- Состав событий задан ADR-010, ADR-011, ADR-008 и задачей 0.7 плана: 15 типов.
- Разработчик ведёт работу на Windows. Docker на момент подготовки плана не запущен, для интеграционных тестов его нужно запустить.
- Параллельно разработчик B делает задачи 0.4-0.6: `docker compose`, `api/openapi.yaml` с ogen, `cmd/api` и `cmd/worker`.

## Goals / Non-Goals

**Goals:**

- После слияния разработчик B может писать проекции и тесты к ним, не дожидаясь команд A.
- Контракт событий зафиксирован в коде и защищён тестом от незаметного изменения формата.
- Публичные сигнатуры `eventstore` достаточны для задач A1 и B1 и не потребуют переделки при их реализации.

**Non-Goals:**

- Advisory-блокировки по продуктам, идемпотентность команд, запись в несколько потоков как готовый помощник: задача A1. Здесь только базовый `Append` в переданной транзакции.
- Таблица checkpoints, цикл проектора, метрика отставания: задача B1. Здесь только тип позиции чтения и функция чтения порции.
- Агрегаты, команды, синхронные таблицы остатков, HTTP, Kafka, Redis, авторизация.
- Проверка бизнес-инвариантов событий (например, `заявлено = принято + отклонено + карантин`): это ответственность команд в A3. События здесь - структуры данных с проверкой только закрытых перечислений при восстановлении.

## Decisions

### 1. Модуль и инструменты

Путь модуля: `github.com/VegasYT/goden/backend`. В `go.mod`: `go 1.27` и `toolchain go1.27.2`, локально установленный Go 1.22 скачивает нужный toolchain сам.

goose подключается как библиотека, а не директивой `tool`. Причина: CLI goose тянет драйверы всех поддерживаемых СУБД, а нам нужно одно действие - применить встроенные миграции. Его выполняет `cmd/migrate` и тестовая обвязка через один и тот же код. Новая миграция создаётся вручную файлом `migrations/<YYYYMMDDHHMMSS>_<имя>.sql`: отметка времени вместо номера исключает одинаковые номера в двух ветках. Это уточняет фразу в `backend/AGENTS.md` о подключении goose через `tool`; файл обновляется.

golangci-lint v2 не добавляется в `go.mod`: его авторы не рекомендуют ставить линтер как зависимость проекта. Версия закрепляется в команде запуска в `AGENTS.md`.

sqlc в этом изменении не используется: запросов `eventstore` мало, и им нужен ручной контроль типа `xid8`. Но sqlc понадобится уже в A2, а его работа с PostgreSQL на Windows не подтверждена. Поэтому в изменение входит разовая проверка генерации на тривиальном запросе с записью результата в `AGENTS.md`. Если sqlc на Windows не работает, там же фиксируется запасной способ запуска.

*Альтернатива:* подключить ogen, sqlc и goose через `tool` сразу, как сказано в плане. Отклонена: ogen относится к задаче 0.5 разработчика B, а неиспользуемые инструменты в `go.mod` только добавляют конфликты слияния.

### 2. Линтер и границы модулей

`.golangci.yml` формата v2: стандартный набор (`errcheck`, `govet`, `staticcheck`, `ineffassign`, `unused`) плюс `depguard`, `errorlint`, `gocritic`, `forbidigo`; форматирование `gofumpt`.

Правила `depguard`:

- файлы в `internal/platform/**` не импортируют `internal/{catalog,inventory,orders,stoplist,recommendations,analytics,notifications}`;
- для каждого доменного модуля `m`: файлы вне `internal/m/**` не импортируют `internal/m/db`.

`forbidigo` запрещает `time.Now` вне пакета `clock`: так правило из `AGENTS.md` проверяется автоматически.

### 3. Таблица событий

```sql
CREATE TABLE events (
    position       bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    txid           xid8        NOT NULL DEFAULT pg_current_xact_id(),
    event_id       uuid        NOT NULL UNIQUE,
    stream_id      text        NOT NULL,
    stream_version bigint      NOT NULL CHECK (stream_version > 0),
    event_type     text        NOT NULL,
    schema_version integer     NOT NULL CHECK (schema_version > 0),
    payload        jsonb       NOT NULL,
    metadata       jsonb       NOT NULL DEFAULT '{}',
    correlation_id uuid,
    causation_id   uuid,
    occurred_at    timestamptz NOT NULL,
    UNIQUE (stream_id, stream_version)
);
CREATE INDEX events_txid_position_idx ON events (txid, position);
```

Неизменяемость обеспечивают триггеры: `BEFORE UPDATE OR DELETE` на строку и `BEFORE TRUNCATE` на таблицу вызывают функцию, которая поднимает исключение. Это дешевле и надёжнее, чем разграничение прав ролей, которых в проекте пока нет.

`occurred_at` приходит от приложения, из `clock.Clock`, а не из `now()` базы: иначе время событий нельзя контролировать в тестах сроков годности.

### 4. API пакета eventstore

```go
type NewEvent struct {            // то, что передаёт вызывающая сторона
    ID            uuid.UUID
    Type          string
    SchemaVersion int
    Payload       json.RawMessage
    Metadata      json.RawMessage
    CorrelationID uuid.UUID
    CausationID   uuid.UUID
    OccurredAt    time.Time
}

type Event struct {               // то, что возвращает чтение
    NewEvent
    StreamID string
    Version  int64
    Position int64
    TxID     uint64
}

type Checkpoint struct{ TxID uint64; Position int64 }

func Append(ctx, tx pgx.Tx, streamID string, expectedVersion int64, events ...NewEvent) (int64, error)
func ReadStream(ctx, q Querier, streamID string, fromVersion int64) ([]Event, error)
func ReadAll(ctx, q Querier, after Checkpoint, limit int) ([]Event, error)
func StreamVersion(ctx, q Querier, streamID string) (int64, error)
var ErrVersionConflict, ErrDuplicateEvent, ErrInvalidInput = ...
```

- `Append` принимает транзакцию, а не пул: событие всегда пишется вместе с чем-то ещё (другой поток, синхронная таблица остатков), и границу транзакции задаёт вызывающая сторона. Запись вне транзакции невозможна по сигнатуре.
- `Querier` - минимальный интерфейс с `Query`, которому удовлетворяют и `pgx.Tx`, и `*pgxpool.Pool`: читать можно и внутри транзакции команды, и снаружи.
- Проверка версии в два шага: сначала `SELECT max(stream_version)` и сравнение с ожидаемой, затем вставка. Первый шаг ловит и отставание, и опережение ожидаемой версии, то есть не допускает пропуска. Гонку между шагами закрывает ограничение `UNIQUE (stream_id, stream_version)`: его нарушение (`23505` на этом ограничении) преобразуется в `ErrVersionConflict`. Нарушение `UNIQUE (event_id)` возвращается отдельной ошибкой.
- После нарушения уникальности транзакция PostgreSQL прервана. Это ожидаемо: команда при конфликте повторяется целиком (ADR-002), повтор внутри той же транзакции не предусмотрен.
- События вставляются одним запросом на вызов, а не по одному.
- `eventstore` работает с `json.RawMessage` и ничего не знает о доменных типах. Сериализацию выполняет слой каталога событий (решение 5).

*Альтернатива:* вставка с подзапросом, вычисляющим следующую версию. Отклонена: она не сообщает вызывающей стороне, что её состояние устарело, а в этом и смысл ожидаемой версии.

`ReadAll`:

```sql
SELECT ... FROM events
WHERE (txid, position) > ($1::text::xid8, $2)
  AND txid < pg_snapshot_xmin(pg_current_snapshot())
ORDER BY txid, position
LIMIT $3
```

`xid8` передаётся и читается через текстовое представление (`$1::text::xid8`, `txid::text`) и хранится в Go как `uint64`: так код не зависит от того, есть ли в pgx встроенный кодек для этого типа. Следующий checkpoint вызывающая сторона берёт из последнего события порции.

### 5. Каталог доменных событий

Каждое событие - Go-структура с JSON-тегами в `snake_case` в файле `events.go` своего модуля. Общая часть лежит в `internal/platform/eventstore`:

```go
type Payload interface {
    EventType() string
    SchemaVersion() int
}

func Register[T Payload]()                      // вызывается из init() модуля
func Encode(p Payload) (json.RawMessage, error)
func Decode(eventType string, schemaVersion int, raw json.RawMessage) (Payload, error)
func RegisteredTypes() []TypeInfo
```

Реестр хранит соответствие "тип, версия схемы" и Go-тип. `Decode` для незарегистрированной пары возвращает ошибку. Закрытые перечисления (причина списания, тип автора, единица измерения, источник партии) - именованные строковые типы с `UnmarshalText`, который отклоняет неизвестное значение.

Имена потоков строят функции рядом с событиями: `catalog.ProductStream(id)`, `catalog.DishStream(id)`, `inventory.DeliveryStream(id)`, `inventory.StockStream(productID)`, `orders.OrderStream(id)`.

Общие типы:

- `Actor{Type, ID}` - в `internal/platform/eventstore`, потому что нужен всем трём модулям и не принадлежит ни одному;
- идентификаторы - `uuid.UUID` (UUIDv7), количество - `int64`, время - `eventstore.Time`: обёртка над `time.Time`, которая при сериализации всегда пишет UTC. Обычный `time.Time` записал бы смещение исходного часового пояса.

Состав полей (черновик для согласования с разработчиком B):

| Событие | Поток | Поля |
| --- | --- | --- |
| `ProductCreated` | `product-` | `product_id`, `name`, `unit` (`g`, `ml`, `pcs`), `category`, `shelf_life_days` (необязательное) |
| `DishCreated` | `dish-` | `dish_id`, `name` |
| `RecipeChanged` | `dish-` | `dish_id`, `ingredients[]`: `product_id`, `quantity` |
| `DeliveryRegistered` | `delivery-` | `delivery_id`, `number`, `delivered_at`, `registered_by`, `items[]`: `item_id`, `product_id`, `declared_quantity`, `expires_at`, `manufactured_at` (необязательное) |
| `DeliveryApproved` | `delivery-` | `delivery_id`, `delivered_at`, `approved_at`, `approved_by`, `items[]`: `item_id`, `product_id`, `declared_quantity`, `accepted_quantity`, `rejected_quantity`, `quarantined_quantity`, `expires_at`, `expired`, `batch_id`, `reject_reason`, `quarantine_reason`, `comment` |
| `DeliveryApprovalCorrected` | `delivery-` | `correction_id`, `delivery_id`, `reason`, `corrected_at`, `corrected_by`, `items[]`: `item_id`, `accepted_quantity`, `rejected_quantity`, `quarantined_quantity`, `batch_id` |
| `QuarantinedDeliveryItemAccepted` | `delivery-` | `delivery_id`, `item_id`, `quantity`, `batch_id`, `comment`, `decided_at`, `decided_by` |
| `QuarantinedDeliveryItemRejected` | `delivery-` | `delivery_id`, `item_id`, `quantity`, `reason`, `comment`, `decided_at`, `decided_by` |
| `BatchReceived` | `stock-` | `batch_id`, `product_id`, `delivery_id`, `item_id`, `quantity`, `expires_at`, `received_at`, `source` (`approval`, `quarantine`, `correction`) |
| `StockConsumed` | `stock-` | `product_id`, `order_id`, `batches[]`: `batch_id`, `quantity` |
| `StockAdjusted` | `stock-` | `product_id`, `batch_id`, `delta`, `reason`, `comment`, `correction_id` (необязательное), `adjusted_by` |
| `StockWrittenOff` | `stock-` | `product_id`, `batch_id`, `quantity`, `reason`, `comment`, `written_off_by` |
| `BatchExpiryWarningIssued` | `stock-` | `product_id`, `batch_id`, `remaining_quantity`, `expires_at`, `warning_period_seconds` |
| `OrderAccepted` | `order-` | `order_id`, `accepted_by`, `items[]`: `dish_id`, `portions` |
| `OrderRejected` | `order-` | `order_id`, `rejected_by`, `items[]`: `dish_id`, `portions`; `shortages[]`: `product_id`, `required_quantity`, `available_quantity` |

Пояснения к составу:

- В `DeliveryApprovalCorrected` позиции содержат итоговое распределение после корректировки, а не разницу: проекции проще заменить значения, чем складывать поправки.
- `OrderAccepted` не дублирует разбивку по партиям: она лежит в `StockConsumed` складских потоков, записанных в той же транзакции. Связь идёт через `order_id` и общий correlation ID.
- `category` у продукта нужна фильтру экрана складского учёта из `docs/plan.md`.

Формат защищает тест с эталонными файлами: для каждого типа в `testdata/` лежит JSON эталонного экземпляра, тест сравнивает сериализацию побайтно и проверяет обратное восстановление. Изменение формата без обновления эталона ломает тест, а обновление эталона видно в ревью.

### 6. Платформенные пакеты

- `config`: структуры `Postgres` и `Log` с тегами caarlos0/env и функция `Load[T]`, которая оборачивает ошибку разбора. Конфигурацию конкретного процесса собирает его `main` из этих частей. Переменные: `DATABASE_URL` (обязательная), `DATABASE_MAX_CONNS`, `LOG_LEVEL`.
- `clock`: интерфейс `Clock` с методом `Now`, `System` (UTC) и `Fake` с `Set` и `Advance`. `Fake` лежит в основном пакете, а не в тестовом: он нужен тестам всех модулей.
- `observability`: `NewLogger(w, level)` на `slog.JSONHandler` и обёртка обработчика, добавляющая correlation ID из контекста; функции `WithCorrelationID` и `CorrelationID`.
- `postgres`: `NewPool`, `InTx(ctx, pool, fn)` с откатом при ошибке и панике, `Migrate(ctx, pool)` на goose `Provider` с встроенными через `embed` миграциями.
- Миграции встраиваются из пакета `backend/migrations` (файл `migrations.go` с `//go:embed *.sql`).

### 7. Тестовая обвязка

Пакет `internal/platform/postgres/pgtest`:

- один контейнер PostgreSQL 18 на процесс тестов пакета, запускается лениво через `sync.Once`;
- миграции применяются один раз к шаблонной базе;
- `pgtest.NewDB(t)` создаёт для теста отдельную базу командой `CREATE DATABASE ... TEMPLATE ...` и возвращает пул; база удаляется по `t.Cleanup`.

Изолированная база на тест нужна, потому что `ReadAll` читает всю таблицу событий: тесты с общей базой видели бы чужие события.

Пакет `internal/platform/eventstore/estest`: `Append(t, pool, streamID, payloads...)` определяет текущую версию потока, сериализует события через реестр и записывает их. Это помощник для проекций разработчика B. Обвязку для Kafka и Redis добавляет B в задаче 0.6.

Интеграционные тесты помечаются тегом сборки `integration`: `go test ./...` остаётся быстрым и не требует Docker, полный прогон - `go test -tags=integration ./...`.

### 8. cmd/migrate

Минимальный `main`: загрузка конфигурации, логгер, пул, `postgres.Migrate`, код возврата. Каталоги `cmd/api` и `cmd/worker` не трогаются, это зона разработчика B.

## Risks / Trade-offs

- [Черновик событий не устроит разработчика B] → состав полей вынесен в таблицу выше и в эталонные JSON-файлы, ревью pull request является согласованием из задачи 0.7. Менять поля до первых проекций дёшево.
- [Docker не запущен или недоступен] → интеграционные тесты отделены тегом сборки, unit-тесты и линтер проходят без Docker. Перед реализацией Docker нужно запустить, иначе требования к Event Store нельзя проверить.
- [sqlc не работает на Windows] → проверяется в этом изменении, до того как он понадобится в A2. Запасной способ - запуск в Docker.
- [Долгая транзакция в базе задерживает `ReadAll`] → это следствие ADR-002, а не дефект реализации. Тест со сценарием незавершённой транзакции документирует поведение; метрику отставания добавляет B1.
- [Конфликт слияния в `go.mod` с веткой разработчика B] → неизбежен и решается перегенерацией `go mod tidy`. Чтобы сократить окно, изменение сливается в `backend` сразу после ревью.
- [Проверка версии в два запроса вместо одного] → лишний запрос на каждый `Append`. Принято ради того, чтобы опережающая ожидаемая версия не создавала пропуск. Для команд A1 значение версии всё равно читается заранее.

## Open Questions

- Нужен ли у события `ProductCreated` срок хранения в днях (`shelf_life_days`), если каждая позиция поставки уже несёт обязательный `expires_at`. Поле оставлено необязательным: по ADR-010 оно используется только для проверки согласованности дат. Решается на ревью с разработчиком B и автором ADR-010, на состав задач не влияет.
- Какой набор значений у `reason` в `StockAdjusted` и `QuarantinedDeliveryItemRejected`. В ADR закрытый набор задан только для `StockWrittenOff`. В черновике эти поля - свободная строка; закрытый набор можно ввести в A3 и A5 без смены версии схемы.
