# Tasks

Часть репо: frontend. Команды запускаются из `frontend/`. Ветка `frontend/simplify`. Эталон видимых правок — страница холста «Simplify: видимые правки». В конце каждой группы `pnpm check` зелёный. Строки `frontend/AGENTS.md` и скиллов, которые описывают изменённое место, правятся в той же задаче.

## 1. Запросы, кэш и поток

- [x] 1.1 Общий `QueryClient`:
  - `src/app/query-client.ts` — `createQueryClient` со `staleTime: pointDataFreshMs` по умолчанию и повтором только при пустом `status` или `status` ≥ 500 (`shouldRetry` в `shared/api`);
  - его используют `main.tsx` и `renderApp`, в тестах `retry: false` без `staleTime: Infinity`;
  - удалить 22 копии `staleTime: pointDataFreshMs`;
  - исключения явно у своих запросов: `/me` с `staleTime: 0`, каталог продуктов с `catalogFreshMs` из `shared/lib/query/freshness.ts`, карточка карантина перед решением со `staleTime: 0`;
  - ключ `catalogProductsQueryOptions` — из `productKeys`.

  Проверка: unit-тест `shouldRetry` (403/404/422 без повтора, 500 и обрыв с повтором); новый тест — карточка поставки при открытии запрашивает `/deliveries/{id}` один раз; `pnpm test`.
- [x] 1.2 Ключи и чтение:
  - `pointKey(pointId)` в `shared/lib/query`, на нём фабрики ключей точки: delivery, dish, hot-dishes, stock, organization, orders, generator, recommendations, analytics, `point-signals.ts`;
  - `getData(url, params)` в `shared/api` как `queryFn` вместо ручного `.data` во всех `queryOptions`.

  Проверка: `pnpm typecheck`; тесты потока и разделов проходят.
- [x] 1.3 Ключ повтора:
  - восемь записей с ручным `'Idempotency-Key'` переходят на `idempotencyHeader`: `dish-queries`, `delivery-queries`, `orders`, `stock` ×2, `approve`, `correct`, `resolve-quarantine`;
  - `registration-form`, `welcome/name-step` и генератор (`test-delivery-card`) берут ключ из `useIdempotencyKey`; повтор генератора — `mutate(variables)`.

  Проверка: поиск `'Idempotency-Key'` в `src` находит только `shared/api`; тесты повтора регистрации, первого входа и генератора проходят.
- [x] 1.4 Перечитывание:
  - `invalidateKeys(queryClient, keys)` в `shared/lib/query` помечает без обрыва начатого запроса;
  - его используют `point-signals.ts`, `orders/model/refresh.ts`, `catalog-writes.ts` и `storeBatch` склада;
  - у `invalidateLater` предел ожидания 3000 мс.

  Проверка: unit-тест `invalidateLater` — серия вызовов каждые 500 мс 10 с даёт перечитывание не реже раза в 3 с и ещё одно после серии; три вызова за полсекунды — одно.
- [x] 1.5 Поток:
  - сигналы `stock`, `expiry`, `stop-list`, `orders` отложенно помечают `analyticsKeys.all`; экспорт `analyticsKeys` из `pages/analytics`;
  - `markOnline` в `use-point-events.ts` не меняет состояние, если связь уже `online` и минута та же;
  - убрать мёртвый статус `reconnecting` из `ConnectionState`, баннера и витрины `/dev/ui`;
  - `allCategories` в `test-realtime.ts` — из типа категорий, с `catalog`.

  Проверка: тест потока — открытая «Аналитика» перечитывается через секунду после `stock` и не перечитывается после одного `deliveries`; тесты `realtime` проходят.

## 2. Роутер и точка

- [x] 2.1 Контекст роута:
  - `beforeLoad` у `/points/$pointId` возвращает `{ point }`;
  - `requireSection(point, section)` синхронный, новый `requirePermission(point, permission, section)` в `app/point-access.ts`;
  - роуты каталога, склада и заказов не зовут `requirePoint` второй раз;
  - загрузчики компании и генератора берут `context.point`;
  - `deliveries.new`, `orders_.new`, `orders.hot`, `stock.reorder` проверяют право через `requirePermission`.

  Проверка: `routes.test.tsx` и тесты разделов проходят, `pnpm typecheck`.
- [x] 2.2 `usePoint(pointId)` в `entities/profile`:
  - 17 страниц берут точку через него;
  - удалить недостижимые запасные ветки: `return null`, `?? ''`, `point?.`;
  - layout'ы заказов, склада и каталога берут точку через `usePoint`, а не из контекста роута.

  Проверка: новый тест — после перечитанного профиля с отключённой точкой «Новый заказ» в layout заказов недоступен и есть пояснение, без перехода.
- [x] 2.3 Сборка роутера:
  - `appRouterOptions` общий для `createAppRouter` и `renderApp`;
  - один тип `RouterContext`;
  - убрать неиспользуемую опцию `session` у `renderApp`, вместе с таблицей скилла `frontend-testing`;
  - `PointSwitcher` выбирает из `useMatches` только путь последнего роута.

  Проверка: все тесты проходят.
- [x] 2.4 Загрузчики:
  - `prefetchAnalytics(queryClient, pointId, search)` в `pages/analytics`; из публичного API слайса убрать то, что было нужно только загрузчику;
  - `/orders/new` заранее запрашивает горячие предложения при праве `recommendations.view`;
  - поменять местами перепутанные комментарии в `points.$pointId.deliveries.index.tsx`.

  Проверка: тесты аналитики и нового заказа проходят; `pnpm check`.

## 3. Предметные помощники

- [x] 3.1 Слайс `entities/actor`:
  - `Actor`, `actorName`, `isMigom` и их тест;
  - удалить `shared/lib/actor`, а в `entities/delivery` — `actorLabel` и второй тип `Actor`;
  - поставки, горячие предложения, склад и заказы зовут `actorName`;
  - сообщения «Поставка № N уже принята: <автор>, <ЧЧ:ММ>. Ваши решения не сохранены» и «Карантин по «<продукт>» уже решён: <автор>, <ЧЧ:ММ>».

  Проверка: тесты поставок — строка списка и история с «Генератор», «уже принята: Илья Смирнов, 14:39», «уже решён: Генератор, …»; поиск `автоматически` в поставках находит только «проставлена автоматически».
- [x] 3.2 `entities/dish`:
  - `menuMatcher(query, { byIngredients })` для поиска на «Кухне» (по продуктам) и в новом заказе (только название);
  - `EmptyMenuState` для «Кухни», стоп-листа и нового заказа;
  - `portionsText` через `countOf`;
  - `portionsOf` принимает только количества, аналитика берёт его вместо `portionsLeft` (удалить функцию и её тест).

  Проверка: тест нового заказа — пустое меню показывает «В меню точки пока нет блюд»; тесты кухни и аналитики проходят.
- [x] 3.3 `entities/delivery`:
  - `allocationText(total, sum, unit, limit)`, `AllocationWarning`, `AutoExpiredReason` и `WriteOffCommentField` вынести из `pages/delivery` и `quarantine-card`;
  - экспортировать `Distribution` и взять его в `decision.ts`;
  - `reasonLabel` проверяет ключ через `Object.hasOwn`.

  Проверка: unit-тест — `reasonLabel('constructor')` возвращает строку как есть; тесты приёмки, исправления и карантина проходят.
- [x] 3.4 Организация и профиль:
  - `storePoint(queryClient, point)` в `entities/organization`;
  - `refreshProfile(queryClient)` в `entities/profile`;
  - их используют `company-writes`, `point-writes`, первый вход и диалог новой компании;
  - `roleChoices` возвращает `{ role, assignable }[]`.

  Проверка: тесты компании, настроек точки, сотрудников и создания компании проходят.
- [x] 3.5 Доступ:
  - `can()` принимает точку или `undefined`; убрать проверки `point !== undefined &&` перед ним;
  - горячие предложения используют `mayWrite`;
  - `AccessNote({ access, disabled, forbidden })` в `entities/profile` заменяет пять копий выбора текста в `ReadOnlyNote`; `InventoryReadOnlyNote` — его заготовка;
  - layout заказов объясняет отключённую точку раньше отсутствия права.

  Проверка: тесты «только чтение» склада, кухни, поставок, карантина и настроек проходят.
- [x] 3.6 Время и окна:
  - в `shared/lib/datetime`:
    - `isSameDay`;
    - `formatEventTime(value, now)` — сегодня время, иначе `formatDateTime`; заменяет `orderTime` и `setAtLabel`;
    - `formatShortDay`, `formatDayRange`;
    - `minuteMs`, `hourMs`, `dayMs` вместо шести копий констант;
  - история склада берёт общий формат даты;
  - `hourWindows = [24, 72, 168]`, их тип, поле адреса и подписи «24 часа / 3 дня / 7 дней»; их берут «Сроки», горячие предложения и прогноз; заголовки «За <окно>».

  Проверка: unit-тест `formatEventTime`; тесты горячих предложений («Илья Смирнов · 10:42», «24 часа»), аналитики («24 часа», «За 7 дней») и «Сроков» проходят; `pnpm check`.

## 4. Тема и диалоги

- [x] 4.1 Плашки:
  - у `MuiAlert` нейтральный вариант `info`: фон поля, кольцо, серый значок;
  - `defaultProps.iconMapping` из нашего набора иконок;
  - `KcAlert` без своих `sx` и `iconMapping`;
  - иконки `MuiSelect` и `MuiAutocomplete` из нашего набора.

  Проверка: тест контраста палитры проходит; поиск `iconMapping` находит только тему.
- [x] 4.2 Диалоги в теме:
  - `MuiDialogTitle` по умолчанию `h2`, у `MuiDialogActions` отступы `8px 24px 24px` и `gap`;
  - убрать `useId`/`aria-labelledby` и ручные отступы в `ConfirmDialog`, `FormDialog`, `NoticeDialog`, фильтрах склада и остальных диалогах;
  - `FormDialog` получает `maxWidth` и необязательный `pending`.

  Проверка: тесты, которые ищут диалог по имени, проходят.
- [x] 4.3 Формы в диалогах:
  - «Списать порчу» и «Исправить остаток» на `FormDialog` и TanStack Form: хуки в `pages/stock/model`, ошибки считаются в рендере по живому остатку;
  - подсказка про поставку в «Исправить остаток» — `InfoNote`;
  - «Свой период» аналитики на `FormDialog`.

  Проверка: новый тест — Enter в поле количества отправляет списание; тесты действий склада и периода аналитики проходят.
- [x] 4.4 Чистка темы:
  - помощник `radius(theme, size)` вместо 28 строк `${…}px`;
  - цвет заголовков h1–h3 через `themeVars` в `MuiTypography`;
  - удалить `sx`, которые повторяют тему: касание 44 px, `actionSx`, `size='small'` у чипов, `text.strong` у заголовков, `shrink`, `fontWeight 600`;
  - убрать `variant='outlined'` у чипов;
  - «отключена» в переключателе — `Chip`;
  - рамки переключателя и карточек первого входа через `themeVars` (тёмная тема);
  - карточки первого входа на `Paper` с радиусом `lg`.

  Проверка: поиск `theme.palette.divider` вне темы пуст; `pnpm typecheck`; тесты оболочки и первого входа проходят.
- [x] 4.5 Скрипт темы:
  - `shared/ui/theme/color-mode-script.ts` с ключом и строкой скрипта;
  - плагин в `vite.config.ts` вставляет скрипт в `head`;
  - блок удалить из `index.html`, файл добавить в `tsconfig.node.json`;
  - тест скрипта выполняет строку напрямую.

  Проверка: тест скрипта темы проходит; после `pnpm build` в `dist/index.html` есть `script#theme-init`.
- [x] 4.6 Предупреждение в `ConfirmDialog` читается из хранилища один раз при открытии и остаётся, пока идёт подтверждение и после ошибки.

  Проверка: новый тест — отметили «Больше не показывать», подтверждение ждёт ответа или упало, а предупреждение на месте; `pnpm check`.

## 5. Общие компоненты

- [x] 5.1 Загрузка и ошибка:
  - `QueryState` в `shared/ui/empty-state`, а `LoadErrorState` принимает `query`; на них 19 мест, `Section` аналитики и `Panel` сводки (`BlockQuery.retry` → `refetch`);
  - `SkeletonList` в истории поставки и панели карантина;
  - `LoadErrorState` на «Новой поставке».

  Проверка: тесты состояний загрузки и ошибки разделов проходят; поиск `onRetry={() => void` в страницах пуст.
- [x] 5.2 `CardList` и `CardListItem` в `shared/ui/card-list` на `Card` темы:
  - заменяют рамочные списки «Сроков», «Пора заказать», истории и предложений заказов, нового заказа, сотрудников и каталога;
  - их же используют «Остатки», «Кухня» и поставки;
  - `catalog-list.tsx` оставляет только своё.

  Проверка: планшетные тесты проходят.
- [x] 5.3 Мелкие общие блоки:
  - `NoResultsState` — 6 мест;
  - `RetryAlert` — 3;
  - `useLeaveConfirm` — 3 формы;
  - `Breadcrumbs` — 3, `DeliveryBreadcrumbs` собирается из них;
  - `SectionTabs` — 3 layout'а;
  - `TimelineItem` — истории склада и поставки;
  - слот `actionBar` у `PageContent` — 5 страниц.

  Проверка: поиск `useBlocker` находит только `shared`; тесты ухода с формы, повтора и вкладок проходят.
- [x] 5.4 Списки:
  - `usePagedSearch` (последняя страница, сброс на первую) в восьми списках;
  - `ListPagination` рисует верхнюю линию только в карточном виде;
  - `SortableHeaderCell` со `sortDirection` у склада, поставок и каталога.

  Проверка: новый тест — у заголовка сортировки каталога есть `aria-sort`; тесты пагинации проходят.
- [x] 5.5 `useNotice` в `shared/ui/notice`:
  - 6 с без действия, 8 с с действием;
  - щелчок мимо не закрывает уведомление с действием;
  - заменяет 10 своих `Snackbar`.

  Проверка: новый тест — «Открыть» после новой точки не пропадает от щелчка мимо; тесты уведомлений разделов проходят.
- [x] 5.6 Значки и пометки:
  - `DisclosureIcon` («›», поворачивается в «⌄») на «Кухне», в истории заказов, на «Остатках», «Пора заказать» и в истории поставки;
  - `EmptyValue` вместо прочерков трёх оттенков;
  - меню партии на `ActionMenu` со значком;
  - «Новая поставка» на отключённой точке — `ReadOnlyNote`;
  - сноска снимка приёмки — `InfoNote`;
  - `KcHtml` вместо семи `dangerouslySetInnerHTML` в теме входа.

  Проверка: поиск `dangerouslySetInnerHTML` находит только `KcHtml`; `pnpm check`.

## 6. Разделы

- [x] 6.1 Склад:
  - панель истории запрашивает движения сразу по `productId`, единицу берёт из продукта, без заготовки из каталога;
  - при закрытии панель держит последний продукт;
  - общий `AvailableAmount`;
  - типы страниц API не экспортируются;
  - «Сроки» получают `access`, хук истории везде `openHistory`;
  - убрать высоту кнопок по ширине окна.

  Проверка: новый тест — запрос движений уходит, не дожидаясь продукта; тесты склада проходят.
- [x] 6.2 Заказы и аналитика:
  - `usePlaceOrder` в `api/order-writes.ts`, итог заказа — отдельный компонент;
  - подробности принятого заказа — `span`;
  - удалить `portionLimit` и его тест;
  - горячие карточки берут `DishName` и `suggestedPortionsText`;
  - блок «когда» в прогнозе собирается один раз.

  Проверка: тесты нового заказа, истории, предложений и аналитики проходят без предупреждения React о вложенности `div` в `p`.
- [x] 6.3 Поставки:
  - `ItemsPanel` для приёмки, исправления и снимка;
  - строки исправления на `ItemTitle` и `ItemCardHeader` (с категорией);
  - `shallowEqual` в `shared/lib` вместо четырёх способов сравнения;
  - `toNewDelivery` возвращает `null`, без `!`;
  - запись позиции без `as never`;
  - снимок берёт точку из поставки;
  - убрать лишние экспорты.

  Проверка: тест исправления — под названием позиции категория; тесты поставок проходят.
- [x] 6.4 Обязательные поля приёмки и карантина краснеют по `isBlurred` или после попытки подтвердить; текст «обязателен» виден сразу. `WriteOffReasonSelect` принимает `onBlur`.

  Проверка: новый тест — после ввода отклонённого количества причина не помечена ошибкой, после ухода с пустого поля помечена; сценарии «Отклонение без комментария» проходят.
- [x] 6.5 Каталог и настройки:
  - `CatalogListBody` и `RowActions(item, readOnly, restoringId)` для продуктов и блюд;
  - `RenameForm` сам сбрасывается по значению и умеет `readOnlyNote`;
  - `ConnectionStatus` и `NumberConnectForm` для Telegram и Мигома;
  - удалить лишние `onSuccess` и проп `children` у шага первого входа;
  - сетка страницы компании через адаптивный `sx`.

  Проверка: тесты каталога, компании и настроек точки проходят.
- [x] 6.6 Стоп-лист на планшете: пояснение о просрочке под числом в ячейке, без отдельной строки и повтора количества.

  Проверка: тест — на планшете «в просроченных партиях, готовить из них нельзя» показан один раз у блюда.
- [x] 6.7 Мёртвый код:
  - `id`/`name` у `QuantityField`;
  - неиспользуемые экспорты: `toApiError`, `deliveriesPath`, `shelfLifeLabel`, `CompanyPoints`, `ExpiryStatus`, типы `*Props`;
  - `ApiError.correlationId`;
  - комментарии, которые пересказывают код.

  Проверка: поиск по удалённым именам пуст; `pnpm check`.

## 7. Тесты

- [x] 7.1 Фикстуры:
  - `entities/product/index.testing.ts` (`testProduct`) и `entities/delivery/index.testing.ts` (`testDelivery`, `testDeliveryItem`);
  - `app/test-*.ts` их реэкспортируют;
  - пять своих фабрик продукта и шесть каркасов позиции заменить.

  Проверка: `pnpm lint:fsd` чистый; тесты проходят.
- [x] 7.2 Профили ролей:
  - карта прав ролей и `testStaffPoint(role, overrides)` в `app/test-router.tsx` вместо ~12 ручных профилей;
  - `cook`, `approver`, `testOrgPoint` — общие.

  Проверка: тесты ролей проходят.
- [x] 7.3 Хелперы:
  - `requestsTo(mock, string | RegExp)`, `paramsOf`, `lastParams`, `lastListParams` в `test-api.ts`, местные копии удалить;
  - копии `idempotencyKeyOf` удалить;
  - `retype(input, value)` в `@shared/lib/test-utils` вместо пяти копий;
  - `pinnedHot` и `openForm` с параметром;
  - `fillWriteOff` и `openTomato` в `test-stock.ts`.

  Проверка: поиск `mock.calls.filter` в `src/app` находит только объяснимые места.
- [x] 7.4 Настройка и устойчивость:
  - `mockReset: true` в настройке Vitest и `vi.useRealTimers()` в общем `afterEach`, ручные сбросы удалить;
  - `renderApp` по умолчанию с широким экраном, планшетные тесты передают `wide: false`;
  - первый вход через `mockGet` вместо очереди ответов;
  - каталог — общий `beforeEach`;
  - сценарии стоп-листа — в `stop-list.test.tsx`;
  - вместо `closest('tr')!`, строки по индексу и склеенного текста — роли и `within`;
  - ключ склада в тесте сигналов — через `stockKeys`.

  Проверка: `pnpm test` два раза подряд зелёный; таблица хелперов в скилле `frontend-testing` совпадает с кодом.

## 8. Итог

- [x] 8.1 В `frontend/AGENTS.md` и скилле `frontend-design-system` нет упоминаний удалённого: `actorLabel`, `shared/lib/actor`, `CatalogCards`, ручного `staleTime`. Новые общие компоненты перечислены.

  Проверка: каждый упомянутый путь существует (поиск по файлам).
- [x] 8.2 Итоговая проверка:
  - `pnpm check` и `pnpm build`;
  - `openspec validate frontend-simplify --strict`;
  - снимки экранов страницы холста «Simplify: видимые правки» на временном стенде (ноутбук, планшет, тёмная тема), расхождения с макетами исправлены; стенд удалён.

  Проверка: все три команды зелёные, снимки отправлены.
