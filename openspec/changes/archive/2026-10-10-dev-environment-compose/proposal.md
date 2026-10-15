# Proposal

## Why

Внешние сервисы проекта сейчас никак не описаны: PostgreSQL каждый поднимает руками одной командой `docker run`, а Kafka, Redis и Keycloak не поднимает никто. Из-за этого заблокированы авторизация (A7) и задачи разработчика B с Kafka и Redis (B5, B6), а frontend не может получить токен. Это задача 0.4 из [backend/plan.md](../../../backend/plan.md); её владелец, разработчик B, к работе не приступал, задачу выполняет разработчик A.

## What Changes

- Появляется папка `infra/` с файлом `infra/compose.yaml`: PostgreSQL 18, Kafka 4 в режиме KRaft, Redis 8 и Keycloak 26. Одна команда `docker compose -f infra/compose.yaml up -d` поднимает всё; у каждого сервиса есть проверка готовности.
- Keycloak при старте импортирует realm `goden` из файла в `infra/keycloak/`. В realm:
  - клиент `goden-api` с 14 client roles - по одной на permission из ADR-006;
  - публичный клиент `goden-web` для frontend;
  - клиент `goden-generator` с сервисной учётной записью и четырьмя правами из ADR-012;
  - составные роли `admin`, `manager`, `approver`, `cook`, `waiter` и по одному тестовому пользователю на каждую.
- Процессы backend (`api`, `worker`, `generator`) в compose не входят: они запускаются с хоста через `go run` и подключаются к сервисам по `localhost`.
- `backend/AGENTS.md` получает раздел о запуске окружения вместо ручного `docker run`.

Порт Keycloak на хосте - 8180: 8080 занят API.

Вне изменения: проверка токенов в backend и сама сервисная учётная запись в коде генератора (A7); Prometheus, Grafana, Loki (A8 и этап 16 общего плана); Dockerfile и запуск процессов backend и frontend в контейнерах; окружение для деплоя.

## Capabilities

### New Capabilities

- `dev-environment`: локальное окружение разработки - состав сервисов, готовность, адреса и содержимое realm Keycloak.

### Modified Capabilities

Нет.

## Impact

- **Файлы:** `infra/compose.yaml`; `infra/keycloak/goden-realm.json`; `backend/AGENTS.md`; `backend/plan.md` (статус 0.4); корневой `AGENTS.md` (папка `infra/` перестаёт быть "планируется").
- **Код backend:** не меняется.
- **Разработчик B:** получает Kafka и Redis для B5 и B6.
- **Frontend:** получает Keycloak с клиентом `goden-web` и тестовыми пользователями.
- **Безопасность:** в репозиторий попадают пароли тестовых пользователей и секрет клиента генератора. Они действуют только для локального realm и помечены как непригодные для любого другого окружения.
