Часть репо: `frontend/`, команды из `frontend/` на Node 22.

## 1. Запросы в entities

- [x] 1.1 Новый слайс `entities/stock`: переезжают `stockKeys`, `stockSorts`/`StockSort`, `StockFilterParams`, `StockListParams`, `ExpiringParams`, `ExpiringBatch` и `expiringBatchesQueryOptions` из `pages/stock/api/stock.ts`. Остальные запросы склада, роут `stock.expiring`, `tab-counts.ts`, `use-batch-write.ts` и `point-signals.ts` берут их из `@entities/stock`. Проверка: `pnpm exec vitest run src/app/stock src/app/realtime src/app/realtime/point-signals.test.ts` без правок тестов, `pnpm typecheck`, `pnpm lint:fsd`.
- [x] 1.2 `entities/dish/api/hot-dishes.ts`: переезжают `hotDishKeys`, `hotDishesQueryOptions`, `HotDish`, `HotReason`, `HotDishParams`. `defaultHotDishesQueryOptions` остаётся в `pages/orders` и строится через entity. В `entities/dish/model` переезжают `stopCaption(dish)` и `hotCaption(dish, now)` из `order-new-page.tsx`, у формы заказа поведение прежнее. Импорты в `pages/orders`, роуте `orders.hot`, `point-signals.ts` и тестах заказов обновляются. Проверка: `pnpm exec vitest run src/app/order src/app/realtime src/pages/orders` без правок сценариев, `pnpm typecheck`, `pnpm lint:fsd`.

## 2. Сводка

- [x] 2.1 `pages/dashboard`: заглушка уходит, остаётся `PageHeader` «Сводка» с адресом точки. Права — `can(point, …)` по точке из профиля. Счётчик — `CardLink` с числом, словом через `countOf`, подписью и цветом важности; ноль окрашен как «в порядке». Сетка на 5 колонок на широком экране и на 3 на узком. Блоки по таблице design:
  - «Истекают за сутки» → «Сроки» с `within=24`;
  - «Стоп-лист» → «Стоп-лист»;
  - «Горячие предложения» → «Заказы › Горячие предложения»;
  - «Ожидают приёмки» → «Поставки» со `status=registered`;
  - «Открытый карантин» → «Поставки» с `panel=quarantine`.

  Подписи — чистые функции модели слайса. Проверка: `app/dashboard.test.tsx`:
  - у управляющего пять счётчиков: «5 партий» и «1 просрочена, спишется сама», «из 24 блюд меню», «в 2 поставках»;
  - нажатие на «Открытый карантин» открывает панель карантина в «Поставках»;
  - у повара только стоп-лист, запросов `/batches/expiring`, `/recommendations/dishes`, `/deliveries` нет;
  - при пустых ответах нули с подписями «в порядке».

  `pnpm typecheck`.
- [x] 2.2 Списки:
  - «Ближайшие сроки»: таблица на широком экране, строки на узком. В строке `ProductHistoryLink`, «из поставки № …», `formatAmount` и `ExpiryLabel` с «просрочена» и «спишется автоматически». Если партий больше 6, «и ещё N партий за сутки» со ссылкой;
  - «Стоп-лист»: все блюда со `stopCaption`;
  - «Предложить гостям»: 3 блюда с `hotCaption` и `portionsText(suggested_portions)`;
  - у каждого списка ссылка в раздел и короткое пустое состояние: «за сутки ничего не истекает», «Все блюда меню можно приготовить» или «В меню точки пока нет блюд», «Продвигать нечего».

  Проверка: дополняются сценарии `app/dashboard.test.tsx`:
  - у управляющего в списках «Брускетта с томатами» с «Не хватает: Томаты · есть 60 г, на порцию 100 г», «Паста карбонара» со «Сливки 33% · ещё 6 ч» и «5 порций»;
  - при `total` 13 — «и ещё 7 партий за сутки».
- [x] 2.3 Независимая загрузка: у каждого блока своя заготовка того же размера, у счётчика и списка с ошибкой — «Повторить» (`LoadErrorState` в списке, компактная ошибка в счётчике). Проверка: `app/dashboard.test.tsx` — меню отвечает 500, остальные блоки показывают данные, после «Повторить» и ответа 200 стоп-лист показан.
- [x] 2.4 Обновление по сигналу. Проверка: сценарий в `app/dashboard.test.tsx` или `app/realtime.test.tsx` — открыта «Сводка», `FakeEventSource` шлёт `stop-list`, меню перечитывается, новое блюдо появляется в счётчике и списке.

## 3. Документы и итог

- [x] 3.1 `frontend/AGENTS.md`:
  - пункт «Сводка» о блоках по правам, запросах разделов и ключах;
  - в пунктах «Склад» и «Заказы» новые места `stockKeys`, истекающих партий и `hotDishKeys`.

  Проверка: поиск по `AGENTS.md` — упомянутые пути существуют.
- [x] 3.2 Полная проверка: `pnpm check` зелёный. Если `routes.test.tsx` и другие тесты, открывающие сводку, падают на запросах без ответа, им добавляются ответы в `test-api`, сценарии не меняются. Скриншоты сводки 1440 и 820, светлая и тёмная тема, сверяются с макетами. Проверка: вывод `pnpm check`, файлы скриншотов.
