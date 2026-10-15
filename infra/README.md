# Инфраструктура

- `compose.yaml` - окружение разработки: внешние сервисы для процессов, запущенных с хоста. Описано в [../backend/AGENTS.md](../backend/AGENTS.md).
- `compose.prod.yaml` - вся система на одном сервере. Описана здесь.

## Сервер

Адреса (домен задаётся в `infra/.env`):

| Адрес | Что отвечает |
| --- | --- |
| `https://goden.pro` | frontend |
| `https://goden.pro/api/v1` | API |
| `https://goden.pro/docs` | документация API (Swagger UI) |
| `https://auth.goden.pro` | Keycloak, консоль администратора: пользователь `admin` |
| `https://grafana.goden.pro` | Grafana, пользователь `admin` |

Наружу открыты только порты 80 и 443 прокси Caddy. Он сам получает и продлевает сертификаты Let's Encrypt. PostgreSQL, Kafka, Prometheus и служебные порты процессов (`/metrics`, `/readyz`) из интернета недоступны.

### Что нужно до первого запуска

1. Записи A в DNS на адрес сервера: `goden.pro`, `auth.goden.pro`, `grafana.goden.pro`. Без них сертификаты не выдаются.
2. Docker с плагином compose на сервере, не меньше 2 ГБ памяти (в работе набор занимает около 1,5 ГБ, сборка образов - дополнительно; больше всего, около 1 ГБ, берёт сборка образа Keycloak с темой).
3. Клон репозитория на сервере, например в `~/goden`. Путь к нему хранится в секрете `DEPLOY_PATH`. Репозиторий закрытый: серверу нужен ключ чтения, добавленный в GitHub (Settings, Deploy keys).

### Первый запуск

```bash
cd ~/goden
cp infra/.env.example infra/.env
nano infra/.env          # заполнить пароли; сгенерировать значение: openssl rand -hex 24
sh infra/deploy.sh
```

Адрес Keycloak для frontend отдельно задавать не нужно: он вшивается в сборку образа из `DOMAIN` и `SCHEME` (`https://auth.<домен>`). Если домен меняется, образ frontend нужно пересобрать, что `infra/deploy.sh` и делает.

Раздел «Генератор» во frontend (запуск генератора и тестовые поставки, виден только владельцу) скрывает `GENERATOR_UI=false` в `infra/.env`. Значение тоже вшивается в сборку, поэтому после изменения выполни `sh infra/deploy.sh`.

Файл `infra/.env` в репозиторий не попадает. Без обязательной переменной запуск останавливается с сообщением, какой не хватает. Пароли разработки (`admin` / `admin`, пароль равен логину) на сервере не действуют.

Первый запуск занимает несколько минут: собираются образы backend, frontend и Keycloak с темой страниц входа, Keycloak импортирует realm.

Проверка:

```bash
docker compose -f infra/compose.prod.yaml ps
curl -s -o /dev/null -w '%{http_code}\n' https://goden.pro/api/v1/me   # 401: API жив и требует токен
```

После запуска есть компания "Демо" с двумя точками: её владелец - пользователь `admin`, пользователи `manager`, `approver`, `cook` и `waiter` назначены на первую точку в своих ролях, генератор на первой точке включён и наполняет каталог и склад. Остановить его - `PUT /api/v1/points/<точка>/generator` под `admin` (идентификаторы точек - в `GET /api/v1/me`; удобно через `https://goden.pro/docs`). Токен:

```bash
curl -s https://auth.goden.pro/realms/goden/protocol/openid-connect/token \
  -d grant_type=password -d client_id=goden-web -d username=admin -d password=<GODEN_PASSWORD_ADMIN>
```

### Выкладка

Автоматически: коммит попадает в `main`, GitHub Actions запускает проверки backend и frontend и при успехе выполняет на сервере `infra/deploy.sh`. Скрипт приводит рабочую копию к этому коммиту и перезапускает изменившиеся сервисы. На push в другие ветки ничего не запускается. Повторить выкладку вручную: страница Actions, workflow `main`, Run workflow.

Вручную на сервере: `sh infra/deploy.sh`.

Рабочую копию на сервере руками не правят: скрипт отбрасывает изменения отслеживаемых файлов. `infra/.env` сохраняется.

Проверки можно отключить переменной репозитория `SKIP_INTEGRATION_TESTS` со значением `true` (Settings, Secrets and variables, Actions, вкладка Variables). Тогда пропускаются все тесты и линтеры backend и frontend, остаётся только сборка, и выкладка начинается сразу после неё. Пока переменная включена, на сервер попадает любой код, который собирается: включай её на время и не забывай убрать.

Секреты репозитория GitHub (Settings, Secrets and variables, Actions):

| Секрет | Значение |
| --- | --- |
| `DEPLOY_HOST` | адрес сервера |
| `DEPLOY_USER` | пользователь SSH; должен входить в группу `docker` |
| `DEPLOY_PATH` | путь к клону репозитория на сервере |
| `DEPLOY_SSH_KEY` | закрытая часть ключа выкладки целиком, без пароля; открытая часть добавлена в `~/.ssh/authorized_keys` на сервере |
| `DEPLOY_KNOWN_HOSTS` | вывод `ssh-keyscan <адрес сервера>` |

### Централизованные логи

Контейнеры пишут в `stdout`/`stderr`, Promtail читает Docker log stream только своего compose-проекта и отправляет записи в Loki, а Grafana обращается к Loki через provisioned datasource с UID `loki`. Конфигурации лежат в `infra/promtail/promtail.yml`, `infra/loki/loki.yml` и `infra/grafana/provisioning/datasources/prometheus.yml`.

В production собираются логи контейнеров проекта `goden-prod`. В окружении разработки собираются только контейнеры проекта `goden`: API, worker и другие процессы, запущенные с хоста, в Loki не попадают.

Минимальный уровень API, worker и generator задаёт `LOG_LEVEL` в `infra/.env`: `debug`, `info`, `warn` или `error`. Значение по умолчанию — `info`, поэтому сохраняются `INFO`, `WARN` и `ERROR`, а `DEBUG` включается явно. После изменения примени настройку:

```bash
docker compose -f infra/compose.prod.yaml up -d --force-recreate api worker generator
```

Логи находятся в секции «Логи» существующего дашборда **Goden backend** (UID `goden-backend`): локально — `http://127.0.0.1:3000/d/goden-backend/goden-backend`, на сервере — `https://grafana.<домен>/d/goden-backend/goden-backend`. Отдельный дашборд для логов не создаётся.

Над дашбордом доступны фильтры:

- «Окружение» и «Сервис» влияют на все логовые панели;
- «Уровень» выбирает `DEBUG`, `INFO`, `WARN`, `ERROR` или несколько уровней;
- «Correlation ID» принимает регулярное выражение, по умолчанию `.*`.

Панель «Ошибки по сервисам за период» считает записи уровня `ERROR` за выбранный интервал. «Структурированные логи» показывает только корректные JSON-записи и применяет все четыре фильтра. «Все контейнерные логи» показывает исходный поток, включая обычный текст; фильтры уровня и `correlation_id` к нему не применяются.

Для произвольного поиска можно открыть Grafana → Explore и выбрать источник `Loki`. Примеры LogQL:

```logql
{environment="production", compose_service="api"}
{environment="production", compose_service="worker"} | json | __error__="" | level="ERROR"
{environment="production"} | json | __error__="" | correlation_id="<correlation-id>"
{environment="production", compose_service="generator"} |= "текст"
```

Метки ограничены `environment`, `compose_project`, `compose_service` и `stream`; `level`, `correlation_id`, сообщение и идентификатор контейнера остаются содержимым записи. Loki хранит данные семь суток (`168h`) в отдельном томе. Состояние:

```bash
docker compose -f infra/compose.prod.yaml ps loki promtail grafana
docker compose -f infra/compose.prod.yaml logs --tail=100 loki promtail grafana
curl -fsS http://127.0.0.1:3100/ready  # только для локального compose.yaml, не production
```

Удалить только историю логов и позиции чтения, не затрагивая PostgreSQL, Kafka, Keycloak и сертификаты:

```bash
docker compose -f infra/compose.prod.yaml rm -sf loki promtail
docker volume rm goden-prod_loki-data goden-prod_promtail-positions
docker compose -f infra/compose.prod.yaml up -d loki promtail grafana
```

Для локального набора имена томов — `goden_loki-data` и `goden_promtail-positions`. Откат выполняется выкладкой предыдущего коммита; перед ней можно остановить новые компоненты командой `docker compose -f infra/compose.prod.yaml stop promtail loki`. Тома можно оставить: они не используются остальными сервисами.

Promtail достиг конца срока поддержки; закреплённый образ оставлен для текущего хакатонного контура. Для дальнейшей эксплуатации его следует заменить на Grafana Alloy по [официальной инструкции миграции](https://grafana.com/docs/loki/latest/send-data/promtail/).

### Мигом на этом сервере

Сервис доставки Мигом (репозиторий `migom_goden`, папка `~/migom`) выкладывается на этот же сервер своим `infra/deploy.sh` и подключается к сети Goden `goden-prod_default`: Caddy Goden открывает его адреса, Мигом пользуется Kafka, Keycloak и API Goden. Выкладки Goden и Мигома идут по очереди (блокировка `/tmp/server-deploy.lock`): две сборки сразу не помещаются в память.

| Адрес | Что отвечает |
| --- | --- |
| `https://migom.site` | API и WebSocket Мигома для мобильных приложений, фото товаров (`/migom-media/`) |
| `https://owner.migom.site` | кабинет владельца |
| `https://admin.migom.site` | кабинет модератора |

Домен задаёт `MIGOM_DOMAIN` (по умолчанию `migom.site`); нужны записи A на сервер для самого домена, `www.`, `owner.` и `admin.`. Пока Мигом не запущен, его адреса отвечают 502. Секреты клиентов Мигома в Keycloak - `MIGOM_BACKEND_SECRET` и `MIGOM_INTEGRATION_SECRET` в `infra/.env` (см. ниже про `keycloak-config`); те же значения задаются в `infra/.env` Мигома. Первый запуск Мигома описан в его README.

### Обслуживание

```bash
docker compose -f infra/compose.prod.yaml logs -f api worker      # логи
docker compose -f infra/compose.prod.yaml down                    # остановить, данные остаются
docker compose -f infra/compose.prod.yaml down -v                 # остановить и стереть данные и сертификаты
```

- Версия с компаниями и точками (ADR-013) несовместима с данными PostgreSQL и Kafka прежних версий: события не содержат компанию и точку. Перед первой её выкладкой на сервер с прежними данными их нужно удалить, как описано в следующем пункте.
- Начать с пустых данных: `docker compose -f infra/compose.prod.yaml down`, затем `docker volume rm goden-prod_postgres-data goden-prod_kafka-data goden-prod_keycloak-data` и `sh infra/deploy.sh`. Том `caddy-data` с сертификатами не удаляй: у Let's Encrypt есть лимит на повторную выдачу. Тома PostgreSQL и Keycloak удаляй вместе: компании и назначения ссылаются на идентификаторы пользователей Keycloak, и с новым Keycloak старые компании остались бы без владельцев. Тестовые пользователи исключение: их идентификаторы заданы в файле realm.
- Keycloak хранит данные в томе `keycloak-data`: пользователи регистрируются сами и не должны пропадать при выкладке. Файл `infra/keycloak/goden-realm.json` применяется только при первом создании тома. Сроки токенов, русский язык форм, тему страниц входа и подтверждение почты после этого выставляет разовая задача `keycloak-config` (`infra/keycloak/configure.sh`) при каждой выкладке, из переменных `infra/.env`. Она же заводит клиентов сервиса доставки Мигом (`migom-api`, `migom-backend`, `migom-integration`), если их нет, и выставляет их секреты из `MIGOM_BACKEND_SECRET` и `MIGOM_INTEGRATION_SECRET`; без секрета клиенту выставляется случайный, и секрет разработки из файла realm на сервере не действует. Остальное (адреса возврата, пароли, другие клиенты) вносят в консоли администратора `https://auth.<домен>` либо пересоздают оба тома данных, как описано выше. Задача входит в Keycloak администратором с паролем `KEYCLOAK_ADMIN_PASSWORD`: если пароль сменили в консоли, задай тот же в `infra/.env`. Её сбой не мешает запуску системы и виден в журнале: `docker compose -f infra/compose.prod.yaml logs keycloak-config`.
- Страницы входа, регистрации и другие страницы Keycloak оформлены своей темой `goden`. Её код лежит во frontend (`frontend/src/app/keycloak-theme/`), образ `goden-keycloak` собирается по `infra/keycloak/Dockerfile` из папки `frontend/`: официальный Keycloak и jar темы. Отдельного шага нет: образ пересобирает обычная выкладка `infra/deploy.sh` (`up --build`), а тему в realm включает задача `keycloak-config`, том пересоздавать не нужно. Сборка ставит зависимости pnpm и Maven, поэтому идёт дольше остальных и требует около 1 ГБ памяти; первая сборка заметно дольше, дальше слой зависимостей pnpm (пока не менялся `pnpm-lock.yaml`) и кеш Maven переиспользуются. Личный кабинет, консоль администратора и письма Keycloak остаются в стандартной теме.
- Откатить тему: выложить предыдущий коммит (`sh infra/deploy.sh <коммит>` или revert в `main`) и в консоли администратора сменить тему входа realm `goden` на `keycloak.v2` (Realm settings, вкладка Themes, Login theme). Задача `keycloak-config` прежней версии тему не сбрасывает.
- Подтверждение почты при регистрации: задать в `infra/.env` `KEYCLOAK_VERIFY_EMAIL=true` и почтовый сервер (`SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`), затем выполнить `docker compose -f infra/compose.prod.yaml up -d --force-recreate keycloak-config`. Выключить: `KEYCLOAK_VERIFY_EMAIL=false` и та же команда. Пользователи при переключении сохраняются; зарегистрировавшийся при включённом подтверждении и не перешедший по ссылке входит сразу после выключения. Без `SMTP_HOST` подтверждение не включается: иначе новые пользователи не смогли бы войти. Пароль - пароль приложения почтового ящика. Перед показом проверь, что письмо приходит и не попадает в спам.
- Включить уведомления в Telegram: задать в `infra/.env` обе переменные `TELEGRAM_BOT_TOKEN` и `TELEGRAM_CHAT_ID` и выполнить `docker compose -f infra/compose.prod.yaml up -d worker`. `TELEGRAM_CHAT_ID` - общий чат: в него приходят только ошибки обработчиков. Уведомления о сроках и списании просрочки приходят в чат точки: его задаёт владелец или управляющий в приложении (`PUT /api/v1/points/<точка>/telegram-chat`), а для первой точки "Демо" - переменная `DEMO_TELEGRAM_CHAT_ID` (задача `migrate` при запуске задаёт этот чат один раз, пока у точки нет своего; позже чат меняется только в приложении, смена переменной на уже заданный чат не влияет). Сбой создания компании "Демо" запуск не останавливает: причина в журнале `migrate`. Точка без чата уведомлений не получает. Бот должен быть добавлен в чат; для группы идентификатор отрицательный. С сервера в РФ Telegram напрямую недоступен: задай ещё `TELEGRAM_PROXY_URL` (`socks5://пользователь:пароль@хост:порт`). Проверить прокси с сервера: `curl -m 10 --proxy "$TELEGRAM_PROXY_URL" https://api.telegram.org/`.
- Разрешить работу с сервером локальному frontend (`pnpm dev`): задать в `infra/.env` `DEV_WEB_URL=http://localhost:5173` и выполнить `docker compose -f infra/compose.prod.yaml up -d --force-recreate api`: адрес добавляется в CORS API. В Keycloak адрес попадает из той же переменной только при первом создании тома; если том уже есть, добавь адрес в консоли администратора: клиент `goden-web`, поля Valid redirect URIs и Web origins. Frontend при этом настраивается на `https://goden.pro` и `https://auth.goden.pro`.
- Сменить пароль тестового пользователя: в консоли администратора Keycloak (Users, вкладка Credentials). Значения `GODEN_PASSWORD_*` из `infra/.env` действуют только при первом создании тома.

### Проверить серверный набор локально

Без домена и сертификатов, по HTTP. Имена `*.localhost` не подходят: внутри контейнеров они ведут на сам контейнер.

```bash
cp infra/.env.example infra/.env     # DOMAIN=goden.test, SCHEME=http, любые пароли
docker compose -f infra/compose.prod.yaml up -d --build
curl --resolve goden.test:80:127.0.0.1 http://goden.test/api/v1/me   # 401 без токена: API отвечает
```
