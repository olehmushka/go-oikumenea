export interface IUpsertDormitoryStayRequest {
    'buildingId': string;
    'room'?: string | null;
    'status'?: string | null;
    'effectiveFrom'?: string | null;
    'effectiveTo'?: string | null;
}
