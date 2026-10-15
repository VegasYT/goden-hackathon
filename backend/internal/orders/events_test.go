package orders_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/orders"
	"goden/backend/internal/organization"
	"goden/backend/internal/platform/eventstore"
	"goden/backend/internal/platform/eventstore/estest"
)

var (
	orderID  = uuid.MustParse("0198c0de-0000-7000-8000-000000000001")
	dishID   = uuid.MustParse("0198c0de-0000-7000-8000-000000000002")
	cream    = uuid.MustParse("0198c0de-0000-7000-8000-000000000003")
	bacon    = uuid.MustParse("0198c0de-0000-7000-8000-000000000004")
	operator = eventstore.Actor{Type: eventstore.ActorUser, ID: "7b2d9c10-operator"}
	items    = []orders.OrderItem{{DishID: dishID, Portions: 2}}

	// point - точка, на которой тесты пакета оформляют заказы; её компания - companyID
	companyID = uuid.MustParse("0198c0de-0000-7000-8000-000000000011")
	point     = organization.Point{ID: uuid.MustParse("0198c0de-0000-7000-8000-000000000012"), CompanyID: companyID, Active: true}
)

func TestEventContract(t *testing.T) {
	samples := []eventstore.Payload{
		orders.OrderAccepted{PointID: point.ID, OrderID: orderID, AcceptedBy: operator, Items: items},
		orders.OrderRejected{PointID: point.ID, OrderID: orderID, RejectedBy: operator, Items: items, Shortages: []orders.Shortage{
			{ProductID: cream, RequiredQuantity: 160, AvailableQuantity: 50},
			{ProductID: bacon, RequiredQuantity: 120, AvailableQuantity: 0},
		}},
		orders.OrderCancelled{PointID: point.ID, OrderID: orderID, CancelledBy: operator, Comment: "гость ушёл"},
	}
	for _, sample := range samples {
		t.Run(sample.EventType(), func(t *testing.T) { estest.Golden(t, sample) })
	}
}

func TestStreamName(t *testing.T) {
	require.Equal(t, "order-0198c0de-0000-7000-8000-000000000001", orders.OrderStream(orderID))
}
