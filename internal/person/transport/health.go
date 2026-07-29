// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Health & vulnerability transport (D-HealthVulnerability, M36): category-level health records
// (pii:special) and insurance coverage (pii:sensitive). Health reads require their own need-to-know code
// person.health.read (D-DataScope); insurance reads require person.read. Writes require person.update.
package transport

import (
	"context"

	personapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/person"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
	"github.com/palantir/pkg/bearertoken"
)

// ---------------------------------------------------------------- health records

func (s Service) ListHealthRecords(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.HealthRecord, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permHealthRead); err != nil {
		return nil, err
	}
	hs, err := s.sensitive.ListHealthRecords(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.HealthRecord, 0, len(hs))
	for _, h := range hs {
		out = append(out, toAPIHealthRecord(h))
	}
	return out, nil
}

func (s Service) UpsertHealthRecord(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertHealthRecordRequest) (personapi.HealthRecord, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.HealthRecord{}, err
	}
	created, err := s.sensitive.UpsertHealthRecord(ctx, domain.HealthRecord{
		PersonID:       personID,
		Kind:           req.Kind,
		Detail:         req.Detail,
		IsPublicRecord: derefOr(req.IsPublicRecord, false),
		AssessedAt:     derefOr(req.AssessedAt, ""),
		LegalBasis:     req.LegalBasis,
		Source:         derefOr(req.Source, ""),
		Confidence:     derefOr(req.Confidence, ""),
	})
	if err != nil {
		return personapi.HealthRecord{}, s.mapError(ctx, err, personID)
	}
	return toAPIHealthRecord(created), nil
}

func (s Service) DeleteHealthRecord(ctx context.Context, token bearertoken.Token, personID, recordID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.sensitive.DeleteHealthRecord(ctx, personID, recordID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---------------------------------------------------------------- insurance

func (s Service) ListInsurance(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.Insurance, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permRead); err != nil {
		return nil, err
	}
	is, err := s.sensitive.ListInsurance(ctx, personID)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.Insurance, 0, len(is))
	for _, i := range is {
		out = append(out, toAPIInsurance(i))
	}
	return out, nil
}

func (s Service) UpsertInsurance(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertInsuranceRequest) (personapi.Insurance, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.Insurance{}, err
	}
	created, err := s.sensitive.UpsertInsurance(ctx, domain.Insurance{
		ID:                derefOr(req.Id, ""),
		PersonID:          personID,
		Type:              req.Type,
		Provider:          derefOr(req.Provider, ""),
		PolicyReference:   derefOr(req.PolicyReference, ""),
		EmployerSponsored: derefOr(req.EmployerSponsored, false),
		ValidFrom:         derefOr(req.ValidFrom, ""),
		ValidTo:           derefOr(req.ValidTo, ""),
		Source:            derefOr(req.Source, ""),
		Confidence:        derefOr(req.Confidence, ""),
	})
	if err != nil {
		return personapi.Insurance{}, s.mapError(ctx, err, personID)
	}
	return toAPIInsurance(created), nil
}

func (s Service) DeleteInsurance(ctx context.Context, token bearertoken.Token, personID, insuranceID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.sensitive.DeleteInsurance(ctx, personID, insuranceID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---- mappers ----

func toAPIHealthRecord(h domain.HealthRecord) personapi.HealthRecord {
	return personapi.HealthRecord{
		Id:             h.ID,
		PersonId:       h.PersonID,
		Kind:           h.Kind,
		Detail:         h.Detail,
		IsPublicRecord: h.IsPublicRecord,
		AssessedAt:     strPtrOrNil(h.AssessedAt),
		LegalBasis:     h.LegalBasis,
		Source:         strPtrOrNil(h.Source),
		Confidence:     strPtrOrNil(h.Confidence),
	}
}

func toAPIInsurance(i domain.Insurance) personapi.Insurance {
	return personapi.Insurance{
		Id:                i.ID,
		PersonId:          i.PersonID,
		Type:              i.Type,
		Provider:          strPtrOrNil(i.Provider),
		PolicyReference:   strPtrOrNil(i.PolicyReference),
		EmployerSponsored: i.EmployerSponsored,
		ValidFrom:         strPtrOrNil(i.ValidFrom),
		ValidTo:           strPtrOrNil(i.ValidTo),
		Source:            strPtrOrNil(i.Source),
		Confidence:        strPtrOrNil(i.Confidence),
	}
}
