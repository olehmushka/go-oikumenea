// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package transport implements the generated vehicleapi.VehicleService (D-Vehicles, M26). It PEP-gates
// each op (vehicle entities are instance-global external reference data, so reads/writes are satisfied
// anywhere), assembles translatable catalog labels (type/brand/model/plate-type names) as locale->text
// maps via the localization service, resolves best-effort default-locale display labels for vehicles
// and registrations (type/brand/model names, company owner names, plate-region names), and maps domain
// sentinels to the Conjure Vehicle:* SerializableErrors. Generated code is never hand-edited.
package transport

import (
	"context"
	"errors"
	"time"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/authorization/pep"
	vehicleapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/vehicle"
	locapp "github.com/olegamysk/go-oikumenea/internal/localization/application"
	"github.com/olegamysk/go-oikumenea/internal/vehicle/application"
	"github.com/olegamysk/go-oikumenea/internal/vehicle/domain"
	"github.com/olegamysk/go-oikumenea/pkg/listing"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
	werror "github.com/palantir/witchcraft-go-error"
)

// i18n entity types the translatable catalog names are stored under (localization store).
const (
	entType       = "vehicle_type"
	entBrand      = "vehicle_brand"
	entModel      = "vehicle_model"
	entNumberType = "vehicle_registration_number_type"
)

const (
	readPerm    = string(authzdomain.PermVehicleRead)
	managePerm  = string(authzdomain.PermVehicleManage)
	catalogPerm = string(authzdomain.PermVehicleCatalogManage)
	// The vehicle↔owner registration link carries its own read code (D-LinkPermissions): vehicle.read
	// lists the vehicles, this discloses WHO they are registered to. Same code gates the registered_to arm.
	registrationReadPerm = string(authzdomain.PermVehicleRegistrationRead)
)

// VehicleService adapts *application.Service to the generated vehicleapi.VehicleService interface.
type VehicleService struct {
	app *application.Service
	loc *locapp.Service
	pep *pep.Enforcer
}

// NewService builds the transport adapter over the vehicle application service, the localization
// service (catalog name maps), and the PEP enforcer.
func NewService(app *application.Service, loc *locapp.Service, enforcer *pep.Enforcer) VehicleService {
	return VehicleService{app: app, loc: loc, pep: enforcer}
}

var _ vehicleapi.VehicleService = VehicleService{}

// ============================ catalogs ============================

func (s VehicleService) ListVehicleTypes(ctx context.Context, token bearertoken.Token) (vehicleapi.VehicleTypeList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return vehicleapi.VehicleTypeList{}, err
	}
	rows, err := s.app.ListVehicleTypes(ctx)
	if err != nil {
		return vehicleapi.VehicleTypeList{}, s.mapError(ctx, err)
	}
	defaults := make(map[string]string, len(rows))
	for _, t := range rows {
		defaults[t.ID] = t.Name
	}
	names, err := s.loc.NamesByID(ctx, entType, defaults)
	if err != nil {
		return vehicleapi.VehicleTypeList{}, s.mapError(ctx, err)
	}
	out := make([]vehicleapi.VehicleType, 0, len(rows))
	for _, t := range rows {
		out = append(out, vehicleTypeAPI(t, names[t.ID]))
	}
	return vehicleapi.VehicleTypeList{Types: out}, nil
}

func (s VehicleService) UpsertVehicleType(ctx context.Context, token bearertoken.Token, req vehicleapi.UpsertVehicleTypeRequest) (vehicleapi.VehicleType, error) {
	if err := s.pep.RequireAnywhere(ctx, token, catalogPerm); err != nil {
		return vehicleapi.VehicleType{}, err
	}
	t, err := s.app.UpsertVehicleType(ctx, req.Code, req.Name, req.ParentId, req.SortOrder)
	if err != nil {
		return vehicleapi.VehicleType{}, s.mapError(ctx, err)
	}
	name, err := s.nameMap(ctx, entType, t.ID, t.Name)
	if err != nil {
		return vehicleapi.VehicleType{}, s.mapError(ctx, err)
	}
	return vehicleTypeAPI(t, name), nil
}

func (s VehicleService) ListBrands(ctx context.Context, token bearertoken.Token, query *string) (vehicleapi.BrandList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return vehicleapi.BrandList{}, err
	}
	rows, err := s.app.ListBrands(ctx, strOr(query))
	if err != nil {
		return vehicleapi.BrandList{}, s.mapError(ctx, err)
	}
	defaults := make(map[string]string, len(rows))
	for _, b := range rows {
		defaults[b.ID] = b.Name
	}
	names, err := s.loc.NamesByID(ctx, entBrand, defaults)
	if err != nil {
		return vehicleapi.BrandList{}, s.mapError(ctx, err)
	}
	out := make([]vehicleapi.Brand, 0, len(rows))
	for _, b := range rows {
		out = append(out, brandAPI(b, names[b.ID]))
	}
	return vehicleapi.BrandList{Brands: out}, nil
}

func (s VehicleService) UpsertBrand(ctx context.Context, token bearertoken.Token, req vehicleapi.UpsertBrandRequest) (vehicleapi.Brand, error) {
	if err := s.pep.RequireAnywhere(ctx, token, catalogPerm); err != nil {
		return vehicleapi.Brand{}, err
	}
	b, err := s.app.UpsertBrand(ctx, req.Code, req.Name, req.CountryId, req.SortOrder)
	if err != nil {
		return vehicleapi.Brand{}, s.mapError(ctx, err)
	}
	name, err := s.nameMap(ctx, entBrand, b.ID, b.Name)
	if err != nil {
		return vehicleapi.Brand{}, s.mapError(ctx, err)
	}
	return brandAPI(b, name), nil
}

func (s VehicleService) ListModels(ctx context.Context, token bearertoken.Token, brandID string) (vehicleapi.ModelList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return vehicleapi.ModelList{}, err
	}
	rows, err := s.app.ListModelsByBrand(ctx, brandID)
	if err != nil {
		return vehicleapi.ModelList{}, s.mapError(ctx, err)
	}
	defaults := make(map[string]string, len(rows))
	for _, m := range rows {
		defaults[m.ID] = m.Name
	}
	names, err := s.loc.NamesByID(ctx, entModel, defaults)
	if err != nil {
		return vehicleapi.ModelList{}, s.mapError(ctx, err)
	}
	out := make([]vehicleapi.Model, 0, len(rows))
	for _, m := range rows {
		out = append(out, modelAPI(m, names[m.ID]))
	}
	return vehicleapi.ModelList{Models: out}, nil
}

func (s VehicleService) UpsertModel(ctx context.Context, token bearertoken.Token, brandID string, req vehicleapi.UpsertModelRequest) (vehicleapi.Model, error) {
	if err := s.pep.RequireAnywhere(ctx, token, catalogPerm); err != nil {
		return vehicleapi.Model{}, err
	}
	m, err := s.app.UpsertModel(ctx, brandID, req.Code, req.Name, req.Generation, req.ManufactureStart, req.ManufactureEnd, req.SortOrder)
	if err != nil {
		return vehicleapi.Model{}, s.mapError(ctx, err)
	}
	name, err := s.nameMap(ctx, entModel, m.ID, m.Name)
	if err != nil {
		return vehicleapi.Model{}, s.mapError(ctx, err)
	}
	return modelAPI(m, name), nil
}

func (s VehicleService) ListRegistrationNumberTypes(ctx context.Context, token bearertoken.Token) (vehicleapi.RegistrationNumberTypeList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return vehicleapi.RegistrationNumberTypeList{}, err
	}
	rows, err := s.app.ListNumberTypes(ctx)
	if err != nil {
		return vehicleapi.RegistrationNumberTypeList{}, s.mapError(ctx, err)
	}
	defaults := make(map[string]string, len(rows))
	for _, n := range rows {
		defaults[n.ID] = n.Name
	}
	names, err := s.loc.NamesByID(ctx, entNumberType, defaults)
	if err != nil {
		return vehicleapi.RegistrationNumberTypeList{}, s.mapError(ctx, err)
	}
	out := make([]vehicleapi.RegistrationNumberType, 0, len(rows))
	for _, n := range rows {
		out = append(out, numberTypeAPI(n, names[n.ID]))
	}
	return vehicleapi.RegistrationNumberTypeList{NumberTypes: out}, nil
}

func (s VehicleService) UpsertRegistrationNumberType(ctx context.Context, token bearertoken.Token, req vehicleapi.UpsertNumberTypeRequest) (vehicleapi.RegistrationNumberType, error) {
	if err := s.pep.RequireAnywhere(ctx, token, catalogPerm); err != nil {
		return vehicleapi.RegistrationNumberType{}, err
	}
	n, err := s.app.UpsertNumberType(ctx, req.Code, req.Name, req.SortOrder)
	if err != nil {
		return vehicleapi.RegistrationNumberType{}, s.mapError(ctx, err)
	}
	name, err := s.nameMap(ctx, entNumberType, n.ID, n.Name)
	if err != nil {
		return vehicleapi.RegistrationNumberType{}, s.mapError(ctx, err)
	}
	return numberTypeAPI(n, name), nil
}

// ============================ vehicles ============================

func (s VehicleService) CreateVehicle(ctx context.Context, token bearertoken.Token, req vehicleapi.CreateVehicleRequest) (vehicleapi.Vehicle, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return vehicleapi.Vehicle{}, err
	}
	v, err := s.app.CreateVehicle(ctx, domain.VehicleInput{
		TypeID:          req.TypeId,
		ModelID:         strOr(req.ModelId),
		VIN:             strOr(req.Vin),
		ColorID:         strOr(req.ColorId),
		ManufactureDate: strOr(req.ManufactureDate),
		Attributes:      strOr(req.Attributes),
	})
	if err != nil {
		return vehicleapi.Vehicle{}, s.mapError(ctx, err)
	}
	return s.vehicleWithLabels(ctx, v)
}

// ListVehicles pages the same set VehicleStats aggregates. The filter comes from the shared
// vehicleFilter helper (stats.go), so a chart segment and a list filter are the same act.
func (s VehicleService) ListVehicles(
	ctx context.Context,
	token bearertoken.Token,
	query *string,
	typeID *string,
	brandID *string,
	modelID *string,
	color *string,
	status *string,
	manufactureDateFrom *string,
	manufactureDateTo *string,
	registrationCountry *string,
	pageSize *int,
	pageToken *string,
) (vehicleapi.VehiclePage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return vehicleapi.VehiclePage{}, err
	}
	limit := pageSizeOr(pageSize)
	f := vehicleFilter(typeID, brandID, modelID, color, status, manufactureDateFrom, manufactureDateTo, registrationCountry)
	rows, err := s.app.ListVehicles(ctx, strOr(query), decodeToken(pageToken), f, limit)
	if err != nil {
		return vehicleapi.VehiclePage{}, s.mapError(ctx, err)
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = encodeToken(rows[len(rows)-1].ID)
	}
	out, err := s.vehiclesWithLabels(ctx, rows)
	if err != nil {
		return vehicleapi.VehiclePage{}, s.mapError(ctx, err)
	}
	page := vehicleapi.VehiclePage{Vehicles: out}
	if next != "" {
		page.NextPageToken = &next
	}
	return page, nil
}

func (s VehicleService) GetVehicle(ctx context.Context, token bearertoken.Token, vehicleID string) (vehicleapi.Vehicle, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return vehicleapi.Vehicle{}, err
	}
	v, err := s.app.GetVehicle(ctx, vehicleID)
	if err != nil {
		return vehicleapi.Vehicle{}, s.mapError(ctx, err)
	}
	return s.vehicleWithLabels(ctx, v)
}

func (s VehicleService) UpdateVehicle(ctx context.Context, token bearertoken.Token, vehicleID string, req vehicleapi.UpdateVehicleRequest) (vehicleapi.Vehicle, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return vehicleapi.Vehicle{}, err
	}
	v, err := s.app.UpdateVehicle(ctx, vehicleID, domain.VehicleUpdate{
		TypeID:          req.TypeId,
		ModelID:         req.ModelId,
		VIN:             req.Vin,
		ColorID:         req.ColorId,
		ManufactureDate: req.ManufactureDate,
		Attributes:      req.Attributes,
		Status:          req.Status,
	})
	if err != nil {
		return vehicleapi.Vehicle{}, s.mapError(ctx, err)
	}
	return s.vehicleWithLabels(ctx, v)
}

func (s VehicleService) DeleteVehicle(ctx context.Context, token bearertoken.Token, vehicleID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.DeleteVehicle(ctx, vehicleID))
}

// ============================ registrations ============================

func (s VehicleService) ListRegistrations(ctx context.Context, token bearertoken.Token, vehicleID string) (vehicleapi.RegistrationList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, registrationReadPerm); err != nil {
		return vehicleapi.RegistrationList{}, err
	}
	rows, err := s.app.ListRegistrationsByVehicle(ctx, vehicleID)
	if err != nil {
		return vehicleapi.RegistrationList{}, s.mapError(ctx, err)
	}
	out, err := s.registrationsWithLabels(ctx, rows)
	if err != nil {
		return vehicleapi.RegistrationList{}, s.mapError(ctx, err)
	}
	return vehicleapi.RegistrationList{Registrations: out}, nil
}

func (s VehicleService) RegisterVehicle(ctx context.Context, token bearertoken.Token, vehicleID string, req vehicleapi.RegisterVehicleRequest) (vehicleapi.Registration, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return vehicleapi.Registration{}, err
	}
	in := domain.RegistrationInput{
		OwnerKind:          req.OwnerKind,
		OwnerID:            req.OwnerId,
		CountryID:          req.CountryId,
		SubdivisionID:      strOr(req.SubdivisionId),
		RegistrationNumber: req.RegistrationNumber,
		NumberTypeID:       strOr(req.NumberTypeId),
	}
	if req.EffectiveFrom != nil {
		t := time.Time(*req.EffectiveFrom)
		in.EffectiveFrom = &t
	}
	reg, err := s.app.RegisterVehicle(ctx, vehicleID, in)
	if err != nil {
		return vehicleapi.Registration{}, s.mapError(ctx, err)
	}
	return s.registrationWithLabels(ctx, reg)
}

func (s VehicleService) CloseRegistration(ctx context.Context, token bearertoken.Token, registrationID string) (vehicleapi.Registration, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return vehicleapi.Registration{}, err
	}
	reg, err := s.app.CloseRegistration(ctx, registrationID)
	if err != nil {
		return vehicleapi.Registration{}, s.mapError(ctx, err)
	}
	return s.registrationWithLabels(ctx, reg)
}

// ============================ brand manufacturers ============================

func (s VehicleService) ListManufacturers(ctx context.Context, token bearertoken.Token, brandID string) (vehicleapi.ManufacturerList, error) {
	if err := s.pep.RequireAnywhere(ctx, token, readPerm); err != nil {
		return vehicleapi.ManufacturerList{}, err
	}
	rows, err := s.app.ListManufacturersByBrand(ctx, brandID)
	if err != nil {
		return vehicleapi.ManufacturerList{}, s.mapError(ctx, err)
	}
	labels, err := s.app.CompanyNamesByIDs(ctx, companyIDsOfManufacturers(rows))
	if err != nil {
		return vehicleapi.ManufacturerList{}, s.mapError(ctx, err)
	}
	out := make([]vehicleapi.Manufacturer, 0, len(rows))
	for _, m := range rows {
		out = append(out, manufacturerAPI(m, labels[m.CompanyID]))
	}
	return vehicleapi.ManufacturerList{Manufacturers: out}, nil
}

func (s VehicleService) AddManufacturer(ctx context.Context, token bearertoken.Token, brandID string, req vehicleapi.AddManufacturerRequest) (vehicleapi.Manufacturer, error) {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return vehicleapi.Manufacturer{}, err
	}
	m, err := s.app.AddManufacturer(ctx, brandID, domain.ManufacturerInput{
		CompanyID:     req.CompanyId,
		EffectiveFrom: strOr(req.EffectiveFrom),
		EffectiveTo:   strOr(req.EffectiveTo),
	})
	if err != nil {
		return vehicleapi.Manufacturer{}, s.mapError(ctx, err)
	}
	labels, err := s.app.CompanyNamesByIDs(ctx, []string{m.CompanyID})
	if err != nil {
		return vehicleapi.Manufacturer{}, s.mapError(ctx, err)
	}
	return manufacturerAPI(m, labels[m.CompanyID]), nil
}

func (s VehicleService) RemoveManufacturer(ctx context.Context, token bearertoken.Token, manufacturerID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, managePerm); err != nil {
		return err
	}
	return s.mapError(ctx, s.app.RemoveManufacturer(ctx, manufacturerID))
}

// ============================ person view ============================

func (s VehicleService) ListPersonVehicles(ctx context.Context, token bearertoken.Token, personID string) (vehicleapi.PersonVehicles, error) {
	if err := s.pep.RequireAnywhere(ctx, token, registrationReadPerm); err != nil {
		return vehicleapi.PersonVehicles{}, err
	}
	rows, err := s.app.ListPersonVehicles(ctx, personID)
	if err != nil {
		return vehicleapi.PersonVehicles{}, s.mapError(ctx, err)
	}
	typeIDs, brandIDs, modelIDs, subIDs := make([]string, 0), make([]string, 0), make([]string, 0), make([]string, 0)
	for _, p := range rows {
		typeIDs = append(typeIDs, p.TypeID)
		if p.BrandID != "" {
			brandIDs = append(brandIDs, p.BrandID)
		}
		if p.ModelID != "" {
			modelIDs = append(modelIDs, p.ModelID)
		}
		if p.SubdivisionID != "" {
			subIDs = append(subIDs, p.SubdivisionID)
		}
	}
	types, err := s.app.TypeNamesByIDs(ctx, typeIDs)
	if err != nil {
		return vehicleapi.PersonVehicles{}, s.mapError(ctx, err)
	}
	brands, err := s.app.BrandNamesByIDs(ctx, brandIDs)
	if err != nil {
		return vehicleapi.PersonVehicles{}, s.mapError(ctx, err)
	}
	models, err := s.app.ModelNamesByIDs(ctx, modelIDs)
	if err != nil {
		return vehicleapi.PersonVehicles{}, s.mapError(ctx, err)
	}
	subs, err := s.app.PlaceNamesByIDs(ctx, subIDs)
	if err != nil {
		return vehicleapi.PersonVehicles{}, s.mapError(ctx, err)
	}
	out := make([]vehicleapi.PersonVehicleRegistration, 0, len(rows))
	for _, p := range rows {
		out = append(out, personRegAPI(p, types[p.TypeID], brands[p.BrandID], models[p.ModelID], subs[p.SubdivisionID]))
	}
	return vehicleapi.PersonVehicles{Registrations: out}, nil
}

// ============================ label assembly ============================

func (s VehicleService) vehicleWithLabels(ctx context.Context, v domain.Vehicle) (vehicleapi.Vehicle, error) {
	out, err := s.vehiclesWithLabels(ctx, []domain.Vehicle{v})
	if err != nil {
		return vehicleapi.Vehicle{}, err
	}
	return out[0], nil
}

func (s VehicleService) vehiclesWithLabels(ctx context.Context, rows []domain.Vehicle) ([]vehicleapi.Vehicle, error) {
	typeIDs, brandIDs, modelIDs := make([]string, 0), make([]string, 0), make([]string, 0)
	for _, v := range rows {
		typeIDs = append(typeIDs, v.TypeID)
		if v.BrandID != "" {
			brandIDs = append(brandIDs, v.BrandID)
		}
		if v.ModelID != "" {
			modelIDs = append(modelIDs, v.ModelID)
		}
	}
	types, err := s.app.TypeNamesByIDs(ctx, typeIDs)
	if err != nil {
		return nil, err
	}
	brands, err := s.app.BrandNamesByIDs(ctx, brandIDs)
	if err != nil {
		return nil, err
	}
	models, err := s.app.ModelNamesByIDs(ctx, modelIDs)
	if err != nil {
		return nil, err
	}
	out := make([]vehicleapi.Vehicle, 0, len(rows))
	for _, v := range rows {
		out = append(out, vehicleAPI(v, types[v.TypeID], brands[v.BrandID], models[v.ModelID]))
	}
	return out, nil
}

func (s VehicleService) registrationWithLabels(ctx context.Context, reg domain.Registration) (vehicleapi.Registration, error) {
	out, err := s.registrationsWithLabels(ctx, []domain.Registration{reg})
	if err != nil {
		return vehicleapi.Registration{}, err
	}
	return out[0], nil
}

func (s VehicleService) registrationsWithLabels(ctx context.Context, rows []domain.Registration) ([]vehicleapi.Registration, error) {
	companyIDs, subIDs := make([]string, 0), make([]string, 0)
	for _, r := range rows {
		if r.OwnerKind == domain.OwnerCompany {
			companyIDs = append(companyIDs, r.OwnerID)
		}
		if r.SubdivisionID != "" {
			subIDs = append(subIDs, r.SubdivisionID)
		}
	}
	companies, err := s.app.CompanyNamesByIDs(ctx, companyIDs)
	if err != nil {
		return nil, err
	}
	subs, err := s.app.PlaceNamesByIDs(ctx, subIDs)
	if err != nil {
		return nil, err
	}
	out := make([]vehicleapi.Registration, 0, len(rows))
	for _, r := range rows {
		out = append(out, registrationAPI(r, companies[r.OwnerID], subs[r.SubdivisionID]))
	}
	return out, nil
}

// ============================ api mappers ============================

func vehicleTypeAPI(t domain.VehicleType, name map[string]string) vehicleapi.VehicleType {
	return vehicleapi.VehicleType{
		Id: t.ID, Code: t.Code, Name: name,
		ParentId: emptyToNil(t.ParentID), RootId: emptyToNil(t.RootID),
		Status: t.Status, SortOrder: t.SortOrder,
	}
}

func brandAPI(b domain.Brand, name map[string]string) vehicleapi.Brand {
	return vehicleapi.Brand{
		Id: b.ID, Code: b.Code, Name: name,
		CountryId: emptyToNil(b.CountryID), Status: b.Status, SortOrder: b.SortOrder,
	}
}

func modelAPI(m domain.Model, name map[string]string) vehicleapi.Model {
	return vehicleapi.Model{
		Id: m.ID, BrandId: m.BrandID, Code: m.Code, Name: name,
		Generation:       emptyToNil(m.Generation),
		ManufactureStart: emptyToNil(m.ManufactureStart),
		ManufactureEnd:   emptyToNil(m.ManufactureEnd),
		Status:           m.Status, SortOrder: m.SortOrder,
	}
}

func numberTypeAPI(n domain.NumberType, name map[string]string) vehicleapi.RegistrationNumberType {
	return vehicleapi.RegistrationNumberType{Id: n.ID, Code: n.Code, Name: name, Status: n.Status, SortOrder: n.SortOrder}
}

func vehicleAPI(v domain.Vehicle, typeLabel, brandLabel, modelLabel string) vehicleapi.Vehicle {
	return vehicleapi.Vehicle{
		Id: v.ID, TypeId: v.TypeID, TypeLabel: emptyToNil(typeLabel),
		ModelId: emptyToNil(v.ModelID), ModelLabel: emptyToNil(modelLabel),
		BrandId: emptyToNil(v.BrandID), BrandLabel: emptyToNil(brandLabel),
		Vin: emptyToNil(v.VIN), ColorId: emptyToNil(v.ColorID),
		ManufactureDate: emptyToNil(v.ManufactureDate), Attributes: emptyToNil(v.Attributes),
		Status: v.Status, CreatedAt: datetime.DateTime(v.CreatedAt), UpdatedAt: datetime.DateTime(v.UpdatedAt),
	}
}

func manufacturerAPI(m domain.Manufacturer, companyLabel string) vehicleapi.Manufacturer {
	return vehicleapi.Manufacturer{
		Id: m.ID, BrandId: m.BrandID, CompanyId: m.CompanyID, CompanyLabel: emptyToNil(companyLabel),
		EffectiveFrom: emptyToNil(m.EffectiveFrom), EffectiveTo: emptyToNil(m.EffectiveTo),
		CreatedAt: datetime.DateTime(m.CreatedAt), UpdatedAt: datetime.DateTime(m.UpdatedAt),
	}
}

func registrationAPI(r domain.Registration, ownerLabel, subLabel string) vehicleapi.Registration {
	out := vehicleapi.Registration{
		Id: r.ID, VehicleId: r.VehicleID, OwnerKind: r.OwnerKind, OwnerId: r.OwnerID,
		OwnerLabel: emptyToNil(ownerLabel), CountryId: r.CountryID,
		SubdivisionId: emptyToNil(r.SubdivisionID), SubdivisionLabel: emptyToNil(subLabel),
		RegistrationNumber: r.RegistrationNumber, NumberTypeId: emptyToNil(r.NumberTypeID),
		Status: r.Status, EffectiveFrom: datetime.DateTime(r.EffectiveFrom),
		CreatedAt: datetime.DateTime(r.CreatedAt), UpdatedAt: datetime.DateTime(r.UpdatedAt),
	}
	if r.EffectiveTo != nil {
		t := datetime.DateTime(*r.EffectiveTo)
		out.EffectiveTo = &t
	}
	return out
}

func personRegAPI(p domain.PersonRegistration, typeLabel, brandLabel, modelLabel, subLabel string) vehicleapi.PersonVehicleRegistration {
	out := vehicleapi.PersonVehicleRegistration{
		Id: p.ID, VehicleId: p.VehicleID, Vin: emptyToNil(p.VIN),
		TypeLabel: emptyToNil(typeLabel), BrandLabel: emptyToNil(brandLabel), ModelLabel: emptyToNil(modelLabel),
		RegistrationNumber: p.RegistrationNumber, CountryId: p.CountryID,
		SubdivisionLabel: emptyToNil(subLabel), Status: p.Status,
		EffectiveFrom: datetime.DateTime(p.EffectiveFrom),
	}
	if p.EffectiveTo != nil {
		t := datetime.DateTime(*p.EffectiveTo)
		out.EffectiveTo = &t
	}
	return out
}

// ============================ helpers ============================

func (s VehicleService) nameMap(ctx context.Context, entityType, id, def string) (map[string]string, error) {
	m, err := s.loc.NamesByID(ctx, entityType, map[string]string{id: def})
	if err != nil {
		return nil, err
	}
	return m[id], nil
}

func (s VehicleService) mapError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, domain.ErrVehicleNotFound):
		return vehicleapi.NewVehicleNotFound("")
	case errors.Is(err, domain.ErrBrandNotFound):
		return vehicleapi.NewBrandNotFound("")
	case errors.Is(err, domain.ErrLinkNotFound):
		return vehicleapi.NewLinkNotFound("")
	case errors.Is(err, domain.ErrRegionInvalid):
		return vehicleapi.NewRegionInvalid("")
	case errors.Is(err, domain.ErrConflict):
		return vehicleapi.NewConflict("code or identifier already exists in scope")
	case errors.Is(err, domain.ErrColorMismatch):
		return vehicleapi.NewInvalid("color is not a vehicle-palette color (D-Color)")
	case errors.Is(err, domain.ErrInvalid):
		return vehicleapi.NewInvalid("invalid request or unknown reference")
	}
	return werror.WrapWithContextParams(ctx, err, "vehicle operation failed")
}

func companyIDsOfManufacturers(rows []domain.Manufacturer) []string {
	out := make([]string, 0, len(rows))
	for _, m := range rows {
		out = append(out, m.CompanyID)
	}
	return out
}

func strOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func emptyToNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ---- pagination tokens (opaque base64 of the last id) ----

// pageSizePolicy mirrors the owning application service's clamp, applied at the wire edge over the
// optional Conjure arg (M56 / pkg/listing).
var pageSizePolicy = listing.PageSize{Default: 50, Max: 200}

func pageSizeOr(p *int) int { return pageSizePolicy.ResolvePtr(p) }

// decodeToken/encodeToken are the opaque keyset cursor over the last row's RID, delegated to the
// shared pkg/listing codec (M56). These endpoints previously emitted base64 StdEncoding, whose
// `+`, `/` and `=` are NOT URL-safe in a query parameter (a `+` decodes to a space, corrupting the
// cursor); listing.EncodeCursor emits RawURL, and its decode stays tolerant of the old alphabet so
// tokens issued before the upgrade keep working. An undecodable token still yields "" — restarting
// at the first page — preserving this transport's existing behaviour.
func decodeToken(p *string) string {
	id, err := listing.DecodeCursorPtr(p)
	if err != nil {
		return ""
	}
	return id
}

func encodeToken(id string) string { return listing.EncodeCursor(id) }
