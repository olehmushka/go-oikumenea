export interface IRegisterServicePrincipalRequest {
    'code': string;
    'name': string;
    'description'?: string | null;
    /** The IdP `iss` the machine's tokens carry. */
    'issuer': string;
    /**
     * The IdP `sub` the machine's tokens carry. For Keycloak this is the service-account user
     * id; a rejected unknown token logs its issuer/subject so it can be copied from there.
     *
     */
    'subject': string;
    'clientId'?: string | null;
}
