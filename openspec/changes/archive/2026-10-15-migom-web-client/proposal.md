## Why

Кабинеты Мигома (владельца и модератора) переходят на единый вход через Keycloak Goden: страница входа и регистрации общая с Goden, пароль в кабинеты Мигома больше не вводится. Для этого нужен публичный клиент кабинетов в realm `goden`.

## What Changes

- Публичный клиент `migom-web` с PKCE (S256), без входа по паролю: адреса возврата и выхода `https://owner.<домен Мигома>`, `https://admin.<домен Мигома>` и для разработки `http://localhost:5174`, `http://localhost:5175`. Токены несут `aud` `migom-api` и `goden-api`: кабинет владельца работает с API Мигома, а тот от имени пользователя ходит в API Goden.
- `keycloak-config` заводит клиента на сервере, если его нет, и при каждой выкладке выставляет адреса возврата по `MIGOM_DOMAIN`.

Вне изменения:
- своя тема страниц входа для Мигома: кабинеты Мигома входят через тему Goden;
- мобильные приложения Мигома: они по-прежнему входят по паролю через `migom-backend`.

## Capabilities

### New Capabilities

Нет.

### Modified Capabilities

- `authorization`: клиент кабинетов Мигома в realm.

## Impact

- `infra/`: `keycloak/goden-realm.json`, `keycloak/configure.sh`, `compose.prod.yaml` (переменная задачи `keycloak-config`).
- `backend/AGENTS.md`: описание клиента. Код backend и `backend/api/openapi.yaml` не меняются.
