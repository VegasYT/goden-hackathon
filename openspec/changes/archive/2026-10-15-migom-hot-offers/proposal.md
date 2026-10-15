## Why

Покупатель Мигома должен видеть в меню блюда, которые Goden советует продвигать (горячие предложения: продукты с истекающим сроком). Учётная запись Мигома читает меню точки, но не рейтинг горячих предложений.

## What Changes

- Учётная запись `migom.integration` на подключённой работающей точке получает право `recommendations.view`: чтение горячих предложений (`GET /points/{pointId}/recommendations/dishes`) и рекомендаций поставок.

Вне изменения:
- ручные настройки горячих предложений (`menu.manage`) Мигому не даются;
- показ предложений покупателю - в Мигоме.

## Capabilities

### New Capabilities

Нет.

### Modified Capabilities

- `authorization`: права Мигома на точке.

## Impact

- `backend/`: `internal/migom/migom.go`, интеграционный тест прав Мигома, `backend/AGENTS.md`. Контракт `backend/api/openapi.yaml` не меняется.
