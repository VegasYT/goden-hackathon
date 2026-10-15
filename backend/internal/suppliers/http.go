package suppliers

import (
	"context"

	"github.com/google/uuid"

	"goden/backend/internal/api/oas"
	"goden/backend/internal/catalog"
	"goden/backend/internal/platform/httpx"
)

// HTTP реализует операции поставщиков из api/openapi.yaml.
type HTTP struct {
	svc     *Service
	catalog *catalog.Service
}

func NewHTTP(svc *Service, products *catalog.Service) *HTTP {
	return &HTTP{svc: svc, catalog: products}
}

// ListSuppliers реализует GET /companies/{companyId}/suppliers.
func (h *HTTP) ListSuppliers(ctx context.Context, params oas.ListSuppliersParams) ([]oas.Supplier, error) {
	suppliers, err := h.svc.List(ctx, params.CompanyId)
	if err != nil {
		return nil, err
	}
	out := make([]oas.Supplier, len(suppliers))
	for i, supplier := range suppliers {
		out[i] = *supplierResponse(supplier, "")
	}
	return out, nil
}

// CreateSupplier реализует POST /companies/{companyId}/suppliers.
func (h *HTTP) CreateSupplier(ctx context.Context, req *oas.NewSupplier, params oas.CreateSupplierParams) (*oas.Supplier, error) {
	supplier, key, err := h.svc.Create(ctx, params.CompanyId, Input{Name: req.Name, PointIDs: req.PointIds})
	if err != nil {
		return nil, err
	}
	return supplierResponse(supplier, key), nil
}

// UpdateSupplier реализует PUT /companies/{companyId}/suppliers/{supplierId}.
func (h *HTTP) UpdateSupplier(ctx context.Context, req *oas.NewSupplier, params oas.UpdateSupplierParams) (*oas.Supplier, error) {
	supplier, err := h.svc.Update(ctx, params.CompanyId, params.SupplierId, Input{Name: req.Name, PointIDs: req.PointIds})
	if err != nil {
		return nil, err
	}
	return supplierResponse(supplier, ""), nil
}

// IssueSupplierKey реализует POST /companies/{companyId}/suppliers/{supplierId}/key.
func (h *HTTP) IssueSupplierKey(ctx context.Context, params oas.IssueSupplierKeyParams) (*oas.Supplier, error) {
	supplier, key, err := h.svc.IssueKey(ctx, params.CompanyId, params.SupplierId)
	if err != nil {
		return nil, err
	}
	return supplierResponse(supplier, key), nil
}

// RevokeSupplierKey реализует DELETE /companies/{companyId}/suppliers/{supplierId}/key.
func (h *HTTP) RevokeSupplierKey(ctx context.Context, params oas.RevokeSupplierKeyParams) (*oas.Supplier, error) {
	supplier, err := h.svc.RevokeKey(ctx, params.CompanyId, params.SupplierId)
	if err != nil {
		return nil, err
	}
	return supplierResponse(supplier, ""), nil
}

// GetSupplierSelf реализует GET /supplier.
func (h *HTTP) GetSupplierSelf(ctx context.Context) (*oas.SupplierSelf, error) {
	id, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	supplier, company, points, err := h.svc.Self(ctx, id)
	if err != nil {
		return nil, err
	}
	out := &oas.SupplierSelf{
		ID: supplier.ID, Name: supplier.Name, CompanyID: company.ID, CompanyName: company.Name,
		Points: make([]oas.SupplierPoint, len(points)),
	}
	for i, point := range points {
		out.Points[i] = oas.SupplierPoint{ID: point.ID, Name: point.Name}
	}
	return out, nil
}

// ListSupplierProducts реализует GET /supplier/products: продукты без сведений о блюдах.
func (h *HTTP) ListSupplierProducts(ctx context.Context, params oas.ListSupplierProductsParams) (*oas.ProductPage, error) {
	id, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	company, err := h.svc.Company(ctx, id)
	if err != nil {
		return nil, err
	}
	// значения по умолчанию подставляет сгенерированный разбор запроса
	products, total, err := h.catalog.ListProducts(ctx, company.ID,
		catalog.ListFilter{Query: params.Q.Or("")}, catalog.Page{Limit: params.Limit.Value, Offset: params.Offset.Value})
	if err != nil {
		return nil, err
	}
	page := &oas.ProductPage{Items: make([]oas.Product, len(products)), Total: total}
	for i, product := range products {
		page.Items[i] = *catalog.ProductResponse(product.Product)
	}
	return page, nil
}

func caller(ctx context.Context) (uuid.UUID, error) {
	actor, _ := httpx.Actor(ctx)
	supplier, ok := FromActor(actor)
	// право supplier.access выдаётся только ключу поставщика; другой клиент поставщиком не является
	if !ok {
		return supplier, ErrNotFound
	}
	return supplier, nil
}

func supplierResponse(s Supplier, key string) *oas.Supplier {
	out := &oas.Supplier{
		ID: s.ID, CompanyID: s.CompanyID, Name: s.Name, PointIds: s.PointIDs, KeyActive: s.KeyActive, CreatedAt: s.CreatedAt,
	}
	if key != "" {
		out.Key = oas.NewOptString(key)
	}
	return out
}
