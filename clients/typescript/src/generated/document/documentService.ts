import { ICreateDocumentRequest } from "./createDocumentRequest";
import { ICreateDocumentTypeRequest } from "./createDocumentTypeRequest";
import { ICreatePersonalCodeRequest } from "./createPersonalCodeRequest";
import { ICreatePersonalCodeSchemeRequest } from "./createPersonalCodeSchemeRequest";
import { IDocument } from "./document";
import { IDocumentPage } from "./documentPage";
import { IDocumentType } from "./documentType";
import { IPersonalCode } from "./personalCode";
import { IPersonalCodeScheme } from "./personalCodeScheme";
import { IUpdateDocumentRequest } from "./updateDocumentRequest";
import { IUpdateDocumentTypeRequest } from "./updateDocumentTypeRequest";
import { IUpdatePersonalCodeRequest } from "./updatePersonalCodeRequest";
import { IUpdatePersonalCodeSchemeRequest } from "./updatePersonalCodeSchemeRequest";
import type { IHttpApiBridge } from "conjure-client";

/** Constant reference to `undefined` that we expect to get minified and therefore reduce total code size */
const __undefined: undefined = undefined;

/**
 * Person-held papers and government personal codes (D-Documents / D-PersonalCodes). Documents and
 * codes are scoped THROUGH THE HOLDER (D-PersonReadScope) + the shadow gate; type/scheme catalogs
 * are instance-admin-managed. Personal-code values are envelope-encrypted at rest (D-CryptoProvider)
 * and validated against their scheme on write. Writes are audited in-process (D-Audit).
 *
 */
export interface IDocumentService {
    /** Attach a paper to a person. Returns Document:DocumentConflict on a duplicate (type, number). */
    attachDocument(personId: string, request: ICreateDocumentRequest): Promise<IDocument>;
    /** List a person's documents, token-paginated. */
    listPersonDocuments(personId: string, pageSize?: number | null, pageToken?: string | null): Promise<IDocumentPage>;
    /** Read one document. */
    getDocument(documentId: string): Promise<IDocument>;
    /** Update number/issuer/issuing-country/validity/attributes/status. Omitted fields are unchanged. */
    updateDocument(documentId: string, request: IUpdateDocumentRequest): Promise<IDocument>;
    /** Soft-delete (reversible) a document. */
    deleteDocument(documentId: string): Promise<void>;
    /** List the paper-type catalog. */
    listDocumentTypes(): Promise<Array<IDocumentType>>;
    /** Add a document type (instance-scope). Returns Document:DocumentTypeConflict if the code is taken. */
    createDocumentType(request: ICreateDocumentTypeRequest): Promise<IDocumentType>;
    /** Edit a document type (instance-scope). `code` is immutable by convention. */
    updateDocumentType(typeId: string, request: IUpdateDocumentTypeRequest): Promise<IDocumentType>;
    /**
     * Attach a national-identifier code to a person. The value is validated against the scheme
     * (Document:PersonalCodeInvalid on failure) and encrypted; a duplicate (scheme, value) returns
     * Document:PersonalCodeDuplicate.
     *
     */
    attachPersonalCode(personId: string, request: ICreatePersonalCodeRequest): Promise<IPersonalCode>;
    /** List a person's personal codes (values decrypted on read; a sensitive action). */
    listPersonPersonalCodes(personId: string): Promise<Array<IPersonalCode>>;
    /** Update a personal code (value re-validated + re-encrypted, and/or status). */
    updatePersonalCode(codeId: string, request: IUpdatePersonalCodeRequest): Promise<IPersonalCode>;
    /** Soft-delete a personal code. */
    deletePersonalCode(codeId: string): Promise<void>;
    /** List the scheme catalog, optionally filtered by country RID (GET /geo/countries) and/or generic category. */
    listPersonalCodeSchemes(country?: string | null, category?: string | null): Promise<Array<IPersonalCodeScheme>>;
    /** Add a scheme (instance-scope). Returns Document:PersonalCodeSchemeConflict if the code is taken. */
    createPersonalCodeScheme(request: ICreatePersonalCodeSchemeRequest): Promise<IPersonalCodeScheme>;
    /** Edit a scheme (instance-scope). `code` is immutable by convention. */
    updatePersonalCodeScheme(schemeCode: string, request: IUpdatePersonalCodeSchemeRequest): Promise<IPersonalCodeScheme>;
}

export class DocumentService implements IDocumentService {
    constructor(private bridge: IHttpApiBridge) {
    }

    /** Attach a paper to a person. Returns Document:DocumentConflict on a duplicate (type, number). */
    public attachDocument(personId: string, request: ICreateDocumentRequest): Promise<IDocument> {
        return this.bridge.call<IDocument>(
            "DocumentService",
            "attachDocument",
            "POST",
            "/document/v1/persons/{personId}/documents",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List a person's documents, token-paginated. */
    public listPersonDocuments(personId: string, pageSize?: number | null, pageToken?: string | null): Promise<IDocumentPage> {
        return this.bridge.call<IDocumentPage>(
            "DocumentService",
            "listPersonDocuments",
            "GET",
            "/document/v1/persons/{personId}/documents",
            __undefined,
            __undefined,
            {
                "pageSize": pageSize,
                "pageToken": pageToken,
            },
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Read one document. */
    public getDocument(documentId: string): Promise<IDocument> {
        return this.bridge.call<IDocument>(
            "DocumentService",
            "getDocument",
            "GET",
            "/document/v1/documents/{documentId}",
            __undefined,
            __undefined,
            __undefined,
            [
                documentId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Update number/issuer/issuing-country/validity/attributes/status. Omitted fields are unchanged. */
    public updateDocument(documentId: string, request: IUpdateDocumentRequest): Promise<IDocument> {
        return this.bridge.call<IDocument>(
            "DocumentService",
            "updateDocument",
            "PUT",
            "/document/v1/documents/{documentId}",
            request,
            __undefined,
            __undefined,
            [
                documentId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Soft-delete (reversible) a document. */
    public deleteDocument(documentId: string): Promise<void> {
        return this.bridge.call<void>(
            "DocumentService",
            "deleteDocument",
            "DELETE",
            "/document/v1/documents/{documentId}",
            __undefined,
            __undefined,
            __undefined,
            [
                documentId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List the paper-type catalog. */
    public listDocumentTypes(): Promise<Array<IDocumentType>> {
        return this.bridge.call<Array<IDocumentType>>(
            "DocumentService",
            "listDocumentTypes",
            "GET",
            "/document/v1/document-types",
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Add a document type (instance-scope). Returns Document:DocumentTypeConflict if the code is taken. */
    public createDocumentType(request: ICreateDocumentTypeRequest): Promise<IDocumentType> {
        return this.bridge.call<IDocumentType>(
            "DocumentService",
            "createDocumentType",
            "POST",
            "/document/v1/document-types",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Edit a document type (instance-scope). `code` is immutable by convention. */
    public updateDocumentType(typeId: string, request: IUpdateDocumentTypeRequest): Promise<IDocumentType> {
        return this.bridge.call<IDocumentType>(
            "DocumentService",
            "updateDocumentType",
            "PUT",
            "/document/v1/document-types/{typeId}",
            request,
            __undefined,
            __undefined,
            [
                typeId,
            ],
            __undefined,
            __undefined
        );
    }

    /**
     * Attach a national-identifier code to a person. The value is validated against the scheme
     * (Document:PersonalCodeInvalid on failure) and encrypted; a duplicate (scheme, value) returns
     * Document:PersonalCodeDuplicate.
     *
     */
    public attachPersonalCode(personId: string, request: ICreatePersonalCodeRequest): Promise<IPersonalCode> {
        return this.bridge.call<IPersonalCode>(
            "DocumentService",
            "attachPersonalCode",
            "POST",
            "/document/v1/persons/{personId}/personal-codes",
            request,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List a person's personal codes (values decrypted on read; a sensitive action). */
    public listPersonPersonalCodes(personId: string): Promise<Array<IPersonalCode>> {
        return this.bridge.call<Array<IPersonalCode>>(
            "DocumentService",
            "listPersonPersonalCodes",
            "GET",
            "/document/v1/persons/{personId}/personal-codes",
            __undefined,
            __undefined,
            __undefined,
            [
                personId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Update a personal code (value re-validated + re-encrypted, and/or status). */
    public updatePersonalCode(codeId: string, request: IUpdatePersonalCodeRequest): Promise<IPersonalCode> {
        return this.bridge.call<IPersonalCode>(
            "DocumentService",
            "updatePersonalCode",
            "PUT",
            "/document/v1/personal-codes/{codeId}",
            request,
            __undefined,
            __undefined,
            [
                codeId,
            ],
            __undefined,
            __undefined
        );
    }

    /** Soft-delete a personal code. */
    public deletePersonalCode(codeId: string): Promise<void> {
        return this.bridge.call<void>(
            "DocumentService",
            "deletePersonalCode",
            "DELETE",
            "/document/v1/personal-codes/{codeId}",
            __undefined,
            __undefined,
            __undefined,
            [
                codeId,
            ],
            __undefined,
            __undefined
        );
    }

    /** List the scheme catalog, optionally filtered by country RID (GET /geo/countries) and/or generic category. */
    public listPersonalCodeSchemes(country?: string | null, category?: string | null): Promise<Array<IPersonalCodeScheme>> {
        return this.bridge.call<Array<IPersonalCodeScheme>>(
            "DocumentService",
            "listPersonalCodeSchemes",
            "GET",
            "/document/v1/personal-code-schemes",
            __undefined,
            __undefined,
            {
                "country": country,
                "category": category,
            },
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Add a scheme (instance-scope). Returns Document:PersonalCodeSchemeConflict if the code is taken. */
    public createPersonalCodeScheme(request: ICreatePersonalCodeSchemeRequest): Promise<IPersonalCodeScheme> {
        return this.bridge.call<IPersonalCodeScheme>(
            "DocumentService",
            "createPersonalCodeScheme",
            "POST",
            "/document/v1/personal-code-schemes",
            request,
            __undefined,
            __undefined,
            __undefined,
            __undefined,
            __undefined
        );
    }

    /** Edit a scheme (instance-scope). `code` is immutable by convention. */
    public updatePersonalCodeScheme(schemeCode: string, request: IUpdatePersonalCodeSchemeRequest): Promise<IPersonalCodeScheme> {
        return this.bridge.call<IPersonalCodeScheme>(
            "DocumentService",
            "updatePersonalCodeScheme",
            "PUT",
            "/document/v1/personal-code-schemes/{schemeCode}",
            request,
            __undefined,
            __undefined,
            [
                schemeCode,
            ],
            __undefined,
            __undefined
        );
    }
}
