# Spec Delta

## MODIFIED Requirements

### Requirement: Realm Keycloak
Keycloak SHALL при старте импортировать realm `goden`. Realm SHALL содержать клиент `goden-api` с client roles для каждого permission из ADR-006, публичный клиент `goden-web` для frontend и клиент `goden-generator` с сервисной учётной записью. Права SHALL попадать в access token в `resource_access.goden-api.roles`, а `goden-api` SHALL присутствовать в `aud`.

#### Scenario: Realm доступен после запуска
- **WHEN** после запуска окружения запрашивают документ обнаружения OpenID Connect realm `goden`
- **THEN** он возвращается со статусом 200

#### Scenario: Все permissions существуют
- **WHEN** перечисляют client roles клиента `goden-api`
- **THEN** их 14 и они совпадают со списком permissions из ADR-006

#### Scenario: Сервисная учётная запись генератора
- **WHEN** клиент `goden-generator` получает токен по client credentials
- **THEN** в `resource_access.goden-api.roles` находятся ровно `catalog.view`, `deliveries.register`, `deliveries.approve`, `orders.accept`, `products.manage` и `dishes.manage`
