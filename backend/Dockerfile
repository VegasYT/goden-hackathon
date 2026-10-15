# Один образ с четырьмя программами backend: api, worker, generator, migrate.
# Сервис выбирает программу командой запуска. Сборка: docker build backend
FROM golang:1.27.2-alpine AS build
WORKDIR /src
# зависимости отдельным слоем: он переиспользуется, пока не менялись go.mod и go.sum
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ ./cmd/...

FROM alpine:3.22
# сертификаты нужны для обращения к Keycloak по HTTPS; wget из busybox - для проверок готовности
RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 goden
COPY --from=build /out/ /usr/local/bin/
USER goden
CMD ["api"]
