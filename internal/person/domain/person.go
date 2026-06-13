// Package domain holds the person module's pure logic: the directory aggregate (Person), its
// per-person Name variants, temporal Citizenship/Residence links, the reversible deactivate -> purge
// lifecycle, and the Repository port it needs from the outside world (overview.md layering). No I/O,
// no framework imports — only the standard library.
//
// A person is instance-global (D-PersonGlobal), account-optional (L-AccountOptional), and holds at
// most one rank PER RANK SYSTEM via the person_ranks link (D-Rank, D-RankSystems) — a DIRECTORY
// attribute that grants no authority; this package never reads rank to make a decision. Names follow the Unicode CLDR fixed field set (D-PersonNamesCLDR):
// DisplayName is authoritative, the structured parts are advisory, and there is no patronymic field
// (the Slavic по-батькові lives in Given2). Calendar dates are carried as ISO-8601 "YYYY-MM-DD"
// strings (a day, not an instant); "" means absent.
package domain

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

// ISODate is the layout person calendar-date fields (birthdate, citizenship/residence windows) are
// formatted with — a day, not an instant (D-PersonBio / D-Geo).
const ISODate = "2006-01-02"

// Sentinel errors mapped to Conjure SerializableErrors by the transport layer. The DB constraints
// (partial-unique code/citizenship, RESTRICT/CASCADE FKs) enforce the same shapes as a backstop.
var (
	ErrNotFound            = errors.New("person not found")
	ErrCodeConflict        = errors.New("person code already exists")
	ErrCitizenshipConflict = errors.New("active citizenship for this country already exists")
	ErrInvalid             = errors.New("invalid person request")
	ErrUnknownRank         = errors.New("rank does not exist")
	ErrUnknownCountry      = errors.New("country does not exist")
	ErrUnknownLocale       = errors.New("locale does not exist")
	ErrNameVariantNotFound = errors.New("name variant not found")
	ErrCitizenshipNotFound = errors.New("citizenship not found")
	ErrResidenceNotFound   = errors.New("residence not found")
	ErrLifecycle           = errors.New("invalid lifecycle transition")
	ErrEmailConflict       = errors.New("active email already exists for this person")
	ErrPhoneConflict       = errors.New("active phone already exists for this person")
	ErrEmailNotFound       = errors.New("email not found")
	ErrPhoneNotFound       = errors.New("phone not found")
	ErrCallSignNotFound    = errors.New("call sign not found")
	ErrCallSignConflict    = errors.New("active call sign with this value already exists for this person")
	ErrUnknownContactType  = errors.New("contact type does not exist")
	ErrUnparseablePhone    = errors.New("phone number could not be parsed")
	// D-PersonSocialChannels (M13)
	ErrUnknownPlatform       = errors.New("platform does not exist")
	ErrPlatformNotMessenger  = errors.New("platform is not a messenger platform")
	ErrChannelNotOwned       = errors.New("the phone/email is not held by this person")
	ErrMessengerLinkNotFound = errors.New("messenger link not found")
	ErrMessengerLinkConflict = errors.New("active messenger link for this channel and platform already exists")
	ErrSocialAccountNotFound = errors.New("social account not found")
	ErrSocialAccountConflict = errors.New("active social account for this platform and identity already exists")
	// D-PersonRelationships (M14)
	ErrUnknownRelationType     = errors.New("relation type does not exist")
	ErrRelationCategory        = errors.New("relation type is not in the expected category")
	ErrSelfRelationship        = errors.New("a person cannot be related to themselves")
	ErrUnknownCounterpart      = errors.New("the counterpart person does not exist")
	ErrRelationshipNotFound    = errors.New("relationship not found")
	ErrUnknownRelationshipKind = errors.New("unknown relationship kind")
	ErrPartnershipConflict     = errors.New("a person already has an active engaged/married partnership")
	ErrRelationshipConflict    = errors.New("an equivalent active relationship already exists")
)

// Social-account attribution vocabularies (D-PersonSocialChannels): source records how the account was
// learned, confidence weights the claim. Both are non-PII metadata on the HOLDS_ACCOUNT link.
var (
	validSource     = map[string]bool{"self_declared": true, "operator_verified": true, "imported": true}
	validConfidence = map[string]bool{"confirmed": true, "probable": true, "possible": true}
)

// Platform categories (D-PersonSocialChannels): a messenger reachability platform vs a standalone
// social-account platform.
const (
	CategoryMessenger = "messenger"
	CategorySocial    = "social"
)

// DefaultConfidence is substituted when a social-account claim omits a confidence weight.
const DefaultConfidence = "possible"

// Status is the person lifecycle state (D-PersonReadScope reversibility window).
type Status string

const (
	StatusActive      Status = "active"
	StatusDeactivated Status = "deactivated"
	StatusPurged      Status = "purged"
)

// Sex is the ISO/IEC 5218 biological-sex value, stored as readable text (D-PersonBio). It is NOT
// gender identity (which would be pii:special and is out of scope).
var validSex = map[string]bool{"not_known": true, "male": true, "female": true, "not_applicable": true}

// iso5218Sex maps the raw ISO/IEC 5218 numeric codes to the canonical readable-text values. Callers
// may submit either form; NormalizeSex collapses the numeric form to text.
var iso5218Sex = map[string]string{"0": "not_known", "1": "male", "2": "female", "9": "not_applicable"}

// NormalizeSex accepts either the canonical readable text (male, female, …) or the ISO/IEC 5218
// numeric code (0, 1, 2, 9) and returns the canonical readable text. Unrecognized input is returned
// unchanged so Validate can reject it with a clear message.
func NormalizeSex(sex string) string {
	if canonical, ok := iso5218Sex[strings.TrimSpace(sex)]; ok {
		return canonical
	}
	return sex
}

// CitizenshipBasis records how a citizenship was acquired (D-Geo).
var validBasis = map[string]bool{"birth": true, "descent": true, "naturalization": true, "other": true}

// DefaultSex / DefaultBasis are the values substituted when the request omits them.
const (
	DefaultSex   = "not_known"
	DefaultBasis = "other"
)

// Name is the Unicode CLDR Person Names fixed field set shared by a Person and each of its name
// variants (D-PersonNamesCLDR). DisplayName is authoritative; every other part is advisory and ""
// when unset. There is intentionally no patronymic field — the по-батькові lives in Given2.
type Name struct {
	DisplayName   string
	Title         string
	Given         string
	Given2        string
	Surname       string
	SurnamePrefix string
	Surname2      string
	Generation    string
	Credentials   string
	Preferred     string
}

func (n Name) validate() error {
	if strings.TrimSpace(n.DisplayName) == "" {
		return wrapInvalid("displayName is required")
	}
	return nil
}

// Person is the directory aggregate root. Attributes is the long-tail JSONB directory grab-bag
// (pii:special ceiling); "" / nil means the default empty object. NameVariants/Citizenships/
// Residences are populated only when a single person is read, and are empty in list responses.
type Person struct {
	ID             string
	Code           string // "" when unset; unique among active persons
	Name                  // embedded CLDR parts (Person.DisplayName etc.)
	Birthdate      string // ISO-8601 date or ""
	DateOfDeath    string // ISO-8601 date or ""; a bio attribute, not a lifecycle state (D-PersonBio)
	Sex            string
	CountryOfBirth string // ISO-3166-1 alpha-2 or ""
	Attributes     []byte // raw JSON; nil/empty => "{}"
	Status         Status
	DeactivatedAt  *time.Time
	PurgeAfter     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time

	Ranks          []PersonRank
	NameVariants   []NameVariant
	Citizenships   []Citizenship
	Residences     []Residence
	Emails         []Email
	Phones         []Phone
	CallSigns      []CallSign
	MessengerLinks []MessengerLink
	SocialAccounts []SocialAccount
}

// Validate enforces the create-time invariants: a valid optional code, a non-empty display name, a
// known sex, and parseable optional birthdate/date_of_death. Unknown rank/country codes are caught
// by the DB FKs and surfaced as ErrUnknownRank / ErrUnknownCountry.
func (p Person) Validate() error {
	if p.Code != "" && !validCode(p.Code) {
		return wrapInvalid("code must be non-empty and contain no whitespace")
	}
	if err := p.Name.validate(); err != nil {
		return err
	}
	if p.Sex != "" && !validSex[p.Sex] {
		return wrapInvalid("sex must be one of not_known|male|female|not_applicable")
	}
	if p.CountryOfBirth != "" && !validCountry(p.CountryOfBirth) {
		return wrapInvalid("countryOfBirth must be a 2-letter ISO-3166-1 alpha-2 code")
	}
	if !validDate(p.Birthdate) {
		return wrapInvalid("birthdate must be an ISO-8601 date (YYYY-MM-DD)")
	}
	if !validDate(p.DateOfDeath) {
		return wrapInvalid("dateOfDeath must be an ISO-8601 date (YYYY-MM-DD)")
	}
	return nil
}

// CanReactivate reports whether the person may be reactivated (only from deactivated).
func (p Person) CanReactivate() bool { return p.Status == StatusDeactivated }

// CanPurge reports whether the person may be purged at time now: it must be deactivated and past the
// grace window (purge_after). A person never deactivated has no purge_after and cannot be purged.
func (p Person) CanPurge(now time.Time) bool {
	return p.Status == StatusDeactivated && p.PurgeAfter != nil && !now.Before(*p.PurgeAfter)
}

// PersonPatch is a partial update (nil = unchanged). An empty-string pointer clears an optional name
// part. Code is immutable by convention and rank is set via the dedicated SetRank path.
type PersonPatch struct {
	DisplayName    *string
	Title          *string
	Given          *string
	Given2         *string
	Surname        *string
	SurnamePrefix  *string
	Surname2       *string
	Generation     *string
	Credentials    *string
	Preferred      *string
	Birthdate      *string
	DateOfDeath    *string
	Sex            *string
	CountryOfBirth *string
	Attributes     []byte // nil = unchanged
}

// Validate enforces the patch invariants for the fields actually present.
func (p PersonPatch) Validate() error {
	if p.DisplayName != nil && strings.TrimSpace(*p.DisplayName) == "" {
		return wrapInvalid("displayName cannot be cleared")
	}
	if p.Sex != nil && !validSex[*p.Sex] {
		return wrapInvalid("sex must be one of not_known|male|female|not_applicable")
	}
	if p.CountryOfBirth != nil && *p.CountryOfBirth != "" && !validCountry(*p.CountryOfBirth) {
		return wrapInvalid("countryOfBirth must be a 2-letter ISO-3166-1 alpha-2 code")
	}
	if p.Birthdate != nil && !validDate(*p.Birthdate) {
		return wrapInvalid("birthdate must be an ISO-8601 date (YYYY-MM-DD)")
	}
	if p.DateOfDeath != nil && !validDate(*p.DateOfDeath) {
		return wrapInvalid("dateOfDeath must be an ISO-8601 date (YYYY-MM-DD)")
	}
	return nil
}

// PersonRank is one rank a person holds, scoped to a rank system (the reified HOLDS_RANK link; one
// rank per system — D-Rank, extended by D-RankSystems). SystemID is derived from the rank, never
// chosen independently. A directory attribute: this package never reads it to make a decision.
type PersonRank struct {
	SystemID string
	RankID   string
}

// NameVariant is a full transliterated name form for one locale (e.g. ukr native, eng Latin) —
// per-person data managed by the person's admins, NOT the instance localization store (D-i18n).
type NameVariant struct {
	ID        string
	PersonID  string
	Locale    string
	Name      // embedded CLDR parts
	IsPrimary bool
}

// Validate enforces a non-empty locale and display name.
func (v NameVariant) Validate() error {
	if strings.TrimSpace(v.Locale) == "" {
		return wrapInvalid("locale is required")
	}
	return v.Name.validate()
}

// Citizenship is a person's effective-dated nationality in a country (D-Geo). A person may hold
// several; at most one active per country, and IsPrimary marks at most one.
type Citizenship struct {
	ID         string
	PersonID   string
	Country    string
	Basis      string
	AcquiredOn string // ISO-8601 date or ""
	LostOn     string // ISO-8601 date or "" (still held)
	IsPrimary  bool
}

// Validate enforces a 2-letter country code, a known basis, and parseable optional dates.
func (c Citizenship) Validate() error {
	if !validCountry(c.Country) {
		return wrapInvalid("country must be a 2-letter ISO-3166-1 code")
	}
	if c.Basis != "" && !validBasis[c.Basis] {
		return wrapInvalid("basis must be one of birth|descent|naturalization|other")
	}
	if !validDate(c.AcquiredOn) || !validDate(c.LostOn) {
		return wrapInvalid("acquiredOn/lostOn must be ISO-8601 dates (YYYY-MM-DD)")
	}
	return nil
}

// Residence is a person's effective-dated residence in a country/region (D-Geo); locator data.
type Residence struct {
	ID        string
	PersonID  string
	Country   string
	Region    string
	ValidFrom string // ISO-8601 date (required)
	ValidTo   string // ISO-8601 date or "" (current)
}

// Validate enforces a 2-letter country code and a required, parseable valid_from (plus optional
// valid_to).
func (r Residence) Validate() error {
	if !validCountry(r.Country) {
		return wrapInvalid("country must be a 2-letter ISO-3166-1 code")
	}
	if r.ValidFrom == "" || !validDate(r.ValidFrom) {
		return wrapInvalid("validFrom is required and must be an ISO-8601 date (YYYY-MM-DD)")
	}
	if !validDate(r.ValidTo) {
		return wrapInvalid("validTo must be an ISO-8601 date (YYYY-MM-DD)")
	}
	return nil
}

// Email is a person's contact email (D-PersonContactChannels). pii:contact; distinct from the login
// email. Provider is derived on write from the address domain (application layer); "" when no mapping.
// ID == "" => insert; otherwise replace that row.
type Email struct {
	ID        string
	PersonID  string
	TypeCode  string
	Address   string
	Provider  string
	IsPrimary bool
}

// Validate enforces a non-empty type code and a basic email shape (one @, non-empty local part, a
// dotted domain). The person_email_types FK enforces a known type as a backstop.
func (e Email) Validate() error {
	if strings.TrimSpace(e.TypeCode) == "" {
		return wrapInvalid("typeCode is required")
	}
	if !validEmail(e.Address) {
		return wrapInvalid("address must be a valid email address")
	}
	return nil
}

// Phone is a person's contact phone (D-PersonContactChannels). Number is stored E.164-normalized and
// Country derived from it (application layer, via libphonenumber); both "" when underivable. pii:contact.
// ID == "" => insert; otherwise replace that row.
type Phone struct {
	ID        string
	PersonID  string
	TypeCode  string
	Number    string
	Country   string
	IsPrimary bool
}

// Validate enforces a non-empty type code and a non-empty number; E.164 normalization (and the
// resulting ErrUnparseablePhone) happens in the application layer where the parser lives.
func (p Phone) Validate() error {
	if strings.TrimSpace(p.TypeCode) == "" {
		return wrapInvalid("typeCode is required")
	}
	if strings.TrimSpace(p.Number) == "" {
		return wrapInvalid("number is required")
	}
	return nil
}

// CallSign is a person's informal identifier / позивний (D-PersonContactChannels). pii:basic; the
// value is required and unique per person among active rows. ID == "" => insert; otherwise replace.
type CallSign struct {
	ID        string
	PersonID  string
	CallSign  string
	IsPrimary bool
}

// Validate enforces a non-empty call sign value (the column is NOT NULL).
func (c CallSign) Validate() error {
	if strings.TrimSpace(c.CallSign) == "" {
		return wrapInvalid("callSign is required")
	}
	return nil
}

// ContactType is a row of an instance-admin contact-kind catalog (person_email_types /
// person_phone_types): a stable code + default-locale name (D-Code/D-i18n). The transport assembles
// the locale->text name map via the localization store.
type ContactType struct {
	Code      string
	Name      string
	Status    string
	SortOrder int
}

// Platform is a row of the instance-admin social-network / messenger catalog (person_platforms): a
// stable code + default-locale name + category (D-PersonSocialChannels / D-Code/D-i18n). The transport
// assembles the locale->text name map via the localization store.
type Platform struct {
	Code      string
	Name      string
	Category  string // messenger | social
	Status    string
	SortOrder int
}

// IsMessenger reports whether the platform may carry a messenger reachability link.
func (p Platform) IsMessenger() bool { return p.Category == CategoryMessenger }

// MessengerLink annotates an existing phone OR email with reachability on a messenger platform
// (D-PersonSocialChannels, layer a). Exactly one of PhoneID/EmailID is set (XOR). ID == "" => insert;
// otherwise replace that row. VerifiedAt is optional.
type MessengerLink struct {
	ID           string
	PhoneID      string
	EmailID      string
	PlatformCode string
	IsPrimary    bool
	VerifiedAt   *time.Time
}

// Validate enforces the XOR channel rule and a non-empty platform code. The platform's messenger
// category and the channel's ownership are checked in the application layer (they need the DB).
func (m MessengerLink) Validate() error {
	if strings.TrimSpace(m.PlatformCode) == "" {
		return wrapInvalid("platformCode is required")
	}
	if (m.PhoneID != "") == (m.EmailID != "") {
		return wrapInvalid("exactly one of phoneId or emailId is required")
	}
	return nil
}

// SocialAccount is a person's standalone social-network account (D-PersonSocialChannels, layer b).
// PlatformUserID is the platform's immutable internal id (the durable key; "" when unknown); Handle is
// the mutable current @handle (rename history kept separately). Source/Confidence weight the claim.
// ID == "" => insert; otherwise replace that row.
type SocialAccount struct {
	ID                   string
	PersonID             string
	PlatformCode         string
	PlatformUserID       string
	Handle               string
	DisplayName          string
	ProfileURL           string
	Language             string
	PlatformVerified     bool
	VerifiedByOperatorAt *time.Time
	Source               string
	Confidence           string
	IsPrimary            bool
}

// Validate enforces a non-empty platform code + handle and a known source/confidence. The platform's
// existence is checked in the application layer via the catalog.
func (a SocialAccount) Validate() error {
	if strings.TrimSpace(a.PlatformCode) == "" {
		return wrapInvalid("platformCode is required")
	}
	if strings.TrimSpace(a.Handle) == "" {
		return wrapInvalid("handle is required")
	}
	if !validSource[a.Source] {
		return wrapInvalid("source must be one of self_declared|operator_verified|imported")
	}
	if !validConfidence[a.Confidence] {
		return wrapInvalid("confidence must be one of confirmed|probable|possible")
	}
	return nil
}

// SocialAccountHandle is one period in a social account's @handle-rename history (D-PersonSocialChannels).
// ValidTo == nil marks the current handle.
type SocialAccountHandle struct {
	ID        string
	AccountID string
	Handle    string
	ValidFrom time.Time
	ValidTo   *time.Time
}

// ---------------------------------------------------------------- person↔person relationships (D-PersonRelationships, M14)

// Relationship link-type tokens — the bare link type names the polymorphic delete dispatches on,
// decoded from a relationship RID's packed type code (D-ResourceIdentifiers). These match the
// person-service Link type codes in the migration's new_id(6, 2, <code>) defaults / pkg/rid registry.
const (
	LinkPartnership  = "partnered_with"  // person link type code 2
	LinkKinship      = "kin_parent_of"   // 3
	LinkGuardianship = "guardian_of"     // 4
	LinkSponsorship  = "sponsor_of"      // 5
	LinkNextOfKin    = "next_of_kin"     // 6
	LinkAssociation  = "associated_with" // 7
)

// personLinkTypeByCode maps the packed type code of a person Link RID to its bare token. Kept here
// (not via pkg/rid) so this domain package stays stdlib-only (hexagonal purity).
var personLinkTypeByCode = map[int]string{
	2: LinkPartnership, 3: LinkKinship, 4: LinkGuardianship,
	5: LinkSponsorship, 6: LinkNextOfKin, 7: LinkAssociation,
}

// Relation-type catalog categories (D-PersonRelationships): the open-ended relation labels are scoped
// to which link type they apply to. Fixed lifecycle statuses (partnership, kinship) do NOT use the
// catalog.
const (
	RelCategorySponsorship = "sponsorship"
	RelCategoryAssociation = "association"
	RelCategoryNextOfKin   = "next_of_kin"
)

// Relationship status / kind vocabularies (TEXT+CHECK mirrors of the migration).
var (
	validPartnershipStatus  = map[string]bool{"engaged": true, "married": true, "divorced": true, "widowed": true, "annulled": true, "dissolved": true}
	activePartnershipStatus = map[string]bool{"engaged": true, "married": true}
	validKinshipStatus      = map[string]bool{"active": true, "disestablished": true}
	validIntervalStatus     = map[string]bool{"active": true, "ended": true} // guardianship, sponsorship, association
	validNextOfKinStatus    = map[string]bool{"active": true, "withdrawn": true}
	validAssociationKind    = map[string]bool{"associate": true, "coi": true, "no_contact": true}
)

// RelationLinkType decodes the link-type token from a relationship RID (a native UUIDv8 carrying the
// packed service/kind/type — D-ResourceIdentifiers), or "" if it is not a person Link RID. Used to
// dispatch the polymorphic delete-by-id. Pure stdlib (no pkg/rid) to keep the domain layer stdlib-only.
func RelationLinkType(rid string) string {
	b, ok := decodeRIDBytes(rid)
	if !ok {
		return ""
	}
	const svcPerson, kindLink = 6, 2
	if int(b[8]&0x3f) != svcPerson || int(b[6]&0x0f) != kindLink {
		return ""
	}
	typeCode := int(b[9]) | (int(b[10]>>4) << 8)
	return personLinkTypeByCode[typeCode]
}

// decodeRIDBytes parses a canonical uuid text RID into its 16 raw bytes (stdlib only).
func decodeRIDBytes(rid string) ([16]byte, bool) {
	hexOnly := strings.ReplaceAll(rid, "-", "")
	if len(hexOnly) != 32 {
		return [16]byte{}, false
	}
	raw, err := hex.DecodeString(hexOnly)
	if err != nil {
		return [16]byte{}, false
	}
	var b [16]byte
	copy(b[:], raw)
	return b, true
}

// RelationType is a row of the instance-admin relation-label catalog (person_relation_types): a stable
// code + default-locale name + category (D-PersonRelationships / D-Code/D-i18n). The transport assembles
// the locale->text name map via the localization store.
type RelationType struct {
	Code      string
	Name      string
	Category  string // sponsorship | association | next_of_kin
	Status    string
	SortOrder int
}

// Partnership is a marriage/engagement between two persons, stored as a canonical pair
// (PersonIDA < PersonIDB; D-PersonRelationships). ID == "" => insert; otherwise replace that row.
type Partnership struct {
	ID            string
	PersonIDA     string
	PersonIDB     string
	Status        string
	EffectiveFrom string // ISO date or ""
	EffectiveTo   string
}

func (p Partnership) Validate() error {
	if !validPartnershipStatus[p.Status] {
		return wrapInvalid("status must be one of engaged|married|divorced|widowed|annulled|dissolved")
	}
	if !validDate(p.EffectiveFrom) || !validDate(p.EffectiveTo) {
		return wrapInvalid("effectiveFrom/effectiveTo must be ISO-8601 dates")
	}
	return nil
}

// IsActivePartnership reports whether the status is one that counts against the single-active rule.
func (p Partnership) IsActivePartnership() bool { return activePartnershipStatus[p.Status] }

// Kinship is a directional parent→child blood/legal parentage link (D-PersonRelationships).
type Kinship struct {
	ID       string
	ParentID string
	ChildID  string
	Status   string
}

func (k Kinship) Validate() error {
	if !validKinshipStatus[k.Status] {
		return wrapInvalid("status must be one of active|disestablished")
	}
	return nil
}

// Guardianship is a legal guardian→ward link, distinct from blood kinship (D-PersonRelationships).
type Guardianship struct {
	ID            string
	GuardianID    string
	WardID        string
	RelationCode  string // "" when unset
	Status        string
	EffectiveFrom string
	EffectiveTo   string
}

func (g Guardianship) Validate() error {
	if !validIntervalStatus[g.Status] {
		return wrapInvalid("status must be one of active|ended")
	}
	if !validDate(g.EffectiveFrom) || !validDate(g.EffectiveTo) {
		return wrapInvalid("effectiveFrom/effectiveTo must be ISO-8601 dates")
	}
	return nil
}

// Sponsorship is a sponsor→sponsored link — godparent / advisor / mentor (D-PersonRelationships).
// RelationCode is required and must reference a category=sponsorship relation type.
type Sponsorship struct {
	ID            string
	SponsorID     string
	SponsoredID   string
	RelationCode  string
	Status        string
	EffectiveFrom string
	EffectiveTo   string
}

func (s Sponsorship) Validate() error {
	if strings.TrimSpace(s.RelationCode) == "" {
		return wrapInvalid("relationCode is required for a sponsorship")
	}
	if !validIntervalStatus[s.Status] {
		return wrapInvalid("status must be one of active|ended")
	}
	if !validDate(s.EffectiveFrom) || !validDate(s.EffectiveTo) {
		return wrapInvalid("effectiveFrom/effectiveTo must be ISO-8601 dates")
	}
	return nil
}

// NextOfKin is an in-directory next-of-kin nomination (subject→contact; D-PersonRelationships).
type NextOfKin struct {
	ID           string
	SubjectID    string
	ContactID    string
	RelationCode string
	Priority     int
	Status       string
}

func (n NextOfKin) Validate() error {
	if !validNextOfKinStatus[n.Status] {
		return wrapInvalid("status must be one of active|withdrawn")
	}
	if n.Priority < 0 {
		return wrapInvalid("priority must be non-negative")
	}
	return nil
}

// Association is a symmetric association / COI / no-contact link, stored as a canonical pair
// (PersonIDA < PersonIDB; D-PersonRelationships).
type Association struct {
	ID           string
	PersonIDA    string
	PersonIDB    string
	RelationCode string
	Kind         string
	Status       string
}

func (a Association) Validate() error {
	if !validAssociationKind[a.Kind] {
		return wrapInvalid("kind must be one of associate|coi|no_contact")
	}
	if !validIntervalStatus[a.Status] {
		return wrapInvalid("status must be one of active|ended")
	}
	return nil
}

func wrapInvalid(msg string) error { return errors.Join(ErrInvalid, errors.New(msg)) }

// validEmail is a deliberately minimal shape check (exactly one @, non-empty local part, a dotted
// domain) — not full RFC 5322. Normalization (trim/lowercase) happens in the application layer.
func validEmail(s string) bool {
	at := strings.IndexByte(s, '@')
	if at <= 0 || at != strings.LastIndexByte(s, '@') {
		return false
	}
	domainPart := s[at+1:]
	if len(domainPart) < 3 || !strings.Contains(domainPart, ".") ||
		strings.HasPrefix(domainPart, ".") || strings.HasSuffix(domainPart, ".") {
		return false
	}
	return !strings.ContainsAny(s, " \t\n\r")
}

// validCode is the shared code shape guard: non-empty, <=128 chars, no whitespace (D-Code:
// operator-assigned, locale-agnostic, immutable by convention).
func validCode(code string) bool {
	if code == "" || len(code) > 128 {
		return false
	}
	return !strings.ContainsFunc(code, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
}

// validCountry checks the ISO-3166-1 alpha-2 shape (existence is enforced by the geo FK).
func validCountry(c string) bool {
	if len(c) != 2 {
		return false
	}
	for _, r := range c {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// validDate reports whether s is empty (absent) or a valid ISO-8601 calendar date.
func validDate(s string) bool {
	if s == "" {
		return true
	}
	_, err := time.Parse(ISODate, s)
	return err == nil
}

// Repository is the persistence port the application service depends on; the pgx/sqlc adapter
// implements it. Each method runs on whatever DBTX the adapter was constructed with, so a write and
// its audit row share one transaction (D-Audit). Reads exclude soft-deleted rows.
type Repository interface {
	// persons
	InsertPerson(ctx context.Context, p Person) (Person, error)
	GetPerson(ctx context.Context, id string) (Person, error)
	// GetActivePersonByCode resolves an active person by stable code (JIT/bootstrap); ErrNotFound
	// when none matches.
	GetActivePersonByCode(ctx context.Context, code string) (Person, error)
	UpdatePerson(ctx context.Context, id string, patch PersonPatch) (Person, error)
	ListPersons(ctx context.Context, after string, limit int) ([]Person, error)
	// ListPersonsByIDs loads the base person rows for a set of RIDs (the directory-list union under
	// D-PersonReadScope resolves visible ids via memberships, then hydrates the rows here).
	ListPersonsByIDs(ctx context.Context, ids []string) ([]Person, error)

	// person ranks (the HOLDS_RANK link; one rank per rank system — D-Rank).
	// UpsertPersonRank sets the person's rank in the system DERIVED from rankID; an unknown/soft-deleted
	// rank yields ErrUnknownRank. ClearPersonRank soft-deletes the active rank in systemID (no-op when
	// none). ListPersonRanks returns the active ranks ordered by rank-system sort order.
	UpsertPersonRank(ctx context.Context, personID, rankID string) (PersonRank, error)
	ClearPersonRank(ctx context.Context, personID, systemID string) error
	ListPersonRanks(ctx context.Context, personID string) ([]PersonRank, error)

	// lifecycle
	Deactivate(ctx context.Context, id string, purgeAfter time.Time) (Person, error)
	Reactivate(ctx context.Context, id string) (Person, error)
	Purge(ctx context.Context, id string) (Person, error) // NULLs PII, removes child rows, status=purged

	// name variants
	UpsertNameVariant(ctx context.Context, v NameVariant) (NameVariant, error)
	ClearPrimaryNameVariants(ctx context.Context, personID string) error
	DeleteNameVariant(ctx context.Context, personID, locale string) error
	ListNameVariants(ctx context.Context, personID string) ([]NameVariant, error)

	// citizenships
	UpsertCitizenship(ctx context.Context, c Citizenship) (Citizenship, error)
	ClearPrimaryCitizenships(ctx context.Context, personID string) error
	DeleteCitizenship(ctx context.Context, personID, country string) error
	ListCitizenships(ctx context.Context, personID string) ([]Citizenship, error)

	// residences (r.ID == "" => insert; otherwise replace that row)
	UpsertResidence(ctx context.Context, r Residence) (Residence, error)
	DeleteResidence(ctx context.Context, personID, residenceID string) error
	ListResidences(ctx context.Context, personID string) ([]Residence, error)

	// emails (e.ID == "" => insert; otherwise replace that row)
	UpsertEmail(ctx context.Context, e Email) (Email, error)
	ClearPrimaryEmails(ctx context.Context, personID string) error
	DeleteEmail(ctx context.Context, personID, emailID string) error
	ListEmails(ctx context.Context, personID string) ([]Email, error)

	// phones (p.ID == "" => insert; otherwise replace that row)
	UpsertPhone(ctx context.Context, p Phone) (Phone, error)
	ClearPrimaryPhones(ctx context.Context, personID string) error
	DeletePhone(ctx context.Context, personID, phoneID string) error
	ListPhones(ctx context.Context, personID string) ([]Phone, error)

	// call signs (c.ID == "" => insert; otherwise replace that row)
	UpsertCallSign(ctx context.Context, c CallSign) (CallSign, error)
	ClearPrimaryCallSigns(ctx context.Context, personID string) error
	DeleteCallSign(ctx context.Context, personID, callSignID string) error
	ListCallSigns(ctx context.Context, personID string) ([]CallSign, error)

	// contact-kind catalogs
	ListEmailTypes(ctx context.Context) ([]ContactType, error)
	ListPhoneTypes(ctx context.Context) ([]ContactType, error)

	// platform catalog (D-PersonSocialChannels)
	ListPlatforms(ctx context.Context) ([]Platform, error)
	GetPlatform(ctx context.Context, code string) (Platform, error) // ErrUnknownPlatform when missing

	// messenger links (m.ID == "" => insert; otherwise replace that row)
	PhonePersonID(ctx context.Context, phoneID string) (string, error) // ErrPhoneNotFound when missing
	EmailPersonID(ctx context.Context, emailID string) (string, error) // ErrEmailNotFound when missing
	UpsertMessengerLink(ctx context.Context, m MessengerLink) (MessengerLink, error)
	ClearPrimaryMessengerLinks(ctx context.Context, personID string) error
	DeleteMessengerLink(ctx context.Context, personID, linkID string) error
	ListMessengerLinks(ctx context.Context, personID string) ([]MessengerLink, error)

	// social accounts (Insert/Update split so the application can keep the handle history)
	InsertSocialAccount(ctx context.Context, a SocialAccount) (SocialAccount, error)
	UpdateSocialAccount(ctx context.Context, a SocialAccount) (SocialAccount, error)
	GetSocialAccount(ctx context.Context, personID, accountID string) (SocialAccount, error)
	ClearPrimarySocialAccounts(ctx context.Context, personID string) error
	DeleteSocialAccount(ctx context.Context, personID, accountID string) error
	ListSocialAccounts(ctx context.Context, personID string) ([]SocialAccount, error)

	// social account handle history
	InsertSocialAccountHandle(ctx context.Context, h SocialAccountHandle) (SocialAccountHandle, error)
	CloseCurrentSocialAccountHandle(ctx context.Context, accountID string) error
	ListSocialAccountHandles(ctx context.Context, accountID string) ([]SocialAccountHandle, error)

	// ---- person↔person relationships (D-PersonRelationships) ----

	// relation-type catalog
	ListRelationTypes(ctx context.Context) ([]RelationType, error)
	GetRelationType(ctx context.Context, code string) (RelationType, error) // ErrUnknownRelationType when missing

	// per-type upsert (struct.ID == "" => insert; otherwise replace that row by id, scoped to an endpoint)
	UpsertPartnership(ctx context.Context, p Partnership) (Partnership, error)
	UpsertKinship(ctx context.Context, k Kinship) (Kinship, error)
	UpsertGuardianship(ctx context.Context, g Guardianship) (Guardianship, error)
	UpsertSponsorship(ctx context.Context, s Sponsorship) (Sponsorship, error)
	UpsertNextOfKin(ctx context.Context, n NextOfKin) (NextOfKin, error)
	UpsertAssociation(ctx context.Context, a Association) (Association, error)

	// HasActivePartnershipExcept reports whether the person has any active engaged/married partnership
	// other than exceptID (the single-active-per-person rule a partial-unique index can't span).
	HasActivePartnershipExcept(ctx context.Context, personID, exceptID string) (bool, error)

	// per-type list touching the person on EITHER endpoint
	ListPartnerships(ctx context.Context, personID string) ([]Partnership, error)
	ListKinships(ctx context.Context, personID string) ([]Kinship, error)
	ListGuardianships(ctx context.Context, personID string) ([]Guardianship, error)
	ListSponsorships(ctx context.Context, personID string) ([]Sponsorship, error)
	ListNextOfKin(ctx context.Context, personID string) ([]NextOfKin, error)
	ListAssociations(ctx context.Context, personID string) ([]Association, error)

	// per-type soft-delete by id, scoped so the person must be an endpoint (returns ErrRelationshipNotFound)
	DeletePartnership(ctx context.Context, personID, id string) error
	DeleteKinship(ctx context.Context, personID, id string) error
	DeleteGuardianship(ctx context.Context, personID, id string) error
	DeleteSponsorship(ctx context.Context, personID, id string) error
	DeleteNextOfKin(ctx context.Context, personID, id string) error
	DeleteAssociation(ctx context.Context, personID, id string) error

	// purge erasure — remove all relationship rows touching the person on EITHER endpoint
	DeleteAllPartnerships(ctx context.Context, personID string) error
	DeleteAllKinships(ctx context.Context, personID string) error
	DeleteAllGuardianships(ctx context.Context, personID string) error
	DeleteAllSponsorships(ctx context.Context, personID string) error
	DeleteAllNextOfKin(ctx context.Context, personID string) error
	DeleteAllAssociations(ctx context.Context, personID string) error
}
