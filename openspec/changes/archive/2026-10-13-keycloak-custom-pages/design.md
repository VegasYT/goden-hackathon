## Context

Мотивация в proposal.md. Факты из кода, от которых зависит подход:

- Frontend: Vite 8, React 19, MUI 9 (`ThemeRoot`, тема с CSS-переменными, `themeVars`), FSD с проверкой steiger, алиасы только по слоям. `src/app/main.tsx` показывает `StartupScreen`, запускает `startSession()` (keycloak-js) через top-level await, затем в `openApp` загружает профиль `GET /me` в кэш Query и только после этого создаёт роутер. В `index.html` есть скрипт `theme-init`, он ставит класс темы до загрузки по `localStorage['goden-color-mode']` или по теме системы.
- Keycloak 26.8.0 из официального образа: в `infra/compose.yaml` в режиме `start-dev` без тома, в `infra/compose.prod.yaml` в режиме `start` с томом `keycloak-data`. Файл realm применяется только при создании данных. Настройки работающего realm выставляет `infra/keycloak/configure.sh` (задача `keycloak-config`).
- В realm `goden` включена регистрация, user profile по умолчанию (логин, почта, имя, фамилия), вход по почте разрешён по умолчанию. `loginTheme`, `resetPasswordAllowed` и `rememberMe` не заданы. Язык только `ru`.
- На машине разработчика нет Maven, а Keycloakify требует его для упаковки jar.

## Goals / Non-Goals

**Goals:**
- Страницы темы строятся из тех же модулей `@shared/ui` и `@shared/ui/theme`, что и приложение, без копий токенов и стилей.
- Приложение, его сборка `pnpm build` и образ `frontend` не меняют поведения.
- Тема собирается в Docker, локально нужен только Docker.

**Non-Goals:**
- Storybook и визуальные снапшот-тесты.
- Переключатель темы и языка на страницах входа.

## Decisions

### 1. Keycloakify в том же Vite-проекте
Плагин `keycloakify/vite-plugin` подключается в `frontend/vite.config.ts` с параметрами `themeName: 'goden'` и `accountThemeImplementation: 'none'`. Целевой jar один, для Keycloak 26 (`keycloakVersionTargets`, вариант для 22–25 выключен). Выбор места сделал пользователь. Отдельный пакет по стартеру Keycloakify отвергнут: дизайн-систему пришлось бы публиковать между пакетами или копировать.

### 2. Код темы в `src/app/keycloak-theme/`
Keycloakify находит папку `keycloak-theme` на любой глубине `src`. В слое `app` FSD разрешает импорт из всех нижних слоёв, а произвольные сегменты допускаются. Поэтому тема лежит как сегмент `app` и импортирует только `@shared/*`. Внутри: `login/KcPage.tsx` (выбор страницы по `pageId`), `login/template.tsx` (каркас), `login/pages/*` (свёрстанные страницы), `login/components/*` (поле пароля, поля профиля, плашка сообщения), `login/i18n.ts`, `login/mocks.ts`. Имена `KcPage.tsx` и `KcContext.ts` задаёт Keycloakify, остальные файлы следуют стилю проекта. Вложенная папка называется `components`, а не `ui`: steiger запрещает имена сегментов внутри сегмента. Сгенерированный `kc.gen.tsx` коммитится и исключается из eslint, prettier и steiger, как `routeTree.gen.ts`.

Альтернативы: отдельный слайс `pages/kc-*` на каждую страницу Keycloak. Это девять слайсов, которые не являются роутами приложения и зависят от `kcContext`. Папка `src/keycloak-theme/` в корне выпала бы из схемы алиасов и потребовала бы исключения в steiger. Если steiger всё же отклонит сегмент `app`, первая задача это покажет, и тогда решение пересматривается до вёрстки страниц.

### 3. Развилка в `main.tsx`
Проверка `window.kcContext` стоит в начале `main.tsx`, до `StartupScreen`, `startSession()` и `openApp`. Если контекст задан, `main.tsx` рисует `<ThemeRoot><KcPage /></ThemeRoot>`: страница Keycloak не запускает keycloak-js, не запрашивает `/me` и не создаёт роутер. Иначе остаётся текущий поток. `KcPage` подгружается лениво, поэтому код темы не попадает в основной чанк приложения. Импорт шрифта остаётся общим.

### 4. Свой каркас и страницы, без CSS Keycloak
`doUseDefaultCss` равен `false` везде. `Template` построен из `StandaloneScreen`, `Logo`, MUI `Paper` и `Alert` с иконками из реестра `Icon`, тип сообщения Keycloak задаёт severity. Свёрстанные страницы: `login.ftl`, `register.ftl`, `login-verify-email.ftl`, `login-update-password.ftl`, `login-update-profile.ftl`, `logout-confirm.ftl`, `login-page-expired.ftl`, `info.ftl`, `error.ftl`. Это обычные HTML-формы `method='post'` на `url.loginAction` с именами полей Keycloak, отправка нативная. Поля из MUI `TextField`, подпись над полем, ошибка в `helperText`: так ошибка связана с полем через `aria-describedby`. После отправки основная кнопка блокируется флагом состояния. Наличие ссылок «Забыли пароль?», «Запомнить меня» и регистрации берётся из `kcContext.realm`, а не задаётся жёстко.

### 5. Поля профиля через `useUserProfileForm`
Регистрация и обновление профиля берут атрибуты и проверки из хука Keycloakify `useUserProfileForm`: валидаторы realm, подтверждение пароля и ошибки сервера по полям. Отрисовка своя, на MUI, и только для нужных типов: text, email, password. Незнакомый тип поля показывается как текстовое поле. Готовый `UserProfileFormFields` из Keycloakify отвергнут, потому что у него разметка PatternFly.

### 6. Прочие страницы через `DefaultPage`
Остальные `pageId` отрисовывает `DefaultPage` из Keycloakify внутри нашего `Template`. Для их нативных полей и кнопок внутри контейнера каркаса есть ограниченный набор стилей на CSS-переменных темы MUI, поэтому цвета и шрифт следуют теме. Включить для них CSS Keycloak (`doUseDefaultCss: true`) нельзя: глобальные стили PatternFly конфликтуют с MUI и дали бы второй визуальный язык.

### 7. Тема страниц следует системе
Страницы Keycloak открываются на другом адресе (`auth.<домен>`, локально порт 8180), поэтому выбор темы из приложения туда не переносится. `ThemeRoot` и скрипт `theme-init` работают как есть: сохранённого выбора на этом адресе нет, значит действует тема системы. Переключателя на страницах нет, это записано в Non-Goals. Вывод из кода, а не решение пользователя.

### 8. Тексты через i18n Keycloakify
Тексты берутся из сообщений Keycloak (`msg`, `msgStr`) с языком `ru`. Свои и изменённые формулировки, например «Логин или почта», задаются в `i18nBuilder.withCustomTranslations({ ru })`. Так тексты страниц живут в одном файле, а сообщения сервера остаются переводами Keycloak.

### 9. Просмотр страниц в `pnpm dev`
Только в DEV, когда в адресе есть параметр `kcPage` (например, `?kcPage=register.ftl&kcState=errors`), `main.tsx` динамически импортирует `mocks.ts` и выставляет `window.kcContext` из `getKcContextMock`. Набор состояний (`kcState`) задаётся в `mocks.ts`. Импорт стоит под `import.meta.env.DEV`, поэтому моки не попадают в сборку. Те же моки используют тесты Vitest. Storybook отвергнут: это новая тяжёлая сборка, а у проекта уже есть подход витрины `/dev/ui`.

### 10. Сборка jar и образ Keycloak
В `package.json` появляется команда `build:keycloak-theme` (`vite build`, затем `keycloakify build`: Keycloakify упаковывает готовый `dist` и сам его не собирает). `infra/keycloak/Dockerfile` состоит из двух стадий:
1. `node:22-bookworm-slim`, Maven и JDK из apt, `pnpm install --frozen-lockfile`, `pnpm build:keycloak-theme`. Кеш Maven в BuildKit cache mount.
2. `quay.io/keycloak/keycloak:26.8.0` и jar в `/opt/keycloak/providers/`.

Контекст сборки — `frontend/`, Dockerfile указывается путём из `infra/`. Сервис `keycloak` в `compose.yaml` и `compose.prod.yaml` получает `build` и образ `goden-keycloak`. Задача `keycloak-config` остаётся на официальном образе: ей нужен только `kcadm.sh`. Отдельный шаг сборки темы в CI отвергнут: деплой уже собирает образы командой `up --build` (`infra/deploy.sh`).

### 11. Включение темы в realm
В `goden-realm.json` появляется `"loginTheme": "goden"`, он действует для новых данных и окружения разработки. В `configure.sh` в настройки добавляется `-s loginTheme=goden`, он действует на работающем сервере при каждой выкладке. Темы писем, личного кабинета и консоли администратора не трогаются.

### 12. Тесты
Тесты Vitest на страницы с моками `kcContext` проверяют только нашу логику:
- вход с ошибкой: сообщение показано, логин подставлен, пароль пуст;
- выключенные в realm действия не показаны;
- ошибки регистрации у своих полей и связаны с ними;
- повторное нажатие не отправляет форму второй раз;
- нераспознанный `pageId` попадает в общий каркас.

Вид страниц проверяется глазами в `pnpm dev` и в Keycloak окружения.

## Risks / Trade-offs

- [Плагин Keycloakify может не поддерживать Vite 8 или rolldown] → Первая задача ставит зависимость и собирает jar в Docker до вёрстки. При несовместимости работа останавливается, и проблема выносится пользователю. Версия Vite молча не понижается.
- [Сборка образа Keycloak на сервере дольше: pnpm install и Maven, около 1 ГБ памяти] → Кеш Maven в cache mount, слой зависимостей pnpm переиспользуется, пока не менялся `pnpm-lock.yaml`. Требование к памяти сервера в `infra/README.md` проверяется после первой сборки.
- [Интеграционный тест пакета `auth` в backend поднимает официальный Keycloak с файлом realm, где `loginTheme: goden`, а темы там нет] → Гипотеза: Keycloak с ненайденной темой страниц пишет предупреждение и использует стандартную, а тест получает токены без страниц. Задача прогоняет этот тест. Если он падает, тест получает образ с темой или переопределение темы.
- [Откат на версию без темы при `loginTheme=goden` в данных сервера] → `configure.sh` старой версии тему не сбрасывает. Порядок отката в Migration Plan.
- [Страницы без отдельной вёрстки выглядят проще] → Их маршруты сейчас недостижимы в настройках realm. Это сознательное решение пользователя.

## Migration Plan

1. Обычная выкладка `infra/deploy.sh`: соберётся `goden-keycloak`, контейнер Keycloak пересоздастся с тем же томом, а `keycloak-config` выставит `loginTheme=goden`.
2. Откат: выкладка предыдущего коммита и смена темы входа realm `goden` на `keycloak.v2` в консоли администратора.
