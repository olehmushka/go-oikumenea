/** A person authored a publication. */
export interface IPublicationAuthorship {
    'id': string;
    'personId': string;
    'publicationId': string;
    'authorOrder'?: number | null;
    'corresponding': boolean;
    'effectiveFrom'?: string | null;
    'effectiveTo'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
