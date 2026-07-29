// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

// Package domain holds the language module's pure logic: the Languoid + WritingSystem registry entries
// and the Repository port it needs (overview.md layering). No I/O, no framework imports — only the
// standard library. Language owns the READ side of the Glottolog languoid forest + ISO-15924 writing
// systems (D-Languages, M18); the registry itself is written by the hermenea import pipeline
// (language-scheme / language-scripts), not here.
package domain

import (
	"context"
)

// Languoid is one node in the Glottolog forest. ID is the RID (the reference key person/unit/locale
// links store); Code is the stable glottocode; optional fields fold to "" when absent.
type Languoid struct {
	ID          string
	Code        string
	Level       string
	Name        string
	ParentID    string
	HasChildren bool
	FamilyCode  string
	ISO639_3    string
	Macroarea   string
	Status      string
}

// WritingSystem is one ISO-15924 script. ID is the RID; Code is the ISO-15924 lookup code.
type WritingSystem struct {
	ID         string
	Code       string
	Name       string
	ScriptType string
}

// Filter narrows a languoid listing (empty / false fields disable each criterion; Limit is clamped
// upstream). Parent restricts to the immediate children of a languoid RID (one tree level); TopLevel
// restricts to the forest roots (no parent). The two combine with the level/family/query criteria.
type Filter struct {
	Level    string
	Family   string
	Parent   string
	TopLevel bool
	Query    string
	Limit    int
	// After is a keyset cursor: when non-empty, only languoids whose code sorts strictly after it are
	// returned (the list is ordered by code). Empty disables the criterion (first page).
	After string
}

// Repository is the language module's port: a read-only view of the languoid + writing-system registry.
type Repository interface {
	ListLanguoids(ctx context.Context, f Filter) ([]Languoid, error)
	GetLanguoid(ctx context.Context, id string) (Languoid, bool, error)
	ListWritingSystems(ctx context.Context) ([]WritingSystem, error)
}
