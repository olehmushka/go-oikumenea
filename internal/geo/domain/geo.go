// Package domain holds the geo module's pure logic: the Country registry entry and the Repository
// port it needs from the outside world (overview.md layering). No I/O, no framework imports — only
// the standard library. Geo owns the read side of the location service's country registry (D-Geo):
// countries are RID-keyed (F-014 / D-ResourceIdentifiers) and referenced by person/document/rank;
// this module lets clients resolve a country to its RID. The registry itself is written by the
// hermenea import pipeline (geo-countries / WOF), not here.
package domain

import "context"

// Country is one entry in the ISO-3166-1 registry. ID is the RID (the reference key other modules
// store); Code is the stable ISO-3166-1 alpha-2 lookup code; Name is the default-locale name.
type Country struct {
	ID     string
	Code   string
	Name   string
	Status string
}

// Repository is the geo module's port: a read-only view of the country registry.
type Repository interface {
	ListCountries(ctx context.Context) ([]Country, error)
}
