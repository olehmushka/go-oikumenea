// Hand-written projections of the API responses the screens render. These mirror
// docs/api/openapi/openapi.json; the fully-typed surface is the generated schema.d.ts used by
// the browser client. Kept narrow on purpose — only the fields the console reads.

export type LocaleMap = Record<string, string>;

export interface Whoami {
  personId?: string;
  accountId?: string;
  email?: string;
}

export interface Unit {
  id: string;
  code: string;
  name?: LocaleMap;
  unitKind?: string;
  level?: number;
  visibility?: string;
  state?: string;
  createdAt?: string;
  updatedAt?: string;
}
export interface UnitPage {
  units: Unit[];
  nextPageToken?: string;
}
export interface UnitRef {
  id: string;
  code: string;
  name?: LocaleMap;
}
export interface UnitRefList {
  units: UnitRef[];
}
export interface Graph {
  id: string;
  code: string;
  name?: LocaleMap;
  isDirectoryOnly?: boolean;
}
export interface GraphList {
  graphs: Graph[];
}

export interface Person {
  id: string;
  code: string;
  displayName?: string;
  given?: string;
  surname?: string;
  patronymic?: string;
  birthdate?: string;
  sex?: string;
  status?: string;
  rankId?: string;
  countryOfBirth?: string;
  citizenships?: Citizenship[];
  residences?: Residence[];
  emails?: Email[];
  phones?: Phone[];
  callSigns?: CallSign[];
  messengerLinks?: MessengerLink[];
  socialAccounts?: SocialAccount[];
  nameVariants?: NameVariant[];
  createdAt?: string;
  deactivatedAt?: string;
  purgeAfter?: string;
}
export interface PersonPage {
  persons: Person[];
  nextPageToken?: string;
}
export interface Citizenship {
  id: string;
  country: string;
  isPrimary?: boolean;
  basis?: string;
}
export interface Residence {
  id: string;
  country: string;
  region?: string;
  validFrom?: string;
  validTo?: string;
}
export interface Email {
  id: string;
  address: string;
  typeCode?: string;
  isPrimary?: boolean;
}
export interface Phone {
  id: string;
  number: string;
  country?: string;
  typeCode?: string;
  isPrimary?: boolean;
}
export interface CallSign {
  id: string;
  callSign: string;
  isPrimary?: boolean;
}

// ── M13: social & messenger channels ────────────────────────────────────────
/** Instance-admin catalog of messenger/social platforms (category gates which links are allowed). */
export interface Platform {
  code: string;
  name?: LocaleMap;
  category: string; // messenger | social
  status?: string;
  sortOrder?: number;
}
/** A reachability annotation over a person's phone XOR email on a messenger platform. */
export interface MessengerLink {
  id: string;
  phoneId?: string;
  emailId?: string;
  platformCode: string;
  isPrimary?: boolean;
  verifiedAt?: string;
}
/** A person's account on a social/messenger platform; the handle is mutable (history kept). */
export interface SocialAccount {
  id: string;
  personId: string;
  platformCode: string;
  platformUserId?: string;
  handle: string;
  displayName?: string;
  profileUrl?: string;
  language?: string;
  platformVerified?: boolean;
  verifiedByOperatorAt?: string;
  source: string; // self_declared | operator_verified | imported
  confidence: string; // confirmed | probable | possible
  isPrimary?: boolean;
}
/** A historical @handle for a social account (validTo null = current). */
export interface SocialAccountHandle {
  id: string;
  accountId: string;
  handle: string;
  validFrom: string;
  validTo?: string;
}

// ── M14: person↔person relationships ────────────────────────────────────────
/** Instance-admin catalog of relation kinds (category scopes which family may reference it). */
export interface RelationType {
  code: string;
  name?: LocaleMap;
  category: string; // sponsorship | association | next_of_kin
  status?: string;
  sortOrder?: number;
}
export interface Partnership {
  id: string;
  personIdA: string;
  personIdB: string;
  status: string;
  effectiveFrom?: string;
  effectiveTo?: string;
}
export interface Kinship {
  id: string;
  parentId: string;
  childId: string;
  status: string;
}
export interface Guardianship {
  id: string;
  guardianId: string;
  wardId: string;
  relationCode?: string;
  status: string;
  effectiveFrom?: string;
  effectiveTo?: string;
}
export interface Sponsorship {
  id: string;
  sponsorId: string;
  sponsoredId: string;
  relationCode: string;
  status: string;
  effectiveFrom?: string;
  effectiveTo?: string;
}
export interface NextOfKin {
  id: string;
  subjectId: string;
  contactId: string;
  relationCode?: string;
  priority: number;
  status: string;
}
export interface Association {
  id: string;
  personIdA: string;
  personIdB: string;
  relationCode?: string;
  kind: string; // associate | coi | no_contact
  status: string;
}
export interface NameVariant {
  id: string;
  locale: string;
  displayName?: string;
  given?: string;
  surname?: string;
  isPrimary?: boolean;
}

export interface Membership {
  id: string;
  personId: string;
  unitId: string;
  positionId?: string;
  status?: string;
  effectiveFrom?: string;
  effectiveTo?: string;
}
export interface MembershipPage {
  memberships: Membership[];
  nextPageToken?: string;
}
export interface Position {
  id: string;
  code: string;
  title?: LocaleMap;
  unitId: string;
  status?: string;
  requiredRankId?: string;
  sortOrder?: number;
  holder?: unknown;
}
export interface PositionPage {
  positions: Position[];
  nextPageToken?: string;
}

export interface Rank {
  id: string;
  code: string;
  name?: LocaleMap;
  abbreviation?: string;
  gradeCode?: string;
  sortOrder?: number;
  systemId?: string;
  typeId: string;
}
export interface RankType {
  id: string;
  code: string;
  name?: LocaleMap;
  sortOrder?: number;
  systemId?: string;
  categoryId: string;
  parentTypeId?: string;
  children?: RankType[];
  ranks: Rank[];
}
export interface RankCategory {
  id: string;
  code: string;
  name?: LocaleMap;
  sortOrder?: number;
  systemId?: string;
  types: RankType[];
}
/** A rank system (e.g. us-armed-forces, nato-generic): the top level of the scheme tree. */
export interface RankSystem {
  id: string;
  code: string;
  name?: LocaleMap;
  sortOrder?: number;
  country?: string;
  categories: RankCategory[];
}
export interface RankScheme {
  systems: RankSystem[];
}
/** A NATO STANAG-2116 grade — reference data for cross-system rank equivalence (not translatable). */
export interface RankGrade {
  code: string;
  tier: string; // officer | warrant | enlisted
  ordinal: number;
  name: string;
}

export interface Role {
  id: string;
  code: string;
  name?: LocaleMap;
  description?: LocaleMap;
  permissions: string[];
  isBase?: boolean;
}
export interface RolePage {
  roles: Role[];
  nextPageToken?: string;
}
export interface Assignment {
  id: string;
  subjectPersonId: string;
  roleId: string;
  targetUnitId: string;
  graphId?: string;
  scope: string;
  grantedAt?: string;
  expiresAt?: string;
  revokedAt?: string;
}
export interface AssignmentPage {
  assignments: Assignment[];
  nextPageToken?: string;
}

export interface Contribution {
  assignmentId?: string;
  roleCode?: string;
  scope?: string;
  graphId?: string;
  viaUnitId?: string;
}
export interface Explanation {
  reason?: string;
  instanceAdmin?: boolean;
  contributions?: Contribution[];
}
export interface AuthorizeResponse {
  allow: boolean;
  explanation?: Explanation;
}

export interface DocumentType {
  id: string;
  code: string;
  name?: LocaleMap;
  status?: string;
  attrSchema?: unknown;
}
export interface PersonalCodeScheme {
  id?: string;
  code: string;
  name?: LocaleMap;
  country?: string;
  status?: string;
}
export interface DocumentDoc {
  id: string;
  personId: string;
  typeId: string;
  number?: string;
  issuer?: string;
  issuingCountry?: string;
  issuedOn?: string;
  expiresOn?: string;
  status?: string;
}

export interface OrderItem {
  id?: string;
  kind?: string;
  personId?: string;
  unitId?: string;
}
export interface Order {
  id: string;
  number?: string;
  issuingUnitId?: string;
  issuedOn?: string;
  status?: string;
  items?: OrderItem[];
  revokedAt?: string;
}
export interface OrderPage {
  orders: Order[];
  nextPageToken?: string;
}
export interface OrderType {
  id: string;
  code: string;
  name?: LocaleMap;
  status?: string;
}

export interface Locale {
  code: string;
  name: string;
  enabled?: boolean;
  isDefault?: boolean;
  sortOrder?: number;
}
export interface LocaleList {
  locales: Locale[];
}

export interface AuditEntry {
  id: string;
  action: string;
  subsystem?: string;
  outcome?: string;
  actorType?: string;
  actorPersonId?: string;
  targetType?: string;
  targetId?: string;
  unitId?: string;
  requestId?: string;
  createdAt?: string;
}
export interface AuditEntryPage {
  entries?: AuditEntry[];
  nextPageToken?: string;
}

export interface VersionInfo {
  version?: string;
  schemaVersion?: string;
  [k: string]: unknown;
}
