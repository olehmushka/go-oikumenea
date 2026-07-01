package wikidataorgs_test

// An opt-in LIVE test that drives the REAL http connector against the public Wikidata SPARQL endpoint
// (D-ExternalOrgs / M30) and maps the response. Network + rate-limited, so it is gated behind
// OIKUMENEA_WIKIDATA_E2E=1 and never runs in CI / the default suite:
//
//	OIKUMENEA_WIKIDATA_E2E=1 go test -run TestLiveWikidataFetch ./internal/hermenea/wikidataorgs/

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/olegamysk/go-oikumenea/internal/hermenea/connector"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/wikidataorgs"
)

func TestLiveWikidataFetch(t *testing.T) {
	if os.Getenv("OIKUMENEA_WIKIDATA_E2E") != "1" {
		t.Skip("set OIKUMENEA_WIKIDATA_E2E=1 to run the live Wikidata fetch")
	}
	const locator = "https://query.wikidata.org/sparql?format=json&query=" +
		"SELECT%20%3Forg%20%3ForgLabel%20%3Fkind%20WHERE%20%7B%20VALUES%20%28%3Fclass%20%3Fkind%29%20%7B%20%28wd%3AQ7278%20%22party%22%29%20%7D%20" +
		"%3Forg%20wdt%3AP31%20%3Fclass%20.%20%3Forg%20wdt%3AP17%20wd%3AQ212%20.%20" +
		"SERVICE%20wikibase%3Alabel%20%7B%20bd%3AserviceParam%20wikibase%3Alanguage%20%22en%22.%20%7D%20%7D%20LIMIT%204"

	c, ok := connector.Default()[domain.ConnectorHTTP]
	if !ok {
		t.Fatal("no http connector")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	batch, err := c.Fetch(ctx, domain.Source{ConnectorType: domain.ConnectorHTTP, Locator: locator})
	if err != nil {
		t.Fatalf("live fetch (likely rate-limit): %v", err)
	}
	recs, err := wikidataorgs.Mapper{}.Map(batch)
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	if len(recs) == 0 {
		t.Fatal("live Wikidata returned 0 mapped records")
	}
	for _, r := range recs {
		t.Logf("live: %v %q kind=%v country=%v", r["wikidataId"], r["name"], r["kind"], r["country"])
		if r["wikidataId"] == "" || r["name"] == "" {
			t.Fatalf("incomplete record: %v", r)
		}
	}
}
