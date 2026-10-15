Части репо указаны у задач. Команды backend - из папки `backend/`.

## 1. Keycloak

- [x] 1.1 `infra/`: маппер получателя `goden-api` у клиента `migom-backend` в `keycloak/goden-realm.json`; `keycloak/configure.sh` добавляет его, если его нет. Проверка: задача на Keycloak без маппера добавляет его, повторный запуск ничего не дублирует; токен по паролю через `migom-backend` несёт `aud` `migom-api` и `goden-api`.
- [x] 1.2 `backend/`: в `TestKeycloakTokens` токен пользователя через `migom-backend` проходит проверку API Goden как пользователь. Проверка: `go test -tags=integration ./internal/platform/auth/`.

## 2. Итог

- [x] 2.1 `backend/AGENTS.md`: описание клиента `migom-backend`. Проверка: текст совпадает с realm.
