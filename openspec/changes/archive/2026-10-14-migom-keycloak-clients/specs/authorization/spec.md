## ADDED Requirements

### Requirement: Клиенты Мигома в realm
Realm `goden` SHALL содержать клиенты сервиса доставки Мигом: `migom-api` - получатель токенов Мигома и конфиденциальный `migom-backend` с входом по паролю пользователя и сервисной учётной записью, которой разрешено заводить пользователей realm и менять им пароль. Токен, выданный `migom-backend`, SHALL нести `aud` `migom-api` и SHALL NOT нести `goden-api`: API Goden такой токен отклоняет.

#### Scenario: Вход пользователя Мигома
- **WHEN** backend Мигома запрашивает токен по паролю пользователя от имени `migom-backend`
- **THEN** Keycloak выдаёт токен с `aud` `migom-api`

#### Scenario: Токен Мигома в API Goden
- **WHEN** токен пользователя, выданный `migom-backend`, приходит в API Goden
- **THEN** API Goden отвечает 401

#### Scenario: Регистрация из Мигома
- **WHEN** сервисная учётная запись `migom-backend` создаёт пользователя через Admin API
- **THEN** пользователь появляется в realm `goden` и может войти по паролю
