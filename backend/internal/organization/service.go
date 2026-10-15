package organization

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"goden/backend/internal/organization/db"
	"goden/backend/internal/platform/command"
	"goden/backend/internal/platform/eventstore"
)

// Service выполняет команды организации, читает её данные и отвечает на вопрос о правах.
type Service struct {
	runner  *command.Runner
	pool    *pgxpool.Pool
	service ServiceNames
}

func NewService(runner *command.Runner, pool *pgxpool.Pool) *Service {
	return &Service{runner: runner, pool: pool}
}

// ServiceNames возвращает имена сервисных авторов по их идентификаторам, например названия поставщиков.
type ServiceNames interface {
	Names(ctx context.Context, actorIDs []string) (map[string]string, error)
}

// SetServiceNames задаёт источник имён сервисных авторов. Задаётся при сборке: модуль поставщиков
// сам зависит от organization.
func (s *Service) SetServiceNames(names ServiceNames) { s.service = names }

// Remember записывает пользователя, обратившегося к системе, или обновляет его почту и имя.
// Почта принадлежит одному пользователю: у прежнего владельца адреса она очищается.
func (s *Service) Remember(ctx context.Context, user User) error {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		emailKey := key(user.Email)
		if emailKey != "" {
			if err := q.ReleaseEmail(ctx, db.ReleaseEmailParams{EmailKey: emailKey, ID: user.ID}); err != nil {
				return err
			}
		}
		return q.UpsertUser(ctx, db.UpsertUserParams{ID: user.ID, Email: user.Email, EmailKey: emailKey, Name: user.Name, Login: user.Login})
	})
	if err != nil {
		return fmt.Errorf("organization: remember user: %w", err)
	}
	return nil
}

// Register записывает пользователя, если система его ещё не знает, и не трогает уже известного:
// его почту и имя ведёт Keycloak. Нужен там, где пользователя называют до его первого входа.
func (s *Service) Register(ctx context.Context, user User) error {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		known, err := q.UserExists(ctx, user.ID)
		if err != nil || known {
			return err
		}
		emailKey := key(user.Email)
		if emailKey != "" {
			if err := q.ReleaseEmail(ctx, db.ReleaseEmailParams{EmailKey: emailKey, ID: user.ID}); err != nil {
				return err
			}
		}
		_, err = q.InsertUserIfNew(ctx, db.InsertUserIfNewParams{ID: user.ID, Email: user.Email, EmailKey: emailKey, Name: user.Name, Login: user.Login})
		return err
	})
	if err != nil {
		return fmt.Errorf("organization: register user: %w", err)
	}
	return nil
}

// Names возвращает имена пользователей по логинам, которыми они записаны авторами событий.
// Пользователя, чей логин системе не известен, в ответе нет.
func (s *Service) Names(ctx context.Context, logins []string) (map[string]string, error) {
	if len(logins) == 0 {
		return nil, nil
	}
	rows, err := db.New(s.pool).ListNamesByLogin(ctx, logins)
	if err != nil {
		return nil, fmt.Errorf("organization: names by login: %w", err)
	}
	names := make(map[string]string, len(rows))
	for _, row := range rows {
		names[row.Login] = row.Name
	}
	return names, nil
}

// CreateCompany создаёт компанию, её владельцем становится ownerID.
func (s *Service) CreateCompany(ctx context.Context, actor eventstore.Actor, idempotencyKey, ownerID, name string) (Company, error) {
	name, err := normalizedName(name)
	if err != nil {
		return Company{}, err
	}
	company := Company{ID: uuid.Must(uuid.NewV7()), Name: name, OwnerID: ownerID, Active: true}

	spec := command.Spec{
		Name: "CreateCompany", Actor: actor, IdempotencyKey: idempotencyKey,
		Request: struct{ OwnerID, Name string }{ownerID, name},
	}
	return command.Run(ctx, s.runner, spec, func(ctx context.Context, tx *command.Tx) (Company, error) {
		event := CompanyCreated{CompanyID: company.ID, Name: company.Name, OwnerID: company.OwnerID}
		if err := record(ctx, tx, CompanyStream(company.ID), event); err != nil {
			return Company{}, err
		}
		return company, nil
	})
}

// RenameCompany меняет название компании.
func (s *Service) RenameCompany(ctx context.Context, actor eventstore.Actor, idempotencyKey string, companyID uuid.UUID, name string) (Company, error) {
	name, err := normalizedName(name)
	if err != nil {
		return Company{}, err
	}
	spec := command.Spec{
		Name: "RenameCompany", Actor: actor, IdempotencyKey: idempotencyKey,
		Request: struct {
			CompanyID uuid.UUID
			Name      string
		}{companyID, name},
	}
	return s.changeCompany(ctx, spec, companyID, CompanyRenamed{CompanyID: companyID, Name: name})
}

// TransferCompany передаёт работающую компанию зарегистрированному пользователю с адресом
// почты email. by - кто передаёт: он обязан быть владельцем в момент команды. Прежний
// владелец теряет все права; назначения нового владельца на точках компании остаются.
func (s *Service) TransferCompany(ctx context.Context, actor eventstore.Actor, idempotencyKey, by string, companyID uuid.UUID, email string) (Company, error) {
	emailKey := key(email)
	if emailKey == "" {
		return Company{}, &ValidationError{Field: "email", Message: "must not be empty"}
	}
	spec := command.Spec{
		Name: "TransferCompany", Actor: actor, IdempotencyKey: idempotencyKey,
		Locks: []string{CompanyStream(companyID)},
		Request: struct {
			CompanyID uuid.UUID
			Email     string
		}{companyID, emailKey},
	}
	return command.Run(ctx, s.runner, spec, func(ctx context.Context, tx *command.Tx) (Company, error) {
		q := db.New(tx)
		company, err := readCompany(ctx, q, companyID)
		if err != nil {
			return Company{}, err
		}
		// право проверено до команды; под блокировкой владелец мог уже смениться
		if company.OwnerID != by {
			return Company{}, ErrNotOwner
		}
		if !company.Active {
			return Company{}, fmt.Errorf("company %s: %w", companyID, ErrDisabled)
		}
		user, err := q.GetUserByEmail(ctx, emailKey)
		if errors.Is(err, pgx.ErrNoRows) {
			return Company{}, fmt.Errorf("%s: %w", emailKey, ErrUnknownUser)
		}
		if err != nil {
			return Company{}, fmt.Errorf("organization: find user: %w", err)
		}
		if user.ID == company.OwnerID {
			return company, nil
		}
		if err := record(ctx, tx, CompanyStream(companyID), CompanyOwnerChanged{CompanyID: companyID, OwnerID: user.ID}); err != nil {
			return Company{}, err
		}
		return readCompany(ctx, q, companyID)
	})
}

// DisableCompany отключает компанию: после этого ни она, ни её точки не меняются.
func (s *Service) DisableCompany(ctx context.Context, actor eventstore.Actor, idempotencyKey string, companyID uuid.UUID) (Company, error) {
	spec := command.Spec{Name: "DisableCompany", Actor: actor, IdempotencyKey: idempotencyKey, Request: companyID}
	return s.changeCompany(ctx, spec, companyID, CompanyDisabled{CompanyID: companyID})
}

// changeCompany записывает событие работающей компании под блокировкой её потока.
func (s *Service) changeCompany(ctx context.Context, spec command.Spec, companyID uuid.UUID, event eventstore.Payload) (Company, error) {
	spec.Locks = []string{CompanyStream(companyID)}
	return command.Run(ctx, s.runner, spec, func(ctx context.Context, tx *command.Tx) (Company, error) {
		q := db.New(tx)
		company, err := readCompany(ctx, q, companyID)
		if err != nil {
			return Company{}, err
		}
		if !company.Active {
			return Company{}, fmt.Errorf("company %s: %w", companyID, ErrDisabled)
		}
		if err := record(ctx, tx, CompanyStream(companyID), event); err != nil {
			return Company{}, err
		}
		return readCompany(ctx, q, companyID)
	})
}

// CreatePoint создаёт точку в работающей компании.
func (s *Service) CreatePoint(ctx context.Context, actor eventstore.Actor, idempotencyKey string, companyID uuid.UUID, name string) (Point, error) {
	name, err := normalizedName(name)
	if err != nil {
		return Point{}, err
	}
	point := Point{ID: uuid.Must(uuid.NewV7()), CompanyID: companyID, Name: name, Active: true}

	spec := command.Spec{
		Name: "CreatePoint", Actor: actor, IdempotencyKey: idempotencyKey,
		// блокировка потока компании: точка не появится в компании, которую отключают в этот момент
		Locks: []string{CompanyStream(companyID)},
		Request: struct {
			CompanyID uuid.UUID
			Name      string
		}{companyID, name},
	}
	return command.Run(ctx, s.runner, spec, func(ctx context.Context, tx *command.Tx) (Point, error) {
		company, err := readCompany(ctx, db.New(tx), companyID)
		if err != nil {
			return Point{}, err
		}
		if !company.Active {
			return Point{}, fmt.Errorf("company %s: %w", companyID, ErrDisabled)
		}
		event := PointCreated{PointID: point.ID, CompanyID: companyID, Name: point.Name}
		if err := record(ctx, tx, PointStream(point.ID), event); err != nil {
			return Point{}, err
		}
		return point, nil
	})
}

// RenamePoint меняет название точки.
func (s *Service) RenamePoint(ctx context.Context, actor eventstore.Actor, idempotencyKey string, pointID uuid.UUID, name string) (Point, error) {
	name, err := normalizedName(name)
	if err != nil {
		return Point{}, err
	}
	spec := command.Spec{
		Name: "RenamePoint", Actor: actor, IdempotencyKey: idempotencyKey,
		Request: struct {
			PointID uuid.UUID
			Name    string
		}{pointID, name},
	}
	return onPoint(ctx, s, spec, pointID, func(ctx context.Context, p pointTx) (Point, error) {
		if err := p.record(ctx, PointRenamed{PointID: pointID, Name: name}); err != nil {
			return Point{}, err
		}
		return readPoint(ctx, p.q, pointID)
	})
}

// DisablePoint отключает точку: её состав больше не меняется, сотрудники теряют на ней права.
func (s *Service) DisablePoint(ctx context.Context, actor eventstore.Actor, idempotencyKey string, pointID uuid.UUID) (Point, error) {
	spec := command.Spec{Name: "DisablePoint", Actor: actor, IdempotencyKey: idempotencyKey, Request: pointID}
	return onPoint(ctx, s, spec, pointID, func(ctx context.Context, p pointTx) (Point, error) {
		if err := p.record(ctx, PointDisabled{PointID: pointID}); err != nil {
			return Point{}, err
		}
		return readPoint(ctx, p.q, pointID)
	})
}

// SetTelegramChat задаёт точке чат Telegram для уведомлений; chatID == nil снимает его.
func (s *Service) SetTelegramChat(ctx context.Context, actor eventstore.Actor, idempotencyKey string, pointID uuid.UUID, chatID *int64) (Point, error) {
	if chatID != nil && *chatID == 0 {
		return Point{}, &ValidationError{Field: "chat_id", Message: "must not be zero"}
	}
	spec := command.Spec{
		Name: "SetPointTelegramChat", Actor: actor, IdempotencyKey: idempotencyKey,
		Request: struct {
			PointID uuid.UUID
			ChatID  *int64
		}{pointID, chatID},
	}
	return onPoint(ctx, s, spec, pointID, func(ctx context.Context, p pointTx) (Point, error) {
		if err := p.record(ctx, PointTelegramChatChanged{PointID: pointID, ChatID: chatID}); err != nil {
			return Point{}, err
		}
		return readPoint(ctx, p.q, pointID)
	})
}

// Assign назначает на точку зарегистрированного пользователя с адресом почты email.
// by - кто назначает: управляющему точки нельзя назначать управляющих.
func (s *Service) Assign(ctx context.Context, actor eventstore.Actor, idempotencyKey, by string, pointID uuid.UUID, email string, role Role) (Staff, error) {
	if err := validRole(role); err != nil {
		return Staff{}, err
	}
	emailKey := key(email)
	if emailKey == "" {
		return Staff{}, &ValidationError{Field: "email", Message: "must not be empty"}
	}
	spec := command.Spec{
		Name: "AssignStaff", Actor: actor, IdempotencyKey: idempotencyKey,
		Request: struct {
			PointID uuid.UUID
			Email   string
			Role    Role
		}{pointID, emailKey, role},
	}
	return onPoint(ctx, s, spec, pointID, func(ctx context.Context, p pointTx) (Staff, error) {
		if err := p.mayManage(ctx, by, role); err != nil {
			return Staff{}, err
		}
		user, err := p.q.GetUserByEmail(ctx, emailKey)
		if errors.Is(err, pgx.ErrNoRows) {
			return Staff{}, fmt.Errorf("%s: %w", emailKey, ErrUnknownUser)
		}
		if err != nil {
			return Staff{}, fmt.Errorf("organization: find user: %w", err)
		}
		if _, err := p.role(ctx, user.ID); err == nil {
			return Staff{}, fmt.Errorf("user %s on point %s: %w", user.ID, pointID, ErrAlreadyAssigned)
		} else if !errors.Is(err, ErrNotFound) {
			return Staff{}, err
		}
		if err := p.record(ctx, StaffAssigned{PointID: pointID, UserID: user.ID, Role: role}); err != nil {
			return Staff{}, err
		}
		return Staff{UserID: user.ID, Email: user.Email, Name: user.Name, Role: role}, nil
	})
}

// ChangeRole меняет роль сотрудника на точке.
func (s *Service) ChangeRole(ctx context.Context, actor eventstore.Actor, idempotencyKey, by string, pointID uuid.UUID, userID string, role Role) (Staff, error) {
	if err := validRole(role); err != nil {
		return Staff{}, err
	}
	spec := command.Spec{
		Name: "ChangeStaffRole", Actor: actor, IdempotencyKey: idempotencyKey,
		Request: struct {
			PointID uuid.UUID
			UserID  string
			Role    Role
		}{pointID, userID, role},
	}
	return onPoint(ctx, s, spec, pointID, func(ctx context.Context, p pointTx) (Staff, error) {
		current, err := p.role(ctx, userID)
		if err != nil {
			return Staff{}, err
		}
		if err := p.mayManage(ctx, by, current, role); err != nil {
			return Staff{}, err
		}
		if current != role {
			if err := p.record(ctx, StaffRoleChanged{PointID: pointID, UserID: userID, Role: role}); err != nil {
				return Staff{}, err
			}
		}
		row, err := p.q.GetStaff(ctx, db.GetStaffParams{PointID: pointID, UserID: userID})
		if err != nil {
			return Staff{}, fmt.Errorf("organization: read staff: %w", err)
		}
		return Staff{UserID: row.UserID, Email: row.Email, Name: row.Name, Role: Role(row.Role)}, nil
	})
}

// Remove снимает сотрудника с точки.
func (s *Service) Remove(ctx context.Context, actor eventstore.Actor, idempotencyKey, by string, pointID uuid.UUID, userID string) error {
	spec := command.Spec{
		Name: "RemoveStaff", Actor: actor, IdempotencyKey: idempotencyKey,
		Request: struct {
			PointID uuid.UUID
			UserID  string
		}{pointID, userID},
	}
	_, err := onPoint(ctx, s, spec, pointID, func(ctx context.Context, p pointTx) (struct{}, error) {
		current, err := p.role(ctx, userID)
		if err != nil {
			return struct{}{}, err
		}
		if err := p.mayManage(ctx, by, current); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, p.record(ctx, StaffRemoved{PointID: pointID, UserID: userID})
	})
	return err
}

// pointTx - работающая точка работающей компании внутри команды, под блокировкой потока точки.
type pointTx struct {
	q       *db.Queries
	point   Point
	company Company
	record  func(ctx context.Context, event eventstore.Payload) error
}

// role возвращает роль пользователя на точке или ErrNotFound.
func (p pointTx) role(ctx context.Context, userID string) (Role, error) {
	role, err := p.q.GetAssignmentRole(ctx, db.GetAssignmentRoleParams{PointID: p.point.ID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("user %s on point %s: %w", userID, p.point.ID, ErrNotFound)
	}
	if err != nil {
		return "", fmt.Errorf("organization: read assignment: %w", err)
	}
	return Role(role), nil
}

// mayManage применяет правило mayManage к вызывающему: его отношение к точке читается под блокировкой.
func (p pointTx) mayManage(ctx context.Context, by string, roles ...Role) error {
	owner := p.company.OwnerID == by
	var caller Role
	if !owner {
		role, err := p.role(ctx, by)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
		caller = role
	}
	return mayManage(owner, caller, roles...)
}

// onPoint выполняет команду над точкой под блокировкой её потока. Отключённые точка и компания не меняются.
func onPoint[T any](ctx context.Context, s *Service, spec command.Spec, pointID uuid.UUID, fn func(context.Context, pointTx) (T, error)) (T, error) {
	spec.Locks = []string{PointStream(pointID)}
	return command.Run(ctx, s.runner, spec, func(ctx context.Context, tx *command.Tx) (T, error) {
		var zero T
		q := db.New(tx)
		point, err := readPoint(ctx, q, pointID)
		if err != nil {
			return zero, err
		}
		company, err := readCompany(ctx, q, point.CompanyID)
		if err != nil {
			return zero, err
		}
		if !point.Active || !company.Active {
			return zero, fmt.Errorf("point %s: %w", pointID, ErrDisabled)
		}
		return fn(ctx, pointTx{q: q, point: point, company: company, record: func(ctx context.Context, event eventstore.Payload) error {
			return record(ctx, tx, PointStream(pointID), event)
		}})
	})
}

// Company возвращает компанию или ErrNotFound.
func (s *Service) Company(ctx context.Context, id uuid.UUID) (Company, error) {
	return readCompany(ctx, db.New(s.pool), id)
}

// WorkingCompany возвращает работающую компанию. Отключённая - ErrDisabled: её каталог не меняется.
func (s *Service) WorkingCompany(ctx context.Context, id uuid.UUID) (Company, error) {
	company, err := readCompany(ctx, db.New(s.pool), id)
	if err != nil {
		return Company{}, err
	}
	if !company.Active {
		return Company{}, fmt.Errorf("company %s: %w", id, ErrDisabled)
	}
	return company, nil
}

// Point возвращает точку или ErrNotFound.
func (s *Service) Point(ctx context.Context, id uuid.UUID) (Point, error) {
	return readPoint(ctx, db.New(s.pool), id)
}

// WorkingPoint возвращает точку, на которой можно вести учёт: работающую точку работающей
// компании. Отключённая точка или компания - ErrDisabled: записи на них отклоняются,
// а читать их историю можно через Point.
func (s *Service) WorkingPoint(ctx context.Context, id uuid.UUID) (Point, error) {
	q := db.New(s.pool)
	point, err := readPoint(ctx, q, id)
	if err != nil {
		return Point{}, err
	}
	company, err := readCompany(ctx, q, point.CompanyID)
	if err != nil {
		return Point{}, err
	}
	if !point.Active || !company.Active {
		return Point{}, fmt.Errorf("point %s: %w", id, ErrDisabled)
	}
	return point, nil
}

// Working сообщает, можно ли вести учёт на точке: работают и она, и её компания.
// Несуществующая точка - тоже "нет".
func (s *Service) Working(ctx context.Context, pointID uuid.UUID) (bool, error) {
	_, err := s.WorkingPoint(ctx, pointID)
	if errors.Is(err, ErrDisabled) || errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// Points возвращает все точки компании, включая отключённые.
func (s *Service) Points(ctx context.Context, companyID uuid.UUID) ([]Point, error) {
	rows, err := db.New(s.pool).ListPointsOfCompany(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("organization: list points: %w", err)
	}
	points := make([]Point, len(rows))
	for i, row := range rows {
		points[i] = Point(row)
	}
	return points, nil
}

// Staff возвращает сотрудников точки.
func (s *Service) Staff(ctx context.Context, pointID uuid.UUID) ([]Staff, error) {
	rows, err := db.New(s.pool).ListStaff(ctx, pointID)
	if err != nil {
		return nil, fmt.Errorf("organization: list staff: %w", err)
	}
	staff := make([]Staff, len(rows))
	for i, row := range rows {
		staff[i] = Staff{UserID: row.UserID, Email: row.Email, Name: row.Name, Role: Role(row.Role)}
	}
	return staff, nil
}

// Profile возвращает компании, которыми пользователь владеет, и точки, где он может работать:
// все точки своих компаний и работающие точки, на которые он назначен.
func (s *Service) Profile(ctx context.Context, userID string) (Profile, error) {
	q := db.New(s.pool)
	companies, err := q.ListCompaniesOfOwner(ctx, userID)
	if err != nil {
		return Profile{}, fmt.Errorf("organization: list companies: %w", err)
	}
	owned, err := q.ListOwnedPoints(ctx, userID)
	if err != nil {
		return Profile{}, fmt.Errorf("organization: list owned points: %w", err)
	}
	assigned, err := q.ListAssignedPoints(ctx, userID)
	if err != nil {
		return Profile{}, fmt.Errorf("organization: list assigned points: %w", err)
	}

	profile := Profile{Companies: make([]Company, len(companies)), Points: make([]Membership, 0, len(owned)+len(assigned))}
	for i, row := range companies {
		profile.Companies[i] = Company(row)
	}
	ownedIDs := make(map[uuid.UUID]bool, len(owned))
	for _, row := range owned {
		ownedIDs[row.ID] = true
		profile.Points = append(profile.Points, Membership{
			Point:       Point{ID: row.ID, CompanyID: row.CompanyID, Name: row.Name, Active: row.Active},
			CompanyName: row.CompanyName, Owner: true, Permissions: permissions(true, ""),
		})
	}
	for _, row := range assigned {
		// владелец, назначенный на свою точку, остаётся владельцем
		if ownedIDs[row.ID] {
			continue
		}
		profile.Points = append(profile.Points, Membership{
			Point:       Point{ID: row.ID, CompanyID: row.CompanyID, Name: row.Name, Active: row.Active},
			CompanyName: row.CompanyName, Role: Role(row.Role), Permissions: permissions(false, Role(row.Role)),
		})
	}
	return profile, nil
}

// PointPermissions возвращает права пользователя на точке: все - владельцу её компании,
// права роли - сотруднику работающей точки, никаких - остальным. Точки нет - ErrNotFound.
// Читает базу при каждом вызове: снятое назначение перестаёт действовать сразу.
func (s *Service) PointPermissions(ctx context.Context, userID string, pointID uuid.UUID) ([]string, error) {
	q := db.New(s.pool)
	point, err := readPoint(ctx, q, pointID)
	if err != nil {
		return nil, err
	}
	company, err := readCompany(ctx, q, point.CompanyID)
	if err != nil {
		return nil, err
	}
	if company.OwnerID == userID {
		return permissions(true, ""), nil
	}
	if !point.Active || !company.Active {
		return nil, nil
	}
	role, err := q.GetAssignmentRole(ctx, db.GetAssignmentRoleParams{PointID: pointID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("organization: read assignment: %w", err)
	}
	return permissions(false, Role(role)), nil
}

// CompanyPermissions возвращает права пользователя в компании: все - владельцу,
// просмотр компании и её каталога - сотруднику любой её работающей точки. Компании нет - ErrNotFound.
func (s *Service) CompanyPermissions(ctx context.Context, userID string, companyID uuid.UUID) ([]string, error) {
	q := db.New(s.pool)
	company, err := readCompany(ctx, q, companyID)
	if err != nil {
		return nil, err
	}
	if company.OwnerID == userID {
		return permissions(true, ""), nil
	}
	works, err := q.WorksInCompany(ctx, db.WorksInCompanyParams{UserID: userID, CompanyID: companyID})
	if err != nil {
		return nil, fmt.Errorf("organization: read assignments: %w", err)
	}
	if !works || !company.Active {
		return nil, nil
	}
	return slices.Clone(staffCompanyPermissions), nil
}

// record добавляет событие в конец потока и применяет его к таблицам.
func record(ctx context.Context, tx *command.Tx, stream string, event eventstore.Payload) error {
	version, err := eventstore.StreamVersion(ctx, tx, stream)
	if err != nil {
		return err
	}
	if _, err := tx.Append(ctx, stream, version, event); err != nil {
		return err
	}
	return appliers[event.EventType()](ctx, db.New(tx), event)
}

// Таблицы организации меняются только функциями из appliers: их содержимое - функция
// от событий, перестроение из Event Store вызывает эти же функции. Новое событие модуля
// добавляется сюда.
var appliers = map[string]func(ctx context.Context, q *db.Queries, p eventstore.Payload) error{
	"CompanyCreated": func(ctx context.Context, q *db.Queries, p eventstore.Payload) error {
		e := p.(CompanyCreated)
		return saved(q.InsertCompany(ctx, db.InsertCompanyParams{ID: e.CompanyID, Name: e.Name, OwnerID: e.OwnerID}))
	},
	"CompanyRenamed": func(ctx context.Context, q *db.Queries, p eventstore.Payload) error {
		e := p.(CompanyRenamed)
		return saved(q.RenameCompany(ctx, db.RenameCompanyParams{ID: e.CompanyID, Name: e.Name}))
	},
	"CompanyOwnerChanged": func(ctx context.Context, q *db.Queries, p eventstore.Payload) error {
		e := p.(CompanyOwnerChanged)
		return saved(q.SetCompanyOwner(ctx, db.SetCompanyOwnerParams{ID: e.CompanyID, OwnerID: e.OwnerID}))
	},
	"CompanyDisabled": func(ctx context.Context, q *db.Queries, p eventstore.Payload) error {
		return saved(q.DisableCompany(ctx, p.(CompanyDisabled).CompanyID))
	},
	"PointCreated": func(ctx context.Context, q *db.Queries, p eventstore.Payload) error {
		e := p.(PointCreated)
		return saved(q.InsertPoint(ctx, db.InsertPointParams{ID: e.PointID, CompanyID: e.CompanyID, Name: e.Name, NameKey: key(e.Name)}))
	},
	"PointRenamed": func(ctx context.Context, q *db.Queries, p eventstore.Payload) error {
		e := p.(PointRenamed)
		return saved(q.RenamePoint(ctx, db.RenamePointParams{ID: e.PointID, Name: e.Name, NameKey: key(e.Name)}))
	},
	"PointDisabled": func(ctx context.Context, q *db.Queries, p eventstore.Payload) error {
		return saved(q.DisablePoint(ctx, p.(PointDisabled).PointID))
	},
	"PointTelegramChatChanged": func(ctx context.Context, q *db.Queries, p eventstore.Payload) error {
		e := p.(PointTelegramChatChanged)
		return saved(q.SetPointTelegramChat(ctx, db.SetPointTelegramChatParams{ID: e.PointID, ChatID: e.ChatID}))
	},
	"StaffAssigned": func(ctx context.Context, q *db.Queries, p eventstore.Payload) error {
		e := p.(StaffAssigned)
		return saved(q.InsertAssignment(ctx, db.InsertAssignmentParams{PointID: e.PointID, UserID: e.UserID, Role: string(e.Role)}))
	},
	"StaffRoleChanged": func(ctx context.Context, q *db.Queries, p eventstore.Payload) error {
		e := p.(StaffRoleChanged)
		return saved(q.UpdateAssignmentRole(ctx, db.UpdateAssignmentRoleParams{PointID: e.PointID, UserID: e.UserID, Role: string(e.Role)}))
	},
	"StaffRemoved": func(ctx context.Context, q *db.Queries, p eventstore.Payload) error {
		e := p.(StaffRemoved)
		return saved(q.DeleteAssignment(ctx, db.DeleteAssignmentParams{PointID: e.PointID, UserID: e.UserID}))
	},
}

// saved превращает нарушение уникальности названия точки в ErrNameTaken: проверка названия
// выполняется ограничением таблицы, поэтому гонка двух одновременных созданий невозможна.
func saved(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName == "organization_points_name_key" {
		return ErrNameTaken
	}
	return fmt.Errorf("organization: save: %w", err)
}

func readCompany(ctx context.Context, q *db.Queries, id uuid.UUID) (Company, error) {
	row, err := q.GetCompany(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Company{}, fmt.Errorf("company %s: %w", id, ErrNotFound)
	}
	if err != nil {
		return Company{}, fmt.Errorf("organization: read company: %w", err)
	}
	return Company(row), nil
}

func readPoint(ctx context.Context, q *db.Queries, id uuid.UUID) (Point, error) {
	row, err := q.GetPoint(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Point{}, fmt.Errorf("point %s: %w", id, ErrNotFound)
	}
	if err != nil {
		return Point{}, fmt.Errorf("organization: read point: %w", err)
	}
	return Point(row), nil
}
