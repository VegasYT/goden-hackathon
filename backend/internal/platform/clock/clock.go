// Package clock даёт коду текущее время через интерфейс, чтобы логику сроков годности можно было тестировать.
package clock

import (
	"sync"
	"time"
)

// Clock - источник текущего времени. Доменный код не вызывает time.Now напрямую.
type Clock interface {
	Now() time.Time
}

// System возвращает системное время в UTC.
type System struct{}

func (System) Now() time.Time { return time.Now().UTC() }

// Fake - часы для тестов: время меняется только через Set и Advance.
type Fake struct {
	mu  sync.Mutex
	now time.Time
}

func NewFake(now time.Time) *Fake { return &Fake{now: now.UTC()} }

func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *Fake) Set(now time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = now.UTC()
}

func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}
