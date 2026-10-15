// Package suppliers - поставщики компании и их ключи (ADR-014): владелец заводит поставщика с точками,
// а поставщик своим ключом регистрирует поставки на этих точках и читает продукты компании.
package suppliers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"goden/backend/internal/organization"
	"goden/backend/internal/platform/auth"
	"goden/backend/internal/platform/clock"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/httpx"
	"goden/backend/internal/suppliers/db"
)

// PermissionAccess - право клиента с ключом поставщика: операции поставщика о себе и продуктах.
const PermissionAccess = "supplier.access"

// pointPermissions - права поставщика на его работающей точке: только регистрация поставки.
var pointPermissions = []string{"deliveries.register"}

const (
	keyPrefix   = "gdn_"
	actorPrefix = "supplier:"
)

var (
	// ErrNotFound - поставщика нет в компании из адреса.
	ErrNotFound = errors.New("suppliers: supplier not found")
	// ErrNameTaken - в компании уже есть поставщик с таким названием.
	ErrNameTaken = errors.New("suppliers: name is already taken")
)

// ValidationError - некорректное поле входных данных.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("suppliers: %s: %s", e.Field, e.Message)
}

// InvalidField возвращает некорректное поле и сообщение для слоя API.
func (e *ValidationError) InvalidField() (field, message string) {
	return e.Field, e.Message
}

// Supplier - поставщик компании.
type Supplier struct {
	ID        uuid.UUID
	CompanyID uuid.UUID
	Name      string
	PointIDs  []uuid.UUID
	KeyActive bool
	CreatedAt time.Time
}

// Input - название и точки поставщика.
type Input struct {
	Name     string
	PointIDs []uuid.UUID
}

type Service struct {
	pool   *pgxpool.Pool
	clock  clock.Clock
	org    *organization.Service
	logger *slog.Logger
}

func NewService(pool *pgxpool.Pool, clk clock.Clock, org *organization.Service, logger *slog.Logger) *Service {
	return &Service{pool: pool, clock: clk, org: org, logger: logger}
}

// Actor - автор действий поставщика.
func Actor(id uuid.UUID) eventstore.Actor {
	return eventstore.Actor{Type: eventstore.ActorService, ID: actorPrefix + id.String()}
}

// FromActor возвращает поставщика автора; false - автор не поставщик.
func FromActor(actor eventstore.Actor) (uuid.UUID, bool) {
	rest, ok := strings.CutPrefix(actor.ID, actorPrefix)
	if actor.Type != eventstore.ActorService || !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(rest)
	return id, err == nil
}

// newKey выпускает ключ и его хеш: в базе хранится только хеш.
func newKey() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("suppliers: random key: %w", err)
	}
	key := keyPrefix + base64.RawURLEncoding.EncodeToString(raw)
	return key, hash(key), nil
}

func hash(key string) []byte {
	sum := sha256.Sum256([]byte(key))
	return sum[:]
}

// Create заводит поставщика с новым ключом и возвращает его вместе с ключом.
func (s *Service) Create(ctx context.Context, companyID uuid.UUID, in Input) (Supplier, string, error) {
	in, err := s.valid(ctx, companyID, in)
	if err != nil {
		return Supplier{}, "", err
	}
	key, keyHash, err := newKey()
	if err != nil {
		return Supplier{}, "", err
	}
	id := uuid.Must(uuid.NewV7())
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		err := q.InsertSupplier(ctx, db.InsertSupplierParams{
			ID: id, CompanyID: companyID, Name: in.Name, KeyHash: keyHash, CreatedAt: s.clock.Now().UTC(),
		})
		if err != nil {
			return saved(err)
		}
		return q.InsertSupplierPoints(ctx, db.InsertSupplierPointsParams{SupplierID: id, PointIds: in.PointIDs})
	})
	if err != nil {
		return Supplier{}, "", fmt.Errorf("suppliers: create: %w", err)
	}
	s.audit(ctx, "supplier key issued", companyID, id)
	supplier, err := s.Get(ctx, companyID, id)
	return supplier, key, err
}

// Update меняет название и точки поставщика.
func (s *Service) Update(ctx context.Context, companyID, id uuid.UUID, in Input) (Supplier, error) {
	in, err := s.valid(ctx, companyID, in)
	if err != nil {
		return Supplier{}, err
	}
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		n, err := q.UpdateSupplierName(ctx, db.UpdateSupplierNameParams{
			ID: id, CompanyID: companyID, Name: in.Name, UpdatedAt: s.clock.Now().UTC(),
		})
		if err != nil {
			return saved(err)
		}
		if n == 0 {
			return ErrNotFound
		}
		if err := q.DeleteSupplierPoints(ctx, id); err != nil {
			return err
		}
		return q.InsertSupplierPoints(ctx, db.InsertSupplierPointsParams{SupplierID: id, PointIds: in.PointIDs})
	})
	if err != nil {
		return Supplier{}, fmt.Errorf("suppliers: update: %w", err)
	}
	return s.Get(ctx, companyID, id)
}

// IssueKey выпускает поставщику новый ключ: прежний сразу перестаёт действовать.
func (s *Service) IssueKey(ctx context.Context, companyID, id uuid.UUID) (Supplier, string, error) {
	if _, err := s.org.WorkingCompany(ctx, companyID); err != nil {
		return Supplier{}, "", err
	}
	key, keyHash, err := newKey()
	if err != nil {
		return Supplier{}, "", err
	}
	if err := s.setKey(ctx, companyID, id, keyHash); err != nil {
		return Supplier{}, "", err
	}
	s.audit(ctx, "supplier key issued", companyID, id)
	supplier, err := s.Get(ctx, companyID, id)
	return supplier, key, err
}

// RevokeKey отзывает ключ поставщика без выдачи нового. Отозвать можно и в отключённой компании.
func (s *Service) RevokeKey(ctx context.Context, companyID, id uuid.UUID) (Supplier, error) {
	if err := s.setKey(ctx, companyID, id, nil); err != nil {
		return Supplier{}, err
	}
	s.audit(ctx, "supplier key revoked", companyID, id)
	return s.Get(ctx, companyID, id)
}

func (s *Service) setKey(ctx context.Context, companyID, id uuid.UUID, keyHash []byte) error {
	n, err := db.New(s.pool).SetSupplierKey(ctx, db.SetSupplierKeyParams{
		KeyHash: keyHash, UpdatedAt: s.clock.Now().UTC(), ID: id, CompanyID: companyID,
	})
	if err != nil {
		return fmt.Errorf("suppliers: set key: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// audit пишет в журнал, кто выпустил или отозвал ключ; сам ключ в журнал не попадает.
func (s *Service) audit(ctx context.Context, msg string, companyID, id uuid.UUID) {
	actor, _ := httpx.Actor(ctx)
	s.logger.InfoContext(ctx, msg, "company_id", companyID, "supplier_id", id, "actor_type", actor.Type, "actor_id", actor.ID)
}

// Get возвращает поставщика компании или ErrNotFound.
func (s *Service) Get(ctx context.Context, companyID, id uuid.UUID) (Supplier, error) {
	q := db.New(s.pool)
	row, err := q.GetSupplier(ctx, db.GetSupplierParams{ID: id, CompanyID: companyID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Supplier{}, ErrNotFound
	}
	if err != nil {
		return Supplier{}, fmt.Errorf("suppliers: read: %w", err)
	}
	suppliers := []Supplier{{ID: row.ID, CompanyID: row.CompanyID, Name: row.Name, KeyActive: row.KeyActive, CreatedAt: row.CreatedAt}}
	if err := withPoints(ctx, q, suppliers); err != nil {
		return Supplier{}, err
	}
	return suppliers[0], nil
}

// List возвращает поставщиков компании по названию.
func (s *Service) List(ctx context.Context, companyID uuid.UUID) ([]Supplier, error) {
	q := db.New(s.pool)
	rows, err := q.ListSuppliers(ctx, companyID)
	if err != nil {
		return nil, fmt.Errorf("suppliers: list: %w", err)
	}
	suppliers := make([]Supplier, len(rows))
	for i, row := range rows {
		suppliers[i] = Supplier{ID: row.ID, CompanyID: row.CompanyID, Name: row.Name, KeyActive: row.KeyActive, CreatedAt: row.CreatedAt}
	}
	return suppliers, withPoints(ctx, q, suppliers)
}

func withPoints(ctx context.Context, q *db.Queries, suppliers []Supplier) error {
	ids := make([]uuid.UUID, len(suppliers))
	for i, supplier := range suppliers {
		ids[i] = supplier.ID
	}
	rows, err := q.ListSupplierPoints(ctx, ids)
	if err != nil {
		return fmt.Errorf("suppliers: list points: %w", err)
	}
	for i := range suppliers {
		suppliers[i].PointIDs = []uuid.UUID{}
		for _, row := range rows {
			if row.SupplierID == suppliers[i].ID {
				suppliers[i].PointIDs = append(suppliers[i].PointIDs, row.PointID)
			}
		}
	}
	return nil
}

// valid проверяет название и точки: компания работает, точки - её работающие точки.
func (s *Service) valid(ctx context.Context, companyID uuid.UUID, in Input) (Input, error) {
	if _, err := s.org.WorkingCompany(ctx, companyID); err != nil {
		return Input{}, err
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return Input{}, &ValidationError{Field: "name", Message: "must not be empty"}
	}
	if len(in.PointIDs) == 0 {
		return Input{}, &ValidationError{Field: "point_ids", Message: "must contain at least one point"}
	}
	points := slices.Clone(in.PointIDs)
	slices.SortFunc(points, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	in.PointIDs = slices.Compact(points)
	for _, id := range in.PointIDs {
		point, err := s.org.WorkingPoint(ctx, id)
		if errors.Is(err, organization.ErrNotFound) || errors.Is(err, organization.ErrDisabled) || (err == nil && point.CompanyID != companyID) {
			return Input{}, &ValidationError{Field: "point_ids", Message: "must be working points of the company"}
		}
		if err != nil {
			return Input{}, err
		}
	}
	return in, nil
}

// saved превращает нарушение уникальности названия в ErrNameTaken.
func saved(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName == "suppliers_company_name" {
		return ErrNameTaken
	}
	return err
}

// Principal - поставщик по ключу.
type Principal struct {
	ID        uuid.UUID
	CompanyID uuid.UUID
}

// ByKey возвращает поставщика с действующим ключом; ok false - ключа нет или он отозван.
func (s *Service) ByKey(ctx context.Context, key string) (Principal, bool, error) {
	row, err := db.New(s.pool).SupplierByKey(ctx, hash(key))
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, false, nil
	}
	if err != nil {
		return Principal{}, false, fmt.Errorf("suppliers: find key: %w", err)
	}
	return Principal{ID: row.ID, CompanyID: row.CompanyID}, true, nil
}

// Self возвращает поставщика по его идентификатору и работающие точки из его списка.
// Отключённая компания - organization.ErrDisabled.
func (s *Service) Self(ctx context.Context, id uuid.UUID) (Supplier, organization.Company, []organization.Point, error) {
	supplier, company, err := s.self(ctx, id)
	if err != nil {
		return Supplier{}, organization.Company{}, nil, err
	}
	var points []organization.Point
	for _, pointID := range supplier.PointIDs {
		point, err := s.org.WorkingPoint(ctx, pointID)
		if errors.Is(err, organization.ErrDisabled) || errors.Is(err, organization.ErrNotFound) {
			continue
		}
		if err != nil {
			return Supplier{}, organization.Company{}, nil, err
		}
		points = append(points, point)
	}
	return supplier, company, points, nil
}

// Company возвращает работающую компанию поставщика; отключённая - organization.ErrDisabled.
func (s *Service) Company(ctx context.Context, id uuid.UUID) (organization.Company, error) {
	_, company, err := s.self(ctx, id)
	return company, err
}

func (s *Service) self(ctx context.Context, id uuid.UUID) (Supplier, organization.Company, error) {
	row, err := db.New(s.pool).SupplierCompany(ctx, id)
	if err != nil {
		return Supplier{}, organization.Company{}, fmt.Errorf("suppliers: read self: %w", err)
	}
	company, err := s.org.WorkingCompany(ctx, row)
	if err != nil {
		return Supplier{}, organization.Company{}, err
	}
	supplier, err := s.Get(ctx, company.ID, id)
	return supplier, company, err
}

// PointPermissions возвращает права поставщика на точке: непустые, только если точка его и работает.
func (s *Service) PointPermissions(ctx context.Context, id, pointID uuid.UUID) ([]string, error) {
	working, err := s.org.Working(ctx, pointID)
	if err != nil || !working {
		return nil, err
	}
	on, err := db.New(s.pool).SupplierOnPoint(ctx, db.SupplierOnPointParams{SupplierID: id, PointID: pointID})
	if err != nil || !on {
		return nil, err
	}
	return pointPermissions, nil
}

// Names возвращает текущие названия поставщиков по идентификаторам сервисных авторов;
// идентификаторы не поставщиков пропускаются.
func (s *Service) Names(ctx context.Context, actorIDs []string) (map[string]string, error) {
	var ids []uuid.UUID
	for _, actorID := range actorIDs {
		if id, ok := FromActor(eventstore.Actor{Type: eventstore.ActorService, ID: actorID}); ok {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := db.New(s.pool).SupplierNames(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("suppliers: names: %w", err)
	}
	names := make(map[string]string, len(rows))
	for _, row := range rows {
		names[Actor(row.ID).ID] = row.Name
	}
	return names, nil
}

type keyVerifier struct {
	svc  *Service
	next auth.Verifier
}

// Verifier проверяет ключ поставщика, остальные токены передаёт next.
func (s *Service) Verifier(next auth.Verifier) auth.Verifier {
	return keyVerifier{svc: s, next: next}
}

func (v keyVerifier) Verify(ctx context.Context, token string) (auth.Principal, error) {
	if !strings.HasPrefix(token, keyPrefix) {
		return v.next.Verify(ctx, token)
	}
	supplier, ok, err := v.svc.ByKey(ctx, token)
	if err != nil {
		return auth.Principal{}, err
	}
	if !ok {
		return auth.Principal{}, fmt.Errorf("%w: unknown supplier key", auth.ErrInvalidToken)
	}
	actor := Actor(supplier.ID)
	principal := auth.NewPrincipal(actor, PermissionAccess)
	// личность не совпадёт ни с одним пользователем Keycloak: владения и назначений у поставщика нет
	principal.User = auth.User{ID: actor.ID}
	return principal, nil
}
