/** One period in a social account's @handle-rename history (D-PersonSocialChannels). validTo null = current. */
export interface ISocialAccountHandle {
    'id': string;
    'accountId': string;
    'handle': string;
    'validFrom': string;
    /** When this handle stopped being current; null = current. */
    'validTo'?: string | null;
}
