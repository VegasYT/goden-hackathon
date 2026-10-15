package organization

import (
	"context"
	"errors"

	"goden/backend/internal/api/oas"
	"goden/backend/internal/platform/auth"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/httpx"
)

// HTTP реализует операции организации из api/openapi.yaml. Право на точке и в компании
// проверяет слой API до вызова обработчика.
type HTTP struct {
	svc *Service
}

func NewHTTP(svc *Service) *HTTP {
	return &HTTP{svc: svc}
}

// caller возвращает автора и личность клиента запроса.
func caller(ctx context.Context) (eventstore.Actor, auth.User, error) {
	actor, hasActor := httpx.Actor(ctx)
	user, hasUser := auth.UserFrom(ctx)
	if !hasActor || !hasUser {
		return eventstore.Actor{}, auth.User{}, errors.New("organization: request has no caller")
	}
	return actor, user, nil
}

// known запоминает клиента запроса: после этого его можно назначить на точку по почте.
func (h *HTTP) known(ctx context.Context) (eventstore.Actor, auth.User, error) {
	actor, user, err := caller(ctx)
	if err != nil {
		return actor, user, err
	}
	known := User{ID: user.ID, Email: user.Email, Name: user.Name}
	if actor.Type == eventstore.ActorUser {
		known.Login = actor.ID
	}
	return actor, user, h.svc.Remember(ctx, known)
}

// GetMe реализует GET /me.
func (h *HTTP) GetMe(ctx context.Context) (*oas.Profile, error) {
	_, user, err := h.known(ctx)
	if err != nil {
		return nil, err
	}
	profile, err := h.svc.Profile(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	out := &oas.Profile{
		ID: user.ID, Email: user.Email, Name: user.Name,
		Companies: make([]oas.Company, len(profile.Companies)), Points: make([]oas.ProfilePoint, len(profile.Points)),
	}
	for i, company := range profile.Companies {
		out.Companies[i] = *companyResponse(company)
	}
	for i, m := range profile.Points {
		point := oas.ProfilePoint{
			ID: m.Point.ID, CompanyID: m.Point.CompanyID, CompanyName: m.CompanyName, Name: m.Point.Name,
			Active: m.Point.Active, Owner: m.Owner, Permissions: m.Permissions,
		}
		if !m.Owner {
			point.Role = oas.NewOptRole(oas.Role(m.Role))
		}
		out.Points[i] = point
	}
	return out, nil
}

// CreateCompany реализует POST /companies.
func (h *HTTP) CreateCompany(ctx context.Context, req *oas.NewCompany, params oas.CreateCompanyParams) (*oas.Company, error) {
	actor, user, err := h.known(ctx)
	if err != nil {
		return nil, err
	}
	company, err := h.svc.CreateCompany(ctx, actor, params.IdempotencyKey.Or(""), user.ID, req.Name)
	if err != nil {
		return nil, err
	}
	return companyResponse(company), nil
}

// GetCompany реализует GET /companies/{companyId}.
func (h *HTTP) GetCompany(ctx context.Context, params oas.GetCompanyParams) (*oas.Company, error) {
	company, err := h.svc.Company(ctx, params.CompanyId)
	if err != nil {
		return nil, err
	}
	return companyResponse(company), nil
}

// RenameCompany реализует PATCH /companies/{companyId}.
func (h *HTTP) RenameCompany(ctx context.Context, req *oas.NewCompany, params oas.RenameCompanyParams) (*oas.Company, error) {
	actor, _, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	company, err := h.svc.RenameCompany(ctx, actor, params.IdempotencyKey.Or(""), params.CompanyId, req.Name)
	if err != nil {
		return nil, err
	}
	return companyResponse(company), nil
}

// TransferCompany реализует PUT /companies/{companyId}/owner.
func (h *HTTP) TransferCompany(ctx context.Context, req *oas.NewOwner, params oas.TransferCompanyParams) (*oas.Company, error) {
	actor, user, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	company, err := h.svc.TransferCompany(ctx, actor, params.IdempotencyKey.Or(""), user.ID, params.CompanyId, req.Email)
	if err != nil {
		return nil, err
	}
	return companyResponse(company), nil
}

// DisableCompany реализует POST /companies/{companyId}/disable.
func (h *HTTP) DisableCompany(ctx context.Context, params oas.DisableCompanyParams) (*oas.Company, error) {
	actor, _, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	company, err := h.svc.DisableCompany(ctx, actor, params.IdempotencyKey.Or(""), params.CompanyId)
	if err != nil {
		return nil, err
	}
	return companyResponse(company), nil
}

// ListPoints реализует GET /companies/{companyId}/points.
func (h *HTTP) ListPoints(ctx context.Context, params oas.ListPointsParams) ([]oas.Point, error) {
	points, err := h.svc.Points(ctx, params.CompanyId)
	if err != nil {
		return nil, err
	}
	out := make([]oas.Point, len(points))
	for i, point := range points {
		out[i] = *pointResponse(point)
	}
	return out, nil
}

// CreatePoint реализует POST /companies/{companyId}/points.
func (h *HTTP) CreatePoint(ctx context.Context, req *oas.NewPoint, params oas.CreatePointParams) (*oas.Point, error) {
	actor, _, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	point, err := h.svc.CreatePoint(ctx, actor, params.IdempotencyKey.Or(""), params.CompanyId, req.Name)
	if err != nil {
		return nil, err
	}
	return pointResponse(point), nil
}

// GetPoint реализует GET /points/{pointId}.
func (h *HTTP) GetPoint(ctx context.Context, params oas.GetPointParams) (*oas.Point, error) {
	point, err := h.svc.Point(ctx, params.PointId)
	if err != nil {
		return nil, err
	}
	return pointResponse(point), nil
}

// RenamePoint реализует PATCH /points/{pointId}.
func (h *HTTP) RenamePoint(ctx context.Context, req *oas.NewPoint, params oas.RenamePointParams) (*oas.Point, error) {
	actor, _, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	point, err := h.svc.RenamePoint(ctx, actor, params.IdempotencyKey.Or(""), params.PointId, req.Name)
	if err != nil {
		return nil, err
	}
	return pointResponse(point), nil
}

// DisablePoint реализует POST /points/{pointId}/disable.
func (h *HTTP) DisablePoint(ctx context.Context, params oas.DisablePointParams) (*oas.Point, error) {
	actor, _, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	point, err := h.svc.DisablePoint(ctx, actor, params.IdempotencyKey.Or(""), params.PointId)
	if err != nil {
		return nil, err
	}
	return pointResponse(point), nil
}

// SetPointTelegramChat реализует PUT /points/{pointId}/telegram-chat.
func (h *HTTP) SetPointTelegramChat(ctx context.Context, req *oas.PointTelegramChat, params oas.SetPointTelegramChatParams) (*oas.Point, error) {
	actor, _, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	var chatID *int64
	if id, set := req.ChatID.Get(); set {
		chatID = &id
	}
	point, err := h.svc.SetTelegramChat(ctx, actor, params.IdempotencyKey.Or(""), params.PointId, chatID)
	if err != nil {
		return nil, err
	}
	return pointResponse(point), nil
}

// ListStaff реализует GET /points/{pointId}/staff.
func (h *HTTP) ListStaff(ctx context.Context, params oas.ListStaffParams) ([]oas.Staff, error) {
	staff, err := h.svc.Staff(ctx, params.PointId)
	if err != nil {
		return nil, err
	}
	out := make([]oas.Staff, len(staff))
	for i, member := range staff {
		out[i] = *staffResponse(member)
	}
	return out, nil
}

// AssignStaff реализует POST /points/{pointId}/staff.
func (h *HTTP) AssignStaff(ctx context.Context, req *oas.NewStaff, params oas.AssignStaffParams) (*oas.Staff, error) {
	actor, user, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	staff, err := h.svc.Assign(ctx, actor, params.IdempotencyKey.Or(""), user.ID, params.PointId, req.Email, Role(req.Role))
	if err != nil {
		return nil, err
	}
	return staffResponse(staff), nil
}

// ChangeStaffRole реализует PUT /points/{pointId}/staff/{userId}.
func (h *HTTP) ChangeStaffRole(ctx context.Context, req *oas.StaffRole, params oas.ChangeStaffRoleParams) (*oas.Staff, error) {
	actor, user, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	staff, err := h.svc.ChangeRole(ctx, actor, params.IdempotencyKey.Or(""), user.ID, params.PointId, params.UserId, Role(req.Role))
	if err != nil {
		return nil, err
	}
	return staffResponse(staff), nil
}

// RemoveStaff реализует DELETE /points/{pointId}/staff/{userId}.
func (h *HTTP) RemoveStaff(ctx context.Context, params oas.RemoveStaffParams) error {
	actor, user, err := caller(ctx)
	if err != nil {
		return err
	}
	return h.svc.Remove(ctx, actor, params.IdempotencyKey.Or(""), user.ID, params.PointId, params.UserId)
}

func companyResponse(c Company) *oas.Company {
	return &oas.Company{ID: c.ID, Name: c.Name, OwnerID: c.OwnerID, Active: c.Active}
}

func pointResponse(p Point) *oas.Point {
	out := &oas.Point{ID: p.ID, CompanyID: p.CompanyID, Name: p.Name, Active: p.Active}
	if p.TelegramChatID != nil {
		out.TelegramChatID = oas.NewOptInt64(*p.TelegramChatID)
	}
	return out
}

func staffResponse(s Staff) *oas.Staff {
	return &oas.Staff{UserID: s.UserID, Email: s.Email, Name: s.Name, Role: oas.Role(s.Role)}
}

// WithNames подставляет авторам-пользователям имена, а сервисным авторам с известным именем (поставщикам) -
// их названия. В событии автор записан логином или идентификатором; имя берётся при ответе, поэтому
// следует за сменой имени в Keycloak и переименованием поставщика.
func (s *Service) WithNames(ctx context.Context, actors []*oas.Actor) error {
	var logins, services []string
	for _, actor := range actors {
		if actor.Type == oas.ActorTypeUser {
			logins = append(logins, actor.ID)
		} else {
			services = append(services, actor.ID)
		}
	}
	names, err := s.Names(ctx, logins)
	if err != nil {
		return err
	}
	var serviceNames map[string]string
	if s.service != nil && len(services) > 0 {
		if serviceNames, err = s.service.Names(ctx, services); err != nil {
			return err
		}
	}
	for _, actor := range actors {
		known := names
		if actor.Type != oas.ActorTypeUser {
			known = serviceNames
		}
		if name, ok := known[actor.ID]; ok {
			actor.Name = oas.NewOptString(name)
		}
	}
	return nil
}
