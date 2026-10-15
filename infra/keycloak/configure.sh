#!/bin/bash
# Настройки realm, которые должны меняться на работающем сервере без пересоздания тома Keycloak:
# сроки токенов, язык форм, тема страниц входа, подтверждение почты, почтовый сервер и клиенты Мигома. Файл realm
# применяется только при первом создании тома, поэтому эти значения выставляются здесь через API
# администратора при каждой выкладке. Запускается разовой задачей keycloak-config из compose.prod.yaml.
set -eu

kcadm=/opt/keycloak/bin/kcadm.sh
config=/tmp/kcadm.config
realm=goden

# Keycloak уже здоров (depends_on), но вход администратора может запоздать на первом запуске
logged_in=false
for _ in $(seq 1 12); do
  if "$kcadm" config credentials --config "$config" --server http://keycloak:8080 --realm master \
    --user "$KEYCLOAK_ADMIN_USER" --password "$KEYCLOAK_ADMIN_PASSWORD" > /dev/null 2>&1; then
    logged_in=true
    break
  fi
  sleep 5
done
if [ "$logged_in" != true ]; then
  echo "keycloak-config: вход администратора не удался. Если пароль меняли в консоли Keycloak," \
    "задай тот же в KEYCLOAK_ADMIN_PASSWORD. Настройки realm не изменены." >&2
  exit 1
fi

verify=${KEYCLOAK_VERIFY_EMAIL:-false}
failed=false
case "$verify" in
  true | false) ;;
  *)
    echo "keycloak-config: KEYCLOAK_VERIFY_EMAIL должно быть true или false, получено '$verify'" >&2
    verify=false
    failed=true
    ;;
esac

settings=(
  -s "accessTokenLifespan=$KEYCLOAK_ACCESS_TOKEN_LIFESPAN"
  -s "ssoSessionIdleTimeout=$KEYCLOAK_SESSION_IDLE_TIMEOUT"
  -s "ssoSessionMaxLifespan=$KEYCLOAK_SESSION_MAX_LIFESPAN"
  -s internationalizationEnabled=true
  -s 'supportedLocales=["ru"]'
  -s defaultLocale=ru
  # тема из образа goden-keycloak (infra/keycloak/Dockerfile)
  -s loginTheme=goden
)

if [ -n "${SMTP_HOST:-}" ]; then
  port=${SMTP_PORT:-465}
  # 465 - TLS с первого байта, остальные порты - STARTTLS
  ssl=false
  starttls=true
  if [ "$port" = 465 ]; then
    ssl=true
    starttls=false
  fi
  settings+=(
    -s "smtpServer.host=$SMTP_HOST"
    -s "smtpServer.port=$port"
    -s "smtpServer.ssl=$ssl"
    -s "smtpServer.starttls=$starttls"
    -s "smtpServer.auth=true"
    -s "smtpServer.user=$SMTP_USER"
    -s "smtpServer.password=$SMTP_PASSWORD"
    -s "smtpServer.from=${SMTP_FROM:-$SMTP_USER}"
    -s "smtpServer.fromDisplayName=Goden"
  )
elif [ "$verify" = true ]; then
  # без почтового сервера письмо не уйдёт, и новые пользователи не смогли бы войти
  echo "keycloak-config: KEYCLOAK_VERIFY_EMAIL=true, но SMTP_HOST не задан: подтверждение почты не включено" >&2
  verify=false
  failed=true
fi
settings+=(-s "verifyEmail=$verify")

"$kcadm" update "realms/$realm" --config "$config" "${settings[@]}"
echo "keycloak-config: realm $realm настроен: тема страниц входа goden, подтверждение почты $verify," \
  "access token ${KEYCLOAK_ACCESS_TOKEN_LIFESPAN} с, сессия без обращений ${KEYCLOAK_SESSION_IDLE_TIMEOUT} с"

# Клиенты Мигома: файл realm заводит их только на новом томе, здесь они появляются на работающем
# сервере. Определения совпадают с goden-realm.json: меняя одно, поменяй другое.
kc() { "$kcadm" "$@" --config "$config" -r "$realm"; }
client_uuid() { kc get clients -q "clientId=$1" --fields id --format csv --noquotes; }
ensure_client() {
  if [ -z "$(client_uuid "$1")" ]; then
    kc create clients -f - > /dev/null <<< "$2"
    echo "keycloak-config: клиент $1 заведён"
  fi
}
# секрет из окружения; без него - случайный, чтобы на сервере не действовал секрет разработки из файла realm
set_secret() {
  local id
  id=$(client_uuid "$1")
  if [ -n "$2" ]; then
    kc update "clients/$id" -s "secret=$2"
  else
    kc create "clients/$id/client-secret" > /dev/null
    echo "keycloak-config: секрет клиента $1 не задан, выставлен случайный"
  fi
}

ensure_client migom-api '{"clientId": "migom-api", "name": "Мигом API", "enabled": true, "bearerOnly": true,
  "description": "Получатель токенов (aud) сервиса доставки Мигом"}'
ensure_client migom-backend '{"clientId": "migom-backend", "name": "Мигом backend", "enabled": true,
  "description": "Backend Мигома: вход пользователей по паролю, обновление токена и учётные записи через Admin API",
  "publicClient": false, "serviceAccountsEnabled": true, "standardFlowEnabled": false, "directAccessGrantsEnabled": true,
  "protocolMappers": [
    {"name": "migom-api-audience", "protocol": "openid-connect", "protocolMapper": "oidc-audience-mapper",
      "config": {"included.client.audience": "migom-api", "access.token.claim": "true", "id.token.claim": "false"}},
    {"name": "goden-api-audience", "protocol": "openid-connect", "protocolMapper": "oidc-audience-mapper",
      "config": {"included.client.audience": "goden-api", "access.token.claim": "true", "id.token.claim": "false"}}]}'
ensure_client migom-integration '{"clientId": "migom-integration", "name": "Мигом в API Goden", "enabled": true,
  "description": "Backend Мигома в API Goden: меню и заказы на точках, где владелец подключил Мигом",
  "publicClient": false, "serviceAccountsEnabled": true, "standardFlowEnabled": false, "directAccessGrantsEnabled": false,
  "protocolMappers": [
    {"name": "goden-api-audience", "protocol": "openid-connect", "protocolMapper": "oidc-audience-mapper",
      "config": {"included.client.audience": "goden-api", "access.token.claim": "true", "id.token.claim": "false"}},
    {"name": "client-id", "protocol": "openid-connect", "protocolMapper": "oidc-usersessionmodel-note-mapper",
      "config": {"user.session.note": "client_id", "claim.name": "client_id", "jsonType.label": "String",
        "access.token.claim": "true", "id.token.claim": "false"}}]}'

# кабинеты Мигома: публичный клиент с PKCE; адреса возврата - по домену Мигома и локальной разработке
ensure_client migom-web '{"clientId": "migom-web", "name": "Мигом: кабинеты", "enabled": true,
  "description": "Кабинеты владельца и модератора Мигома: вход с PKCE, токен действует в API Мигома и Goden",
  "publicClient": true, "standardFlowEnabled": true, "directAccessGrantsEnabled": false,
  "attributes": {"pkce.code.challenge.method": "S256"},
  "protocolMappers": [
    {"name": "migom-api-audience", "protocol": "openid-connect", "protocolMapper": "oidc-audience-mapper",
      "config": {"included.client.audience": "migom-api", "access.token.claim": "true", "id.token.claim": "false"}},
    {"name": "goden-api-audience", "protocol": "openid-connect", "protocolMapper": "oidc-audience-mapper",
      "config": {"included.client.audience": "goden-api", "access.token.claim": "true", "id.token.claim": "false"}}]}'
migom_domain=${MIGOM_DOMAIN:-migom.site}
web_urls="https://owner.$migom_domain https://admin.$migom_domain http://localhost:5174 http://localhost:5175"
redirects=$(printf '"%s/*",' $web_urls)
origins=$(printf '"%s",' $web_urls)
logout=$(printf '%s/*##' $web_urls)
kc update "clients/$(client_uuid migom-web)" -s "redirectUris=[${redirects%,}]" -s "webOrigins=[${origins%,}]"   -s "attributes.\"post.logout.redirect.uris\"=${logout%\#\#}"

# маппер, появившийся после того, как клиент уже завели: Мигом ходит в API Goden от имени пользователя
backend=$(client_uuid migom-backend)
if ! kc get "clients/$backend/protocol-mappers/models" --fields name --format csv --noquotes | grep -qx goden-api-audience; then
  kc create "clients/$backend/protocol-mappers/models" -f - > /dev/null <<< '{"name": "goden-api-audience",
    "protocol": "openid-connect", "protocolMapper": "oidc-audience-mapper",
    "config": {"included.client.audience": "goden-api", "access.token.claim": "true", "id.token.claim": "false"}}'
  echo "keycloak-config: клиенту migom-backend добавлен получатель goden-api"
fi

api=$(client_uuid goden-api)
if ! kc get "clients/$api/roles/migom.integration" > /dev/null 2>&1; then
  kc create "clients/$api/roles" -s name=migom.integration     -s "description=Учётная запись сервиса доставки Мигом: права на точках, где владелец подключил Мигом"
fi
# повторное назначение роли ничего не меняет
kc add-roles --uusername service-account-migom-integration --cclientid goden-api --rolename migom.integration
kc add-roles --uusername service-account-migom-backend --cclientid realm-management   --rolename manage-users --rolename view-users --rolename query-users
set_secret migom-backend "${MIGOM_BACKEND_SECRET:-}"
set_secret migom-integration "${MIGOM_INTEGRATION_SECRET:-}"
echo "keycloak-config: клиенты Мигома настроены"

if [ "$failed" = true ]; then
  exit 1
fi
