# Сборка frontend и раздача статики. Сборка: docker build frontend
FROM node:22-alpine AS build
WORKDIR /src
RUN corepack enable
# зависимости отдельным слоем: он переиспользуется, пока не менялся pnpm-lock.yaml
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY . .
# Адрес Keycloak вшивается в сборку: Vite подставляет переменные VITE_* при сборке, а не при запуске.
# Без аргументов сборка смотрит на локальный Keycloak (значения по умолчанию в src/shared/config/env.ts).
ARG VITE_KEYCLOAK_URL
ARG VITE_KEYCLOAK_REALM
ARG VITE_KEYCLOAK_CLIENT_ID
# Раздел «Генератор» у владельца; без аргумента он включён.
ARG VITE_GENERATOR_ENABLED
ENV VITE_KEYCLOAK_URL=$VITE_KEYCLOAK_URL VITE_KEYCLOAK_REALM=$VITE_KEYCLOAK_REALM VITE_KEYCLOAK_CLIENT_ID=$VITE_KEYCLOAK_CLIENT_ID VITE_GENERATOR_ENABLED=$VITE_GENERATOR_ENABLED
RUN pnpm build

FROM caddy:2.10-alpine
COPY Caddyfile /etc/caddy/Caddyfile
COPY --from=build /src/dist /srv
