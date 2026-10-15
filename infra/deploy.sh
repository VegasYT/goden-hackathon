#!/bin/sh
# Выкладка на сервере: привести рабочую копию к коммиту и перезапустить изменившиеся сервисы.
# Запускает GitHub Actions по SSH; вручную: sh infra/deploy.sh [коммит], по умолчанию origin/main.
set -eu

# Выкладки Goden и Мигома на одном сервере идут по очереди: две сборки сразу не помещаются в память
exec 9>/tmp/server-deploy.lock
flock 9

cd "$(dirname "$0")/.."
git fetch --quiet origin
# Рабочая копия сервера не редактируется вручную: локальные правки отслеживаемых файлов
# здесь отбрасываются. infra/.env в репозиторий не входит и сохраняется.
git checkout --quiet --force -B main "${1:-origin/main}"

# Тема Keycloak и frontend собирают одно и то же приложение; вместе им не хватает памяти сервера,
# поэтому по очереди. Команда up ниже берёт уже собранные слои из кеша.
docker compose -f infra/compose.prod.yaml build keycloak
docker compose -f infra/compose.prod.yaml build frontend
docker compose -f infra/compose.prod.yaml up -d --build --remove-orphans
# старые слои после пересборки, иначе диск сервера заполнится
docker image prune --force > /dev/null
git log --oneline -1
