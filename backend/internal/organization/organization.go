package organization

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
)

var (
	// ErrNotFound - компании, точки или назначения нет.
	ErrNotFound = errors.New("organization: not found")
	// ErrNameTaken - в компании уже есть точка с таким названием.
	ErrNameTaken = errors.New("organization: name is already taken")
	// ErrAlreadyAssigned - у пользователя уже есть назначение на этой точке.
	ErrAlreadyAssigned = errors.New("organization: user is already assigned to the point")
	// ErrDisabled - компания или точка отключена и не меняется.
	ErrDisabled = errors.New("organization: disabled")
	// ErrUnknownUser - с таким адресом почты никто не обращался к системе.
	ErrUnknownUser = errors.New("organization: no registered user with this email")
	// ErrForbidden - вызывающему это действие запрещено; причину добавляет место отказа.
	ErrForbidden = errors.New("organization: forbidden")
	// ErrNotOwner - передать компанию может только её владелец; это тоже ErrForbidden.
	ErrNotOwner = fmt.Errorf("only the owner transfers the company: %w", ErrForbidden)
)

// ValidationError - некорректное поле входных данных команды.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("organization: %s: %s", e.Field, e.Message)
}

// InvalidField возвращает некорректное поле и сообщение для слоя API.
func (e *ValidationError) InvalidField() (field, message string) {
	return e.Field, e.Message
}

// User - пользователь, обращавшийся к системе.
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	// Login - логин из токена: им автор записан в событиях, по нему подставляется имя.
	Login string `json:"login,omitempty"`
}

// Company - сеть точек одного владельца.
type Company struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	OwnerID string    `json:"owner_id"`
	Active  bool      `json:"active"`
}

// Point - точка: склад и кухня.
type Point struct {
	ID        uuid.UUID `json:"id"`
	CompanyID uuid.UUID `json:"company_id"`
	Name      string    `json:"name"`
	Active    bool      `json:"active"`
	// TelegramChatID - чат Telegram для уведомлений точки, если он задан.
	TelegramChatID *int64 `json:"telegram_chat_id,omitempty"`
}

// Staff - сотрудник точки.
type Staff struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Role   Role   `json:"role"`
}

// Membership - точка в профиле пользователя: кем он на ней является и что может.
type Membership struct {
	Point       Point
	CompanyName string
	// Owner - пользователь владеет компанией точки; Role при этом пуста.
	Owner       bool
	Role        Role
	Permissions []string
}

// Profile - компании пользователя и точки, на которых он работает.
type Profile struct {
	Companies []Company
	Points    []Membership
}

// Права, которых нет в ADR-006: они появились вместе с компаниями и точками.
const (
	PermissionCompanyView = "company.view"
	// PermissionCompanyManage есть только у владельца: компания, её точки и каталог.
	PermissionCompanyManage = "company.manage"
	PermissionPointView     = "point.view"
	PermissionStaffView     = "staff.view"
	PermissionStaffManage   = "staff.manage"
	// PermissionMenuManage - выключать и включать блюда компании на точке.
	PermissionMenuManage = "menu.manage"
	// PermissionNotificationsManage - задавать точке чат Telegram для уведомлений.
	PermissionNotificationsManage = "notifications.manage"
)

// staffCompanyPermissions - права сотрудника любой работающей точки в компании: видеть её
// и читать её каталог. catalog.view есть у каждой роли, поэтому роль здесь не важна.
var staffCompanyPermissions = []string{PermissionCompanyView, "catalog.view"}

// Права ролей на точке. Каталог компании меняет только владелец, поэтому
// products.manage и dishes.manage нет ни у одной роли.
var rolePermissions = map[Role][]string{
	RoleManager: {
		PermissionPointView, PermissionStaffView, PermissionStaffManage, PermissionMenuManage, PermissionNotificationsManage,
		"catalog.view", "deliveries.view", "deliveries.register", "deliveries.approve", "orders.view", "orders.cancel", "inventory.view",
		"inventory.edit", "kitchen.view", "analytics.view", "recommendations.view",
	},
	RoleApprover: {
		PermissionPointView, PermissionStaffView,
		"catalog.view", "deliveries.view", "deliveries.approve", "inventory.view",
	},
	RoleCook: {PermissionPointView, PermissionStaffView, "kitchen.view", "catalog.view"},
	RoleWaiter: {
		PermissionPointView, PermissionStaffView,
		"orders.accept", "orders.cancel", "orders.view", "kitchen.view", "catalog.view", "recommendations.view",
	},
}

// ownerPermissions - все права: владелец может всё в своей компании и на её точках.
var ownerPermissions = []string{
	PermissionCompanyView, PermissionCompanyManage, PermissionPointView, PermissionStaffView, PermissionStaffManage,
	PermissionMenuManage, PermissionNotificationsManage,
	"catalog.view", "deliveries.view", "deliveries.register", "deliveries.approve",
	"orders.view", "orders.accept", "orders.cancel", "inventory.view", "inventory.edit", "kitchen.view",
	"products.manage", "dishes.manage", "analytics.view", "recommendations.view", "generator.manage",
}

// permissions возвращает права пользователя на точке: владельца, роли или никакие.
func permissions(owner bool, role Role) []string {
	if owner {
		return slices.Clone(ownerPermissions)
	}
	return slices.Clone(rolePermissions[role])
}

func validRole(role Role) error {
	if _, known := rolePermissions[role]; !known {
		return &ValidationError{Field: "role", Message: "must be one of manager, approver, cook, waiter"}
	}
	return nil
}

// mayManage решает, может ли вызывающий назначить, сменить или снять перечисленные роли:
// владелец - любые, управляющий точки - любые, кроме управляющего. Проверяются и прежняя,
// и новая роль назначения: управляющий не снимает управляющего и не делает им другого.
func mayManage(owner bool, caller Role, roles ...Role) error {
	if owner {
		return nil
	}
	if caller != RoleManager || slices.Contains(roles, RoleManager) {
		return fmt.Errorf("only the owner manages managers: %w", ErrForbidden)
	}
	return nil
}

// normalizedName обрезает пробелы и отклоняет пустое название.
func normalizedName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", &ValidationError{Field: "name", Message: "must not be empty"}
	}
	return name, nil
}

// key - название или почта в виде, в котором проверяется совпадение: без учёта регистра и пробелов по краям.
func key(text string) string {
	return strings.ToLower(strings.TrimSpace(text))
}
