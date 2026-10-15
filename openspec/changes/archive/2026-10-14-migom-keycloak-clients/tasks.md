Часть репо: `infra/` и `backend/`, команды из корня репозитория.

## 1. Realm

- [x] 1.1 Клиенты `migom-api` и `migom-backend` с audience-mapper и сервисной учётной записью в `infra/keycloak/goden-realm.json`. Проверка: Keycloak окружения пересоздан с файлом realm; токен по паролю `admin` от `migom-backend` содержит `aud` `migom-api`; токен сервисной учётной записи создаёт и удаляет пользователя через Admin API; запрос с токеном `migom-backend` к API Goden получает 401.

## 2. Документация

- [x] 2.1 `backend/AGENTS.md`: клиенты Мигома в разделе Keycloak. Проверка: упомянутые клиенты есть в файле realm.
