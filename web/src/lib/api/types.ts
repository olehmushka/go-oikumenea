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
  sortOrder?: number;
  typeId: string;
}
export interface RankType {
  id: string;
  code: string;
  name?: LocaleMap;
  sortOrder?: number;
  categoryId: string;
  ranks: Rank[];
}
export interface RankCategory {
  id: string;
  code: string;
  name?: LocaleMap;
  sortOrder?: number;
  types: RankType[];
}
export interface RankScheme {
  categories: RankCategory[];
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
