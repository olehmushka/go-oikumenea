/** A company's economic-activity classification (M:N), one primary + secondaries. */
export interface IIndustryAssignment {
    'id': string;
    'companyId': string;
    'industryClassId': string;
    'isPrimary': boolean;
    'createdAt': string;
    'updatedAt': string;
}
