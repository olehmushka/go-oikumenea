// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Differential test for the set-based geo-places merge (R-05): random batches of synthetic places —
// creates, edition bumps, no-op re-imports, in-batch duplicates, intra-chunk parent references (child
// BEFORE its parent in the same chunk), and chunked envelopes — are applied through the real
// handler/merge SQL, while a naive in-Go oracle replays the documented per-record semantics. After
// every import the DB table state (name, source_version, status, resolved parent) and the returned
// Summary must equal the oracle exactly. Mirrors the M48 property-test style (oracle ≡ optimized).
package dataimport_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/olehmushka/go-oikumenea/internal/dataimport/application"
	"github.com/olehmushka/go-oikumenea/internal/dataimport/domain"
)

// diffWofBase reserves a synthetic WOF-id range no real dataset uses, so the test owns its rows.
const diffWofBase int64 = 990_000_000

// diffWofEnd bounds the range (other suites park unrelated synthetic rows at random giant wof_ids).
const diffWofEnd int64 = diffWofBase + 1_000_000

type oraclePlace struct {
	version   string
	name      string
	status    string
	parentWof int64 // 0 = none
}

// oracleApply replays one chunk with the documented per-record semantics (last-duplicate-wins within
// a chunk; insert absent, update on a different edition, skip otherwise) and returns the Summary the
// handler must report.
func oracleApply(state map[int64]*oraclePlace, recs []geoRec, version string) domain.Summary {
	var sum domain.Summary
	dedup := make(map[int64]geoRec, len(recs))
	order := make([]int64, 0, len(recs))
	for _, r := range recs {
		if _, ok := dedup[r.wofID]; ok {
			sum.Skipped++
		} else {
			order = append(order, r.wofID)
		}
		dedup[r.wofID] = r
	}
	for _, id := range order {
		r := dedup[id]
		cur, ok := state[id]
		switch {
		case !ok:
			state[id] = &oraclePlace{version: version, name: r.name, status: r.status, parentWof: r.parentWof}
			sum.Created++
		case cur.version != version:
			*cur = oraclePlace{version: version, name: r.name, status: r.status, parentWof: r.parentWof}
			sum.Updated++
		default:
			sum.Skipped++
		}
	}
	return sum
}

type geoRec struct {
	wofID     int64
	name      string
	status    string
	parentWof int64
	placetype string
}

func (r geoRec) record() domain.Record {
	rec := domain.Record{
		"wofId":     float64(r.wofID),
		"placetype": r.placetype,
		"name":      r.name,
	}
	if r.status == "retired" {
		rec["isCurrent"] = false
	}
	if r.parentWof != 0 {
		rec["parentId"] = float64(r.parentWof)
	}
	return rec
}

func TestGeoPlacesMergeDifferential(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	svc := newService(t, pool)
	cleanup := func() {
		if _, err := pool.Exec(ctx, "DELETE FROM oikumenea.geo_places WHERE wof_id >= $1 AND wof_id < $2", diffWofBase, diffWofEnd); err != nil {
			t.Fatalf("cleanup: %v", err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	rng := rand.New(rand.NewSource(494901))
	state := map[int64]*oraclePlace{}
	nextID := diffWofBase
	existing := []int64{} // ids known to the oracle (candidates for parents / re-imports)

	// levels keeps parent placetypes strictly coarser than the child's so the CHECK passes; parents may
	// nevertheless appear AFTER their children inside a chunk (the two-phase merge must not care).
	levels := []string{"country", "region", "county", "locality"}
	levelOf := map[int64]int{}

	for round := 0; round < 12; round++ {
		version := fmt.Sprintf("v%d", 1+rng.Intn(4)) // editions repeat, so skips and updates both occur
		var recs []geoRec
		known := append([]int64(nil), existing...) // ids already in the oracle before this round

		// New places: pick a level; parent is a random shallower existing node or a new-in-this-batch one.
		for i := 0; i < 5+rng.Intn(20); i++ {
			lvl := rng.Intn(len(levels))
			var parent int64
			if lvl > 0 {
				// candidates: existing or earlier-in-batch nodes at a strictly shallower level
				var cands []int64
				for _, id := range existing {
					if levelOf[id] < lvl {
						cands = append(cands, id)
					}
				}
				for _, r := range recs {
					if levelOf[r.wofID] < lvl {
						cands = append(cands, r.wofID)
					}
				}
				if len(cands) == 0 {
					lvl = 0
				} else {
					parent = cands[rng.Intn(len(cands))]
				}
			}
			id := nextID
			nextID++
			levelOf[id] = lvl
			recs = append(recs, geoRec{
				wofID:     id,
				name:      fmt.Sprintf("Place %d r%d", id-diffWofBase, round),
				status:    []string{"active", "retired"}[rng.Intn(2)],
				parentWof: parent,
				placetype: levels[lvl],
			})
			existing = append(existing, id)
		}
		// Re-imports of known places (same or different edition — the oracle decides update vs skip).
		for i := 0; len(known) > 0 && i < rng.Intn(15); i++ {
			id := known[rng.Intn(len(known))]
			recs = append(recs, geoRec{
				wofID:     id,
				name:      fmt.Sprintf("Place %d r%d re", id-diffWofBase, round),
				status:    "active",
				parentWof: state[id].parentWof, // keep the stored parent
				placetype: levels[levelOf[id]],
			})
		}
		// In-batch duplicates (last occurrence wins).
		if len(recs) > 2 {
			for i := 0; i < rng.Intn(3); i++ {
				dup := recs[rng.Intn(len(recs))]
				dup.name += " dup"
				recs = append(recs, dup)
			}
		}
		// Shuffle: children may precede parents within the chunk.
		rng.Shuffle(len(recs), func(i, j int) { recs[i], recs[j] = recs[j], recs[i] })

		// Split into 1–3 chunks and send as a chunked run (exercising the ChunkInfo path end to end). A
		// random split may put a child ahead of its new parent's chunk — that would legitimately fail the
		// across-chunk parent-first contract the real mapper guarantees — so such rounds go unsplit.
		nChunks := 1 + rng.Intn(3)
		per := (len(recs) + nChunks - 1) / nChunks
		for c := 0; c < nChunks; c++ {
			if parentSplitAcrossChunks(recs, c*per, min((c+1)*per, len(recs)), state) {
				nChunks, per = 1, len(recs)
				break
			}
		}
		var got, want domain.Summary
		runID := fmt.Sprintf("diff-run-%d", round)
		for c := 0; c < nChunks; c++ {
			lo, hi := c*per, min((c+1)*per, len(recs))
			if lo >= len(recs) {
				break
			}
			chunkRecs := recs[lo:hi]
			sum, err := svc.Import(ctx, domain.ObjectTypeGeoPlaces, application.Envelope{
				ObjectType:    domain.ObjectTypeGeoPlaces,
				Source:        "diff-test",
				SourceVersion: version,
				Records:       toRecords(chunkRecs),
				Chunk:         domain.ChunkInfo{Chunked: true, RunID: runID, Seq: c + 1, IsLast: hi == len(recs)},
			})
			if err != nil {
				t.Fatalf("round %d chunk %d import: %v", round, c+1, err)
			}
			got.Created += sum.Created
			got.Updated += sum.Updated
			got.Skipped += sum.Skipped
			w := oracleApply(state, chunkRecs, version)
			want.Created += w.Created
			want.Updated += w.Updated
			want.Skipped += w.Skipped
		}
		if got != want {
			t.Fatalf("round %d summary: merge=%+v oracle=%+v", round, got, want)
		}
		assertGeoStateMatches(t, pool, state)
	}
}

func toRecords(rs []geoRec) []domain.Record {
	out := make([]domain.Record, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.record())
	}
	return out
}

// parentSplitAcrossChunks reports whether the chunk [lo,hi) references a parent that is neither in the
// oracle state (already in the DB) nor in this or an earlier chunk — i.e. the random split broke the
// across-chunk parent-first contract the real mapper guarantees.
func parentSplitAcrossChunks(recs []geoRec, lo, hi int, state map[int64]*oraclePlace) bool {
	avail := make(map[int64]bool, hi)
	for i := 0; i < hi; i++ {
		avail[recs[i].wofID] = true
	}
	for i := lo; i < hi; i++ {
		p := recs[i].parentWof
		if p == 0 {
			continue
		}
		if _, inDB := state[p]; !inDB && !avail[p] {
			return true
		}
	}
	return false
}

// assertGeoStateMatches reads every synthetic row back (parent resolved to its WOF id) and compares
// it field-by-field with the oracle.
func assertGeoStateMatches(t *testing.T, pool *pgxpool.Pool, state map[int64]*oraclePlace) {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT g.wof_id, g.name, g.status, COALESCE(g.source_version, ''), COALESCE(p.wof_id, 0)
		FROM oikumenea.geo_places g
		LEFT JOIN oikumenea.geo_places p ON p.id = g.parent_id
		WHERE g.wof_id >= $1 AND g.wof_id < $2`, diffWofBase, diffWofEnd)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	defer rows.Close()
	got := map[int64]oraclePlace{}
	for rows.Next() {
		var (
			id int64
			op oraclePlace
		)
		if err := rows.Scan(&id, &op.name, &op.status, &op.version, &op.parentWof); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[id] = op
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	if len(got) != len(state) {
		t.Fatalf("row count: db=%d oracle=%d", len(got), len(state))
	}
	for id, want := range state {
		g, ok := got[id]
		if !ok {
			t.Fatalf("wof %d missing from db", id)
		}
		if g != *want {
			t.Fatalf("wof %d diverged: db=%+v oracle=%+v", id, g, *want)
		}
	}
}
