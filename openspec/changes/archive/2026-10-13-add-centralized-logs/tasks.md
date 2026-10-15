# Tasks

Часть репо: `infra/`. Все команды запускаются из корня репозитория `C:\work\hacks\goden\Goden`.

## 1. Конвейер Loki и Promtail

- [x] 1.1 Добавить single-binary конфигурацию Loki с TSDB/filesystem, retention `168h`, отдельным каталогом данных и readiness; подключить Loki с закреплённым образом и именованным томом в `infra/compose.yaml` и `infra/compose.prod.yaml`; проверка: `docker compose -f infra/compose.yaml config --quiet` и `docker compose --env-file infra/.env -f infra/compose.prod.yaml config --quiet` завершаются успешно, `docker compose -f infra/compose.yaml up -d --wait loki` переводит Loki в healthy.
- [x] 1.2 Добавить конфигурацию Promtail с Docker discovery только текущего compose-проекта, Docker pipeline, labels `environment`, `compose_project`, `compose_service`, `stream` и файлом positions; подключить закреплённый EOL-образ, read-only Docker mounts и отдельный том positions в оба compose; проверка: после `docker compose -f infra/compose.yaml up -d --wait promtail` запрос Loki по `compose_project="goden"` возвращает запись контейнера проекта, а labels результата не содержат container ID, `correlation_id`, `level` или `msg`.
- [x] 1.3 Проверить продолжение сбора после пересоздания контейнера и изоляцию чужого compose-проекта; проверка: после пересоздания одного сервиса его новые записи находятся по прежнему `compose_service`, а тестовая запись контейнера с другим compose project label отсутствует в запросе Loki проекта `goden`.
- [x] 1.4 Передать `LOG_LEVEL` в контейнеры API, worker и generator, использовать `INFO` по умолчанию и добавить настройку в `infra/.env.example`; проверка: итоговый `docker compose -f infra/compose.prod.yaml config` содержит `LOG_LEVEL: info` у всех трёх сервисов без override и выбранное значение `DEBUG` или `ERROR` при заданной переменной.

## 2. Grafana и эксплуатация

- [x] 2.1 Добавить provisioned datasource Loki с UID `loki`, внутренним URL и зависимостью Grafana от готового Loki, сохранив Prometheus источником по умолчанию; проверка: после `docker compose -f infra/compose.yaml up -d --wait grafana` Grafana API возвращает datasource `loki` без ручного создания, а его health check успешен.
- [x] 2.2 Проверить в запросах datasource фильтрацию по сервису, разбор JSON backend через `| json`, отдельный поиск `DEBUG`, `INFO`, `WARN`, `ERROR`, поиск по `correlation_id`, сохранение обычного текста и пустой результат без ошибки; проверка: запросы через Grafana datasource proxy по тестовым записям возвращают только выбранный уровень, ожидаемые JSON- и текстовые записи, а запрос по отсутствующему значению возвращает пустой `data.result` со статусом success.
- [x] 2.3 Обновить `infra/README.md`: описать `LOG_LEVEL` и его default `INFO`, четыре уровня, состав логового конвейера, границы локального сбора, примеры LogQL по сервису/уровню/correlation ID, семидневное хранение, просмотр состояния, очистку только логовых томов, откат и EOL Promtail; проверка: все упомянутые пути существуют, а команды compose и LogQL из документации выполняются как написано.

## 3. Сквозная проверка

- [x] 3.1 Проверить отказоустойчивость и сохранение: остановить Loki, убедиться, что остальные healthy-сервисы продолжают работать, запустить Loki снова, получить новые записи, затем пересоздать Loki и Promtail без удаления томов и убедиться, что прежние логи и позиции сохранены; проверка: `docker compose -f infra/compose.yaml ps`, health endpoints и запросы Loki/Grafana подтверждают каждый переход.
- [x] 3.2 Выполнить итоговую проверку инфраструктуры и OpenSpec; проверка: оба compose-файла проходят `docker compose ... config --quiet`, локальный набор проходит `docker compose -f infra/compose.yaml up -d --wait`, Loki/Promtail/Grafana имеют healthy, сквозной запрос по `compose_service` возвращает запись, а `openspec validate add-centralized-logs --strict` завершается успешно.
