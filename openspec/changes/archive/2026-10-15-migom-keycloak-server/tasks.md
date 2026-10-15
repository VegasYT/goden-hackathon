Части репо указаны у задач.

## 1. Настройка Keycloak

- [x] 1.1 `infra/`: в `keycloak/configure.sh` завести клиентов `migom-api`, `migom-backend`, `migom-integration`, роль `migom.integration` и роли сервисных учётных записей, если их нет; выставить секреты из `MIGOM_BACKEND_SECRET` и `MIGOM_INTEGRATION_SECRET`, без них - случайные. Проверка: задача на Keycloak без клиентов Мигома заводит их, повторный запуск ничего не дублирует, токен `client_credentials` выдаётся по секрету из окружения и не выдаётся по секрету разработки.
- [x] 1.2 `infra/`: переменные в `compose.prod.yaml` (задача `keycloak-config`), `.env.example`, `README.md`. Проверка: `docker compose -f infra/compose.prod.yaml config` с тестовым `.env` проходит.

## 2. Сервер

- [x] 2.1 `infra/`: выкладка на стенд. Проверка: в журнале `keycloak-config` клиенты заведены, сайт и вход работают, пользователи на месте.
