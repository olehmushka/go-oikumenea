/** A person RESIDED_IN_DORMITORY — a dedicated stay (person ↔ dorm building, room, period). */
export interface IDormitoryStay {
    'id': string;
    'personId': string;
    'buildingId': string;
    'room'?: string | null;
    /** One of active | ended. */
    'status': string;
    'effectiveFrom'?: string | null;
    'effectiveTo'?: string | null;
    'createdAt': string;
    'updatedAt': string;
}
