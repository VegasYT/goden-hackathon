## Why

Клиенты Мигома (`migom-api`, `migom-backend`, `migom-integration`) есть в файле realm, но файл применяется только к новому тому Keycloak. На сервере том уже есть, а пересоздать его - значит удалить зарегистрированных пользователей и отнять у них их компании. Кроме того, при новом томе клиенты получили бы секреты разработки, которые лежат в репозитории.

## What Changes

- Задача `keycloak-config` при каждой выкладке заводит клиентов Мигома, если их нет: клиентов, роль `migom.integration` клиента `goden-api` и роли сервисных учётных записей.
- Секреты `migom-backend` и `migom-integration` берутся из `MIGOM_BACKEND_SECRET` и `MIGOM_INTEGRATION_SECRET` в `infra/.env` и выставляются при каждой выкладке, поэтому меняются правкой `.env`.
- Если секрет не задан, клиенту выставляется случайный секрет, который никто не знает: секрет разработки на сервере не действует.

Вне изменения:
- выкладка самого Мигома (`migom.site`, Caddy, его сервисы);
- клиент `migom-web` для единого входа в вебе;
- остальные клиенты и настройки realm.

## Capabilities

### New Capabilities

Нет.

### Modified Capabilities

- `deployment`: клиенты Мигома на сервере.

## Impact

- `infra/`: `keycloak/configure.sh`, `compose.prod.yaml` (переменные задачи `keycloak-config`), `.env.example`, `README.md`.
- `backend/` и `frontend/`: кода не касается, `backend/api/openapi.yaml` не меняется.
