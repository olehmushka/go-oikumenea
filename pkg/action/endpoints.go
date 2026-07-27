// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package action

// Endpoint is the HTTP endpoint an action invocation targets — the invocation counterpart of Params
// (review-2026-09 R-33). It is DERIVED from the Conjure IR by tools/genactionendpoints (see
// endpoints_gen.go), never hand-written, so it cannot drift from the contract: the generator fails the
// build if an action does not bind to exactly one real endpoint.
type Endpoint struct {
	Method string // GET/POST/PUT/DELETE
	Path   string // Conjure path template, e.g. "/person/v1/persons/{personId}/emails/{emailId}"
	// PathParams are the path parameter names in path order. By convention the first is the target
	// object's own RID (the object the action runs on); any remainder are sub-resource ids the caller
	// must supply (e.g. the email id of a person.email.upsert update). A create has only the parent, a
	// top-level create none.
	PathParams []string
}

// EndpointFor returns the endpoint binding for an action code (zero, false when the code is unknown).
// Backs the httpMethod/httpPath fields on AuditService.listActionTypes and the console's action runner.
func EndpointFor(code string) (Endpoint, bool) {
	e, ok := actionEndpoints[code]
	return e, ok
}
