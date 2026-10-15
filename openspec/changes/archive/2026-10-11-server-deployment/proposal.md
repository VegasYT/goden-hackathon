## Why

Систему можно запустить только на машине разработчика: процессы backend стартуют с хоста, образов для них нет. Критерии оценки кейса отдельно отмечают решение, которое "задеплоено и можно посмотреть по ссылке". Есть сервер (`194.154.24.234`) и домен `goden.pro`.

## What Changes

- Образы для backend (`api`, `worker`, `generator`, `migrate` в одном образе) и frontend.
- Файл `infra/compose.prod.yaml`: вся система в контейнерах, запуск одной командой.
- Обратный прокси Caddy с автоматическим получением и продлением сертификатов Let's Encrypt:
  - `https://goden.pro` - frontend;
  - `https://goden.pro/api/v1` - API;
  - `https://goden.pro/docs` - документация API (Swagger UI);
  - `https://auth.goden.pro` - Keycloak;
  - `https://grafana.goden.pro` - Grafana со входом по паролю.
- Секреты сервера (пароли Keycloak, PostgreSQL, Grafana, секрет генератора, пароли демонстрационных пользователей) задаются в файле `.env` на сервере и в репозиторий не попадают. Файл realm принимает их через подстановки со значениями по умолчанию: локальное окружение работает как раньше.
- GitHub Actions:
  - запускаются только при попадании коммита в `main`;
  - проверки: backend (сборка, все тесты, линтер) и frontend (`pnpm check`, сборка);
  - выкладка на сервер по SSH после успешных проверок.
- Инструкция по первому запуску сервера и по выкладке в `infra/README.md`.

Вне изменения:

- исправление находок из `backend/review.md`, в том числе создание топиков Kafka: до исправления события в Kafka на сервере не публикуются, на работу API и интерфейса это не влияет;
- интеграция frontend с API и Keycloak: сейчас frontend - оболочка с заглушками страниц;
- сборка образов в GitHub и реестр образов: образы собираются на сервере;
- резервное копирование, несколько серверов, выкладка без простоя;
- постоянное хранение данных Keycloak: realm создаётся из файла при каждом пересоздании контейнера, как локально.

## Capabilities

### New Capabilities

- `deployment`: запуск всей системы на сервере, адреса и сертификаты, секреты.
- `ci`: автоматические проверки и выкладка.

### Modified Capabilities

- `dev-environment`: файл realm получает подстановки для паролей и секрета генератора; значения по умолчанию совпадают с нынешними.

## Impact

- `backend/`: `Dockerfile`, `.dockerignore`. Код не меняется.
- `frontend/`: `Dockerfile`, `.dockerignore`. Код не меняется.
- `infra/`: `compose.prod.yaml`, `caddy/Caddyfile`, `prometheus/prometheus.prod.yml`, `.env.example`, `README.md`, правка `keycloak/goden-realm.json`.
- корень: `.github/workflows/`.
- Сервер: Docker, файл `.env`, открытые порты 80 и 443. В DNS нужны записи A для `goden.pro`, `auth.goden.pro` и `grafana.goden.pro`.
- `backend/api/openapi.yaml` не меняется.
