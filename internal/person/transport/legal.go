// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Criminal / arrest / court records transport (D-LegalRecords, M38): category-level records
// (pii:special, GDPR Art. 10). Reads require the person.legal-record.read need-to-know code; writes
// require person.update. Sealed/expunged (suppressed) records are withheld unless the caller ALSO
// holds person.legal-record.read-suppressed — computed here (non-erroring probe) and passed to the
// application as includeSuppressed, mirroring the audit sensitive-reader redaction gate.
package transport

import (
	"context"

	personapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/person"
	"github.com/olegamysk/go-oikumenea/internal/person/domain"
	"github.com/palantir/pkg/bearertoken"
)

func (s Service) ListLegalRecords(ctx context.Context, token bearertoken.Token, personID string) ([]personapi.LegalRecord, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permLegalRecordRead); err != nil {
		return nil, err
	}
	includeSuppressed, err := s.pep.AllowedAnywhere(ctx, token, permLegalRecordReadSuppressed)
	if err != nil {
		return nil, err
	}
	rs, err := s.sensitive.ListLegalRecords(ctx, personID, includeSuppressed)
	if err != nil {
		return nil, s.mapError(ctx, err, personID)
	}
	out := make([]personapi.LegalRecord, 0, len(rs))
	for _, r := range rs {
		out = append(out, toAPILegalRecord(r))
	}
	return out, nil
}

func (s Service) UpsertLegalRecord(ctx context.Context, token bearertoken.Token, personID string, req personapi.UpsertLegalRecordRequest) (personapi.LegalRecord, error) {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return personapi.LegalRecord{}, err
	}
	created, err := s.sensitive.UpsertLegalRecord(ctx, domain.LegalRecord{
		ID:               derefOr(req.Id, ""),
		PersonID:         personID,
		Kind:             req.Kind,
		Disposition:      req.Disposition,
		Detail:           req.Detail,
		Jurisdiction:     derefOr(req.Jurisdiction, ""),
		OccurredAt:       derefOr(req.OccurredAt, ""),
		DispositionDate:  derefOr(req.DispositionDate, ""),
		IsSuppressed:     derefOr(req.IsSuppressed, false),
		SuppressedReason: derefOr(req.SuppressedReason, ""),
		LegalBasis:       req.LegalBasis,
		Source:           derefOr(req.Source, ""),
		Confidence:       derefOr(req.Confidence, ""),
	})
	if err != nil {
		return personapi.LegalRecord{}, s.mapError(ctx, err, personID)
	}
	return toAPILegalRecord(created), nil
}

func (s Service) DeleteLegalRecord(ctx context.Context, token bearertoken.Token, personID, recordID string) error {
	if err := s.pep.RequireAnywhere(ctx, token, permUpdate); err != nil {
		return err
	}
	if err := s.sensitive.DeleteLegalRecord(ctx, personID, recordID); err != nil {
		return s.mapError(ctx, err, personID)
	}
	return nil
}

// ---- mappers ----

func toAPILegalRecord(r domain.LegalRecord) personapi.LegalRecord {
	return personapi.LegalRecord{
		Id:               r.ID,
		PersonId:         r.PersonID,
		Kind:             r.Kind,
		Disposition:      r.Disposition,
		Detail:           r.Detail,
		Jurisdiction:     strPtrOrNil(r.Jurisdiction),
		OccurredAt:       strPtrOrNil(r.OccurredAt),
		DispositionDate:  strPtrOrNil(r.DispositionDate),
		IsSuppressed:     r.IsSuppressed,
		SuppressedReason: strPtrOrNil(r.SuppressedReason),
		LegalBasis:       r.LegalBasis,
		Source:           strPtrOrNil(r.Source),
		Confidence:       strPtrOrNil(r.Confidence),
	}
}
