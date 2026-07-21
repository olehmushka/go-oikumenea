// Package transport implements hermenea's generated Conjure HermeneaService (M16 / D-Hermenea): the
// push-trigger endpoint + the source/run/job read endpoints over hermenea's own DB. Inbound auth (the
// OIKUMENEA_HERMENEA_TOKEN shared secret) is enforced by the Authenticator middleware, so handlers
// assume an authenticated caller. Generated code in internal/conjure is never hand-edited.
package transport

import (
	"context"
	"errors"
	"time"

	hermeneaapi "github.com/olegamysk/go-oikumenea/internal/conjure/oikumenea/hermenea"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/application"
	"github.com/olegamysk/go-oikumenea/internal/hermenea/domain"
	"github.com/palantir/pkg/bearertoken"
)

// listLimit caps the run/job read endpoints.
const listLimit = 100

// Service adapts *application.Service to the generated HermeneaService interface.
type Service struct {
	app *application.Service
}

// NewService builds the transport adapter.
func NewService(app *application.Service) Service { return Service{app: app} }

var _ hermeneaapi.HermeneaService = Service{}

func (s Service) TriggerSync(ctx context.Context, _ bearertoken.Token, source string) (hermeneaapi.JobRef, error) {
	jobID, status, err := s.app.TriggerSync(ctx, source)
	if errors.Is(err, domain.ErrUnknownSource) {
		return hermeneaapi.JobRef{}, hermeneaapi.NewUnknownSource(source)
	}
	if err != nil {
		return hermeneaapi.JobRef{}, err
	}
	return hermeneaapi.JobRef{JobId: jobID, Status: status}, nil
}

func (s Service) ListSources(ctx context.Context, _ bearertoken.Token) ([]hermeneaapi.ImportSource, error) {
	srcs, err := s.app.ListSources(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]hermeneaapi.ImportSource, 0, len(srcs))
	for _, src := range srcs {
		out = append(out, hermeneaapi.ImportSource{
			Code:          src.Code,
			Name:          src.Name,
			ConnectorType: src.FetcherType,
			ObjectType:    src.ObjectType,
			Locator:       src.Locator,
			Cron:          strPtr(src.Cron),
			Enabled:       src.Enabled,
		})
	}
	return out, nil
}

func (s Service) ListRuns(ctx context.Context, _ bearertoken.Token) ([]hermeneaapi.ImportRun, error) {
	runs, err := s.app.ListRuns(ctx, listLimit)
	if err != nil {
		return nil, err
	}
	out := make([]hermeneaapi.ImportRun, 0, len(runs))
	for _, r := range runs {
		out = append(out, hermeneaapi.ImportRun{
			Id:            r.ID,
			SourceCode:    r.SourceCode,
			SourceVersion: strPtr(r.SourceVersion),
			Status:        r.Status,
			Created:       r.Created,
			Updated:       r.Updated,
			Skipped:       r.Skipped,
			Error:         strPtr(r.Error),
			StartedAt:     r.StartedAt.UTC().Format(time.RFC3339),
			FinishedAt:    timePtr(r.FinishedAt),
		})
	}
	return out, nil
}

func (s Service) ListJobs(ctx context.Context, _ bearertoken.Token) ([]hermeneaapi.WorkerJob, error) {
	jobs, err := s.app.ListJobs(ctx, listLimit)
	if err != nil {
		return nil, err
	}
	out := make([]hermeneaapi.WorkerJob, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, hermeneaapi.WorkerJob{
			Id:          j.ID,
			JobType:     j.JobType,
			SourceCode:  strPtr(j.SourceCode),
			Status:      j.Status,
			Attempts:    j.Attempts,
			MaxAttempts: j.MaxAttempts,
			RunAfter:    j.RunAfter.UTC().Format(time.RFC3339),
			LastError:   strPtr(j.LastError),
		})
	}
	return out, nil
}

// CheckWatchlist runs a live screening check (D-Watchlists, M34): the synchronous oikumenea→hermenea
// surface. Auth (OIKUMENEA_HERMENEA_TOKEN) is enforced by the middleware.
func (s Service) CheckWatchlist(ctx context.Context, _ bearertoken.Token, req hermeneaapi.WatchlistQuery) (hermeneaapi.WatchlistResult, error) {
	res, err := s.app.CheckWatchlist(ctx, domain.WatchlistQuery{
		SubjectKey:  req.SubjectKey,
		FullName:    req.FullName,
		Birthdate:   valOrEmpty(req.Birthdate),
		Nationality: valOrEmpty(req.Nationality),
	})
	if err != nil {
		return hermeneaapi.WatchlistResult{}, err
	}
	lists := res.Lists
	if lists == nil {
		lists = []string{}
	}
	return hermeneaapi.WatchlistResult{
		OnList:       res.OnList,
		Lists:        lists,
		Program:      strPtr(res.Program),
		MatchScore:   res.MatchScore,
		CheckedAt:    res.CheckedAt.UTC().Format(time.RFC3339),
		NextCheckDue: timePtr(res.NextCheckDue),
	}, nil
}

func valOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func timePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	v := t.UTC().Format(time.RFC3339)
	return &v
}
