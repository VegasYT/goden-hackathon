## Why

Мигом (сервис доставки, репозиторий `C:\VS_code\Migom`) переводит вход на Keycloak Goden: один realm `goden` на оба продукта, чтобы владелец входил в них одной учётной записью. Для этого в realm нужны клиенты Мигома: backend Мигома входит по паролю пользователя от имени своего клиента и заводит пользователей через Admin API.

## What Changes

- Клиент `migom-api` (bearer-only): получатель токенов Мигома (`aud`).
- Клиент `migom-backend` (конфиденциальный): вход по паролю (`directAccessGrantsEnabled`) и обновление токена; сервисная учётная запись с ролями `manage-users`, `view-users`, `query-users` клиента `realm-management`, чтобы заводить пользователей и менять им пароль; токены несут `aud` `migom-api`.
- Секрет `migom-backend` задаётся подстановкой `MIGOM_BACKEND_SECRET`, локально - `migom-backend-dev-secret`.

Вне изменения:
- клиент для входа в веб Мигома перенаправлением (этап 2 плана Мигома);
- применение клиентов на сервере (`infra/keycloak/configure.sh`): делается вместе с выкладкой Мигома;
- права Мигома в API Goden (наличие блюд и заказы).

## Capabilities

### New Capabilities

Нет.

### Modified Capabilities

- `authorization`: клиенты Мигома в realm.

## Impact

- `infra/`: `infra/keycloak/goden-realm.json`.
- `backend/`: `backend/AGENTS.md` (раздел Keycloak).
- API Goden и frontend не меняются: токены `migom-backend` без `aud` `goden-api` API Goden отклоняет.
