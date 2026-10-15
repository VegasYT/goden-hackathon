## Why

Владелец заведения в Мигоме и владелец точки в Goden - один и тот же пользователь общего Keycloak. Чтобы подключить заведение к точке без ручного переноса номеров между системами, Мигом должен обращаться в API Goden от имени этого пользователя: показать его точки, подключить к выбранной точке заведение и прочитать её меню. Сейчас токен Мигома несёт только `aud` `migom-api`, и API Goden его отклоняет.

## What Changes

- Токены клиента `migom-backend` несут `aud` `goden-api` вместе с `migom-api`: API Goden принимает токен пользователя Мигома с правами этого пользователя (владение и назначения, ADR-013), не больше.
- `keycloak-config` добавляет этот получатель клиенту `migom-backend` на работающем сервере, если его нет.

Вне изменения:
- обмен токена (token exchange) вместо общего получателя;
- новые операции API Goden: Мигому хватает `GET /me`, `PUT /points/{pointId}/migom` и `GET /points/{pointId}/menu`.

## Capabilities

### New Capabilities

Нет.

### Modified Capabilities

- `authorization`: токен пользователя Мигома принимается API Goden.

## Impact

- `infra/`: `keycloak/goden-realm.json`, `keycloak/configure.sh`.
- `backend/`: тест токенов Keycloak (`internal/platform/auth`), `backend/AGENTS.md`; код и `backend/api/openapi.yaml` не меняются.
