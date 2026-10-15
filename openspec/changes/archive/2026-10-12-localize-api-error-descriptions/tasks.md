# Tasks

Часть репо: `backend/`. Команды реализации и проверки запускаются из `backend/`, кроме явно указанной OpenSpec-команды.

## 1. Локализация API-ошибок

- [x] 1.1 Локализовать в `internal/api` заголовки и детали ошибок операций, авторизации, маршрутизации и проверки контракта; добавить табличные тесты для всех классов ошибок, шаблонов валидации и русского fallback, проверка: `go test ./internal/api`.
- [x] 1.2 Локализовать прямые ответы `internal/platform/httpx` для ошибок тела запроса и паники, дополнить тесты проверкой русских описаний и неизменных машинных кодов, проверка: `go test ./internal/platform/httpx`.

## 2. Итоговая проверка

- [x] 2.1 Проверить отсутствие английских пользовательских описаний во всех конструкторах `Problem` и выполнить полный обязательный набор backend-проверок, проверка: `rg -n 'Title:|Detail:|Message:' internal/api internal/platform/httpx`, `go build ./...`, `go test ./...`, `go test -tags=integration ./...`, `$env:GOTOOLCHAIN='go1.27.2'; go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...`.
