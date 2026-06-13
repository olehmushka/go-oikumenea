// Package rid decodes go-oikumenea Resource Identifiers (D-ResourceIdentifiers).
//
// A RID is a native UUIDv8 (RFC 9562 §5.8) whose bits pack a decomposable, self-describing key:
// app | service | kind | type-code | timestamp | random. The Go layer carries RIDs as their
// canonical uuid text (pgx scans/encodes uuid<->string natively); this package reads the packed
// fields back out and renders the human form oikumenea:<service>:<kind>:<type>:<uuid>.
//
// Byte layout (0-indexed, big-endian) — must match oikumenea.new_id() / the rid_* SQL decoders:
//
//	0..5  unix-ms timestamp
//	6     version(4b)=8 << 4 | kind(4b)
//	7     app
//	8     variant(2b)=0b10 << 6 | service(6b)
//	9     type low 8 bits
//	10    type high 4 bits << 4 | random
//	11..15 random
package rid

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// App is the constant app/company code packed into every RID (byte 7).
const App = 1

// Kind is the object/link/action discriminator (byte 6 low nibble).
type Kind int

const (
	KindObject Kind = 1
	KindLink   Kind = 2
	KindAction Kind = 3
)

func (k Kind) String() string {
	switch k {
	case KindObject:
		return "object"
	case KindLink:
		return "link"
	case KindAction:
		return "action"
	default:
		return "unknown"
	}
}

// Service codes — mirror oikumenea.platform_rid_services (migration 0000).
const (
	SvcPlatform   = 1
	SvcI18n       = 2
	SvcAudit      = 3
	SvcTenant     = 4
	SvcRank       = 5
	SvcPerson     = 6
	SvcMembership = 7
	SvcAuthz      = 8
	SvcAccount    = 9
	SvcDocument   = 10
	SvcOrder      = 11
)

// ActionType is the generic per-service type code for an Action RID (the specific action name lives
// in audit_log.action, so the RID only encodes kind=action).
const ActionType = 0

// RID is a decoded resource identifier backed by its 16 raw bytes.
type RID struct{ b [16]byte }

// Parse accepts a canonical uuid text (the wire/DB form) and returns its decoded RID. It also
// accepts the rendered oikumenea:<service>:<kind>:<type>:<uuid> form, taking the trailing uuid.
func Parse(s string) (RID, error) {
	if i := strings.LastIndexByte(s, ':'); i >= 0 && strings.HasPrefix(s, "oikumenea:") {
		s = s[i+1:]
	}
	u, err := uuid.Parse(s)
	if err != nil {
		return RID{}, fmt.Errorf("rid: parse %q: %w", s, err)
	}
	return RID{b: u}, nil
}

// MustParse is Parse that panics on error (tests / known-good literals).
func MustParse(s string) RID {
	r, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return r
}

func (r RID) App() int      { return int(r.b[7]) }
func (r RID) Service() int  { return int(r.b[8] & 0x3f) }
func (r RID) Kind() Kind    { return Kind(r.b[6] & 0x0f) }
func (r RID) TypeCode() int { return int(r.b[9]) | (int(r.b[10]>>4) << 8) }
func (r RID) Version() int  { return int(r.b[6] >> 4) }
func (r RID) UUID() string  { return uuid.UUID(r.b).String() }
func (r RID) IsZero() bool  { return r.b == [16]byte{} }

// ServiceName returns the owning module name, or "" if the code is unknown.
func (r RID) ServiceName() string { return serviceNames[r.Service()] }

// TypeName returns the per-(service,kind) type name, or "" if unknown.
func (r RID) TypeName() string {
	return typeNames[typeKey{service: r.Service(), kind: int(r.Kind()), code: r.TypeCode()}]
}

// String renders the human, self-describing form oikumenea:<service>:<kind>:<type>:<uuid>. Unknown
// service/type codes fall back to their numeric value so the form is always well-defined.
func (r RID) String() string {
	svc := r.ServiceName()
	if svc == "" {
		svc = fmt.Sprintf("s%d", r.Service())
	}
	typ := r.TypeName()
	if typ == "" {
		typ = fmt.Sprintf("t%d", r.TypeCode())
	}
	return "oikumenea:" + svc + ":" + r.Kind().String() + ":" + typ + ":" + r.UUID()
}

// IsRID reports whether s is a well-formed go-oikumenea RID: a valid uuid carrying our app code and
// UUIDv8 version. (Cheap structural guard; does not verify the service/type are registered.)
func IsRID(s string) bool {
	r, err := Parse(s)
	return err == nil && r.Version() == 8 && r.App() == App
}

// Kinder helpers used by dispatch sites.

// LinkType returns the bare link type name if s is a Link RID of the given service, else "".
func LinkType(s string, service int) string {
	r, err := Parse(s)
	if err != nil || r.Service() != service || r.Kind() != KindLink {
		return ""
	}
	return r.TypeName()
}

// IsAction reports whether s is an Action RID (kind=action).
func IsAction(s string) bool {
	r, err := Parse(s)
	return err == nil && r.Kind() == KindAction
}
