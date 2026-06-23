/** Record a founder of this company (person or company holder). */
export interface IRecordFoundingRequest {
    'holderKind': string;
    'holderId': string;
    'foundedOn'?: string | null;
}
