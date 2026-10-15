## Why

Сервис доставки Мигом выкладывается на тот же сервер, что и Goden, под доменом `migom.site`. Порты 80 и 443 сервера занимает Caddy Goden, поэтому адреса Мигома открывает он. Сборки Goden и Мигома одновременно не помещаются в память сервера.

## What Changes

- Caddy Goden отдаёт `migom.site` (API и WebSocket Мигома, фото из его MinIO по `/migom-media/`), `owner.migom.site` и `admin.migom.site` (кабинеты), `www.migom.site` перенаправляет на `migom.site`. Домен задаёт `MIGOM_DOMAIN`, по умолчанию `migom.site`.
- `infra/deploy.sh` ждёт окончания другой выкладки на сервере (общая блокировка с выкладкой Мигома).

Вне изменения:
- выкладка самого Мигома: она в его репозитории;
- общие с Мигомом сервисы, кроме сети: Kafka, Keycloak и API Goden Мигом использует как обычный клиент.

## Capabilities

### New Capabilities

Нет.

### Modified Capabilities

- `deployment`: адреса Мигома за прокси сервера, очередь выкладок.

## Impact

- `infra/`: `caddy/Caddyfile`, `compose.prod.yaml` (переменная прокси), `deploy.sh`, `.env.example`, `README.md`.
- `backend/` и `frontend/`: не меняются, `backend/api/openapi.yaml` не меняется.
