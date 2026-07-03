// API response types for the console — now thin aliases over the generated TS SDK (oikumenea-client),
// so the UI shares the contract's types and typed SDK method returns flow through pages/components
// without re-projection. Only genuinely web-local shapes stay hand-written here.

import type {
  audit,
  authorization,
  document,
  hermenea,
  identityfederation,
  localization,
  membership,
  order,
  person,
  platform,
  rank,
  religion,
  tenant,
} from "oikumenea-client";

/** Translatable label map (locale → text) — the i18n shape the contract returns for `name`. */
export type LocaleMap = Record<string, string>;

// ── identity / whoami ────────────────────────────────────────────────────────
export type Whoami = identityfederation.IWhoami;

// ── tenant ───────────────────────────────────────────────────────────────────
export type Unit = tenant.IUnit;
export type UnitPage = tenant.IUnitPage;
export type Domain = tenant.IDomain;
export type Organization = tenant.IOrganization;
export type UnitKind = tenant.IUnitKind;
export type Visibility = tenant.Visibility;
export type UnitRef = tenant.IUnitRef;
export type UnitRefList = tenant.IUnitRefList;
export type Graph = tenant.IGraph;
export type GraphList = tenant.IGraphList;
export type UnitLanguage = tenant.IUnitLanguage;
export type UnitCodeEvent = tenant.IUnitCodeEvent;
export type UnitCodeEventList = tenant.IUnitCodeEventList;

// ── person ───────────────────────────────────────────────────────────────────
export type Person = person.IPerson;
export type PersonPage = person.IPersonPage;
export type Citizenship = person.ICitizenship;
export type Residence = person.IResidence;
export type Address = person.IAddress;
export type Email = person.IEmail;
export type Phone = person.IPhone;
export type CallSign = person.ICallSign;
export type Platform = person.IPlatform;
export type MessengerLink = person.IMessengerLink;
export type SocialAccount = person.ISocialAccount;
export type SocialAccountHandle = person.ISocialAccountHandle;
export type PersonLanguage = person.IPersonLanguage;
export type RelationType = person.IRelationType;
export type Partnership = person.IPartnership;
export type Kinship = person.IKinship;
export type Guardianship = person.IGuardianship;
export type Sponsorship = person.ISponsorship;
export type NextOfKin = person.INextOfKin;
export type Association = person.IAssociation;
export type NameVariant = person.INameVariant;
export type PersonRank = person.IPersonRank;
// physical identity (M31, D-PhysicalIdentity)
export type PhysicalDescription = person.IPhysicalDescription;
export type DistinguishingMark = person.IDistinguishingMark;
export type EthnicityType = person.IEthnicityType;
export type Ethnicity = person.IEthnicity;

// ── localization / language ──────────────────────────────────────────────────
export type LocaleLanguage = localization.ILocaleLanguage;
export type Locale = localization.ILocale;
export type LocaleList = localization.ILocaleList;

// ── religion (M22–M25) ───────────────────────────────────────────────────────
export type ClergyGrade = religion.IClergyGrade;
export type GradeCategory = religion.IGradeCategory;
export type OfficeType = religion.IOfficeType;
export type ClergyCredential = religion.IClergyCredential;
export type AffiliationType = religion.IAffiliationType;
export type Affiliation = religion.IAffiliation;

// ── membership ───────────────────────────────────────────────────────────────
export type Membership = membership.IMembership;
export type MembershipPage = membership.IMembershipPage;
export type Position = membership.IPosition;
export type PositionPage = membership.IPositionPage;

// ── rank ─────────────────────────────────────────────────────────────────────
export type Rank = rank.IRank;
export type RankType = rank.IRankType;
export type RankCategory = rank.IRankCategory;
export type RankSystem = rank.IRankSystem;
export type RankScheme = rank.IRankScheme;
export type RankGrade = rank.IRankGrade;

// ── authorization ────────────────────────────────────────────────────────────
export type Role = authorization.IRole;
export type RolePage = authorization.IRolePage;
export type Assignment = authorization.IAssignment;
export type AssignmentPage = authorization.IAssignmentPage;
export type Contribution = authorization.IContribution;
export type Explanation = authorization.IExplanation;
export type AuthorizeResponse = authorization.IAuthorizeResponse;

// ── document ─────────────────────────────────────────────────────────────────
export type DocumentType = document.IDocumentType;
export type PersonalCodeScheme = document.IPersonalCodeScheme;
export type DocumentDoc = document.IDocument;

// ── order ────────────────────────────────────────────────────────────────────
export type OrderItem = order.IOrderItem;
export type Order = order.IOrder;
export type OrderPage = order.IOrderPage;
export type OrderType = order.IOrderType;

// ── audit ────────────────────────────────────────────────────────────────────
export type AuditEntry = audit.IAuditEntry;
export type AuditEntryPage = audit.IAuditEntryPage;

// ── platform ─────────────────────────────────────────────────────────────────
export type VersionInfo = platform.IVersionInfo;

// ── hermenea import control (M16) ────────────────────────────────────────────
export type ImportSource = hermenea.IImportSource;
export type ImportRun = hermenea.IImportRun;
export type WorkerJob = hermenea.IWorkerJob;
export type JobRef = hermenea.IJobRef;
