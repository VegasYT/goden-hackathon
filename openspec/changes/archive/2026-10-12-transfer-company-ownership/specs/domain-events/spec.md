## MODIFIED Requirements

### Requirement: Состав каталога и потоки событий
Система SHALL определять следующие доменные события и потоки, в которые они записываются:

- поток компании `company-<companyId>`: `CompanyCreated`, `CompanyRenamed`, `CompanyOwnerChanged`, `CompanyDisabled`;
- поток точки `point-<pointId>`: `PointCreated`, `PointRenamed`, `PointDisabled`, `PointTelegramChatChanged`, `StaffAssigned`, `StaffRoleChanged`, `StaffRemoved`;
- поток продукта `product-<productId>`: `ProductCreated`;
- поток блюда `dish-<dishId>`: `DishCreated`, `RecipeChanged`;
- поток меню точки `menu-<pointId>`: `DishAvailabilityChanged`;
- поток поставки `delivery-<deliveryId>`: `DeliveryRegistered`, `DeliveryApproved`, `DeliveryApprovalCorrected`, `QuarantinedDeliveryItemAccepted`, `QuarantinedDeliveryItemRejected`;
- складской поток продукта на точке `stock-<pointId>-<productId>`: `BatchReceived`, `StockConsumed`, `StockAdjusted`, `StockWrittenOff`, `BatchExpiryWarningIssued`;
- поток заказа `order-<orderId>`: `OrderAccepted`, `OrderRejected`.

Партия SHALL NOT иметь собственного потока: события партий пишутся в складской поток их продукта на точке. Каждый тип события SHALL иметь уникальное имя и версию схемы, начиная с 1.

#### Scenario: Имя потока строится по идентификатору
- **WHEN** для точки `0198c0de-0000-7000-8000-000000000001` и продукта `0198c0de-0000-7000-8000-000000000002` запрашивают имя складского потока
- **THEN** результат равен `stock-0198c0de-0000-7000-8000-000000000001-0198c0de-0000-7000-8000-000000000002`

#### Scenario: Все события каталога зарегистрированы
- **WHEN** запрашивают список зарегистрированных типов событий
- **THEN** в нём ровно 27 перечисленных типов
