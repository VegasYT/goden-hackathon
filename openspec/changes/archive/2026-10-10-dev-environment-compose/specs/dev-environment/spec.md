# Spec Delta

## Purpose

Локальное окружение разработки даёт всем участникам одинаковый набор внешних сервисов - PostgreSQL, Kafka, Redis и Keycloak с готовым realm - одной командой, без ручной настройки.

## ADDED Requirements

### Requirement: Запуск окружения одной командой
Репозиторий SHALL содержать описание окружения в папке `infra/`, которое одной командой `docker compose -f infra/compose.yaml up -d` из корня поднимает PostgreSQL 18, Kafka 4 в режиме KRaft, Redis 8 и Keycloak 26. Каждый сервис SHALL иметь проверку готовности, а версии образов SHALL быть закреплены. Порты сервисов SHALL публиковаться только на `127.0.0.1` и SHALL переопределяться переменными окружения.

#### Scenario: Чистый запуск
- **WHEN** на машине с Docker выполняют `docker compose -f infra/compose.yaml up -d --wait` в свежем клоне репозитория
- **THEN** команда завершается успешно, все четыре сервиса находятся в состоянии healthy

#### Scenario: Повторный запуск сохраняет данные
- **WHEN** окружение останавливают командой `down` и запускают снова
- **THEN** данные PostgreSQL и Kafka сохранены

#### Scenario: Полный сброс
- **WHEN** выполняют `down -v` и запускают окружение снова
- **THEN** PostgreSQL и Kafka пусты, а realm Keycloak создан заново из файла

#### Scenario: Занятый порт
- **WHEN** порт сервиса на хосте занят и его переопределяют переменной окружения
- **THEN** сервис публикуется на новом порту без правки файлов репозитория

### Requirement: Backend работает с окружением
Процессы backend, запущенные с хоста, SHALL подключаться к сервисам окружения по адресам, записанным в `backend/AGENTS.md`. Порт Keycloak на хосте SHALL NOT совпадать с портами процессов backend.

#### Scenario: Миграции и API
- **WHEN** на запущенном окружении выполняют `cmd/migrate` и запускают `cmd/api` с адресом базы из документации
- **THEN** миграции применяются, `/readyz` отвечает 200

#### Scenario: Генератор
- **WHEN** на том же окружении запускают `cmd/generator` и включают его через API
- **THEN** в базе появляются события поставок и заказов

#### Scenario: Kafka и Redis доступны с хоста
- **WHEN** с хоста отправляют сообщение в Kafka и читают его, а в Redis записывают и читают ключ
- **THEN** обе операции проходят по адресам из документации

### Requirement: Realm Keycloak
Keycloak SHALL при старте импортировать realm `goden`. Realm SHALL содержать клиент `goden-api` с client roles для каждого permission из ADR-006, публичный клиент `goden-web` для frontend и клиент `goden-generator` с сервисной учётной записью. Права SHALL попадать в access token в `resource_access.goden-api.roles`, а `goden-api` SHALL присутствовать в `aud`.

#### Scenario: Realm доступен после запуска
- **WHEN** после запуска окружения запрашивают документ обнаружения OpenID Connect realm `goden`
- **THEN** он возвращается со статусом 200

#### Scenario: Все permissions существуют
- **WHEN** перечисляют client roles клиента `goden-api`
- **THEN** их 14 и они совпадают со списком permissions из ADR-006

#### Scenario: Сервисная учётная запись генератора
- **WHEN** клиент `goden-generator` получает токен по client credentials
- **THEN** в `resource_access.goden-api.roles` находятся ровно `catalog.view`, `deliveries.register`, `deliveries.approve` и `orders.accept`

### Requirement: Роли и тестовые пользователи
Realm SHALL содержать составные роли `admin`, `manager`, `approver`, `cook` и `waiter`, собранные из permissions, и по одному тестовому пользователю на каждую роль. Роль `admin` SHALL включать все permissions; `approver` SHALL включать `deliveries.view`, `deliveries.approve` и `inventory.view`; `manager` SHALL включать `inventory.view`; `generator.manage` SHALL входить только в `admin`. Учётные данные тестовых пользователей SHALL быть записаны в документации с пометкой, что они действуют только в локальном окружении.

#### Scenario: Токен приёмщика
- **WHEN** тестовый пользователь с ролью `approver` получает токен
- **THEN** в `resource_access.goden-api.roles` находятся ровно `deliveries.view`, `deliveries.approve` и `inventory.view`

#### Scenario: Токен администратора
- **WHEN** тестовый пользователь с ролью `admin` получает токен
- **THEN** в `resource_access.goden-api.roles` находятся все 14 permissions

#### Scenario: Управление генератором
- **WHEN** получают токены всех тестовых пользователей
- **THEN** `generator.manage` есть только у пользователя с ролью `admin`
