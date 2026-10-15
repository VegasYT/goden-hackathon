//go:build integration

// Package dockertest готовит окружение процесса тестов к запуску контейнеров testcontainers.
package dockertest

import (
	"os"
	"runtime"
)

// Setup выставляет переменные, без которых testcontainers не работает с Docker Desktop
// на Windows. Вызывается перед запуском первого контейнера; на других системах ничего не делает.
func Setup() {
	if runtime.GOOS != "windows" {
		return
	}
	// testcontainers передаёт служебному контейнеру ryuk путь к именованному каналу Docker,
	// и тот не стартует. Внутри Docker Desktop сокет лежит по обычному пути.
	setDefault("TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE", "/var/run/docker.sock")
	// Без DOCKER_HOST testcontainers ищет Docker проверкой существования именованного канала.
	// Когда к каналу одновременно подключаются тесты многих пакетов, проверка отвечает
	// "занято", и библиотека решает, что Docker нет.
	setDefault("DOCKER_HOST", "npipe:////./pipe/docker_engine")
}

func setDefault(name, value string) {
	if os.Getenv(name) == "" {
		_ = os.Setenv(name, value)
	}
}
