# Spec Delta

## MODIFIED Requirements

### Requirement: Запуск окружения одной командой
Репозиторий SHALL содержать описание окружения в папке `infra/`, которое одной командой `docker compose -f infra/compose.yaml up -d` из корня поднимает PostgreSQL 18, Kafka 4 в режиме KRaft, Redis 8, Keycloak 26, Prometheus, Loki, Promtail и Grafana. Каждый сервис SHALL иметь проверку готовности, а версии образов SHALL быть закреплены. Порты сервисов SHALL публиковаться только на `127.0.0.1` и SHALL переопределяться переменными окружения.

#### Scenario: Чистый запуск
- **WHEN** на машине с Docker выполняют `docker compose -f infra/compose.yaml up -d --wait` в свежем клоне репозитория
- **THEN** команда завершается успешно, все сервисы находятся в состоянии healthy

#### Scenario: Повторный запуск сохраняет данные
- **WHEN** окружение останавливают командой `down` и запускают снова
- **THEN** данные PostgreSQL и Kafka сохранены

#### Scenario: Полный сброс
- **WHEN** выполняют `down -v` и запускают окружение снова
- **THEN** PostgreSQL и Kafka пусты, а realm Keycloak создан заново из файла

#### Scenario: Занятый порт
- **WHEN** порт сервиса на хосте занят и его переопределяют переменной окружения
- **THEN** сервис публикуется на новом порту без правки файлов репозитория
