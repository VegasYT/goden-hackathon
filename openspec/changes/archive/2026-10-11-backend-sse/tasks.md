# Tasks

Часть репо: `backend/`. Все команды запускаются из `backend/`, кроме явно указанных OpenSpec-проверок.

## 1. Интеграционные события

- [x] 1.1 Добавить `catalog.v1` и публикацию `ProductCreated`, `DishCreated`, `RecipeChanged` в существующем Kafka envelope, дополнить тесты маршрутизации событий; проверка: `go test ./internal/platform/kafka/`

## 2. Realtime-модуль

- [x] 2.1 Реализовать преобразование Kafka envelope в отсортированные категории инвалидирования и in-memory hub с ограниченной очередью и отключением медленного клиента, покрыть поддерживаемые и игнорируемые события unit-тестами; проверка: `go test ./internal/realtime/`
- [x] 2.2 Реализовать SSE-handler с `retry`, начальным `sync`, `update`, keepalive, flush и завершением по context, покрыть первое подключение, reconnect с `Last-Event-ID` и медленного клиента тестами; проверка: `go test ./internal/realtime/`

## 3. Контракт и безопасность API

- [x] 3.1 Описать `GET /events` и схемы SSE в `api/openapi.yaml`, перегенерировать ogen-код и добавить операцию в матрицу API; проверка: генерация ogen не оставляет diff при повторном запуске и `go test -tags=integration ./internal/api/ -run TestEveryOperationHasARequest`
- [x] 3.2 Зарегистрировать ручной маршрут `GET /api/v1/events` с общей проверкой bearer token, режимом `AUTH_DISABLED`, problem-ответом, correlation ID и CORS; покрыть успешное подключение и 401 интеграционными тестами, обновить матрицу authenticated-only операций; проверка: `go test -tags=integration ./internal/api/ -run 'Realtime|PermissionMatrix'`

## 4. Kafka-reader и lifecycle

- [x] 4.1 Добавить API-процессу независимое чтение `catalog.v1`, `stock.v1`, `orders.v1`, `expiry.v1` с конца, повтор после временного сбоя и корректное завершение; REST не должен терять готовность при недоступной Kafka; проверка: `go test ./cmd/api/ ./internal/realtime/`
- [x] 4.2 Покрыть сквозной сценарий с PostgreSQL и Kafka: после принятого заказа подключённый клиент получает все четыре категории, после разрыва новое соединение получает `sync`, а два клиента получают один сигнал независимо; проверка: `go test -tags=integration ./internal/api/ -run Realtime`

## 5. Документация и итоговая проверка

- [x] 5.1 Обновить `backend/AGENTS.md` и `backend/plan.md`: настройки Kafka процесса API, устройство SSE и статус B7; проверить существование упомянутых путей и команд; проверка: `rg -n "SSE|KAFKA_BROKERS|B7" AGENTS.md plan.md internal api cmd`
- [x] 5.2 Отформатировать код и выполнить полный обязательный набор backend-проверок; проверка: `go build ./...`, `go test ./...`, `go test -tags=integration ./...`, `GOTOOLCHAIN=go1.27.2 go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...`
