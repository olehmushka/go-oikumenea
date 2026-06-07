// Package transport implements the order module's generated Conjure OrderService interface: it
// translates the wire contract to/from the application service, assembles localized type `name` maps
// via the localization service (cross-module query — overview.md), and maps domain errors to Conjure
// SerializableErrors (D-Conjure). Generated code in internal/conjure is never hand-edited.
//
// A type `name` is a translatable label returned as a `locale -> text` map; orders/items reference
// their unit/person/position/rank/type by id and carry verbatim data (no maps).
//
// Authorization: orders are unit-scoped on issuing_unit_id (+ shadow gate). The two UNIT-keyed
// endpoints (create / list a unit's orders) gate precisely at that unit; the id-keyed endpoints
// (get/update/issue/revoke) and the person-scoped list carry no unit in the request, so they use the
// coarse "holds the permission anywhere (or is instance admin)" form pending the load-then-check +
// shadow-gate tightening (the shared M7/M8 follow-up across the id-keyed read/write surfaces). Type
// catalog management uses instance-scope permissions, which RequireAnywhere satisfies only from an
// instance-admin grant.
package transport

import (
	"context"
	"errors"
	"time"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	orderapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/order"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/internal/order/application"
	"github.com/olegamysk/go-oikumenea/internal/order/domain"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
	werror "github.com/palantir/witchcraft-go-error"
)

// typeEntity is the localization entity-type key for the translatable order-type name (D-i18n).
const typeEntity = "order_type"

// Service adapts *application.Service to the generated orderapi.OrderService interface, holding the
// localization service for `name` map assembly and the PEP enforcer for authorization.
type Service struct {
	app *application.Service
	loc *locapp.Service
	pep *pep.Enforcer
}

// NewService builds the transport adapter over the order application service, the localization
// service, and the PEP enforcer.
func NewService(app *application.Service, loc *locapp.Service, enforcer *pep.Enforcer) Service {
	return Service{app: app, loc: loc, pep: enforcer}
}

var _ orderapi.OrderService = Service{}

// ---------------------------------------------------------------- orders

func (s Service) CreateOrder(ctx context.Context, token bearertoken.Token, unitID string, req orderapi.CreateOrderRequest) (orderapi.Order, error) {
	if err := s.pep.Require(ctx, token, string(authzdomain.PermOrderCreate), unitID); err != nil {
		return orderapi.Order{}, err
	}
	created, err := s.app.CreateOrder(ctx, unitID, application.CreateOrderInput{
		Number:   derefOr(req.Number, ""),
		IssuedOn: derefOr(req.IssuedOn, ""),
		Items:    itemsFromInput(req.Items),
	})
	if err != nil {
		return orderapi.Order{}, s.mapError(ctx, err, errCtx{})
	}
	return toAPIOrder(created), nil
}

func (s Service) GetOrder(ctx context.Context, token bearertoken.Token, orderID string) (orderapi.Order, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermOrderRead)); err != nil {
		return orderapi.Order{}, err
	}
	o, err := s.app.GetOrder(ctx, orderID)
	if err != nil {
		return orderapi.Order{}, s.mapError(ctx, err, errCtx{orderID: orderID})
	}
	return toAPIOrder(o), nil
}

func (s Service) UpdateOrder(ctx context.Context, token bearertoken.Token, orderID string, req orderapi.UpdateOrderRequest) (orderapi.Order, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermOrderCreate)); err != nil {
		return orderapi.Order{}, err
	}
	var items *[]domain.OrderItem
	if req.Items != nil {
		converted := itemsFromInput(*req.Items)
		items = &converted
	}
	updated, err := s.app.UpdateOrder(ctx, orderID, application.UpdateOrderInput{
		Number:   req.Number,
		IssuedOn: req.IssuedOn,
		Items:    items,
	})
	if err != nil {
		return orderapi.Order{}, s.mapError(ctx, err, errCtx{orderID: orderID})
	}
	return toAPIOrder(updated), nil
}

func (s Service) IssueOrder(ctx context.Context, token bearertoken.Token, orderID string) (orderapi.Order, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermOrderIssue)); err != nil {
		return orderapi.Order{}, err
	}
	issued, err := s.app.IssueOrder(ctx, orderID)
	if err != nil {
		return orderapi.Order{}, s.mapError(ctx, err, errCtx{orderID: orderID})
	}
	return toAPIOrder(issued), nil
}

func (s Service) RevokeOrder(ctx context.Context, token bearertoken.Token, orderID string, req orderapi.RevokeOrderRequest) (orderapi.Order, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermOrderRevoke)); err != nil {
		return orderapi.Order{}, err
	}
	revoked, err := s.app.RevokeOrder(ctx, orderID, req.RevokingOrderId)
	if err != nil {
		return orderapi.Order{}, s.mapError(ctx, err, errCtx{orderID: orderID})
	}
	return toAPIOrder(revoked), nil
}

func (s Service) ListUnitOrders(ctx context.Context, token bearertoken.Token, unitID string, pageSize *int, pageToken *string) (orderapi.OrderPage, error) {
	if err := s.pep.Require(ctx, token, string(authzdomain.PermOrderRead), unitID); err != nil {
		return orderapi.OrderPage{}, err
	}
	page, err := s.app.ListOrdersByUnit(ctx, unitID, derefOr(pageSize, 0), derefOr(pageToken, ""))
	if err != nil {
		return orderapi.OrderPage{}, s.mapError(ctx, err, errCtx{})
	}
	return toAPIOrderPage(page), nil
}

func (s Service) ListPersonOrders(ctx context.Context, token bearertoken.Token, personID string, pageSize *int, pageToken *string) (orderapi.OrderPage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermOrderRead)); err != nil {
		return orderapi.OrderPage{}, err
	}
	page, err := s.app.ListOrdersByPerson(ctx, personID, derefOr(pageSize, 0), derefOr(pageToken, ""))
	if err != nil {
		return orderapi.OrderPage{}, s.mapError(ctx, err, errCtx{})
	}
	return toAPIOrderPage(page), nil
}

// ---------------------------------------------------------------- order types

func (s Service) ListOrderTypes(ctx context.Context, token bearertoken.Token) ([]orderapi.OrderType, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermOrderTypeRead)); err != nil {
		return nil, err
	}
	types, err := s.app.ListOrderTypes(ctx)
	if err != nil {
		return nil, s.mapError(ctx, err, errCtx{})
	}
	defaults := make(map[string]string, len(types))
	for _, t := range types {
		defaults[t.ID] = t.Name
	}
	names, err := s.loc.NamesByID(ctx, typeEntity, defaults)
	if err != nil {
		return nil, s.mapError(ctx, err, errCtx{})
	}
	out := make([]orderapi.OrderType, 0, len(types))
	for _, t := range types {
		out = append(out, toAPIOrderType(t, names[t.ID]))
	}
	return out, nil
}

func (s Service) CreateOrderType(ctx context.Context, token bearertoken.Token, req orderapi.CreateOrderTypeRequest) (orderapi.OrderType, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermOrderTypeManage)); err != nil {
		return orderapi.OrderType{}, err
	}
	created, err := s.app.CreateOrderType(ctx, domain.OrderType{
		Code:      req.Code,
		Name:      req.Name,
		Category:  domain.OrderCategory(req.Category),
		Effect:    domain.OrderEffect(req.Effect),
		SortOrder: req.SortOrder,
	})
	if err != nil {
		return orderapi.OrderType{}, s.mapError(ctx, err, errCtx{})
	}
	return s.orderTypeToAPI(ctx, created)
}

func (s Service) UpdateOrderType(ctx context.Context, token bearertoken.Token, typeID string, req orderapi.UpdateOrderTypeRequest) (orderapi.OrderType, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermOrderTypeManage)); err != nil {
		return orderapi.OrderType{}, err
	}
	updated, err := s.app.UpdateOrderType(ctx, typeID, domain.OrderTypePatch{Name: req.Name, Status: req.Status, SortOrder: req.SortOrder})
	if err != nil {
		return orderapi.OrderType{}, s.mapError(ctx, err, errCtx{typeID: typeID})
	}
	return s.orderTypeToAPI(ctx, updated)
}

// ---------------------------------------------------------------- response assembly

func (s Service) orderTypeToAPI(ctx context.Context, t domain.OrderType) (orderapi.OrderType, error) {
	names, err := s.loc.NamesByID(ctx, typeEntity, map[string]string{t.ID: t.Name})
	if err != nil {
		return orderapi.OrderType{}, err
	}
	return toAPIOrderType(t, names[t.ID]), nil
}

func itemsFromInput(in []orderapi.OrderItemInput) []domain.OrderItem {
	out := make([]domain.OrderItem, 0, len(in))
	for _, it := range in {
		out = append(out, domain.OrderItem{
			TypeID:        it.TypeId,
			PersonID:      it.PersonId,
			UnitID:        derefOr(it.UnitId, ""),
			PositionID:    derefOr(it.PositionId, ""),
			RankID:        derefOr(it.RankId, ""),
			EffectiveFrom: derefOr(it.EffectiveFrom, ""),
			EffectiveTo:   derefOr(it.EffectiveTo, ""),
			Note:          derefOr(it.Note, ""),
		})
	}
	return out
}

func toAPIOrder(o domain.Order) orderapi.Order {
	items := make([]orderapi.OrderItem, 0, len(o.Items))
	for _, it := range o.Items {
		items = append(items, toAPIOrderItem(it))
	}
	return orderapi.Order{
		Id:               o.ID,
		Number:           strPtrOrNil(o.Number),
		IssuedOn:         strPtrOrNil(o.IssuedOn),
		IssuingUnitId:    o.IssuingUnitID,
		Status:           string(o.Status),
		RevokedByOrderId: strPtrOrNil(o.RevokedByOrderID),
		RevokedAt:        dtPtr(o.RevokedAt),
		Items:            items,
		CreatedAt:        datetime.DateTime(o.CreatedAt),
		UpdatedAt:        datetime.DateTime(o.UpdatedAt),
	}
}

func toAPIOrderItem(it domain.OrderItem) orderapi.OrderItem {
	return orderapi.OrderItem{
		Id:            it.ID,
		OrderId:       it.OrderID,
		TypeId:        it.TypeID,
		PersonId:      it.PersonID,
		UnitId:        strPtrOrNil(it.UnitID),
		PositionId:    strPtrOrNil(it.PositionID),
		RankId:        strPtrOrNil(it.RankID),
		EffectiveFrom: strPtrOrNil(it.EffectiveFrom),
		EffectiveTo:   strPtrOrNil(it.EffectiveTo),
		Note:          strPtrOrNil(it.Note),
		CreatedAt:     datetime.DateTime(it.CreatedAt),
		UpdatedAt:     datetime.DateTime(it.UpdatedAt),
	}
}

func toAPIOrderPage(p application.OrderPage) orderapi.OrderPage {
	orders := make([]orderapi.Order, 0, len(p.Orders))
	for _, o := range p.Orders {
		orders = append(orders, toAPIOrder(o))
	}
	return orderapi.OrderPage{Orders: orders, NextPageToken: strPtrOrNil(p.NextPageToken)}
}

func toAPIOrderType(t domain.OrderType, name map[string]string) orderapi.OrderType {
	return orderapi.OrderType{
		Id:        t.ID,
		Code:      t.Code,
		Name:      name,
		Category:  string(t.Category),
		Effect:    string(t.Effect),
		Status:    string(t.Status),
		SortOrder: t.SortOrder,
		CreatedAt: datetime.DateTime(t.CreatedAt),
		UpdatedAt: datetime.DateTime(t.UpdatedAt),
	}
}

// ---------------------------------------------------------------- error mapping

// errCtx carries the identifiers an endpoint can name in a Conjure error (only the relevant fields are
// set per call).
type errCtx struct {
	orderID string
	typeID  string
}

// mapError translates domain/application errors into the Conjure SerializableError contract. An
// auto-applied effect that violated a target module's invariant on issue arrives wrapped in
// ErrEffectFailed and surfaces as Order:OrderEffectFailed (FAILED_PRECONDITION) carrying the safe
// underlying message; the order stayed draft.
func (s Service) mapError(ctx context.Context, err error, c errCtx) error {
	switch {
	case errors.Is(err, domain.ErrOrderNotFound):
		return orderapi.NewOrderNotFound(c.orderID)
	case errors.Is(err, domain.ErrOrderConflict):
		return orderapi.NewOrderConflict("this issuing unit already has an order with this number")
	case errors.Is(err, domain.ErrAlreadyIssued):
		return orderapi.NewOrderAlreadyIssued(c.orderID)
	case errors.Is(err, domain.ErrNotIssued):
		return orderapi.NewOrderNotIssued(c.orderID)
	case errors.Is(err, domain.ErrOrderTypeNotFound):
		return orderapi.NewOrderTypeNotFound(c.typeID)
	case errors.Is(err, domain.ErrOrderTypeConflict):
		return orderapi.NewOrderTypeConflict("an order type with this code already exists")
	case errors.Is(err, domain.ErrEffectFailed):
		return orderapi.NewOrderEffectFailed(err.Error())
	case errors.Is(err, domain.ErrUnknownType):
		return orderapi.NewOrderInvalid("order type does not exist")
	case errors.Is(err, domain.ErrUnknownPerson):
		return orderapi.NewOrderInvalid("person does not exist")
	case errors.Is(err, domain.ErrUnknownUnit):
		return orderapi.NewOrderInvalid("unit does not exist")
	case errors.Is(err, domain.ErrUnknownPosition):
		return orderapi.NewOrderInvalid("position does not exist")
	case errors.Is(err, domain.ErrUnknownRank):
		return orderapi.NewOrderInvalid("rank does not exist")
	case errors.Is(err, domain.ErrOrderInvalid):
		return orderapi.NewOrderInvalid(err.Error())
	default:
		return werror.WrapWithContextParams(ctx, err, "order request failed")
	}
}

// ---------------------------------------------------------------- value helpers

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func dtPtr(t *time.Time) *datetime.DateTime {
	if t == nil {
		return nil
	}
	d := datetime.DateTime(*t)
	return &d
}

func derefOr[T any](p *T, fallback T) T {
	if p == nil {
		return fallback
	}
	return *p
}
