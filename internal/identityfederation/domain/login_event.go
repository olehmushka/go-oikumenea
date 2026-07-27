// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"context"
	"time"
)

// LoginContext is the security-relevant kind of a recorded login/IP occurrence (D-LoginSecurityLog,
// M37). Delegated auth yields no explicit "login" step, so in practice `registration` marks the
// just-in-time account create/extend (D-JIT) and `login` marks a validated request; `activity` is
// reserved for a future finer split. The value is part of the dedup key.
type LoginContext string

const (
	LoginContextLogin        LoginContext = "login"
	LoginContextActivity     LoginContext = "activity"
	LoginContextRegistration LoginContext = "registration"
)

// LoginEvent is one deduped login/IP occurrence for an account (oikumenea.account_login_events,
// Object 9,1,4). pii:contact; retention-bounded; purge-erased. The resolved_* / IsVPN / IsTor fields
// are the IP-intelligence seam and stay nil until a resolver ships (deferred).
type LoginEvent struct {
	ID              string
	AccountID       string
	Context         LoginContext
	IP              string
	FirstSeenAt     time.Time
	LastSeenAt      time.Time
	OccurrenceCount int
	ResolvedCountry *string
	ResolvedISP     *string
	IsVPN           *bool
	IsTor           *bool
	UserAgent       *string
}

// IPIntel is the (deferred) IP-intelligence overlay a resolver may attach to an event. All-nil = the
// no-op default (D-LoginSecurityLog "as built": raw ip + user_agent only, resolver deferred).
type IPIntel struct {
	Country *string
	ISP     *string
	IsVPN   *bool
	IsTor   *bool
}

// LoginEventRepository is the login-security-log port. RecordSeen is the bounded-dedup upsert (one row
// per (account, context, ip) per window; a bump within the window, else a new row); the others serve
// the read surface, the purge fan-out, and the retention sweep. Bound to a db.DBTX like the account
// Repository, so EraseByPerson runs inside the person-purge transaction.
type LoginEventRepository interface {
	RecordSeen(ctx context.Context, accountID string, c LoginContext, ip string, userAgent *string, intel IPIntel, windowSeconds int) error
	ListByAccount(ctx context.Context, accountID, afterID string, limit int) ([]LoginEvent, error)
	EraseByPerson(ctx context.Context, personID string) (int64, error)
	DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error)
}
