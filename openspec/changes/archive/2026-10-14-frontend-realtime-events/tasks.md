Часть репо: `frontend/`, команды из `frontend/` на Node 22. Задача 4.1 правит `docs/` в корне репо.

## 1. Поток с токеном

- [x] 1.1 Зависимость `eventsource` 5.x (`pnpm add eventsource`) и `FakeEventSource` в `shared/lib/test-utils`: экземпляры по порядку создания, `open()`, `emit(type, data)`, `fail({ closed })`, `close()`, запомненный `fetch` из опций. `vitest.setup.ts` подменяет пакет `eventsource` этим классом. Проверка: `pnpm typecheck`, `pnpm exec vitest run src/app/routes.test.tsx` (оболочка точки по-прежнему рисуется).
- [x] 1.2 `shared/api/event-stream.ts` — `openEventStream(path, { onOpen, onEvent, onDrop })` → `{ close, reconnect }`:
  - `fetch` с `Authorization` из `getAccessToken()` на каждое подключение;
  - сторож тишины: тело через `TransformStream` с отметкой времени, проверка раз в 15 с, больше 45 с — переподключение;
  - если пакет закрыл подключение после ошибки, новое подключение с паузой 3, 6, 12, 24, 30 с, сброс паузы после `open`;
  - `reconnect()` закрывает и подключает сразу.

  Проверка: `shared/api/event-stream.test.ts` на фальшивых таймерах — заголовок авторизации с токеном, переподключение после 45 с тишины и не раньше, растущая пауза после отказа и сброс после `open`; `pnpm exec vitest run src/shared/api`.

## 2. Перечитывание данных точки

- [x] 2.1 `entities/delivery`: `refreshDeliveries(queryClient, pointId)` — карточка и история сразу (`invalidateQueries` без списков и сводки), списки и сводка через `listRefreshDelayMs` на том же склеенном таймере, что `storeDelivery`. Проверка: дополнить `cache.test.ts` — история и карточка устарели сразу, списки нет, через секунду устарели и списки, три вызова подряд дают одно отложенное перечитывание; `pnpm exec vitest run src/entities/delivery`.
- [x] 2.2 `app/realtime/point-signals.ts`: категории → действия по таблице design; `refreshAfterSync(queryClient, pointId, { reconnect })` — первый раз только данные старше `pointDataFreshMs` без перезапуска идущих запросов, после переподключения все данные точки. Проверка: `point-signals.test.ts` на `QueryClient` с заготовленными запросами — `stop-list` помечает `kitchen` и не трогает `deliveries` и чужую точку, `stock` и `expiry` в одном сигнале перечитывают склад одним вызовом, первый `sync` не трогает свежие, повторный трогает; `pnpm exec vitest run src/app/realtime`.

## 3. Подписка точки и связь в оболочке

- [x] 3.1 `app/realtime/use-point-events.ts`: открывает поток выбранной точки в эффекте по `pointId` и закрывает при смене или уходе; `sync` и `update` разбирает по схемам `RealtimeSync` и `RealtimeUpdate` и передаёт в `point-signals`; ведёт `ConnectionState` (до подключения `undefined`, `online` с временем последнего сигнала, `offline` через 5 с без подключения с `dataAt` и `onRetry`). `PointLayout` передаёт состояние в `AppShell`. Проверка: `pnpm typecheck`, `pnpm lint:fsd`.
- [x] 3.2 Сценарии в `app/realtime.test.tsx` через `renderApp` и `FakeEventSource`:
  - на «Кухне» сигнал `stock`+`stop-list` перечитывает меню, список поставок не запрашивается;
  - открытые данные после первого `sync` не загружаются повторно, после обрыва и нового `sync` загружаются;
  - на «Поставках» сигнал `deliveries` перечитывает список через секунду, серия из трёх даёт один запрос;
  - индикатор «в сети · обновлено ЧЧ:ММ»; обрыв на 3 с без плашки; обрыв дольше 5 с — плашка, «Повторить» создаёт новое подключение;
  - смена точки закрывает прежний поток и открывает новый.

  Проверка: `pnpm exec vitest run src/app/realtime.test.tsx`.

## 4. Документы и итог

- [x] 4.1 `docs/ADR.md` (ADR-005, «Решение» и «Последствия») и `docs/plan.md` (раздел 5): веб-клиент не поллит; пока связи нет, оболочка показывает время данных, после переподключения клиент перечитывает данные точки. Проверка: `grep -n "polling\|периодически обновляет" docs/ADR.md docs/plan.md` не находит прежних формулировок.
- [x] 4.2 `frontend/AGENTS.md`: поток точки в `app/realtime`, новый раздел данных точки получает строку в таблице категорий, если его меняют сигналы; `FakeEventSource` в тестах. Проверка: упомянутые пути существуют (`ls`).
- [x] 4.3 Скриншоты Playwright на моках (временные `preview.html` и `src/preview-mock.tsx`, мок отвечает на `/me` и отдаёт поток через подменённый `eventsource`, порт не 5173): индикатор «в сети» на 1440 и 820, плашка «Нет связи» в светлой и тёмной теме. Harness удалить. Проверка: `git status` без файлов harness.
- [x] 4.4 Итоговый прогон. Проверка: `pnpm check` и `pnpm build` проходят.
