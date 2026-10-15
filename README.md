<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="frontend/src/shared/ui/logo/logo-dark.svg">
    <img src="frontend/src/shared/ui/logo/logo-light.svg" alt="Goden" width="260">
  </picture>
</p>

<p align="center">
  Учёт остатков, сроков годности и стоп-листов для сети ресторанов.<br>
  Кейс компании "Флагман".
</p>

<p align="center">
  <a href="https://goden.pro">Демо</a> ·
  <a href="https://goden.pro/docs">API</a> ·
  <a href="docs/Case.md">Кейс</a> ·
  <a href="docs/ADR.md">Архитектурные решения</a>
</p>

<p align="center">
  <a href="goden-demo.mp4"><img src="assets/goden-demo.jpg" alt="Видео: Goden за 47 секунд" width="860"></a><br>
  <sub>Goden за 47 секунд: нажмите, чтобы посмотреть видео</sub>
</p>

## Что умеет

- **Поставки.** Регистрация, приёмка по позициям, карантин и исправление приёмки. Просроченное принять нельзя.
- **Склад.** Партии со сроками годности, списание по FEFO: первой уходит партия с ближайшим сроком.
- **Заказы.** Проверка ингредиентов и списание в одной транзакции. Не хватает продукта - заказ отклонён. Принятый заказ можно отменить с комментарием, продукты вернутся в партии; в истории видно, кто заказ оформил и кто отменил.
- **Стоп-лист.** Блюдо само уходит из меню, когда на него не хватает ингредиентов, и возвращается после поставки.
- **Сроки годности.** Предупреждение до истечения срока и отметка просроченных партий, уведомления в Telegram. Просрочку списывает управляющий: система сама ничего не списывает.
- **Горячие предложения.** Блюда, которые стоит продавать в первую очередь, с объяснением причины. Блюдо можно закрепить в списке или скрыть до конца дня.
- **Каталог.** Продукты, блюда и рецептуры компании: поиск, правка, архив.
- **Аналитика.** Списания, дефицитные ингредиенты, прогноз стоп-листа, рекомендации поставок.
- **Компании и точки.** Владелец создаёт компанию и точки, назначает сотрудников: управляющий, приёмщик, повар, официант.
- **Доставка через Мигом.** Владелец подключает к точке заведение сервиса доставки [Мигом](https://migom.site) ([код](https://github.com/VegasYT/migom_goden)). Мигом берёт меню и горячие предложения точки, оформляет и отменяет заказы, его кабинеты входят через Keycloak Goden.

## Как устроено

```mermaid
flowchart LR
    web[Frontend] -->|REST, SSE| api[API]
    gen[Генератор] -->|REST| api
    migom[Мигом] -->|REST| api
    api --> pg[(PostgreSQL<br>Event Store)]
    pg --> worker[Worker]
    worker -->|события| kafka[(Kafka)]
    kafka -->|сигналы| api
    worker --> tg[Telegram]
    api -.-> redis[(Redis)]
    web -.-> kc[Keycloak]
    api -.-> kc
```

Состояние меняется только добавлением событий в PostgreSQL. Остатки и партии пишутся в той же транзакции, списки и аналитика строятся из событий асинхронно. Kafka разносит события подписчикам: уведомлениям и потоку обновлений интерфейса.

| Часть | Технологии |
| --- | --- |
| Backend | Go, PostgreSQL 18, Kafka, Redis, ogen, sqlc |
| Frontend | React 19, TypeScript, Vite, MUI, TanStack Router и Query |
| Доступ | Keycloak, права по владению компанией и роли на точке |
| Инфраструктура | Docker Compose, Caddy, Prometheus, Grafana, Loki, GitHub Actions |

## Запуск

Нужны Docker, Go 1.27 и pnpm.

```bash
# внешние сервисы: PostgreSQL, Kafka, Redis, Keycloak
docker compose -f infra/compose.yaml up -d --wait

# backend, каждая команда в своём терминале
cd backend
export DATABASE_URL="postgres://goden:goden@localhost:15432/goden?sslmode=disable"
export OIDC_ISSUER="http://localhost:8180/realms/goden"
DEMO_COMPANY=true go run ./cmd/migrate
go run ./cmd/api
go run ./cmd/worker

# frontend
cd frontend
pnpm install
pnpm dev
```

Приложение откроется на `http://localhost:5173`. Вход: `admin` / `admin`, он владеет демонстрационной компанией. Подробности и остальные пользователи - в [backend/AGENTS.md](backend/AGENTS.md).

Запуск всей системы на сервере одной командой описан в [infra/README.md](infra/README.md).

## Репозиторий

| Папка | Содержимое |
| --- | --- |
| [`backend/`](backend/AGENTS.md) | API, worker, генератор событий, миграции |
| [`frontend/`](frontend/AGENTS.md) | веб-клиент |
| [`infra/`](infra/README.md) | окружение разработки и серверный набор |
| [`docs/`](docs/) | кейс, архитектурные решения, план |
| [`openspec/`](openspec/specs/) | требования к системе по возможностям |

Контракт API - [`backend/api/openapi.yaml`](backend/api/openapi.yaml): из него генерируются сервер и типы клиента.
