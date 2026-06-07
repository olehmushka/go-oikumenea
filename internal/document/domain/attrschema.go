package domain

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"
)

// Per-document-type attribute schema (D-DocumentAttrSchema). A document type may declare, in its
// optional attr_schema JSONB, the fields a document's `attributes` may/must carry; a document write is
// then validated against it. The schema shape is deliberately minimal (a field-spec map, not full
// JSON-Schema), validated by standard-library code in the spirit of pkg/personalcode:
//
//	{ "fields": { "<name>": { "type": "string|number|boolean|date", "required": bool, "enum": [...]? } } }
//
// When a type has no attr_schema, `attributes` is free-form.

// attrFieldType enumerates the value kinds a schema field may declare.
var attrFieldTypes = map[string]bool{"string": true, "number": true, "boolean": true, "date": true}

type attrFieldSpec struct {
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Enum     []string `json:"enum,omitempty"`
}

type attrSchemaDoc struct {
	Fields map[string]attrFieldSpec `json:"fields"`
}

// ValidateAttrSchema checks that a raw attr_schema is well-formed (parses, every field declares a
// known type, enums are only on string/date fields). Empty/NULL is valid (no schema). Used when a
// type's schema is set via create/update so a malformed schema is rejected at the boundary.
func ValidateAttrSchema(raw json.RawMessage) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var doc attrSchemaDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return wrapInvalid(ErrDocumentInvalid, "attrSchema is not valid JSON of shape {fields:{name:{type,required,enum?}}}")
	}
	for name, spec := range doc.Fields {
		if !attrFieldTypes[spec.Type] {
			return wrapInvalid(ErrDocumentInvalid, fmt.Sprintf("attrSchema field %q has unknown type %q (want string|number|boolean|date)", name, spec.Type))
		}
		if len(spec.Enum) > 0 && spec.Type != "string" && spec.Type != "date" {
			return wrapInvalid(ErrDocumentInvalid, fmt.Sprintf("attrSchema field %q: enum is only allowed on string/date fields", name))
		}
	}
	return nil
}

// ValidateAttributes validates a document's `attributes` against its type's attr_schema
// (D-DocumentAttrSchema): unknown keys are rejected, required keys enforced, and each present value's
// declared type / enum checked. A nil/empty schema accepts any attributes (free-form). Violations are
// ErrDocumentInvalid.
func ValidateAttributes(schemaRaw, attrsRaw json.RawMessage) error {
	if len(schemaRaw) == 0 || string(schemaRaw) == "null" {
		return nil
	}
	var schema attrSchemaDoc
	if err := json.Unmarshal(schemaRaw, &schema); err != nil {
		// A stored schema that no longer parses is treated as no constraint rather than blocking writes.
		return nil
	}

	attrs := map[string]json.RawMessage{}
	if len(attrsRaw) > 0 && string(attrsRaw) != "null" {
		if err := json.Unmarshal(attrsRaw, &attrs); err != nil {
			return wrapInvalid(ErrDocumentInvalid, "attributes must be a JSON object")
		}
	}

	for key := range attrs {
		if _, ok := schema.Fields[key]; !ok {
			return wrapInvalid(ErrDocumentInvalid, fmt.Sprintf("attribute %q is not declared by the document type's schema", key))
		}
	}
	for name, spec := range schema.Fields {
		raw, present := attrs[name]
		if !present {
			if spec.Required {
				return wrapInvalid(ErrDocumentInvalid, fmt.Sprintf("attribute %q is required by the document type's schema", name))
			}
			continue
		}
		if err := checkAttrValue(name, spec, raw); err != nil {
			return err
		}
	}
	return nil
}

// checkAttrValue validates a single present attribute value against its field spec.
func checkAttrValue(name string, spec attrFieldSpec, raw json.RawMessage) error {
	switch spec.Type {
	case "string", "date":
		var v string
		if err := json.Unmarshal(raw, &v); err != nil {
			return wrapInvalid(ErrDocumentInvalid, fmt.Sprintf("attribute %q must be a %s", name, spec.Type))
		}
		if spec.Type == "date" {
			if _, err := time.Parse("2006-01-02", v); err != nil {
				return wrapInvalid(ErrDocumentInvalid, fmt.Sprintf("attribute %q must be an ISO-8601 date (YYYY-MM-DD)", name))
			}
		}
		if len(spec.Enum) > 0 && !slices.Contains(spec.Enum, v) {
			return wrapInvalid(ErrDocumentInvalid, fmt.Sprintf("attribute %q must be one of the allowed values", name))
		}
	case "number":
		var v float64
		if err := json.Unmarshal(raw, &v); err != nil {
			return wrapInvalid(ErrDocumentInvalid, fmt.Sprintf("attribute %q must be a number", name))
		}
	case "boolean":
		var v bool
		if err := json.Unmarshal(raw, &v); err != nil {
			return wrapInvalid(ErrDocumentInvalid, fmt.Sprintf("attribute %q must be a boolean", name))
		}
	}
	return nil
}
