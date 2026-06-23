import { ICallSign } from "./callSign";
import { ICitizenship } from "./citizenship";
import { IEmail } from "./email";
import { IMessengerLink } from "./messengerLink";
import { INameVariant } from "./nameVariant";
import { IPersonRank } from "./personRank";
import { IPhone } from "./phone";
import { IResidence } from "./residence";
import { ISocialAccount } from "./socialAccount";

/**
 * An individual personnel record. Names follow the Unicode CLDR fixed field set
 * (D-PersonNamesCLDR): displayName is authoritative, the structured parts are advisory. There
 * is no patronymic field — the Slavic по-батькові lives in given2. nameVariants/citizenships/
 * residences are populated by getPerson and empty in list responses.
 *
 */
export interface IPerson {
    /** The person's URN RID (carried as a plain string). */
    'id': string;
    /** Optional stable, locale-agnostic external id (e.g. personnel/service number); unique among active persons. */
    'code'?: string | null;
    /** The canonical full name form; authoritative for search/display. */
    'displayName': string;
    'title'?: string | null;
    'given'?: string | null;
    /** Second given name; also holds the Slavic по-батькові / Icelandic patronymic. */
    'given2'?: string | null;
    'surname'?: string | null;
    /** Nobiliary / genealogical particle (van, von, de, bin). */
    'surnamePrefix'?: string | null;
    /** Second surname (Hispanic / Lusophone). */
    'surname2'?: string | null;
    /** Generational suffix (Jr., Sr., III). */
    'generation'?: string | null;
    /** Post-nominal credentials (PhD, MD). */
    'credentials'?: string | null;
    /** Known-as / nickname. */
    'preferred'?: string | null;
    /** ISO-8601 calendar date of birth (YYYY-MM-DD); a day, not an instant. */
    'birthdate'?: string | null;
    /** ISO-8601 calendar date of death (YYYY-MM-DD); a bio attribute, not a lifecycle state — a deceased person stays an active record (D-PersonBio). */
    'dateOfDeath'?: string | null;
    /** Biological sex, ISO/IEC 5218 as text — one of not_known | male | female | not_applicable. */
    'sex': string;
    /** Country RID (resolve via GET /geo/countries), validated against the geo registry (D-Geo). */
    'countryOfBirth'?: string | null;
    /** Free-form long-tail directory fields (JSONB). pii:special ceiling — no special-category data without the envelope seam. */
    'attributes'?: any | null;
    /** The ranks the person holds — at most one per rank system (a DIRECTORY attribute; never an authz input — D-Rank). Empty when unranked. Populated by getPerson; empty in list responses. */
    'ranks': Array<IPersonRank>;
    /** Lifecycle status — one of active | deactivated | purged. */
    'status': string;
    'deactivatedAt'?: string | null;
    /** End of the reversibility window; purge is refused before it. */
    'purgeAfter'?: string | null;
    'createdAt': string;
    'updatedAt': string;
    /** Locale-tagged transliterations. Populated by getPerson; empty in list responses. */
    'nameVariants': Array<INameVariant>;
    /** The person's citizenships. Populated by getPerson; empty in list responses. */
    'citizenships': Array<ICitizenship>;
    /** The person's residence history. Populated by getPerson; empty in list responses. */
    'residences': Array<IResidence>;
    /** The person's contact emails. Populated by getPerson; empty in list responses. */
    'emails': Array<IEmail>;
    /** The person's contact phones. Populated by getPerson; empty in list responses. */
    'phones': Array<IPhone>;
    /** The person's call signs. Populated by getPerson; empty in list responses. */
    'callSigns': Array<ICallSign>;
    /** The person's messenger reachability links. Populated by getPerson; empty in list responses. */
    'messengerLinks': Array<IMessengerLink>;
    /** The person's standalone social accounts. Populated by getPerson; empty in list responses. */
    'socialAccounts': Array<ISocialAccount>;
}
