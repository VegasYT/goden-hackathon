Часть репо: `frontend/`, все команды запускаются из `frontend/` на Node 22. Работа идёт в ветке `frontend/stock-tabs`.

## 1. Вкладки склада и общие части

- [x] 1.1 Действия с партией без строки остатков: `productStock(batches, now)` в `pages/stock/model`, `WriteOffDialog`, `AdjustDialog` и `ProductLine` принимают `Product`; диалоги, уведомление и «История» вынесены из `StockPage` в `BatchActionDialogs`; `HistoryPanel` берёт «доступно · N партий» из `productStock` по партиям продукта. Проверка: unit-тест `productStock` (просроченная партия не входит в доступное), `pnpm exec vitest run src/app/stock.test.tsx src/app/stock-actions.test.tsx`, `pnpm typecheck`.

- [x] 1.2 Роуты вкладок: `points.$pointId.stock.tsx` становится layout (шапка, `Tabs` через новый `TabLink` в `shared/ui/link`, `HistoryPanel`, search `{ history }`), «Остатки» переезжают в `points.$pointId.stock.index.tsx`, заготовки `expiring` и `reorder`; `requirePermission` в `app/point-access.ts` для `reorder`; числа вкладок — запросы `stockKeys.expiring` (`within_hours=24, limit=1`) и `recommendationKeys.deliveries` (первая страница по умолчанию, только с `recommendations.view`); `stockApi` в `app/test-stock.ts` отвечает на них. Проверка: тест в `src/app/stock-tabs.test.tsx` — у управляющего «Сроки 4» и «Пора заказать 5»; у приёмщика нет «Пора заказать» и запросов рекомендаций, по адресу `reorder` — «Нет доступа к разделу»; ссылка `stock?history=…` открывает «Остатки» с панелью (существующий тест стоп-листа), `pnpm lint:fsd`.

## 2. Вкладка «Сроки»

- [x] 2.1 Список истекающих партий: запрос с `within_hours`, `limit`, `offset`; период `SegmentedControl` 24 часа / 3 дня / 7 дней в адресе (`within`, по умолчанию 72), смена периода — первая страница; таблица на ноутбуке и карточки на планшете со статусом через `ExpiryLabel`, номером поставки-ссылкой и названием продукта-ссылкой на историю (`TextLink color='inherit' underline='hover'`); пояснение о самосписании; `ListPagination`; заготовка, «За <период> ничего не истекает» с «Показать за 7 дней», ошибка с «Повторить». Проверка: тест в `src/app/stock-expiring.test.tsx` — порядок и статусы (просроченная, «ещё 17 ч»), смена периода отправляет `within_hours=168` и `offset=0` и пишет период в адрес, пустое состояние переключает на 7 дней, название продукта открывает историю поверх «Сроков».

- [x] 2.2 Списание со «Сроков»: «Списать» у годной партии открывает `BatchActionDialogs` с этой партией, у просроченной действий нет; без права или на отключённой точке — `ReadOnlyNote`; `storeBatch` перечитывает `['points', pointId, 'stock', 'expiring']` и `['points', pointId, 'recommendations']`. Проверка: тест в `src/app/stock-expiring.test.tsx` — после списания список и число на вкладке перечитаны, показано уведомление; у приёмщика нет «Списать» и есть пояснение.

## 3. Вкладка «Пора заказать»

- [x] 3.1 Рекомендации: `recommendationKeys` и запрос с явными `window_hours=24`, `cover_hours=24`, `target_hours=72`; строка параметров; таблица и карточки: доступно («нет в наличии»), расход («не было»), «Хватит на» (часы вниз, «меньше часа», «—»), «Заказать», блюда — два и «и ещё N» с подсказкой; название продукта открывает историю; «Как считаем» свёрнуто и раскрывается; `ListPagination`; заготовка, «Заказывать пока нечего», ошибка с «Повторить». Проверка: тест в `src/app/stock-reorder.test.tsx` — строки рукколы («9 ч») и пармезана («не было», «—»), «и ещё 1», «Как считаем» раскрывает пояснение, пустое состояние.

## 4. Ссылка на продукт в стоп-листе

- [x] 4.1 В `pages/stop-list/ui/stop-dish-card.tsx` ссылка на историю продукта — `TextLink color='inherit' underline='hover'`. Проверка: `pnpm exec vitest run src/app/kitchen.test.tsx` (ссылка «Молоко» ведёт на историю).

## 5. Завершение

- [x] 5.1 `frontend/AGENTS.md`: вкладки склада — дочерние роуты, `requirePermission`, `TabLink`, `recommendationKeys`, `productStock`, ссылка на продукт без выделения. Проверка: упомянутые пути существуют.

- [x] 5.2 Полная проверка: `pnpm check` и `pnpm build`.
