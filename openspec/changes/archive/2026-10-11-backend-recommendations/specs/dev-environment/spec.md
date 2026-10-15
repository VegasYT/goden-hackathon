# Spec Delta

## ADDED Requirements

### Requirement: Доступ ролей к меню и рекомендациям
Роли `waiter` и `manager` SHALL включать право `recommendations.view`; роли `manager`, `cook` и `waiter` SHALL включать право `kitchen.view`.

#### Scenario: Официант видит горячие предложения
- **WHEN** тестовый пользователь с ролью `waiter` получает токен
- **THEN** в его правах есть `recommendations.view` и `kitchen.view`

#### Scenario: Повар не видит рекомендаций
- **WHEN** тестовый пользователь с ролью `cook` получает токен
- **THEN** в его правах нет `recommendations.view`
