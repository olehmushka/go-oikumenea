import { IAddEthnicityRequest } from "./addEthnicityRequest";
import { IAddNameAliasRequest } from "./addNameAliasRequest";
import { IAddress } from "./address";
import { IAssociation } from "./association";
import { ICallSign } from "./callSign";
import { ICitizenship } from "./citizenship";
import { ICreatePersonRequest } from "./createPersonRequest";
import { ICreateProvisionalPersonRequest } from "./createProvisionalPersonRequest";
import { ICryptoWallet } from "./cryptoWallet";
import { IDeactivateRequest } from "./deactivateRequest";
import { IDistinguishingMark } from "./distinguishingMark";
import { IEmail } from "./email";
import { IEmailType } from "./emailType";
import { IEthnicity } from "./ethnicity";
import { IEthnicityType } from "./ethnicityType";
import { IExternalReference } from "./externalReference";
import { IGovernmentPosition } from "./governmentPosition";
import { IGuardianship } from "./guardianship";
import { IKinship } from "./kinship";
import { ILobbyingRelationship } from "./lobbyingRelationship";
import { IMergePersonRequest } from "./mergePersonRequest";
import { IMessengerLink } from "./messengerLink";
import { INameVariant } from "./nameVariant";
import { INextOfKin } from "./nextOfKin";
import { IPartnership } from "./partnership";
import { IPartyMembership } from "./partyMembership";
import { IPerson } from "./person";
import { IPersonLanguage } from "./personLanguage";
import { IPersonPage } from "./personPage";
import { IPersonality } from "./personality";
import { IPhone } from "./phone";
import { IPhoneType } from "./phoneType";
import { IPhysicalDescription } from "./physicalDescription";
import { IPlatform } from "./platform";
import { IPoliticalLeaning } from "./politicalLeaning";
import { IRegulatorySanction } from "./regulatorySanction";
import { IRelationType } from "./relationType";
import { IResidence } from "./residence";
import { ISetRankRequest } from "./setRankRequest";
import { ISocialAccount } from "./socialAccount";
import { ISocialAccountHandle } from "./socialAccountHandle";
import { ISponsorship } from "./sponsorship";
import { IUpdateEthnicityRequest } from "./updateEthnicityRequest";
import { IUpdatePersonRequest } from "./updatePersonRequest";
import { IUpsertAddressRequest } from "./upsertAddressRequest";
import { IUpsertAssociationRequest } from "./upsertAssociationRequest";
import { IUpsertCallSignRequest } from "./upsertCallSignRequest";
import { IUpsertCitizenshipRequest } from "./upsertCitizenshipRequest";
import { IUpsertCryptoWalletRequest } from "./upsertCryptoWalletRequest";
import { IUpsertDistinguishingMarkRequest } from "./upsertDistinguishingMarkRequest";
import { IUpsertEmailRequest } from "./upsertEmailRequest";
import { IUpsertEthnicityTypeRequest } from "./upsertEthnicityTypeRequest";
import { IUpsertExternalReferenceRequest } from "./upsertExternalReferenceRequest";
import { IUpsertGovernmentPositionRequest } from "./upsertGovernmentPositionRequest";
import { IUpsertGuardianshipRequest } from "./upsertGuardianshipRequest";
import { IUpsertKinshipRequest } from "./upsertKinshipRequest";
import { IUpsertLobbyingRelationshipRequest } from "./upsertLobbyingRelationshipRequest";
import { IUpsertMessengerLinkRequest } from "./upsertMessengerLinkRequest";
import { IUpsertNameVariantRequest } from "./upsertNameVariantRequest";
import { IUpsertNextOfKinRequest } from "./upsertNextOfKinRequest";
import { IUpsertPartnershipRequest } from "./upsertPartnershipRequest";
import { IUpsertPartyMembershipRequest } from "./upsertPartyMembershipRequest";
import { IUpsertPersonLanguageRequest } from "./upsertPersonLanguageRequest";
import { IUpsertPersonalityRequest } from "./upsertPersonalityRequest";
import { IUpsertPhoneRequest } from "./upsertPhoneRequest";
import { IUpsertPhysicalDescriptionRequest } from "./upsertPhysicalDescriptionRequest";
import { IUpsertPoliticalLeaningRequest } from "./upsertPoliticalLeaningRequest";
import { IUpsertRegulatorySanctionRequest } from "./upsertRegulatorySanctionRequest";
import { IUpsertResidenceRequest } from "./upsertResidenceRequest";
import { IUpsertSocialAccountRequest } from "./upsertSocialAccountRequest";
import { IUpsertSponsorshipRequest } from "./upsertSponsorshipRequest";
import { IWatchlistMatch } from "./watchlistMatch";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * The personnel directory (D-PersonGlobal). Reads gate on `person.read` via the read-scope rule
 * (D-PersonReadScope); writes on `person.create`/`person.update`/`person.rank.assign`/
 * `person.lifecycle`/`person.purge` — all enforced once authorization (M7) lands. Writes are
 * audited in-process (D-Audit). The module never reads rank to make a decision (D-Rank).
 *
 */
export interface IPersonService {
    /** Create a person (no account, no unit needed). Returns Person:PersonConflict if the code is taken. */
    createPerson(request: ICreatePersonRequest): Promise<IPerson>;
    /**
     * Create a minimal-PII PROVISIONAL stub (D-OverlayFoundation, M29) — an unresolved external /
     * edge-target person so a relationship or overlay edge points at a real node. Resolve it later
     * via mergePerson. Only displayName is required.
     *
     */
    createProvisionalPerson(request: ICreateProvisionalPersonRequest): Promise<IPerson>;
    /**
     * Resolve the provisional stub {personId} INTO a canonical person (D-OverlayFoundation, M29):
     * re-homes the stub's edges (and every other module's references) onto the canonical person in
     * one transaction, then tombstones the stub. {personId} must be provisional; `intoPersonId` must
     * be a distinct, non-provisional person. Returns the canonical Person. Returns Person:PersonInvalid
     * when the source is not provisional or the target is invalid.
     *
     */
    mergePerson(personId: string, request: IMergePersonRequest): Promise<IPerson>;
    /** Read one person with its name variants, citizenships, and residences. */
    getPerson(personId: string): Promise<IPerson>;
    /** Update names, birthdate, date_of_death, sex, country_of_birth, attributes. `code` is immutable; rank via setRank. */
    updatePerson(personId: string, request: IUpdatePersonRequest): Promise<IPerson>;
    /** Search/list the directory, token-paginated. (The read-scope union is applied once authz lands, M7.) */
    listPersons(pageSize?: number | null, pageToken?: string | null, query?: string | null): Promise<IPersonPage>;
    /** Set or clear the person's rank in one rank system (one rank per system, a directory attribute; D-Rank). Returns Person:PersonInvalid for an unknown rank. */
    setRank(personId: string, request: ISetRankRequest): Promise<IPerson>;
    /** Begin reversible deactivation (opens the grace window before purge). */
    deactivatePerson(personId: string, request: IDeactivateRequest): Promise<IPerson>;
    /** Cancel deactivation within the grace window. Returns Person:PersonLifecycleConflict if not deactivated. */
    reactivatePerson(personId: string): Promise<IPerson>;
    /**
     * Hard-erase PII after the grace window (idempotent): NULLs every PII column and removes
     * citizenship/residence/name-variant rows, keeping the id as a tombstone so audit history
     * stays intact. Returns Person:PersonLifecycleConflict before purgeAfter.
     *
     */
    purgePerson(personId: string): Promise<IPerson>;
    /** Add or replace a locale name form (transliteration). Keyed by (person, locale). */
    upsertNameVariant(personId: string, request: IUpsertNameVariantRequest): Promise<INameVariant>;
    /** Remove a name variant (the canonical transliteration for a locale). */
    deleteNameVariant(personId: string, locale: string): Promise<void>;
    /** Add an alias name form (aka/former_legal/maiden/pseudonym/cover). Returns the created NameVariant. */
    addNameAlias(personId: string, request: IAddNameAliasRequest): Promise<INameVariant>;
    /** Remove an alias by its RID. */
    deleteNameAlias(personId: string, aliasId: string): Promise<void>;
    /** List a person's effective-dated physical descriptions (D-PhysicalIdentity). */
    listPhysicalDescriptions(personId: string): Promise<Array<IPhysicalDescription>>;
    /** Add a physical description, or replace one when id is supplied. Returns Person:PersonInvalid for a bad measurement/blood type. */
    upsertPhysicalDescription(personId: string, request: IUpsertPhysicalDescriptionRequest): Promise<IPhysicalDescription>;
    /** Remove a physical description by id. */
    deletePhysicalDescription(personId: string, descriptionId: string): Promise<void>;
    /** List a person's distinguishing marks (D-PhysicalIdentity; pii:special ceiling). */
    listDistinguishingMarks(personId: string): Promise<Array<IDistinguishingMark>>;
    /** Add a distinguishing mark, or replace one when id is supplied. Returns Person:PersonInvalid for an unknown kind. */
    upsertDistinguishingMark(personId: string, request: IUpsertDistinguishingMarkRequest): Promise<IDistinguishingMark>;
    /** Remove a distinguishing mark by id. */
    deleteDistinguishingMark(personId: string, markId: string): Promise<void>;
    /** List a person's addresses (D-PersonAddresses, M32; pii:contact), primary first. */
    listAddresses(personId: string): Promise<Array<IAddress>>;
    /** Add an address, or replace one when id is supplied. Returns Person:PersonInvalid for an unknown role/location. */
    upsertAddress(personId: string, request: IUpsertAddressRequest): Promise<IAddress>;
    /** Remove an address by id. */
    deleteAddress(personId: string, addressId: string): Promise<void>;
    /** List a person's party memberships (D-InstitutionalTies, M33; pii:special, decrypted). */
    listPartyMemberships(personId: string): Promise<Array<IPartyMembership>>;
    /** Add a party membership, or replace one when id is supplied. Requires legalBasis (Art. 9). */
    upsertPartyMembership(personId: string, request: IUpsertPartyMembershipRequest): Promise<IPartyMembership>;
    /** Remove a party membership by id. */
    deletePartyMembership(personId: string, membershipId: string): Promise<void>;
    /** List a person's government positions (D-InstitutionalTies, M33; pii:basic). */
    listGovernmentPositions(personId: string): Promise<Array<IGovernmentPosition>>;
    /** Add a government position, or replace one when id is supplied. */
    upsertGovernmentPosition(personId: string, request: IUpsertGovernmentPositionRequest): Promise<IGovernmentPosition>;
    /** Remove a government position by id. */
    deleteGovernmentPosition(personId: string, positionId: string): Promise<void>;
    /** List a person's lobbying relationships (D-InstitutionalTies, M33; pii:basic). */
    listLobbyingRelationships(personId: string): Promise<Array<ILobbyingRelationship>>;
    /** Add a lobbying relationship, or replace one when id is supplied. */
    upsertLobbyingRelationship(personId: string, request: IUpsertLobbyingRelationshipRequest): Promise<ILobbyingRelationship>;
    /** Remove a lobbying relationship by id. */
    deleteLobbyingRelationship(personId: string, relationshipId: string): Promise<void>;
    /** List a person's external references (D-InstitutionalTies, M33; pii:basic). */
    listExternalReferences(personId: string): Promise<Array<IExternalReference>>;
    /** Add an external reference (idempotent by url), or replace one when id is supplied. */
    upsertExternalReference(personId: string, request: IUpsertExternalReferenceRequest): Promise<IExternalReference>;
    /** Remove an external reference by id. */
    deleteExternalReference(personId: string, referenceId: string): Promise<void>;
    /**
     * Run a live watchlist screening check for a person (D-Watchlists, M34). Routes egress OUT to the
     * hermenea companion (which owns the OFAC/EU/UN/INTERPOL providers + a ≤24h cache), combines the
     * returned match metadata with the locally-derived PEP flag (M33 government positions), and
     * persists the single per-person WatchlistMatch. Audited. Only match metadata is stored — never
     * the lists.
     *
     */
    checkWatchlists(personId: string): Promise<IWatchlistMatch>;
    /** The person's most recent watchlist screening result, or null if never screened. */
    getWatchlistMatch(personId: string): Promise<IWatchlistMatch | null>;
    /** List a person's regulatory-sanction overlay rows (D-Watchlists, M34; pii:sensitive). */
    listRegulatorySanctions(personId: string): Promise<Array<IRegulatorySanction>>;
    /** Add a regulatory sanction, or replace one when id is supplied. */
    upsertRegulatorySanction(personId: string, request: IUpsertRegulatorySanctionRequest): Promise<IRegulatorySanction>;
    /** Remove a regulatory sanction by id. */
    deleteRegulatorySanction(personId: string, sanctionId: string): Promise<void>;
    /** List a person's crypto-wallet attributions (D-PersonOverlays, M35; pii:sensitive). */
    listCryptoWallets(personId: string): Promise<Array<ICryptoWallet>>;
    /** Add a crypto wallet, or replace one when id is supplied. */
    upsertCryptoWallet(personId: string, request: IUpsertCryptoWalletRequest): Promise<ICryptoWallet>;
    /** Remove a crypto wallet by id. */
    deleteCryptoWallet(personId: string, walletId: string): Promise<void>;
    /** List a person's declared/assessed personality profiles (D-PersonOverlays, M35; pii:sensitive). */
    listPersonalities(personId: string): Promise<Array<IPersonality>>;
    /** Add a personality profile, or replace one when id is supplied. */
    upsertPersonality(personId: string, request: IUpsertPersonalityRequest): Promise<IPersonality>;
    /** Remove a personality profile by id. */
    deletePersonality(personId: string, personalityId: string): Promise<void>;
    /**
     * The person's inferred political leaning, or null if none (D-PersonOverlays, M35; pii:special,
     * decrypted). Never a declared party affiliation.
     *
     */
    getPoliticalLeaning(personId: string): Promise<IPoliticalLeaning | null>;
    /** Set the person's inferred political leaning (replaces the single active row). Requires legalBasis. */
    setPoliticalLeaning(personId: string, request: IUpsertPoliticalLeaningRequest): Promise<IPoliticalLeaning>;
    /** Remove the person's inferred political leaning. */
    deletePoliticalLeaning(personId: string): Promise<void>;
    /**
     * List the declared-ethnicity taxonomy (D-PhysicalIdentity amendment, M43). Optionally filter to
     * the forest roots (topLevel), the immediate children of a parent RID (parent, for lazy tree
     * expansion), or a name/code substring (query). `hasChildren` is set on each entry.
     *
     */
    listEthnicityTypes(topLevel?: boolean | null, parent?: string | null, query?: string | null, limit?: number | null): Promise<Array<IEthnicityType>>;
    /** Fetch one ethnicity type by RID, including its group-level associated languages + homeland countries. */
    getEthnicityType(ethnicityTypeId: string): Promise<IEthnicityType>;
    /** Add or update a declared-ethnicity catalog entry (instance-admin managed). */
    upsertEthnicityType(request: IUpsertEthnicityTypeRequest): Promise<IEthnicityType>;
    /** List a person's declared ethnicities with the value decrypted (D-PhysicalIdentity / D-SpecialPII). */
    listEthnicities(personId: string): Promise<Array<IEthnicity>>;
    /** Declare an ethnicity (envelope-encrypted, lawful-basis-gated). Returns Person:PersonInvalid for an unknown code/legal basis. */
    addEthnicity(personId: string, request: IAddEthnicityRequest): Promise<IEthnicity>;
    /** Re-declare the ethnicity value and/or flip legal basis / status. */
    updateEthnicity(personId: string, ethnicityId: string, request: IUpdateEthnicityRequest): Promise<IEthnicity>;
    /** Remove a declared ethnicity by id. */
    deleteEthnicity(personId: string, ethnicityId: string): Promise<void>;
    /** List a person's citizenships. */
    listCitizenships(personId: string): Promise<Array<ICitizenship>>;
    /** Add or replace the active citizenship for a country. Returns Person:PersonInvalid for an unknown country. */
    upsertCitizenship(personId: string, request: IUpsertCitizenshipRequest): Promise<ICitizenship>;
    /** Remove a citizenship by country RID. */
    deleteCitizenship(personId: string, country: string): Promise<void>;
    /** List a person's residence history. */
    listResidences(personId: string): Promise<Array<IResidence>>;
    /** Add a residence row, or replace one when id is supplied. Returns Person:PersonInvalid for an unknown country. */
    upsertResidence(personId: string, request: IUpsertResidenceRequest): Promise<IResidence>;
    /** Remove a residence row by id. */
    deleteResidence(personId: string, residenceId: string): Promise<void>;
    /** List a person's contact emails. */
    listEmails(personId: string): Promise<Array<IEmail>>;
    /** Add or replace a contact email. Returns Person:PersonConflict if the address is taken, Person:PersonInvalid for an unknown type or malformed address. */
    upsertEmail(personId: string, request: IUpsertEmailRequest): Promise<IEmail>;
    /** Remove a contact email by id. */
    deleteEmail(personId: string, emailId: string): Promise<void>;
    /** List a person's contact phones. */
    listPhones(personId: string): Promise<Array<IPhone>>;
    /** Add or replace a contact phone. Returns Person:PersonConflict if the number is taken, Person:PersonInvalid for an unknown type or unparseable number. */
    upsertPhone(personId: string, request: IUpsertPhoneRequest): Promise<IPhone>;
    /** Remove a contact phone by id. */
    deletePhone(personId: string, phoneId: string): Promise<void>;
    /** List a person's call signs. */
    listCallSigns(personId: string): Promise<Array<ICallSign>>;
    /** Add or replace a call sign. Returns Person:PersonConflict if the value is already held by the person. */
    upsertCallSign(personId: string, request: IUpsertCallSignRequest): Promise<ICallSign>;
    /** Remove a call sign by id. */
    deleteCallSign(personId: string, callSignId: string): Promise<void>;
    /** List the contact-email type catalog (locale -> text names; D-i18n). */
    listEmailTypes(): Promise<Array<IEmailType>>;
    /** List the contact-phone type catalog (locale -> text names; D-i18n). */
    listPhoneTypes(): Promise<Array<IPhoneType>>;
    /** List the social/messenger platform catalog (locale -> text names; D-i18n; D-PersonSocialChannels). */
    listPlatforms(): Promise<Array<IPlatform>>;
    /** List a person's messenger reachability links. */
    listMessengerLinks(personId: string): Promise<Array<IMessengerLink>>;
    /**
     * Add or replace a messenger link over one of the person's phones/emails. Returns
     * Person:PersonConflict if an active link for the channel+platform exists, Person:PersonInvalid
     * for an unknown / non-messenger platform or a channel not held by the person.
     *
     */
    upsertMessengerLink(personId: string, request: IUpsertMessengerLinkRequest): Promise<IMessengerLink>;
    /** Remove a messenger link by id. */
    deleteMessengerLink(personId: string, messengerLinkId: string): Promise<void>;
    /** List a person's standalone social accounts. */
    listSocialAccounts(personId: string): Promise<Array<ISocialAccount>>;
    /**
     * Add or replace a social account. A handle rename is recorded in the account's handle history.
     * Returns Person:PersonConflict on a duplicate active account, Person:PersonInvalid for an
     * unknown platform or bad source/confidence.
     *
     */
    upsertSocialAccount(personId: string, request: IUpsertSocialAccountRequest): Promise<ISocialAccount>;
    /** Remove a social account by id (its handle history cascades). */
    deleteSocialAccount(personId: string, socialAccountId: string): Promise<void>;
    /** List one social account's @handle-rename history (most recent first). */
    listSocialAccountHandles(personId: string, socialAccountId: string): Promise<Array<ISocialAccountHandle>>;
    /** List the languages the person speaks (native first, then by name; D-Languages, M18). */
    listPersonLanguages(personId: string): Promise<Array<IPersonLanguage>>;
    /**
     * Add or update a language the person speaks (keyed on languageId). Returns Person:PersonInvalid
     * when languageId does not resolve to a level='language' languoid or cefrLevel is invalid.
     *
     */
    upsertPersonLanguage(personId: string, request: IUpsertPersonLanguageRequest): Promise<IPersonLanguage>;
    /** Remove a language the person speaks, by languoid id. Idempotent within the active set. */
    deletePersonLanguage(personId: string, languageId: string): Promise<void>;
    /** List the person↔person relation-type catalog (locale -> text names; D-i18n; D-PersonRelationships). */
    listRelationTypes(): Promise<Array<IRelationType>>;
    /** List partnerships (marriage/engagement) touching the person. */
    listPartnerships(personId: string): Promise<Array<IPartnership>>;
    /**
     * Add or replace a partnership between the person and the partner. Returns Person:PersonConflict
     * when either person already has an active engaged/married partnership, Person:PersonInvalid for a
     * self-pair, unknown partner, or bad status.
     *
     */
    upsertPartnership(personId: string, request: IUpsertPartnershipRequest): Promise<IPartnership>;
    /** List parent/child kinships touching the person. */
    listKinships(personId: string): Promise<Array<IKinship>>;
    /** Add or replace a parent→child kinship. Returns Person:PersonConflict on a duplicate active pair, Person:PersonInvalid for a self-edge, unknown counterpart, or bad role. */
    upsertKinship(personId: string, request: IUpsertKinshipRequest): Promise<IKinship>;
    /** List guardianships touching the person. */
    listGuardianships(personId: string): Promise<Array<IGuardianship>>;
    /** Add or replace a guardian→ward link. Returns Person:PersonInvalid for a self-edge, unknown counterpart, unknown relation code, or bad role. */
    upsertGuardianship(personId: string, request: IUpsertGuardianshipRequest): Promise<IGuardianship>;
    /** List sponsorships (godparent/advisor/mentor) touching the person. */
    listSponsorships(personId: string): Promise<Array<ISponsorship>>;
    /** Add or replace a sponsor→sponsored link. relationCode must be a category=sponsorship code. Returns Person:PersonInvalid for a self-edge, unknown counterpart, or wrong relation category. */
    upsertSponsorship(personId: string, request: IUpsertSponsorshipRequest): Promise<ISponsorship>;
    /** List next-of-kin nominations touching the person (priority-ordered). */
    listNextOfKin(personId: string): Promise<Array<INextOfKin>>;
    /** Nominate or replace a next-of-kin contact for the person. Returns Person:PersonInvalid for a self-nomination, unknown contact, or wrong relation category. */
    upsertNextOfKin(personId: string, request: IUpsertNextOfKinRequest): Promise<INextOfKin>;
    /** List associations (associate/COI/no-contact) touching the person. */
    listAssociations(personId: string): Promise<Array<IAssociation>>;
    /** Add or replace a symmetric association. Returns Person:PersonInvalid for a self-pair, unknown counterpart, wrong relation category, or bad kind. */
    upsertAssociation(personId: string, request: IUpsertAssociationRequest): Promise<IAssociation>;
    /**
     * Remove any person↔person link by id (the link type is decoded from the RID). The path person
     * must be one of the link's endpoints. Idempotent.
     *
     */
    deleteRelationship(personId: string, relationshipId: string): Promise<void>;
}

export class PersonService implements IPersonService {
    constructor(private bridge: IHttpApiBridge) {
    }

    /** Create a person (no account, no unit needed). Returns Person:PersonConflict if the code is taken. */
    public createPerson(request: ICreatePersonRequest): Promise<IPerson> {
        return this.bridge.call<IPerson>(
            "PersonService",
            "createPerson",
            "POST",
            "/person/v1/persons",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * Create a minimal-PII PROVISIONAL stub (D-OverlayFoundation, M29) — an unresolved external /
     * edge-target person so a relationship or overlay edge points at a real node. Resolve it later
     * via mergePerson. Only displayName is required.
     *
     */
    public createProvisionalPerson(request: ICreateProvisionalPersonRequest): Promise<IPerson> {
        return this.bridge.call<IPerson>(
            "PersonService",
            "createProvisionalPerson",
            "POST",
            "/person/v1/provisional-persons",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /**
     * Resolve the provisional stub {personId} INTO a canonical person (D-OverlayFoundation, M29):
     * re-homes the stub's edges (and every other module's references) onto the canonical person in
     * one transaction, then tombstones the stub. {personId} must be provisional; `intoPersonId` must
     * be a distinct, non-provisional person. Returns the canonical Person. Returns Person:PersonInvalid
     * when the source is not provisional or the target is invalid.
     *
     */
    public mergePerson(personId: string, request: IMergePersonRequest): Promise<IPerson> {
        return this.bridge.call<IPerson>(
            "PersonService",
            "mergePerson",
            "POST",
            "/person/v1/persons/{personId}/merge",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Read one person with its name variants, citizenships, and residences. */
    public getPerson(personId: string): Promise<IPerson> {
        return this.bridge.call<IPerson>(
            "PersonService",
            "getPerson",
            "GET",
            "/person/v1/persons/{personId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Update names, birthdate, date_of_death, sex, country_of_birth, attributes. `code` is immutable; rank via setRank. */
    public updatePerson(personId: string, request: IUpdatePersonRequest): Promise<IPerson> {
        return this.bridge.call<IPerson>(
            "PersonService",
            "updatePerson",
            "PUT",
            "/person/v1/persons/{personId}",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Search/list the directory, token-paginated. (The read-scope union is applied once authz lands, M7.) */
    public listPersons(pageSize?: number | null, pageToken?: string | null, query?: string | null): Promise<IPersonPage> {
        return this.bridge.call<IPersonPage>(
            "PersonService",
            "listPersons",
            "GET",
            "/person/v1/persons",
            __undefined,
            __undefined,
            {
                "pageSize": pageSize,
                "pageToken": pageToken,
                "query": query,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Set or clear the person's rank in one rank system (one rank per system, a directory attribute; D-Rank). Returns Person:PersonInvalid for an unknown rank. */
    public setRank(personId: string, request: ISetRankRequest): Promise<IPerson> {
        return this.bridge.call<IPerson>(
            "PersonService",
            "setRank",
            "PUT",
            "/person/v1/persons/{personId}/rank",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Begin reversible deactivation (opens the grace window before purge). */
    public deactivatePerson(personId: string, request: IDeactivateRequest): Promise<IPerson> {
        return this.bridge.call<IPerson>(
            "PersonService",
            "deactivatePerson",
            "POST",
            "/person/v1/persons/{personId}/deactivate",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Cancel deactivation within the grace window. Returns Person:PersonLifecycleConflict if not deactivated. */
    public reactivatePerson(personId: string): Promise<IPerson> {
        return this.bridge.call<IPerson>(
            "PersonService",
            "reactivatePerson",
            "POST",
            "/person/v1/persons/{personId}/reactivate",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * Hard-erase PII after the grace window (idempotent): NULLs every PII column and removes
     * citizenship/residence/name-variant rows, keeping the id as a tombstone so audit history
     * stays intact. Returns Person:PersonLifecycleConflict before purgeAfter.
     *
     */
    public purgePerson(personId: string): Promise<IPerson> {
        return this.bridge.call<IPerson>(
            "PersonService",
            "purgePerson",
            "POST",
            "/person/v1/persons/{personId}/purge",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Add or replace a locale name form (transliteration). Keyed by (person, locale). */
    public upsertNameVariant(personId: string, request: IUpsertNameVariantRequest): Promise<INameVariant> {
        return this.bridge.call<INameVariant>(
            "PersonService",
            "upsertNameVariant",
            "PUT",
            "/person/v1/persons/{personId}/name-variants",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Remove a name variant (the canonical transliteration for a locale). */
    public deleteNameVariant(personId: string, locale: string): Promise<void> {
        return this.bridge.call<void>(
            "PersonService",
            "deleteNameVariant",
            "DELETE",
            "/person/v1/persons/{personId}/name-variants/{locale}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                locale,
            ],
            __undefined,
            __undefined
        );
    }

    /** Add an alias name form (aka/former_legal/maiden/pseudonym/cover). Returns the created NameVariant. */
    public addNameAlias(personId: string, request: IAddNameAliasRequest): Promise<INameVariant> {
        return this.bridge.call<INameVariant>(
            "PersonService",
            "addNameAlias",
            "POST",
            "/person/v1/persons/{personId}/name-aliases",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Remove an alias by its RID. */
    public deleteNameAlias(personId: string, aliasId: string): Promise<void> {
        return this.bridge.call<void>(
            "PersonService",
            "deleteNameAlias",
            "DELETE",
            "/person/v1/persons/{personId}/name-aliases/{aliasId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                aliasId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List a person's effective-dated physical descriptions (D-PhysicalIdentity). */
    public listPhysicalDescriptions(personId: string): Promise<Array<IPhysicalDescription>> {
        return this.bridge.call<Array<IPhysicalDescription>>(
            "PersonService",
            "listPhysicalDescriptions",
            "GET",
            "/person/v1/persons/{personId}/physical-descriptions",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Add a physical description, or replace one when id is supplied. Returns Person:PersonInvalid for a bad measurement/blood type. */
    public upsertPhysicalDescription(personId: string, request: IUpsertPhysicalDescriptionRequest): Promise<IPhysicalDescription> {
        return this.bridge.call<IPhysicalDescription>(
            "PersonService",
            "upsertPhysicalDescription",
            "PUT",
            "/person/v1/persons/{personId}/physical-descriptions",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Remove a physical description by id. */
    public deletePhysicalDescription(personId: string, descriptionId: string): Promise<void> {
        return this.bridge.call<void>(
            "PersonService",
            "deletePhysicalDescription",
            "DELETE",
            "/person/v1/persons/{personId}/physical-descriptions/{descriptionId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                descriptionId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List a person's distinguishing marks (D-PhysicalIdentity; pii:special ceiling). */
    public listDistinguishingMarks(personId: string): Promise<Array<IDistinguishingMark>> {
        return this.bridge.call<Array<IDistinguishingMark>>(
            "PersonService",
            "listDistinguishingMarks",
            "GET",
            "/person/v1/persons/{personId}/distinguishing-marks",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Add a distinguishing mark, or replace one when id is supplied. Returns Person:PersonInvalid for an unknown kind. */
    public upsertDistinguishingMark(personId: string, request: IUpsertDistinguishingMarkRequest): Promise<IDistinguishingMark> {
        return this.bridge.call<IDistinguishingMark>(
            "PersonService",
            "upsertDistinguishingMark",
            "PUT",
            "/person/v1/persons/{personId}/distinguishing-marks",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Remove a distinguishing mark by id. */
    public deleteDistinguishingMark(personId: string, markId: string): Promise<void> {
        return this.bridge.call<void>(
            "PersonService",
            "deleteDistinguishingMark",
            "DELETE",
            "/person/v1/persons/{personId}/distinguishing-marks/{markId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                markId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List a person's addresses (D-PersonAddresses, M32; pii:contact), primary first. */
    public listAddresses(personId: string): Promise<Array<IAddress>> {
        return this.bridge.call<Array<IAddress>>(
            "PersonService",
            "listAddresses",
            "GET",
            "/person/v1/persons/{personId}/addresses",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Add an address, or replace one when id is supplied. Returns Person:PersonInvalid for an unknown role/location. */
    public upsertAddress(personId: string, request: IUpsertAddressRequest): Promise<IAddress> {
        return this.bridge.call<IAddress>(
            "PersonService",
            "upsertAddress",
            "PUT",
            "/person/v1/persons/{personId}/addresses",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Remove an address by id. */
    public deleteAddress(personId: string, addressId: string): Promise<void> {
        return this.bridge.call<void>(
            "PersonService",
            "deleteAddress",
            "DELETE",
            "/person/v1/persons/{personId}/addresses/{addressId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                addressId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List a person's party memberships (D-InstitutionalTies, M33; pii:special, decrypted). */
    public listPartyMemberships(personId: string): Promise<Array<IPartyMembership>> {
        return this.bridge.call<Array<IPartyMembership>>(
            "PersonService",
            "listPartyMemberships",
            "GET",
            "/person/v1/persons/{personId}/party-memberships",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Add a party membership, or replace one when id is supplied. Requires legalBasis (Art. 9). */
    public upsertPartyMembership(personId: string, request: IUpsertPartyMembershipRequest): Promise<IPartyMembership> {
        return this.bridge.call<IPartyMembership>(
            "PersonService",
            "upsertPartyMembership",
            "PUT",
            "/person/v1/persons/{personId}/party-memberships",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Remove a party membership by id. */
    public deletePartyMembership(personId: string, membershipId: string): Promise<void> {
        return this.bridge.call<void>(
            "PersonService",
            "deletePartyMembership",
            "DELETE",
            "/person/v1/persons/{personId}/party-memberships/{membershipId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                membershipId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List a person's government positions (D-InstitutionalTies, M33; pii:basic). */
    public listGovernmentPositions(personId: string): Promise<Array<IGovernmentPosition>> {
        return this.bridge.call<Array<IGovernmentPosition>>(
            "PersonService",
            "listGovernmentPositions",
            "GET",
            "/person/v1/persons/{personId}/government-positions",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Add a government position, or replace one when id is supplied. */
    public upsertGovernmentPosition(personId: string, request: IUpsertGovernmentPositionRequest): Promise<IGovernmentPosition> {
        return this.bridge.call<IGovernmentPosition>(
            "PersonService",
            "upsertGovernmentPosition",
            "PUT",
            "/person/v1/persons/{personId}/government-positions",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Remove a government position by id. */
    public deleteGovernmentPosition(personId: string, positionId: string): Promise<void> {
        return this.bridge.call<void>(
            "PersonService",
            "deleteGovernmentPosition",
            "DELETE",
            "/person/v1/persons/{personId}/government-positions/{positionId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                positionId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List a person's lobbying relationships (D-InstitutionalTies, M33; pii:basic). */
    public listLobbyingRelationships(personId: string): Promise<Array<ILobbyingRelationship>> {
        return this.bridge.call<Array<ILobbyingRelationship>>(
            "PersonService",
            "listLobbyingRelationships",
            "GET",
            "/person/v1/persons/{personId}/lobbying-relationships",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Add a lobbying relationship, or replace one when id is supplied. */
    public upsertLobbyingRelationship(personId: string, request: IUpsertLobbyingRelationshipRequest): Promise<ILobbyingRelationship> {
        return this.bridge.call<ILobbyingRelationship>(
            "PersonService",
            "upsertLobbyingRelationship",
            "PUT",
            "/person/v1/persons/{personId}/lobbying-relationships",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Remove a lobbying relationship by id. */
    public deleteLobbyingRelationship(personId: string, relationshipId: string): Promise<void> {
        return this.bridge.call<void>(
            "PersonService",
            "deleteLobbyingRelationship",
            "DELETE",
            "/person/v1/persons/{personId}/lobbying-relationships/{relationshipId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                relationshipId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List a person's external references (D-InstitutionalTies, M33; pii:basic). */
    public listExternalReferences(personId: string): Promise<Array<IExternalReference>> {
        return this.bridge.call<Array<IExternalReference>>(
            "PersonService",
            "listExternalReferences",
            "GET",
            "/person/v1/persons/{personId}/external-references",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Add an external reference (idempotent by url), or replace one when id is supplied. */
    public upsertExternalReference(personId: string, request: IUpsertExternalReferenceRequest): Promise<IExternalReference> {
        return this.bridge.call<IExternalReference>(
            "PersonService",
            "upsertExternalReference",
            "PUT",
            "/person/v1/persons/{personId}/external-references",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Remove an external reference by id. */
    public deleteExternalReference(personId: string, referenceId: string): Promise<void> {
        return this.bridge.call<void>(
            "PersonService",
            "deleteExternalReference",
            "DELETE",
            "/person/v1/persons/{personId}/external-references/{referenceId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                referenceId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * Run a live watchlist screening check for a person (D-Watchlists, M34). Routes egress OUT to the
     * hermenea companion (which owns the OFAC/EU/UN/INTERPOL providers + a ≤24h cache), combines the
     * returned match metadata with the locally-derived PEP flag (M33 government positions), and
     * persists the single per-person WatchlistMatch. Audited. Only match metadata is stored — never
     * the lists.
     *
     */
    public checkWatchlists(personId: string): Promise<IWatchlistMatch> {
        return this.bridge.call<IWatchlistMatch>(
            "PersonService",
            "checkWatchlists",
            "POST",
            "/person/v1/persons/{personId}/watchlist-check",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** The person's most recent watchlist screening result, or null if never screened. */
    public getWatchlistMatch(personId: string): Promise<IWatchlistMatch | null> {
        return this.bridge.call<IWatchlistMatch | null>(
            "PersonService",
            "getWatchlistMatch",
            "GET",
            "/person/v1/persons/{personId}/watchlist-match",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List a person's regulatory-sanction overlay rows (D-Watchlists, M34; pii:sensitive). */
    public listRegulatorySanctions(personId: string): Promise<Array<IRegulatorySanction>> {
        return this.bridge.call<Array<IRegulatorySanction>>(
            "PersonService",
            "listRegulatorySanctions",
            "GET",
            "/person/v1/persons/{personId}/regulatory-sanctions",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Add a regulatory sanction, or replace one when id is supplied. */
    public upsertRegulatorySanction(personId: string, request: IUpsertRegulatorySanctionRequest): Promise<IRegulatorySanction> {
        return this.bridge.call<IRegulatorySanction>(
            "PersonService",
            "upsertRegulatorySanction",
            "PUT",
            "/person/v1/persons/{personId}/regulatory-sanctions",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Remove a regulatory sanction by id. */
    public deleteRegulatorySanction(personId: string, sanctionId: string): Promise<void> {
        return this.bridge.call<void>(
            "PersonService",
            "deleteRegulatorySanction",
            "DELETE",
            "/person/v1/persons/{personId}/regulatory-sanctions/{sanctionId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                sanctionId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List a person's crypto-wallet attributions (D-PersonOverlays, M35; pii:sensitive). */
    public listCryptoWallets(personId: string): Promise<Array<ICryptoWallet>> {
        return this.bridge.call<Array<ICryptoWallet>>(
            "PersonService",
            "listCryptoWallets",
            "GET",
            "/person/v1/persons/{personId}/crypto-wallets",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Add a crypto wallet, or replace one when id is supplied. */
    public upsertCryptoWallet(personId: string, request: IUpsertCryptoWalletRequest): Promise<ICryptoWallet> {
        return this.bridge.call<ICryptoWallet>(
            "PersonService",
            "upsertCryptoWallet",
            "PUT",
            "/person/v1/persons/{personId}/crypto-wallets",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Remove a crypto wallet by id. */
    public deleteCryptoWallet(personId: string, walletId: string): Promise<void> {
        return this.bridge.call<void>(
            "PersonService",
            "deleteCryptoWallet",
            "DELETE",
            "/person/v1/persons/{personId}/crypto-wallets/{walletId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                walletId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List a person's declared/assessed personality profiles (D-PersonOverlays, M35; pii:sensitive). */
    public listPersonalities(personId: string): Promise<Array<IPersonality>> {
        return this.bridge.call<Array<IPersonality>>(
            "PersonService",
            "listPersonalities",
            "GET",
            "/person/v1/persons/{personId}/personalities",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Add a personality profile, or replace one when id is supplied. */
    public upsertPersonality(personId: string, request: IUpsertPersonalityRequest): Promise<IPersonality> {
        return this.bridge.call<IPersonality>(
            "PersonService",
            "upsertPersonality",
            "PUT",
            "/person/v1/persons/{personId}/personalities",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Remove a personality profile by id. */
    public deletePersonality(personId: string, personalityId: string): Promise<void> {
        return this.bridge.call<void>(
            "PersonService",
            "deletePersonality",
            "DELETE",
            "/person/v1/persons/{personId}/personalities/{personalityId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                personalityId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * The person's inferred political leaning, or null if none (D-PersonOverlays, M35; pii:special,
     * decrypted). Never a declared party affiliation.
     *
     */
    public getPoliticalLeaning(personId: string): Promise<IPoliticalLeaning | null> {
        return this.bridge.call<IPoliticalLeaning | null>(
            "PersonService",
            "getPoliticalLeaning",
            "GET",
            "/person/v1/persons/{personId}/political-leaning",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Set the person's inferred political leaning (replaces the single active row). Requires legalBasis. */
    public setPoliticalLeaning(personId: string, request: IUpsertPoliticalLeaningRequest): Promise<IPoliticalLeaning> {
        return this.bridge.call<IPoliticalLeaning>(
            "PersonService",
            "setPoliticalLeaning",
            "PUT",
            "/person/v1/persons/{personId}/political-leaning",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Remove the person's inferred political leaning. */
    public deletePoliticalLeaning(personId: string): Promise<void> {
        return this.bridge.call<void>(
            "PersonService",
            "deletePoliticalLeaning",
            "DELETE",
            "/person/v1/persons/{personId}/political-leaning",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * List the declared-ethnicity taxonomy (D-PhysicalIdentity amendment, M43). Optionally filter to
     * the forest roots (topLevel), the immediate children of a parent RID (parent, for lazy tree
     * expansion), or a name/code substring (query). `hasChildren` is set on each entry.
     *
     */
    public listEthnicityTypes(topLevel?: boolean | null, parent?: string | null, query?: string | null, limit?: number | null): Promise<Array<IEthnicityType>> {
        return this.bridge.call<Array<IEthnicityType>>(
            "PersonService",
            "listEthnicityTypes",
            "GET",
            "/person/v1/ethnicity-types",
            __undefined,
            __undefined,
            {
                "topLevel": topLevel,
                "parent": parent,
                "query": query,
                "limit": limit,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Fetch one ethnicity type by RID, including its group-level associated languages + homeland countries. */
    public getEthnicityType(ethnicityTypeId: string): Promise<IEthnicityType> {
        return this.bridge.call<IEthnicityType>(
            "PersonService",
            "getEthnicityType",
            "GET",
            "/person/v1/ethnicity-types/{ethnicityTypeId}",
            __undefined,
            __undefined,
            __undefined,
            [
                ethnicityTypeId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Add or update a declared-ethnicity catalog entry (instance-admin managed). */
    public upsertEthnicityType(request: IUpsertEthnicityTypeRequest): Promise<IEthnicityType> {
        return this.bridge.call<IEthnicityType>(
            "PersonService",
            "upsertEthnicityType",
            "PUT",
            "/person/v1/ethnicity-types",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** List a person's declared ethnicities with the value decrypted (D-PhysicalIdentity / D-SpecialPII). */
    public listEthnicities(personId: string): Promise<Array<IEthnicity>> {
        return this.bridge.call<Array<IEthnicity>>(
            "PersonService",
            "listEthnicities",
            "GET",
            "/person/v1/persons/{personId}/ethnicities",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Declare an ethnicity (envelope-encrypted, lawful-basis-gated). Returns Person:PersonInvalid for an unknown code/legal basis. */
    public addEthnicity(personId: string, request: IAddEthnicityRequest): Promise<IEthnicity> {
        return this.bridge.call<IEthnicity>(
            "PersonService",
            "addEthnicity",
            "POST",
            "/person/v1/persons/{personId}/ethnicities",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Re-declare the ethnicity value and/or flip legal basis / status. */
    public updateEthnicity(personId: string, ethnicityId: string, request: IUpdateEthnicityRequest): Promise<IEthnicity> {
        return this.bridge.call<IEthnicity>(
            "PersonService",
            "updateEthnicity",
            "PUT",
            "/person/v1/persons/{personId}/ethnicities/{ethnicityId}",
            request,
            __undefined,
            __undefined,
            [
                personId,
                ethnicityId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Remove a declared ethnicity by id. */
    public deleteEthnicity(personId: string, ethnicityId: string): Promise<void> {
        return this.bridge.call<void>(
            "PersonService",
            "deleteEthnicity",
            "DELETE",
            "/person/v1/persons/{personId}/ethnicities/{ethnicityId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                ethnicityId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List a person's citizenships. */
    public listCitizenships(personId: string): Promise<Array<ICitizenship>> {
        return this.bridge.call<Array<ICitizenship>>(
            "PersonService",
            "listCitizenships",
            "GET",
            "/person/v1/persons/{personId}/citizenships",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Add or replace the active citizenship for a country. Returns Person:PersonInvalid for an unknown country. */
    public upsertCitizenship(personId: string, request: IUpsertCitizenshipRequest): Promise<ICitizenship> {
        return this.bridge.call<ICitizenship>(
            "PersonService",
            "upsertCitizenship",
            "PUT",
            "/person/v1/persons/{personId}/citizenships",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Remove a citizenship by country RID. */
    public deleteCitizenship(personId: string, country: string): Promise<void> {
        return this.bridge.call<void>(
            "PersonService",
            "deleteCitizenship",
            "DELETE",
            "/person/v1/persons/{personId}/citizenships/{country}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                country,
            ],
            __undefined,
            __undefined
        );
    }

    /** List a person's residence history. */
    public listResidences(personId: string): Promise<Array<IResidence>> {
        return this.bridge.call<Array<IResidence>>(
            "PersonService",
            "listResidences",
            "GET",
            "/person/v1/persons/{personId}/residences",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Add a residence row, or replace one when id is supplied. Returns Person:PersonInvalid for an unknown country. */
    public upsertResidence(personId: string, request: IUpsertResidenceRequest): Promise<IResidence> {
        return this.bridge.call<IResidence>(
            "PersonService",
            "upsertResidence",
            "PUT",
            "/person/v1/persons/{personId}/residences",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Remove a residence row by id. */
    public deleteResidence(personId: string, residenceId: string): Promise<void> {
        return this.bridge.call<void>(
            "PersonService",
            "deleteResidence",
            "DELETE",
            "/person/v1/persons/{personId}/residences/{residenceId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                residenceId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List a person's contact emails. */
    public listEmails(personId: string): Promise<Array<IEmail>> {
        return this.bridge.call<Array<IEmail>>(
            "PersonService",
            "listEmails",
            "GET",
            "/person/v1/persons/{personId}/emails",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Add or replace a contact email. Returns Person:PersonConflict if the address is taken, Person:PersonInvalid for an unknown type or malformed address. */
    public upsertEmail(personId: string, request: IUpsertEmailRequest): Promise<IEmail> {
        return this.bridge.call<IEmail>(
            "PersonService",
            "upsertEmail",
            "PUT",
            "/person/v1/persons/{personId}/emails",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Remove a contact email by id. */
    public deleteEmail(personId: string, emailId: string): Promise<void> {
        return this.bridge.call<void>(
            "PersonService",
            "deleteEmail",
            "DELETE",
            "/person/v1/persons/{personId}/emails/{emailId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                emailId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List a person's contact phones. */
    public listPhones(personId: string): Promise<Array<IPhone>> {
        return this.bridge.call<Array<IPhone>>(
            "PersonService",
            "listPhones",
            "GET",
            "/person/v1/persons/{personId}/phones",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Add or replace a contact phone. Returns Person:PersonConflict if the number is taken, Person:PersonInvalid for an unknown type or unparseable number. */
    public upsertPhone(personId: string, request: IUpsertPhoneRequest): Promise<IPhone> {
        return this.bridge.call<IPhone>(
            "PersonService",
            "upsertPhone",
            "PUT",
            "/person/v1/persons/{personId}/phones",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Remove a contact phone by id. */
    public deletePhone(personId: string, phoneId: string): Promise<void> {
        return this.bridge.call<void>(
            "PersonService",
            "deletePhone",
            "DELETE",
            "/person/v1/persons/{personId}/phones/{phoneId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                phoneId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List a person's call signs. */
    public listCallSigns(personId: string): Promise<Array<ICallSign>> {
        return this.bridge.call<Array<ICallSign>>(
            "PersonService",
            "listCallSigns",
            "GET",
            "/person/v1/persons/{personId}/call-signs",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Add or replace a call sign. Returns Person:PersonConflict if the value is already held by the person. */
    public upsertCallSign(personId: string, request: IUpsertCallSignRequest): Promise<ICallSign> {
        return this.bridge.call<ICallSign>(
            "PersonService",
            "upsertCallSign",
            "PUT",
            "/person/v1/persons/{personId}/call-signs",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Remove a call sign by id. */
    public deleteCallSign(personId: string, callSignId: string): Promise<void> {
        return this.bridge.call<void>(
            "PersonService",
            "deleteCallSign",
            "DELETE",
            "/person/v1/persons/{personId}/call-signs/{callSignId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                callSignId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List the contact-email type catalog (locale -> text names; D-i18n). */
    public listEmailTypes(): Promise<Array<IEmailType>> {
        return this.bridge.call<Array<IEmailType>>(
            "PersonService",
            "listEmailTypes",
            "GET",
            "/person/v1/person/email-types",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** List the contact-phone type catalog (locale -> text names; D-i18n). */
    public listPhoneTypes(): Promise<Array<IPhoneType>> {
        return this.bridge.call<Array<IPhoneType>>(
            "PersonService",
            "listPhoneTypes",
            "GET",
            "/person/v1/person/phone-types",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** List the social/messenger platform catalog (locale -> text names; D-i18n; D-PersonSocialChannels). */
    public listPlatforms(): Promise<Array<IPlatform>> {
        return this.bridge.call<Array<IPlatform>>(
            "PersonService",
            "listPlatforms",
            "GET",
            "/person/v1/person/platforms",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** List a person's messenger reachability links. */
    public listMessengerLinks(personId: string): Promise<Array<IMessengerLink>> {
        return this.bridge.call<Array<IMessengerLink>>(
            "PersonService",
            "listMessengerLinks",
            "GET",
            "/person/v1/persons/{personId}/messenger-links",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * Add or replace a messenger link over one of the person's phones/emails. Returns
     * Person:PersonConflict if an active link for the channel+platform exists, Person:PersonInvalid
     * for an unknown / non-messenger platform or a channel not held by the person.
     *
     */
    public upsertMessengerLink(personId: string, request: IUpsertMessengerLinkRequest): Promise<IMessengerLink> {
        return this.bridge.call<IMessengerLink>(
            "PersonService",
            "upsertMessengerLink",
            "PUT",
            "/person/v1/persons/{personId}/messenger-links",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Remove a messenger link by id. */
    public deleteMessengerLink(personId: string, messengerLinkId: string): Promise<void> {
        return this.bridge.call<void>(
            "PersonService",
            "deleteMessengerLink",
            "DELETE",
            "/person/v1/persons/{personId}/messenger-links/{messengerLinkId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                messengerLinkId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List a person's standalone social accounts. */
    public listSocialAccounts(personId: string): Promise<Array<ISocialAccount>> {
        return this.bridge.call<Array<ISocialAccount>>(
            "PersonService",
            "listSocialAccounts",
            "GET",
            "/person/v1/persons/{personId}/social-accounts",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * Add or replace a social account. A handle rename is recorded in the account's handle history.
     * Returns Person:PersonConflict on a duplicate active account, Person:PersonInvalid for an
     * unknown platform or bad source/confidence.
     *
     */
    public upsertSocialAccount(personId: string, request: IUpsertSocialAccountRequest): Promise<ISocialAccount> {
        return this.bridge.call<ISocialAccount>(
            "PersonService",
            "upsertSocialAccount",
            "PUT",
            "/person/v1/persons/{personId}/social-accounts",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Remove a social account by id (its handle history cascades). */
    public deleteSocialAccount(personId: string, socialAccountId: string): Promise<void> {
        return this.bridge.call<void>(
            "PersonService",
            "deleteSocialAccount",
            "DELETE",
            "/person/v1/persons/{personId}/social-accounts/{socialAccountId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                socialAccountId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List one social account's @handle-rename history (most recent first). */
    public listSocialAccountHandles(personId: string, socialAccountId: string): Promise<Array<ISocialAccountHandle>> {
        return this.bridge.call<Array<ISocialAccountHandle>>(
            "PersonService",
            "listSocialAccountHandles",
            "GET",
            "/person/v1/persons/{personId}/social-accounts/{socialAccountId}/handles",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                socialAccountId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List the languages the person speaks (native first, then by name; D-Languages, M18). */
    public listPersonLanguages(personId: string): Promise<Array<IPersonLanguage>> {
        return this.bridge.call<Array<IPersonLanguage>>(
            "PersonService",
            "listPersonLanguages",
            "GET",
            "/person/v1/persons/{personId}/languages",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * Add or update a language the person speaks (keyed on languageId). Returns Person:PersonInvalid
     * when languageId does not resolve to a level='language' languoid or cefrLevel is invalid.
     *
     */
    public upsertPersonLanguage(personId: string, request: IUpsertPersonLanguageRequest): Promise<IPersonLanguage> {
        return this.bridge.call<IPersonLanguage>(
            "PersonService",
            "upsertPersonLanguage",
            "PUT",
            "/person/v1/persons/{personId}/languages",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Remove a language the person speaks, by languoid id. Idempotent within the active set. */
    public deletePersonLanguage(personId: string, languageId: string): Promise<void> {
        return this.bridge.call<void>(
            "PersonService",
            "deletePersonLanguage",
            "DELETE",
            "/person/v1/persons/{personId}/languages/{languageId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                languageId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List the person↔person relation-type catalog (locale -> text names; D-i18n; D-PersonRelationships). */
    public listRelationTypes(): Promise<Array<IRelationType>> {
        return this.bridge.call<Array<IRelationType>>(
            "PersonService",
            "listRelationTypes",
            "GET",
            "/person/v1/person/relation-types",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** List partnerships (marriage/engagement) touching the person. */
    public listPartnerships(personId: string): Promise<Array<IPartnership>> {
        return this.bridge.call<Array<IPartnership>>(
            "PersonService",
            "listPartnerships",
            "GET",
            "/person/v1/persons/{personId}/partnerships",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * Add or replace a partnership between the person and the partner. Returns Person:PersonConflict
     * when either person already has an active engaged/married partnership, Person:PersonInvalid for a
     * self-pair, unknown partner, or bad status.
     *
     */
    public upsertPartnership(personId: string, request: IUpsertPartnershipRequest): Promise<IPartnership> {
        return this.bridge.call<IPartnership>(
            "PersonService",
            "upsertPartnership",
            "PUT",
            "/person/v1/persons/{personId}/partnerships",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List parent/child kinships touching the person. */
    public listKinships(personId: string): Promise<Array<IKinship>> {
        return this.bridge.call<Array<IKinship>>(
            "PersonService",
            "listKinships",
            "GET",
            "/person/v1/persons/{personId}/kinships",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Add or replace a parent→child kinship. Returns Person:PersonConflict on a duplicate active pair, Person:PersonInvalid for a self-edge, unknown counterpart, or bad role. */
    public upsertKinship(personId: string, request: IUpsertKinshipRequest): Promise<IKinship> {
        return this.bridge.call<IKinship>(
            "PersonService",
            "upsertKinship",
            "PUT",
            "/person/v1/persons/{personId}/kinships",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List guardianships touching the person. */
    public listGuardianships(personId: string): Promise<Array<IGuardianship>> {
        return this.bridge.call<Array<IGuardianship>>(
            "PersonService",
            "listGuardianships",
            "GET",
            "/person/v1/persons/{personId}/guardianships",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Add or replace a guardian→ward link. Returns Person:PersonInvalid for a self-edge, unknown counterpart, unknown relation code, or bad role. */
    public upsertGuardianship(personId: string, request: IUpsertGuardianshipRequest): Promise<IGuardianship> {
        return this.bridge.call<IGuardianship>(
            "PersonService",
            "upsertGuardianship",
            "PUT",
            "/person/v1/persons/{personId}/guardianships",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List sponsorships (godparent/advisor/mentor) touching the person. */
    public listSponsorships(personId: string): Promise<Array<ISponsorship>> {
        return this.bridge.call<Array<ISponsorship>>(
            "PersonService",
            "listSponsorships",
            "GET",
            "/person/v1/persons/{personId}/sponsorships",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Add or replace a sponsor→sponsored link. relationCode must be a category=sponsorship code. Returns Person:PersonInvalid for a self-edge, unknown counterpart, or wrong relation category. */
    public upsertSponsorship(personId: string, request: IUpsertSponsorshipRequest): Promise<ISponsorship> {
        return this.bridge.call<ISponsorship>(
            "PersonService",
            "upsertSponsorship",
            "PUT",
            "/person/v1/persons/{personId}/sponsorships",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List next-of-kin nominations touching the person (priority-ordered). */
    public listNextOfKin(personId: string): Promise<Array<INextOfKin>> {
        return this.bridge.call<Array<INextOfKin>>(
            "PersonService",
            "listNextOfKin",
            "GET",
            "/person/v1/persons/{personId}/next-of-kin",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Nominate or replace a next-of-kin contact for the person. Returns Person:PersonInvalid for a self-nomination, unknown contact, or wrong relation category. */
    public upsertNextOfKin(personId: string, request: IUpsertNextOfKinRequest): Promise<INextOfKin> {
        return this.bridge.call<INextOfKin>(
            "PersonService",
            "upsertNextOfKin",
            "PUT",
            "/person/v1/persons/{personId}/next-of-kin",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List associations (associate/COI/no-contact) touching the person. */
    public listAssociations(personId: string): Promise<Array<IAssociation>> {
        return this.bridge.call<Array<IAssociation>>(
            "PersonService",
            "listAssociations",
            "GET",
            "/person/v1/persons/{personId}/associations",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Add or replace a symmetric association. Returns Person:PersonInvalid for a self-pair, unknown counterpart, wrong relation category, or bad kind. */
    public upsertAssociation(personId: string, request: IUpsertAssociationRequest): Promise<IAssociation> {
        return this.bridge.call<IAssociation>(
            "PersonService",
            "upsertAssociation",
            "PUT",
            "/person/v1/persons/{personId}/associations",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * Remove any person↔person link by id (the link type is decoded from the RID). The path person
     * must be one of the link's endpoints. Idempotent.
     *
     */
    public deleteRelationship(personId: string, relationshipId: string): Promise<void> {
        return this.bridge.call<void>(
            "PersonService",
            "deleteRelationship",
            "DELETE",
            "/person/v1/persons/{personId}/relationships/{relationshipId}",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
                relationshipId,
            ],
            __undefined,
            __undefined
        );
    }
}
