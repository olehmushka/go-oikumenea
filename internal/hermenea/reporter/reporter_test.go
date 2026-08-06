// Copyright 2026 Oleh Mushka
// SPDX-License-Identifier: Apache-2.0

package reporter

import (
	"context"
	"testing"

	connectorapi "github.com/olehmushka/go-oikumenea/internal/conjure/oikumenea/connector"
	"github.com/olehmushka/go-oikumenea/internal/hermenea/domain"
	"github.com/palantir/pkg/bearertoken"
)

// fakeClient captures the last request for each self-service call. The read methods are embedded (nil)
// and never invoked — the reporter only writes.
type fakeClient struct {
	connectorapi.ConnectorServiceClient
	lastRegister *connectorapi.RegisterConnectorRequest
	lastReport   *connectorapi.ReportSyncRunRequest
}

func (f *fakeClient) RegisterConnector(_ context.Context, _ bearertoken.Token, req connectorapi.RegisterConnectorRequest) (connectorapi.Connector, error) {
	f.lastRegister = &req
	return connectorapi.Connector{}, nil
}

func (f *fakeClient) ReportSyncRun(_ context.Context, _ bearertoken.Token, req connectorapi.ReportSyncRunRequest) (connectorapi.SyncRun, error) {
	f.lastReport = &req
	return connectorapi.SyncRun{}, nil
}

// TestNilReporterIsNoOp: a nil *Reporter is a valid null-object — Register/ReportRun return nil without
// panicking, so a service built without a reporter reports nothing rather than crashing (R-11).
func TestNilReporterIsNoOp(t *testing.T) {
	var r *Reporter
	if err := r.Register(context.Background(), "c", "n", "d", nil); err != nil {
		t.Fatalf("nil Register: %v", err)
	}
	if err := r.ReportRun(context.Background(), "s", "run-1", domain.RunRunning, domain.ImportSummary{}, ""); err != nil {
		t.Fatalf("nil ReportRun: %v", err)
	}
}

// TestRegisterMapsSources: a domain.Source maps to the wire SourceDeclaration — objectType/schedule are
// present when non-empty and absent (nil) when empty, so a lookup-only or unscheduled source declares
// no spurious values.
func TestRegisterMapsSources(t *testing.T) {
	fc := &fakeClient{}
	r := &Reporter{client: fc, token: bearertoken.Token("t")}
	sources := []domain.Source{
		{Code: "geo-places", Name: "WOF places", ObjectType: "geo-places", Cron: "@daily", Enabled: true},
		{Code: "lookup-only", Name: "no target", ObjectType: "", Cron: "", Enabled: false},
	}
	if err := r.Register(context.Background(), "hermenea", "Hermenea", "desc", sources); err != nil {
		t.Fatalf("Register: %v", err)
	}
	got := fc.lastRegister
	if got == nil {
		t.Fatal("RegisterConnector was not called")
	}
	if got.Code != "hermenea" || got.Description == nil || *got.Description != "desc" {
		t.Fatalf("connector fields: code=%q desc=%v", got.Code, got.Description)
	}
	if len(got.Sources) != 2 {
		t.Fatalf("want 2 sources, got %d", len(got.Sources))
	}
	s0 := got.Sources[0]
	if s0.ObjectType == nil || *s0.ObjectType != "geo-places" {
		t.Fatalf("source[0] objectType = %v, want geo-places", s0.ObjectType)
	}
	if s0.Schedule == nil || *s0.Schedule != "@daily" {
		t.Fatalf("source[0] schedule = %v, want @daily", s0.Schedule)
	}
	if s0.Enabled == nil || !*s0.Enabled {
		t.Fatalf("source[0] enabled = %v, want true", s0.Enabled)
	}
	s1 := got.Sources[1]
	if s1.ObjectType != nil {
		t.Fatalf("source[1] objectType = %v, want nil (lookup-only)", *s1.ObjectType)
	}
	if s1.Schedule != nil {
		t.Fatalf("source[1] schedule = %v, want nil (unscheduled)", *s1.Schedule)
	}
	if s1.Enabled == nil || *s1.Enabled {
		t.Fatalf("source[1] enabled = %v, want false", s1.Enabled)
	}
}

// TestReportRunMapsClose: a terminal report carries the source code, external run id, state, counts and
// error verbatim.
func TestReportRunMapsClose(t *testing.T) {
	fc := &fakeClient{}
	r := &Reporter{client: fc, token: bearertoken.Token("t")}
	sum := domain.ImportSummary{Created: 3, Updated: 2, Skipped: 1}
	if err := r.ReportRun(context.Background(), "geo-places", "run-42", domain.RunFailed, sum, "boom"); err != nil {
		t.Fatalf("ReportRun: %v", err)
	}
	got := fc.lastReport
	if got == nil {
		t.Fatal("ReportSyncRun was not called")
	}
	if got.SourceCode != "geo-places" || got.State != domain.RunFailed {
		t.Fatalf("sourceCode=%q state=%q", got.SourceCode, got.State)
	}
	if got.ExternalRunId == nil || *got.ExternalRunId != "run-42" {
		t.Fatalf("externalRunId = %v", got.ExternalRunId)
	}
	if got.Created == nil || *got.Created != 3 || got.Updated == nil || *got.Updated != 2 || got.Skipped == nil || *got.Skipped != 1 {
		t.Fatalf("counts = %v/%v/%v", got.Created, got.Updated, got.Skipped)
	}
	if got.Error == nil || *got.Error != "boom" {
		t.Fatalf("error = %v", got.Error)
	}
	// A terminal report MUST carry finishedAt — the core rejects a finished run without it (400). This
	// pins the regression the M53 live e2e caught.
	if got.FinishedAt == nil {
		t.Fatal("terminal report is missing finishedAt (core rejects it 400)")
	}
}

// TestReportRunOpenHasNoFinishedAt: a running report must NOT carry finishedAt (the core rejects a
// running run that does, 400) — the other half of the invariant.
func TestReportRunOpenHasNoFinishedAt(t *testing.T) {
	fc := &fakeClient{}
	r := &Reporter{client: fc, token: bearertoken.Token("t")}
	if err := r.ReportRun(context.Background(), "geo-places", "run-7", domain.RunRunning, domain.ImportSummary{}, ""); err != nil {
		t.Fatalf("ReportRun: %v", err)
	}
	if fc.lastReport.FinishedAt != nil {
		t.Fatal("running report must not carry finishedAt")
	}
}
