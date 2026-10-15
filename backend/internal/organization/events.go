// Package organization ведёт компании, точки и назначения сотрудников и отвечает,
// какие права есть у пользователя на точке и в компании (ADR-013).
package organization

import (
	"github.com/google/uuid"

	"goden/backend/internal/platform/eventstore"
)

func init() {
	eventstore.Register[CompanyCreated]()
	eventstore.Register[CompanyRenamed]()
	eventstore.Register[CompanyOwnerChanged]()
	eventstore.Register[CompanyDisabled]()
	eventstore.Register[PointCreated]()
	eventstore.Register[PointRenamed]()
	eventstore.Register[PointDisabled]()
	eventstore.Register[PointTelegramChatChanged]()
	eventstore.Register[StaffAssigned]()
	eventstore.Register[StaffRoleChanged]()
	eventstore.Register[StaffRemoved]()
}

// CompanyStream - поток компании.
func CompanyStream(companyID uuid.UUID) string { return "company-" + companyID.String() }

// PointStream - поток точки и её назначений: состав точки меняется по версиям одного потока.
func PointStream(pointID uuid.UUID) string { return "point-" + pointID.String() }

// Role - роль сотрудника на точке.
type Role string

const (
	RoleManager  Role = "manager"
	RoleApprover Role = "approver"
	RoleCook     Role = "cook"
	RoleWaiter   Role = "waiter"
)

func (r *Role) UnmarshalText(text []byte) error {
	return eventstore.ParseEnum(r, text, "role", RoleManager, RoleApprover, RoleCook, RoleWaiter)
}

type CompanyCreated struct {
	CompanyID uuid.UUID `json:"company_id"`
	Name      string    `json:"name"`
	// первый владелец; дальше он меняется событием CompanyOwnerChanged
	OwnerID string `json:"owner_id"`
}

func (CompanyCreated) EventType() string  { return "CompanyCreated" }
func (CompanyCreated) SchemaVersion() int { return 1 }

// CompanyOwnerChanged - владелец передал компанию другому пользователю.
type CompanyOwnerChanged struct {
	CompanyID uuid.UUID `json:"company_id"`
	OwnerID   string    `json:"owner_id"`
}

func (CompanyOwnerChanged) EventType() string  { return "CompanyOwnerChanged" }
func (CompanyOwnerChanged) SchemaVersion() int { return 1 }

type CompanyRenamed struct {
	CompanyID uuid.UUID `json:"company_id"`
	Name      string    `json:"name"`
}

func (CompanyRenamed) EventType() string  { return "CompanyRenamed" }
func (CompanyRenamed) SchemaVersion() int { return 1 }

type CompanyDisabled struct {
	CompanyID uuid.UUID `json:"company_id"`
}

func (CompanyDisabled) EventType() string  { return "CompanyDisabled" }
func (CompanyDisabled) SchemaVersion() int { return 1 }

type PointCreated struct {
	PointID   uuid.UUID `json:"point_id"`
	CompanyID uuid.UUID `json:"company_id"`
	Name      string    `json:"name"`
}

func (PointCreated) EventType() string  { return "PointCreated" }
func (PointCreated) SchemaVersion() int { return 1 }

type PointRenamed struct {
	PointID uuid.UUID `json:"point_id"`
	Name    string    `json:"name"`
}

func (PointRenamed) EventType() string  { return "PointRenamed" }
func (PointRenamed) SchemaVersion() int { return 1 }

type PointDisabled struct {
	PointID uuid.UUID `json:"point_id"`
}

func (PointDisabled) EventType() string  { return "PointDisabled" }
func (PointDisabled) SchemaVersion() int { return 1 }

// PointTelegramChatChanged - точке задали чат Telegram для уведомлений или сняли его.
type PointTelegramChatChanged struct {
	PointID uuid.UUID `json:"point_id"`
	// идентификатор чата Telegram; без него уведомления точки уходят в общий чат окружения
	ChatID *int64 `json:"chat_id,omitempty"`
}

func (PointTelegramChatChanged) EventType() string  { return "PointTelegramChatChanged" }
func (PointTelegramChatChanged) SchemaVersion() int { return 1 }

type StaffAssigned struct {
	PointID uuid.UUID `json:"point_id"`
	UserID  string    `json:"user_id"`
	Role    Role      `json:"role"`
}

func (StaffAssigned) EventType() string  { return "StaffAssigned" }
func (StaffAssigned) SchemaVersion() int { return 1 }

type StaffRoleChanged struct {
	PointID uuid.UUID `json:"point_id"`
	UserID  string    `json:"user_id"`
	Role    Role      `json:"role"`
}

func (StaffRoleChanged) EventType() string  { return "StaffRoleChanged" }
func (StaffRoleChanged) SchemaVersion() int { return 1 }

type StaffRemoved struct {
	PointID uuid.UUID `json:"point_id"`
	UserID  string    `json:"user_id"`
}

func (StaffRemoved) EventType() string  { return "StaffRemoved" }
func (StaffRemoved) SchemaVersion() int { return 1 }
