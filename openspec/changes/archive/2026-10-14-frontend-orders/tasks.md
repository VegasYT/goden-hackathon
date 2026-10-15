Часть репо: `frontend/`, команды из `frontend/` на Node 22.

## 1. Контракт, ключи и поток

- [x] 1.1 `pnpm api:types`: в `schema.gen.ts` появляются `Shortage.product` и категория `orders`. Склеенный отложенный `invalidateQueries` выносится из `entities/delivery/model/cache.ts` в `shared/lib/query` (`invalidateLater(queryClient, queryKey, delayMs)`, таймер по клиенту и ключу), `scheduleListRefresh` поставок переходит на него. Проверка: `pnpm exec vitest run src/entities/delivery` без изменений в тестах, `pnpm typecheck` падает только на строке `orders` в `point-signals.ts`.
- [x] 1.2 `pages/orders/api/orders.ts`: `orderKeys` (`all`, `lists`, `list`), `orderListQueryOptions` (статус, `limit`, `offset`, `keepPointData`), `placeOrder(pointId, body, key)`. `model/refresh.ts`: `refreshOrders` — списки заказов через `listRefreshDelayMs` по `invalidateLater`. В `point-signals.ts` строка `orders: refreshOrders`. Публичный API слайса — `orderKeys` и `refreshOrders`. Проверка: `point-signals.test.ts` — `orders` помечает списки заказов только после задержки и не трогает склад, кухню и другую точку; `pnpm typecheck`, `pnpm lint:fsd`.

## 2. Раздел и история

- [x] 2.1 Роуты `points.$pointId.orders.tsx` (layout: `requireSection`, счётчик предложений при `recommendations.view`), `orders.index.tsx` (история) и `ui/orders-layout.tsx`:
  - вкладки «История» и «Горячие предложения» через `TabLink` и `CountBadge`, вторая — только с правом;
  - «Оформить вручную» — `ButtonLink` `outlined`, только с `orders.accept`; на отключённой точке неактивна с `ReadOnlyNote` по `writeAccess`.

  Заглушка `SectionStubPage` уходит. `model/search.ts` — статус и `pageFields` в адресе. Проверка: `app/orders.test.tsx` — у официанта обе вкладки и «Оформить вручную», у управляющего действия нет, на отключённой точке действие неактивно с пояснением; `pnpm typecheck`.
- [x] 2.2 `ui/history-page.tsx`:
  - таблица от 1200 px, карточки уже;
  - время по `formatDateTime`, блюда с порциями, чип статуса, «Не хватило: …»;
  - раскрытие через `useToggleSet`: состав, таблица «нужно / было / не хватило» по `formatAmount`, «Продукт удалён из каталога» без `product`;
  - фильтр — `SegmentedControl`, пагинация — `ListPagination` и `useLastPageFallback`;
  - заготовка строк, `LoadErrorState`, пустые состояния «Заказов пока нет» и «Отклонённых заказов нет» с «Показать все заказы».

  Проверка: `app/orders.test.tsx`:
  - фильтр «Отклонённые» уходит в запрос и в адрес;
  - раскрытый отказ показывает «160 мл», «90 мл», «70 мл», два продукта названы оба;
  - пустой фильтр предлагает показать все;
  - переход на вторую страницу запрашивает `offset` 10.
- [x] 2.3 История по сигналу. Проверка: сценарий в `app/realtime.test.tsx` — открыта история, сигнал `orders` перечитывает список через секунду, серия из трёх даёт один запрос, сигнал `orders` не перечитывает горячие предложения.

## 3. Горячие предложения

- [x] 3.1 `api/hot-dishes.ts`: `hotDishKeys` (`['points', p, 'hot-dishes']`, их перечитывает сигнал `recommendations`), `hotDishesQueryOptions` (горизонт, страница), `defaultHotDishesQueryOptions` (первая страница по умолчанию: число на вкладке и отметки в форме). `model/explain.ts` — текст причины по полям ответа: уйдёт `min(suggested × required, at_risk)` или «всё», «меньше порции» при `significance < 1`, «только в этом блюде» или «ещё в N блюдах». Проверка: `model/explain.test.ts` — часть под риском, всё под риском, меньше порции, единственный и несколько потребителей; `pnpm exec vitest run src/pages/orders`.
- [x] 3.2 Роут `orders.hot.tsx` (`beforeLoad`: `recommendations.view`) и `ui/hot-page.tsx`:
  - `InfoNote` над списком;
  - горизонт `SegmentedControl` 24 / 72 / 168 в адресе, по умолчанию 72;
  - таблица или карточки: блюдо, причины с `ExpiryLabel` по `expires_at` и `ProductHistoryLink`, «Стоит продать», «Можно приготовить»;
  - `ListPagination`, «Продвигать нечего» с «Показать на 7 дней», заготовка строк и ошибка.

  Проверка: `app/orders-hot.test.tsx`:
  - строка карбонары с «500 мл под риском», «ещё 6 ч» и «На порцию 80 мл: 5 порций израсходуют 400 мл из 500 мл · используется ещё в 1 блюде»;
  - выбор «7 дней» отправляет `horizon_hours=168` и пишется в адрес;
  - пустой ответ показывает «Продвигать нечего»;
  - без `recommendations.view` вкладки нет, а её адрес отвечает «Нет доступа».

## 4. Оформление заказа

- [x] 4.1 `model/order-form.ts`: предел порций по меню, блюда из стоп-листа и вне меню не заказываются, превышение после обновления меню, итог (блюд, порций, состав), тело запроса только из блюд с порциями больше нуля. Проверка: `model/order-form.test.ts` — предел, стоп-лист, блюдо пропало из меню, превышение после уменьшения порций, пустой заказ не даёт тела; `pnpm exec vitest run src/pages/orders`.
- [x] 4.2 Роут `points.$pointId.orders_.new.tsx` (`ForbiddenError` без `orders.accept`, предзагрузка меню) и `ui/order-new-page.tsx`:
  - `PageHeader` с путём «Заказы → Новый заказ»;
  - поиск по названию, таблица или карточки меню с `QuantityField` (`max` из меню), отметка «предложить» по первой странице предложений;
  - `ActionBar` с итогом и «Оформить заказ»;
  - порции в состоянии страницы, `useIdempotencyKey`;
  - результаты: принят — сообщение и очистка; отклонён — `Alert` с продуктами и сохранёнными порциями; сеть или 409 — «Заказ не оформлен» с «Повторить»; 422 — имя выключенного блюда по меню до перечитывания и снятие его порций;
  - после записи — `dishKeys.all` и `hotDishKeys.all` сразу, `refreshOrders`;
  - отключённая точка — `ReadOnlyNote`, оформление неактивно.

  Проверка: `app/order-new.test.tsx`:
  - предел «больше не приготовить», блюдо стоп-листа не выбирается, отметка «предложить»;
  - принятый заказ: `POST` с составом и ключом, сообщение, порции очищены, меню перечитано;
  - отклонённый заказ: сливки с нужным и доступным количеством, порции на месте, после обновления меню «можно только 1» и кнопка неактивна;
  - обрыв и «Повторить» дают тот же ключ;
  - 422: «Сырники со сметаной больше не продаются на точке», остальные порции на месте;
  - без `orders.accept` по адресу «Нет доступа».

## 5. Документы и итог

- [x] 5.1 `frontend/AGENTS.md`: пункт про раздел заказов (`pages/orders`, `orderKeys`, `hotDishKeys`, которые перечитывает сигнал `recommendations`, `invalidateLater` в `shared/lib/query`, оформление по меню с `ActionBar`). Проверка: упомянутые пути существуют (`ls`), `pnpm format:check`.
- [x] 5.2 Скриншоты Playwright на моках: временные `preview.html` и `src/preview-mock.tsx`, порт не 5173. Снять историю с раскрытым отказом, горячие предложения, новый заказ и отказ на 1440, новый заказ на 820, историю в тёмной теме. Harness удалить. Проверка: файлы скриншотов есть, `git status` без файлов harness.
- [x] 5.3 Итоговый прогон. Проверка: `pnpm check` и `pnpm build` проходят.
