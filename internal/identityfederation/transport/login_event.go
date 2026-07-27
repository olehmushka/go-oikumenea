// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"

	authzdomain "github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	identityapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/identityfederation"
	"github.com/olegamysk/go-oikumenea/internal/identityfederation/domain"
	"github.com/palantir/pkg/bearertoken"
	"github.com/palantir/pkg/datetime"
)

const defaultLoginEventPageSize = 50

// ListAccountLoginEvents pages an account's login/IP security history newest-first (M37 /
// D-LoginSecurityLog), gated on the instance-scope account.security-log.read. RequireAnywhere denies a
// service principal (no person id) and any non-admin (an instance-scope code is admin-only via pdp.go).
func (s Service) ListAccountLoginEvents(ctx context.Context, token bearertoken.Token, accountID string, pageSize *int, pageToken *string) (identityapi.LoginEventPage, error) {
	if err := s.pep.RequireAnywhere(ctx, token, string(authzdomain.PermAccountSecurityLogRead)); err != nil {
		return identityapi.LoginEventPage{}, err
	}
	limit := defaultLoginEventPageSize
	if pageSize != nil && *pageSize > 0 {
		limit = *pageSize
	}
	// Fetch one extra to decide whether a next page exists without a second count query.
	rows, err := s.app.ListLoginEvents(ctx, accountID, derefStr(pageToken), limit+1)
	if err != nil {
		return identityapi.LoginEventPage{}, s.mapError(ctx, err, errCtx{accountID: accountID})
	}
	var next *string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1].ID
		next = &last
	}
	out := make([]identityapi.LoginEvent, 0, len(rows))
	for _, e := range rows {
		out = append(out, toAPILoginEvent(e))
	}
	return identityapi.LoginEventPage{Events: out, NextPageToken: next}, nil
}

func toAPILoginEvent(e domain.LoginEvent) identityapi.LoginEvent {
	return identityapi.LoginEvent{
		Id:              e.ID,
		AccountId:       e.AccountID,
		Context:         string(e.Context),
		Ip:              e.IP,
		FirstSeenAt:     datetime.DateTime(e.FirstSeenAt),
		LastSeenAt:      datetime.DateTime(e.LastSeenAt),
		OccurrenceCount: e.OccurrenceCount,
		ResolvedCountry: e.ResolvedCountry,
		ResolvedIsp:     e.ResolvedISP,
		IsVpn:           e.IsVPN,
		IsTor:           e.IsTor,
		UserAgent:       e.UserAgent,
	}
}
