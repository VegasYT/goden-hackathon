# syntax=docker/dockerfile:1
# Keycloak с темой страниц входа goden. Контекст сборки - папка frontend/:
# docker build -f infra/keycloak/Dockerfile frontend
FROM node:22-bookworm-slim AS theme
# Keycloakify упаковывает тему в jar через Maven, поэтому в стадии сборки нужны Maven и JDK
RUN apt-get update \
  && apt-get install -y --no-install-recommends maven openjdk-17-jdk-headless \
  && rm -rf /var/lib/apt/lists/*
WORKDIR /src
RUN corepack enable
# зависимости отдельным слоем: он переиспользуется, пока не менялся pnpm-lock.yaml
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY . .
# зависимости Maven лежат в обычном ~/.m2, его кеш переживает пересборку образа
ENV KEYCLOAKIFY_USE_DEFAULT_MAVEN_REPO=true
RUN --mount=type=cache,target=/root/.m2 pnpm build:keycloak-theme

FROM quay.io/keycloak/keycloak:26.8.0
COPY --from=theme /src/dist_keycloak/goden-keycloak-theme.jar /opt/keycloak/providers/
