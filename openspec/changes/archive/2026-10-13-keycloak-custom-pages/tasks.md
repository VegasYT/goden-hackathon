# Tasks

Части репо: `frontend/` (разделы 1–4, команды из папки `frontend/`, правила в `frontend/AGENTS.md`), `infra/` и `backend/` (раздел 5, команды из корня репозитория). Ссылки вида «§N» указывают на решения в `design.md`.

## 1. Подключение Keycloakify и проверка совместимости

- [x] 1.1 Добавить `keycloakify` в зависимости и плагин в `vite.config.ts` (`themeName: 'goden'`, `accountThemeImplementation: 'none'`, только jar для Keycloak 26, §1). Добавить команду `build:keycloak-theme` в `package.json`. Создать минимальный `src/app/keycloak-theme/login/KcPage.tsx` на `DefaultPage` и сгенерировать `kc.gen.tsx` (§2). Проверка: `pnpm build` проходит и даёт прежний `dist`; `docker build -f ../infra/keycloak/Dockerfile .` из задачи 1.2 собирает образ с jar. Если плагин несовместим с Vite 8, работа останавливается с отчётом пользователю (Risks)
- [x] 1.2 Создать `infra/keycloak/Dockerfile` из двух стадий: сборка темы на `node:22-bookworm-slim` с Maven и JDK и кешем Maven в cache mount, затем `quay.io/keycloak/keycloak:26.8.0` с jar в `/opt/keycloak/providers/` (§10). Проверка: образ собирается; в `/opt/keycloak/providers/` лежит jar темы `goden`
- [x] 1.3 Исключить `kc.gen.tsx` из eslint, prettier и steiger, как `routeTree.gen.ts` (§2). Проверка: `pnpm lint`, `pnpm format:check` и `pnpm lint:fsd` проходят с сегментом `app/keycloak-theme`. Если steiger отклоняет сегмент, решение §2 пересматривается до раздела 2

## 2. Точка входа, каркас и просмотр в dev

- [x] 2.1 Развилка в начале `src/app/main.tsx` (§3): при `window.kcContext` ленивая загрузка `KcPage` внутри `ThemeRoot` без `StartupScreen`, `startSession()` и загрузки профиля `/me`, иначе текущий поток. Только в DEV параметр `kcPage` (и `kcState`) подставляет `window.kcContext` из `login/mocks.ts` на `getKcContextMock` (§9). Проверка: `pnpm typecheck` и `pnpm build` проходят; в `dist` нет кода моков (поиск по имени модуля `mocks` и по строке тестовых данных); основной чанк приложения не содержит страниц темы
- [x] 2.2 `login/i18n.ts` на `i18nBuilder`: язык `ru` и свои формулировки, в том числе «Логин или почта» (§8). Проверка: `pnpm typecheck`
- [x] 2.3 `login/Template.tsx` (§4): `StandaloneScreen`, `Logo`, карточка, заголовок страницы, сообщения Keycloak в `Alert` (error, warning, success, info без статусного цвета) с иконками из реестра `Icon`. Подключить его к `DefaultPage` для прочих страниц, добавить ограниченные стили нативных полей и кнопок на CSS-переменных темы (§6). Проверка: тест — нераспознанный `pageId` показывается в каркасе с логотипом; тест — сообщение типа error показано с иконкой и ролью alert

## 3. Свёрстанные страницы

- [x] 3.1 Страница `login.ftl` (§4): поля «Логин или почта» и «Пароль» с кнопкой показа пароля и `aria-label`, «Войти», ссылка на регистрацию. Ссылки «Забыли пароль?» и «Запомнить меня» показываются по флагам `kcContext.realm`. Блокировка кнопки после отправки, фокус в первом незаполненном поле. Проверка: тесты на моках — ошибка входа показана, логин подставлен из контекста, пароль пуст; при выключенных флагах realm ссылки «Забыли пароль?» и флажка нет; повторное нажатие «Войти» после отправки не отправляет форму второй раз
- [x] 3.2 Поля профиля на `useUserProfileForm` с отрисовкой на MUI для text, email и password, незнакомый тип отрисовывается как текст (§5); страницы `register.ftl` и `login-update-profile.ftl`. Проверка: тесты на моках — ошибка сервера «логин занят» показана у поля логина и связана с ним (`aria-describedby`), введённые значения сохранены; несовпадение паролей показано у поля подтверждения
- [x] 3.3 Страницы `login-update-password.ftl` и `login-verify-email.ftl` (адрес почты и повторная отправка письма). Проверка: тест — ошибка несовпадения нового пароля показана у поля подтверждения; `pnpm typecheck`
- [x] 3.4 Страницы `logout-confirm.ftl`, `login-page-expired.ftl`, `info.ftl`, `error.ftl`: сообщение и продолжающее действие, ссылка в приложение на `error.ftl`, если она есть в контексте. Проверка: `pnpm typecheck`; каждая страница открывается в `pnpm dev` по `?kcPage=<pageId>` без ошибок в консоли (скриншот светлой и тёмной темы)
- [x] 3.5 Состояния в `mocks.ts` для просмотра: ошибка входа, ошибки регистрации, подтверждение почты, сообщения всех четырёх типов. Проверка: каждое состояние открывается в `pnpm dev` по `?kcPage=…&kcState=…` (скриншоты); без параметра `pnpm dev` открывает приложение и уходит на вход Keycloak

## 4. Документация frontend

- [x] 4.1 В `frontend/AGENTS.md` описать тему Keycloak: где лежит код, просмотр страниц через `?kcPage=`, команда `build:keycloak-theme` (нужен Maven, поэтому обычно сборка идёт в Docker), правило «только `@shared/*`». В скилле `frontend-design-system` упомянуть, что страницы входа строятся на той же системе. Проверка: упомянутые пути и команды существуют

## 5. Keycloak в окружении и на сервере

- [x] 5.1 Сервис `keycloak` в `infra/compose.yaml` и `infra/compose.prod.yaml` собирается из `infra/keycloak/Dockerfile` с контекстом `../frontend` в образ `goden-keycloak` (§10); `keycloak-config` остаётся на официальном образе. Проверка: `docker compose -f infra/compose.yaml config` и `docker compose -f infra/compose.prod.yaml config` с заполненным окружением проходят; `docker compose -f infra/compose.yaml up -d --build --wait keycloak` поднимает healthy Keycloak
- [ ] 5.2 `"loginTheme": "goden"` в `infra/keycloak/goden-realm.json` и `-s loginTheme=goden` в `infra/keycloak/configure.sh` (§11). Проверка: в окружении разработки страница `http://localhost:8180/realms/goden/account` перенаправляет на вход в оформлении Goden (скриншот); тестовый пользователь `manager` входит через неё во frontend `pnpm dev` и попадает в приложение (на «Сводку» точки при демонстрационной компании `DEMO_COMPANY=true`, иначе на первый вход); скрипт `configure.sh` против локального Keycloak выставляет `loginTheme`, это видно при чтении realm через `kcadm.sh get realms/goden`
- [x] 5.3 Интеграционный тест пакета `auth` в backend с изменённым файлом realm (Risks). Проверка: `go test ./internal/platform/auth/...` из `backend/` проходит, включая `auth_integration_test.go`
- [x] 5.4 Документация: `infra/README.md` (образ Keycloak собирается с темой, откат темы из Migration Plan, время и память сборки), `backend/AGENTS.md` (Keycloak окружения собирается из образа, команда пересборки после изменения темы `docker compose -f infra/compose.yaml up -d --build --wait keycloak`). Проверка: упомянутые файлы и команды существуют

## 6. Итог

- [ ] 6.1 Полные проверки. Проверка: `pnpm check` из `frontend/` проходит; `docker compose -f infra/compose.yaml up -d --build --wait` из корня проходит; страницы входа и регистрации в Keycloak окружения открываются в оформлении Goden (скриншоты светлой и тёмной темы)
