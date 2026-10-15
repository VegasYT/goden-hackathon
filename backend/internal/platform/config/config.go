// Package config загружает настройки процесса из переменных окружения.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

// Postgres - подключение к базе данных.
type Postgres struct {
	URL      string `env:"DATABASE_URL,required,notEmpty"`
	MaxConns int32  `env:"DATABASE_MAX_CONNS" envDefault:"10"`
}

// Log - настройки логгера.
type Log struct {
	Level slog.Level `env:"LOG_LEVEL" envDefault:"info"`
}

// Redis - необязательный кеш чтения. Пустой URL отключает кеш.
type Redis struct {
	URL      string        `env:"REDIS_URL"`
	CacheTTL time.Duration `env:"REDIS_CACHE_TTL" envDefault:"1m"`
	Timeout  time.Duration `env:"REDIS_TIMEOUT" envDefault:"100ms"`
}

// Load читает конфигурацию типа T, собранную процессом из частей этого пакета.
// В тексте ошибки называется переменная окружения, но не её значение: там может быть секрет.
func Load[T any]() (T, error) {
	cfg, err := env.ParseAs[T]()
	if err == nil {
		return cfg, nil
	}

	var agg env.AggregateError
	if !errors.As(err, &agg) {
		return cfg, errors.New("config: invalid environment")
	}
	problems := make([]string, 0, len(agg.Errors))
	for _, e := range agg.Errors {
		problems = append(problems, describe(e, reflect.TypeFor[T]()))
	}
	return cfg, fmt.Errorf("config: %s", strings.Join(problems, "; "))
}

func describe(err error, cfgType reflect.Type) string {
	var notSet env.VarIsNotSetError
	if errors.As(err, &notSet) {
		return notSet.Key + " is required"
	}
	var empty env.EmptyVarError
	if errors.As(err, &empty) {
		return empty.Key + " must not be empty"
	}
	var parse env.ParseError
	if errors.As(err, &parse) {
		if key := envKey(cfgType, parse.Name); key != "" {
			return fmt.Sprintf("%s has invalid value, expected %s", key, parse.Type)
		}
	}
	return "invalid environment variable"
}

// envKey ищет имя переменной окружения по имени поля, включая вложенные структуры.
func envKey(t reflect.Type, field string) string {
	if t.Kind() != reflect.Struct {
		return ""
	}
	for i := range t.NumField() {
		f := t.Field(i)
		if f.Name == field {
			if key, _, _ := strings.Cut(f.Tag.Get("env"), ","); key != "" {
				return key
			}
		}
		if key := envKey(f.Type, field); key != "" {
			return key
		}
	}
	return ""
}
