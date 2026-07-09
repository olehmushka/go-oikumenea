// gen-wof-synthetic writes a synthetic Who's-On-First SQLite distribution for the R-05 / M49
// acceptance run: a national-scale geo-places dataset (default 1,000,000 places — 1 country +
// regions + counties + localities, parent-first via wof:hierarchy) in exactly the shape the
// `wof-sqlite` connector + wof.GeoPlacesMapper consume (spr + geojson tables). Compress the output
// with `bzip2 wof-synthetic.db` and serve it over HTTP for the compose run:
//
//	go run ./scripts/gen-wof-synthetic -out /tmp/wof-synthetic.db -places 1000000
//	bzip2 -f /tmp/wof-synthetic.db
//	(cd /tmp && python3 -m http.server 8777)   # locator: http://host.docker.internal:8777/wof-synthetic.db.bz2
//
// The country row carries countryCode UA (present in the seeded geo_countries registry) so the
// denormalized country FK resolves; wof ids start at 3e9, far above any real WOF id, so the
// synthetic world never collides with a real backfill.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

const idBase int64 = 3_000_000_000

func main() {
	out := flag.String("out", "wof-synthetic.db", "output SQLite path (overwritten)")
	places := flag.Int("places", 1_000_000, "total place count (country+regions+counties+localities)")
	regions := flag.Int("regions", 30, "region count")
	counties := flag.Int("counties", 3000, "county count")
	flag.Parse()

	start := time.Now()
	_ = os.Remove(*out)
	db, err := sql.Open("sqlite", *out)
	if err != nil {
		log.Fatalf("open %s: %v", *out, err)
	}
	defer db.Close()

	for _, stmt := range []string{
		`PRAGMA journal_mode = OFF`,
		`PRAGMA synchronous = OFF`,
		`CREATE TABLE spr (id INTEGER PRIMARY KEY, placetype TEXT, country TEXT, is_current INTEGER)`,
		`CREATE TABLE geojson (id INTEGER, is_alt INTEGER, body BLOB)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			log.Fatalf("ddl %q: %v", stmt, err)
		}
	}

	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("begin: %v", err)
	}
	spr, err := tx.Prepare(`INSERT INTO spr (id, placetype, country, is_current) VALUES (?, ?, 'UA', 1)`)
	if err != nil {
		log.Fatalf("prepare spr: %v", err)
	}
	geo, err := tx.Prepare(`INSERT INTO geojson (id, is_alt, body) VALUES (?, 0, ?)`)
	if err != nil {
		log.Fatalf("prepare geojson: %v", err)
	}

	rng := rand.New(rand.NewSource(1))
	next := idBase
	insert := func(placetype string, hierarchy string) int64 {
		id := next
		next++
		lon := 22.0 + rng.Float64()*18 // roughly UA-shaped bbox
		lat := 44.0 + rng.Float64()*8
		body := fmt.Sprintf(`{"properties":{"wof:id":%d,"wof:name":"Synth %s %d","wof:population":%d,"wof:hierarchy":[%s]},"geometry":{"type":"Point","coordinates":[%.5f,%.5f]}}`,
			id, placetype, id-idBase, 100+rng.Intn(1_000_000), hierarchy, lon, lat)
		if _, err := spr.Exec(id, placetype); err != nil {
			log.Fatalf("insert spr %d: %v", id, err)
		}
		if _, err := geo.Exec(id, []byte(body)); err != nil {
			log.Fatalf("insert geojson %d: %v", id, err)
		}
		return id
	}

	country := insert("country", "{}")
	regionIDs := make([]int64, 0, *regions)
	for i := 0; i < *regions; i++ {
		regionIDs = append(regionIDs, insert("region", fmt.Sprintf(`{"country_id":%d}`, country)))
	}
	countyIDs := make([]int64, 0, *counties)
	for i := 0; i < *counties; i++ {
		r := regionIDs[rng.Intn(len(regionIDs))]
		countyIDs = append(countyIDs, insert("county", fmt.Sprintf(`{"country_id":%d,"region_id":%d}`, country, r)))
	}
	localities := *places - 1 - *regions - *counties
	for i := 0; i < localities; i++ {
		k := countyIDs[rng.Intn(len(countyIDs))]
		insert("locality", fmt.Sprintf(`{"country_id":%d,"county_id":%d}`, country, k))
		if (i+1)%200_000 == 0 {
			log.Printf("  %d localities…", i+1)
		}
	}

	_ = spr.Close()
	_ = geo.Close()
	if err := tx.Commit(); err != nil {
		log.Fatalf("commit: %v", err)
	}
	// Both indexes mirror what the real WOF dist ships — without geojson(id) the mapper's per-placetype
	// join degrades to O(rows²).
	for _, idx := range []string{
		`CREATE INDEX spr_placetype ON spr (placetype, id)`,
		`CREATE INDEX geojson_id ON geojson (id, is_alt)`,
	} {
		if _, err := db.Exec(idx); err != nil {
			log.Fatalf("index: %v", err)
		}
	}
	fi, _ := os.Stat(*out)
	log.Printf("wrote %s: %d places (%d regions, %d counties, %d localities), %.1f MB in %s",
		*out, *places, *regions, *counties, localities, float64(fi.Size())/(1<<20), time.Since(start).Round(time.Second))
}
