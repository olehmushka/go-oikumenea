import { IAssociation } from "./association";
import { ICallSign } from "./callSign";
import { ICitizenship } from "./citizenship";
import { ICreatePersonRequest } from "./createPersonRequest";
import { ICreateProvisionalPersonRequest } from "./createProvisionalPersonRequest";
import { IDeactivateRequest } from "./deactivateRequest";
import { IEmail } from "./email";
import { IEmailType } from "./emailType";
import { IGuardianship } from "./guardianship";
import { IKinship } from "./kinship";
import { IMergePersonRequest } from "./mergePersonRequest";
import { IMessengerLink } from "./messengerLink";
import { INameVariant } from "./nameVariant";
import { INextOfKin } from "./nextOfKin";
import { IPartnership } from "./partnership";
import { IPerson } from "./person";
import { IPersonLanguage } from "./personLanguage";
import { IPersonPage } from "./personPage";
import { IPhone } from "./phone";
import { IPhoneType } from "./phoneType";
import { IPlatform } from "./platform";
import { IRelationType } from "./relationType";
import { IResidence } from "./residence";
import { ISetRankRequest } from "./setRankRequest";
import { ISocialAccount } from "./socialAccount";
import { ISocialAccountHandle } from "./socialAccountHandle";
import { ISponsorship } from "./sponsorship";
import { IUpdatePersonRequest } from "./updatePersonRequest";
import { IUpsertAssociationRequest } from "./upsertAssociationRequest";
import { IUpsertCallSignRequest } from "./upsertCallSignRequest";
import { IUpsertCitizenshipRequest } from "./upsertCitizenshipRequest";
import { IUpsertEmailRequest } from "./upsertEmailRequest";
import { IUpsertGuardianshipRequest } from "./upsertGuardianshipRequest";
import { IUpsertKinshipRequest } from "./upsertKinshipRequest";
import { IUpsertMessengerLinkRequest } from "./upsertMessengerLinkRequest";
import { IUpsertNameVariantRequest } from "./upsertNameVariantRequest";
import { IUpsertNextOfKinRequest } from "./upsertNextOfKinRequest";
import { IUpsertPartnershipRequest } from "./upsertPartnershipRequest";
import { IUpsertPersonLanguageRequest } from "./upsertPersonLanguageRequest";
import { IUpsertPhoneRequest } from "./upsertPhoneRequest";
import { IUpsertResidenceRequest } from "./upsertResidenceRequest";
import { IUpsertSocialAccountRequest } from "./upsertSocialAccountRequest";
import { IUpsertSponsorshipRequest } from "./upsertSponsorshipRequest";
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
    /** Remove a name variant. */
    deleteNameVariant(personId: string, locale: string): Promise<void>;
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

    /** Remove a name variant. */
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
