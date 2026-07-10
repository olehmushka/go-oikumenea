// Unit tests for the epoch-validated grant cache protocol (D-AuthzGrantCache / review R-01.2):
// fresh-within-TTL hits read nothing; stale entries revalidate with one epoch read; an epoch bump
// forces a refetch; concurrent misses collapse into one fetch (singleflight).
package application

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/olegamysk/go-oikumenea/internal/authorization/domain"
	"github.com/olegamysk/go-oikumenea/internal/platform/db"
	"github.com/palantir/pkg/metrics"
)

// epochRepo is a controllable authority store: mutable epoch + per-query counters.
type epochRepo struct {
	domain.Repository
	epoch      *atomic.Int64
	adminReads *atomic.Int64 // proxy for "full authority fetch" count
	epochReads *atomic.Int64
	grants     []domain.ActiveGrant
}

func (r epochRepo) IsActiveInstanceAdmin(ctx context.Context, personID string) (bool, error) {
	r.adminReads.Add(1)
	return false, nil
}

func (r epochRepo) ActiveGrantsForSubject(ctx context.Context, subjectPersonID string) ([]domain.ActiveGrant, error) {
	return r.grants, nil
}

func (r epochRepo) ReadAuthzEpoch(ctx context.Context) (int64, error) {
	r.epochReads.Add(1)
	return r.epoch.Load(), nil
}

func newCacheTestService(grants []domain.ActiveGrant) (*Service, *epochRepo, *time.Time) {
	repo := &epochRepo{epoch: &atomic.Int64{}, adminReads: &atomic.Int64{}, epochReads: &atomic.Int64{}, grants: grants}
	svc := NewService(nil, func(conn db.DBTX) domain.Repository { return *repo }, nil, domain.NewPDP(fakeClosure{}), nil)
	now := time.Now()
	svc.grants.now = func() time.Time { return now }
	return svc, repo, &now
}

func TestGrantCacheFreshHitReadsNothing(t *testing.T) {
	svc, repo, _ := newCacheTestService([]domain.ActiveGrant{grantOn("u1", "person.read")})
	ctx := context.Background()

	if _, _, err := svc.cachedAuthority(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	if got := repo.adminReads.Load(); got != 1 {
		t.Fatalf("fetches after miss = %d, want 1", got)
	}
	for i := 0; i < 5; i++ {
		if _, _, err := svc.cachedAuthority(ctx, "p1"); err != nil {
			t.Fatal(err)
		}
	}
	if got := repo.adminReads.Load(); got != 1 {
		t.Errorf("fetches after 5 fresh hits = %d, want 1", got)
	}
	if got := repo.epochReads.Load(); got != 1 {
		t.Errorf("epoch reads after 5 fresh hits = %d, want 1 (only the initial miss)", got)
	}
}

func TestGrantCacheStaleRevalidatesWithOneEpochRead(t *testing.T) {
	svc, repo, now := newCacheTestService([]domain.ActiveGrant{grantOn("u1", "person.read")})
	ctx := context.Background()

	if _, _, err := svc.cachedAuthority(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(grantCacheTTL + time.Millisecond) // entry goes stale, epoch unchanged

	if _, _, err := svc.cachedAuthority(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	if got := repo.adminReads.Load(); got != 1 {
		t.Errorf("fetches after epoch-valid revalidation = %d, want 1 (grants join must be skipped)", got)
	}
	if got := repo.epochReads.Load(); got != 2 {
		t.Errorf("epoch reads = %d, want 2 (miss + revalidation)", got)
	}
}

// TestGrantCacheMetrics drives the seam through miss → hits → reset and asserts the R-20 counters
// and the entries gauge move, reading them off an isolated registry injected on the context (the
// server uses metrics.FromContext(ctx), which prefers a ctx-installed registry).
func TestGrantCacheMetrics(t *testing.T) {
	svc, _, _ := newCacheTestService([]domain.ActiveGrant{grantOn("u1", "person.read")})
	reg := metrics.NewRootMetricsRegistry()
	ctx := metrics.WithRegistry(context.Background(), reg)

	// First call: a miss (full fetch); next five: fresh hits.
	for i := 0; i < 6; i++ {
		if _, _, err := svc.cachedAuthority(ctx, "p1"); err != nil {
			t.Fatal(err)
		}
	}
	if got := reg.Counter(metricGrantCacheMisses).Count(); got != 1 {
		t.Errorf("misses = %d, want 1", got)
	}
	if got := reg.Counter(metricGrantCacheHits).Count(); got != 5 {
		t.Errorf("hits = %d, want 5", got)
	}
	if got := reg.Gauge(metricGrantCacheEntries).Value(); got != 1 {
		t.Errorf("entries gauge = %d, want 1", got)
	}

	svc.grants.reset(ctx)
	if got := reg.Counter(metricGrantCacheResets).Count(); got != 1 {
		t.Errorf("resets = %d, want 1", got)
	}
	if got := reg.Gauge(metricGrantCacheEntries).Value(); got != 0 {
		t.Errorf("entries gauge after reset = %d, want 0", got)
	}
}

func TestGrantCacheEpochBumpForcesRefetch(t *testing.T) {
	svc, repo, now := newCacheTestService([]domain.ActiveGrant{grantOn("u1", "person.read")})
	ctx := context.Background()

	if _, _, err := svc.cachedAuthority(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	repo.epoch.Add(1)                                // a grant/revoke committed elsewhere
	*now = now.Add(grantCacheTTL + time.Millisecond) // TTL expires → validation sees the bump

	if _, _, err := svc.cachedAuthority(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	if got := repo.adminReads.Load(); got != 2 {
		t.Errorf("fetches after epoch bump = %d, want 2 (stale grants must be refetched)", got)
	}
}

func TestGrantCacheResetDropsEntries(t *testing.T) {
	svc, repo, _ := newCacheTestService([]domain.ActiveGrant{grantOn("u1", "person.read")})
	ctx := context.Background()

	if _, _, err := svc.cachedAuthority(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	svc.grants.reset(ctx) // what every local authority-mutating write does after commit
	if _, _, err := svc.cachedAuthority(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	if got := repo.adminReads.Load(); got != 2 {
		t.Errorf("fetches after local reset = %d, want 2 (reset must invalidate immediately)", got)
	}
}

func TestGrantCacheSingleflightCollapsesConcurrentMisses(t *testing.T) {
	svc, repo, _ := newCacheTestService([]domain.ActiveGrant{grantOn("u1", "person.read")})
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := svc.cachedAuthority(ctx, "p1"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	// Not all 32 collapse into literally one flight (goroutines may arrive after the first flight
	// completes), but concurrent misses must not each run the full fetch.
	if got := repo.adminReads.Load(); got > 4 {
		t.Errorf("fetches under 32 concurrent misses = %d, want ≤4 (singleflight must collapse)", got)
	}
}
