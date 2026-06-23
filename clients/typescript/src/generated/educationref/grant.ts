/** A funding grant held by an institution. */
export interface IGrant {
    'id': string;
    'institutionId': string;
    'code': string;
    'title': string;
    'funder'?: string | null;
    'funderRef'?: string | null;
    'amount'?: string | null;
    'currency'?: string | null;
    'startOn'?: string | null;
    'endOn'?: string | null;
    'status': string;
    'createdAt': string;
    'updatedAt': string;
}
