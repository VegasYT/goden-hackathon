//go:build integration

package api_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"goden/backend/internal/api/apitest"
	"goden/backend/internal/api/oas"
)

func newSupplier(t *testing.T, owner *oas.Client, company *oas.Company, name string, points ...*oas.Point) *oas.Supplier {
	t.Helper()
	ids := make([]uuid.UUID, len(points))
	for i, point := range points {
		ids[i] = point.ID
	}
	supplier, err := owner.CreateSupplier(ctx, &oas.NewSupplier{Name: name, PointIds: ids}, oas.CreateSupplierParams{CompanyId: company.ID})
	require.NoError(t, err)
	return supplier
}

func supplierDelivery(product *oas.Product) *oas.NewDelivery {
	return &oas.NewDelivery{Number: "НК-7", DeliveredAt: oct(10), Items: []oas.NewDeliveryItem{deliveryItem(product, 1000, oct(15))}}
}

// Владелец заводит поставщика и выдаёт ключ. С ключом поставщик видит себя и продукты компании без блюд
// и регистрирует поставки только на своих работающих точках; автор поставки - поставщик под своим названием.
func TestSupplierRegistersDeliveriesWithKey(t *testing.T) {
	s := apitest.New(t)
	k := newKitchen(t, s)
	archived := ownProduct(t, k.network, "Молоко старое")
	_, err := k.owner.ArchiveProduct(ctx, oas.ArchiveProductParams{CompanyId: k.company.ID, ProductId: archived.ID})
	require.NoError(t, err)

	created := newSupplier(t, k.owner, k.company, "Овощебаза Юг", k.a, k.b)
	require.True(t, strings.HasPrefix(created.Key.Value, "gdn_"), "the key is returned on creation")
	require.True(t, created.KeyActive)
	listed, err := k.owner.ListSuppliers(ctx, oas.ListSuppliersParams{CompanyId: k.company.ID})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.False(t, listed[0].Key.Set, "the list never returns the key")
	require.ElementsMatch(t, []uuid.UUID{k.a.ID, k.b.ID}, listed[0].PointIds)

	supplier := s.WithToken(t, created.Key.Value)
	_, err = k.owner.DisablePoint(ctx, oas.DisablePointParams{PointId: k.b.ID})
	require.NoError(t, err)
	self, err := supplier.GetSupplierSelf(ctx)
	require.NoError(t, err)
	require.Equal(t, "Овощебаза Юг", self.Name)
	require.Equal(t, k.company.ID, self.CompanyID)
	require.Equal(t, []oas.SupplierPoint{{ID: k.a.ID, Name: k.a.Name}}, self.Points, "a disabled point is not listed")

	products, err := supplier.ListSupplierProducts(ctx, oas.ListSupplierProductsParams{})
	require.NoError(t, err)
	require.Len(t, products.Items, 1, "archived products are not listed")
	require.Equal(t, k.cream.ID, products.Items[0].ID)
	require.False(t, products.Items[0].Dishes.Set, "the supplier does not see the use in dishes")
	require.False(t, products.Items[0].UsedIn != nil, "the supplier does not see the use in dishes")

	delivery, err := supplier.RegisterDelivery(ctx, supplierDelivery(k.cream), oas.RegisterDeliveryParams{PointId: k.a.ID})
	require.NoError(t, err)
	require.Equal(t, oas.DeliveryStatusRegistered, delivery.Status)

	// поставщика переименовали: история называет его новым названием
	_, err = k.owner.UpdateSupplier(ctx, &oas.NewSupplier{Name: "Юг-Овощи", PointIds: []uuid.UUID{k.a.ID}},
		oas.UpdateSupplierParams{CompanyId: k.company.ID, SupplierId: created.ID})
	require.NoError(t, err)
	history, err := k.owner.GetDeliveryHistory(ctx, oas.GetDeliveryHistoryParams{PointId: k.a.ID, DeliveryId: delivery.ID})
	require.NoError(t, err)
	author := history.Items[0].Actor
	require.Equal(t, oas.ActorTypeService, author.Type)
	require.Equal(t, "supplier:"+created.ID.String(), author.ID)
	require.Equal(t, "Юг-Овощи", author.Name.Value)

	// только регистрация и только на своих работающих точках
	_, err = supplier.RegisterDelivery(ctx, supplierDelivery(k.cream), oas.RegisterDeliveryParams{PointId: k.b.ID})
	requireForbidden(t, err, "deliveries.register")
	_, err = supplier.GetDelivery(ctx, oas.GetDeliveryParams{PointId: k.a.ID, DeliveryId: delivery.ID})
	requireForbidden(t, err, "deliveries.view")
	_, err = supplier.ApproveDelivery(ctx, acceptAllItems(delivery), oas.ApproveDeliveryParams{PointId: k.a.ID, DeliveryId: delivery.ID})
	requireForbidden(t, err, "deliveries.approve")
	other := newNetwork(t, s)
	_, err = supplier.RegisterDelivery(ctx, supplierDelivery(k.cream), oas.RegisterDeliveryParams{PointId: other.a.ID})
	requireForbidden(t, err, "deliveries.register")

	// продукт другой компании
	foreign := ownProduct(t, other, "Сливки чужие")
	_, err = supplier.RegisterDelivery(ctx, supplierDelivery(foreign), oas.RegisterDeliveryParams{PointId: k.a.ID})
	requireProblem(t, err, http.StatusUnprocessableEntity)

	// операции поставщика не открываются токеном пользователя
	_, err = k.owner.ListSupplierProducts(ctx, oas.ListSupplierProductsParams{})
	requireForbidden(t, err, "supplier.access")

	// компанию отключили - поставщику недоступно ничего
	_, err = k.owner.DisableCompany(ctx, oas.DisableCompanyParams{CompanyId: k.company.ID})
	require.NoError(t, err)
	_, err = supplier.ListSupplierProducts(ctx, oas.ListSupplierProductsParams{})
	requireProblem(t, err, http.StatusForbidden)
	_, err = supplier.RegisterDelivery(ctx, supplierDelivery(k.cream), oas.RegisterDeliveryParams{PointId: k.a.ID})
	requireForbidden(t, err, "deliveries.register")
	_, err = k.owner.UpdateSupplier(ctx, &oas.NewSupplier{Name: "Юг", PointIds: []uuid.UUID{k.a.ID}},
		oas.UpdateSupplierParams{CompanyId: k.company.ID, SupplierId: created.ID})
	requireProblem(t, err, http.StatusConflict)
}

// Перевыпуск и отзыв лишают прежний ключ силы сразу.
func TestSupplierKeyReissueAndRevoke(t *testing.T) {
	s := apitest.New(t)
	n := newNetwork(t, s)
	created := newSupplier(t, n.owner, n.company, "Молочная ферма", n.a)
	old := s.WithToken(t, created.Key.Value)
	_, err := old.GetSupplierSelf(ctx)
	require.NoError(t, err)

	reissued, err := n.owner.IssueSupplierKey(ctx, oas.IssueSupplierKeyParams{CompanyId: n.company.ID, SupplierId: created.ID})
	require.NoError(t, err)
	require.NotEqual(t, created.Key.Value, reissued.Key.Value)
	_, err = old.GetSupplierSelf(ctx)
	requireProblem(t, err, http.StatusUnauthorized)
	fresh := s.WithToken(t, reissued.Key.Value)
	_, err = fresh.GetSupplierSelf(ctx)
	require.NoError(t, err)

	revoked, err := n.owner.RevokeSupplierKey(ctx, oas.RevokeSupplierKeyParams{CompanyId: n.company.ID, SupplierId: created.ID})
	require.NoError(t, err)
	require.False(t, revoked.KeyActive)
	require.False(t, revoked.Key.Set)
	_, err = fresh.GetSupplierSelf(ctx)
	requireProblem(t, err, http.StatusUnauthorized)

	// ключ с префиксом, которого никто не выпускал
	_, err = s.WithToken(t, "gdn_unknown").GetSupplierSelf(ctx)
	requireProblem(t, err, http.StatusUnauthorized)
}

// Поставщиками управляет только владелец; название уникально, точки - работающие точки компании.
func TestSupplierValidation(t *testing.T) {
	s := apitest.New(t)
	n := newNetwork(t, s)
	manager, _ := hire(t, s, n.owner, n.a, "manager", oas.RoleManager)
	newSupplier(t, n.owner, n.company, "Овощебаза Юг", n.a)

	_, err := manager.ListSuppliers(ctx, oas.ListSuppliersParams{CompanyId: n.company.ID})
	requireForbidden(t, err, "company.manage")

	create := func(name string, points ...uuid.UUID) error {
		_, err := n.owner.CreateSupplier(ctx, &oas.NewSupplier{Name: name, PointIds: points}, oas.CreateSupplierParams{CompanyId: n.company.ID})
		return err
	}
	problem := requireProblem(t, create("овощебаза юг", n.a.ID), http.StatusConflict)
	require.Equal(t, "name", problem.Errors[0].Field)
	problem = requireProblem(t, create("Без точек"), http.StatusUnprocessableEntity)
	require.Equal(t, "point_ids", problem.Errors[0].Field)

	other := newNetwork(t, s)
	problem = requireProblem(t, create("Чужая точка", other.a.ID), http.StatusUnprocessableEntity)
	require.Equal(t, "point_ids", problem.Errors[0].Field)
	_, err = n.owner.DisablePoint(ctx, oas.DisablePointParams{PointId: n.b.ID})
	require.NoError(t, err)
	problem = requireProblem(t, create("Отключённая точка", n.b.ID), http.StatusUnprocessableEntity)
	require.Equal(t, "point_ids", problem.Errors[0].Field)

	// поставщик другой компании в адресе не находится
	foreign := newSupplier(t, other.owner, other.company, "Чужой", other.a)
	_, err = n.owner.IssueSupplierKey(ctx, oas.IssueSupplierKeyParams{CompanyId: n.company.ID, SupplierId: foreign.ID})
	requireProblem(t, err, http.StatusNotFound)
}
