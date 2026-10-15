package organization_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/organization"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/eventstore/estest"
)

var (
	companyID = uuid.MustParse("0198c0de-0000-7000-8000-000000000011")
	pointID   = uuid.MustParse("0198c0de-0000-7000-8000-000000000012")
)

// chatID - чат Telegram: у групп идентификатор отрицательный
var chatID int64 = -1001234567890

const userID = "5f0c1b0e-8a53-4c0f-9d0a-2f6f3b1f7a01"

func TestEventContract(t *testing.T) {
	samples := []eventstore.Payload{
		organization.CompanyCreated{CompanyID: companyID, Name: "Пиццерии", OwnerID: userID},
		organization.CompanyRenamed{CompanyID: companyID, Name: "Пиццерии Севера"},
		organization.CompanyOwnerChanged{CompanyID: companyID, OwnerID: userID},
		organization.CompanyDisabled{CompanyID: companyID},
		organization.PointCreated{PointID: pointID, CompanyID: companyID, Name: "Точка на Ленина"},
		organization.PointRenamed{PointID: pointID, Name: "Точка на Мира"},
		organization.PointDisabled{PointID: pointID},
		organization.PointTelegramChatChanged{PointID: pointID, ChatID: &chatID},
		organization.StaffAssigned{PointID: pointID, UserID: userID, Role: organization.RoleCook},
		organization.StaffRoleChanged{PointID: pointID, UserID: userID, Role: organization.RoleManager},
		organization.StaffRemoved{PointID: pointID, UserID: userID},
	}
	for _, sample := range samples {
		t.Run(sample.EventType(), func(t *testing.T) { estest.Golden(t, sample) })
	}
}

func TestStreamNames(t *testing.T) {
	require.Equal(t, "company-0198c0de-0000-7000-8000-000000000011", organization.CompanyStream(companyID))
	require.Equal(t, "point-0198c0de-0000-7000-8000-000000000012", organization.PointStream(pointID))
}

func TestRoleIsClosedSet(t *testing.T) {
	_, err := eventstore.Decode("StaffAssigned", 1, json.RawMessage(`{"role":"admin"}`))

	require.ErrorContains(t, err, "admin")
}
