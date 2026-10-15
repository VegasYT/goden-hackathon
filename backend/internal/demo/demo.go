// Package demo создаёт демонстрационную компанию с тестовыми пользователями realm: после неё
// каждый тестовый пользователь входит и сразу работает в своей роли (ADR-013).
package demo

import (
	"context"
	"errors"
	"fmt"

	"goden/backend/internal/generator"
	"goden/backend/internal/organization"
	"goden/backend/internal/platform/command"
	"goden/backend/internal/platform/eventstore"
)

// Actor - автор событий демонстрационной компании.
var Actor = eventstore.Actor{Type: eventstore.ActorService, ID: "goden-demo"}

// Тестовые пользователи realm. Идентификаторы заданы в infra/keycloak/goden-realm.json и не
// меняются при пересоздании realm: по ним компанию можно создать до первого входа пользователей.
var (
	// Owner - владелец демонстрационной компании.
	Owner = organization.User{ID: "0a000000-0000-4000-8000-000000000001", Email: "admin@goden.local", Name: "admin", Login: "admin"}
	// Staff - сотрудники первой точки и их роли.
	Staff = []Member{
		{organization.User{ID: "0a000000-0000-4000-8000-000000000002", Email: "manager@goden.local", Name: "manager", Login: "manager"}, organization.RoleManager},
		{organization.User{ID: "0a000000-0000-4000-8000-000000000003", Email: "approver@goden.local", Name: "approver", Login: "approver"}, organization.RoleApprover},
		{organization.User{ID: "0a000000-0000-4000-8000-000000000004", Email: "cook@goden.local", Name: "cook", Login: "cook"}, organization.RoleCook},
		{organization.User{ID: "0a000000-0000-4000-8000-000000000005", Email: "waiter@goden.local", Name: "waiter", Login: "waiter"}, organization.RoleWaiter},
	}
)

// Member - тестовый пользователь и его роль на первой точке.
type Member struct {
	User organization.User
	Role organization.Role
}

const (
	CompanyName = "Демо"
	FirstPoint  = "Точка на Ленина"
	SecondPoint = "Точка на Мира"
)

// Seed создаёт компанию "Демо" с двумя точками: владелец - Owner, сотрудники Staff назначены
// на первую точку, на ней включён генератор. chatID, если не ноль, становится чатом Telegram
// первой точки, пока у неё нет своего.
//
// Всё создаётся командами с постоянными ключами идемпотентности, как у обычного владельца:
// повторный вызов ничего не добавляет и не отменяет того, что владелец изменил после создания.
// Каталог не создаётся: его наполняет генератор. Чат задаётся последним: отказ в нём
// (точку отключили, чат уже задавали другим) не мешает остальному.
func Seed(ctx context.Context, org *organization.Service, gen *generator.Service, chatID int64) error {
	// назначить можно только пользователя, который уже известен системе
	for _, user := range append([]organization.User{Owner}, users()...) {
		// уже известного пользователя не трогаем: его имя и почту ведёт Keycloak
		if err := org.Register(ctx, user); err != nil {
			return err
		}
	}
	company, err := org.CreateCompany(ctx, Actor, "demo-company", Owner.ID, CompanyName)
	if err != nil {
		return fmt.Errorf("demo: company: %w", err)
	}
	first, err := org.CreatePoint(ctx, Actor, "demo-point-1", company.ID, FirstPoint)
	if err != nil {
		return fmt.Errorf("demo: first point: %w", err)
	}
	if _, err := org.CreatePoint(ctx, Actor, "demo-point-2", company.ID, SecondPoint); err != nil {
		return fmt.Errorf("demo: second point: %w", err)
	}
	for _, member := range Staff {
		_, err := org.Assign(ctx, Actor, "demo-staff-"+member.User.Name, Owner.ID, first.ID, member.User.Email, member.Role)
		if err != nil {
			return fmt.Errorf("demo: assign %s: %w", member.User.Name, err)
		}
	}
	// генератор включается только там, где его настройки ещё не задавали: остановку владельцем
	// повторный запуск не отменяет
	settings, err := gen.Settings(ctx, first.ID)
	if err != nil {
		return err
	}
	if settings.UpdatedAt.IsZero() {
		settings.Running = true
		if _, err := gen.Update(ctx, first, settings); err != nil {
			return fmt.Errorf("demo: generator: %w", err)
		}
	}

	// Чат из настройки окружения задаётся один раз и только точке без чата: чат, который владелец
	// задал или снял в приложении, запуск не перезаписывает. Сменить его потом можно в приложении.
	// Точка перечитывается: first - сохранённый результат её создания, чата в нём нет никогда.
	current, err := org.Point(ctx, first.ID)
	if err != nil {
		return err
	}
	if chatID != 0 && current.TelegramChatID == nil {
		_, err := org.SetTelegramChat(ctx, Actor, "demo-chat", first.ID, &chatID)
		// ключ уже использован с другим чатом: чат задавали раньше, владелец его снял
		if err != nil && !errors.Is(err, command.ErrKeyReused) {
			return fmt.Errorf("demo: telegram chat: %w", err)
		}
	}
	return nil
}

func users() []organization.User {
	out := make([]organization.User, len(Staff))
	for i, member := range Staff {
		out[i] = member.User
	}
	return out
}
