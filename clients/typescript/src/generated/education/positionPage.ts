import { IEducationPosition } from "./educationPosition";

export interface IPositionPage {
    'positions': Array<IEducationPosition>;
    'nextPageToken'?: string | null;
}
