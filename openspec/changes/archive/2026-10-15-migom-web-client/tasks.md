Части репо указаны у задач.

## 1. Keycloak

- [x] 1.1 `infra/`: клиент `migom-web` в `keycloak/goden-realm.json`; `keycloak/configure.sh` заводит его и выставляет адреса по `MIGOM_DOMAIN`. Проверка: задача на локальном Keycloak заводит клиента с PKCE и адресами, повторный запуск ничего не дублирует.
- [x] 1.2 `backend/AGENTS.md`: описание клиента. Проверка: текст совпадает с realm.

## 2. Сервер

- [ ] 2.1 Выкладка на стенд. Проверка: вход в `https://owner.migom.site` через Keycloak.
