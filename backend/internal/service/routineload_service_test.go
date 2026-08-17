package service

import (
	"context"
	"errors"
	"testing"

	"github.com/sangyun-han/StarLens/backend/internal/alert"
	"github.com/sangyun-han/StarLens/backend/internal/model"
	"github.com/sangyun-han/StarLens/backend/internal/repository"
)

type fakeRoutineLoadRepo struct {
	result repository.RoutineLoadRows
	err    error
}

func (f fakeRoutineLoadRepo) Jobs(context.Context) (repository.RoutineLoadRows, error) {
	return f.result, f.err
}

const sampleStatistic = `{"receivedBytes":1024000,"errorRows":150,"committedTaskNum":40,` +
	`"loadedRows":9800,"loadRowsRate":120,"abortedTaskNum":2,"totalRows":10000,` +
	`"unselectedRows":50,"receivedBytesRate":2048,"taskExecuteTimeMs":60000}`

// Rows in SHOW ROUTINE LOAD shape (CamelCase columns, lower-cased by Row).
func showStyleRows() repository.RoutineLoadRows {
	return repository.RoutineLoadRows{
		Source: repository.RoutineLoadSourceShow,
		Rows: []repository.Row{
			{
				"id": "10101", "name": "orders_load", "dbname": "shop", "tablename": "orders",
				"state": "running", "datasourcetype": "KAFKA", "currenttasknum": "3",
				"createtime": "2026-08-01 10:00:00", "pausetime": "NULL", "endtime": "NULL",
				"statistic":            sampleStatistic,
				"progress":             `{"0":"1000","1":"2000"}`,
				"latestsourceposition": `{"0":"1500","1":"2600"}`,
				"reasonofstatechanged": "",
			},
			{
				"id": "10102", "name": "clicks_load", "dbname": "web", "tablename": "clicks",
				"state": "PAUSED", "datasourcetype": "KAFKA",
				"reasonofstatechanged": "ErrorReason{errCode = 102, msg='too many filtered rows'}",
				"errorlogurls":         "http://be:8040/api/_load_error_log?file=x",
			},
			{
				"id": "10103", "name": "old_load", "dbname": "shop", "tablename": "legacy",
				"state": "STOPPED",
			},
		},
	}
}

// The same jobs in information_schema.routine_load_jobs shape (SNAKE_CASE).
func infoSchemaStyleRows() repository.RoutineLoadRows {
	return repository.RoutineLoadRows{
		Source: repository.RoutineLoadSourceInfoSchema,
		Rows: []repository.Row{
			{
				"id": "10101", "name": "orders_load", "db_name": "shop", "table_name": "orders",
				"state": "RUNNING", "data_source_type": "KAFKA", "current_task_num": "3",
				"create_time":              "2026-08-01 10:00:00",
				"statistics":               sampleStatistic,
				"progress":                 `{"0":"1000"}`,
				"latest_source_position":   `{"0":"1500"}`,
				"reasons_of_state_changed": "",
			},
		},
	}
}

func newTestService(rows repository.RoutineLoadRows) *RoutineLoadService {
	return NewRoutineLoadService(fakeRoutineLoadRepo{result: rows}, RoutineLoadAlertPolicy{
		ErrorRowsRatio:    0.01,
		ErrorRowsMinTotal: 1_000,
		MaxOffsetLag:      500,
	})
}

func TestRoutineLoadSnapshotMapsShowStyleRows(t *testing.T) {
	snapshot, err := newTestService(showStyleRows()).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}

	if len(snapshot.Jobs) != 3 {
		t.Fatalf("jobs = %d, want 3", len(snapshot.Jobs))
	}

	// Paused jobs sort before running ones, terminal stopped jobs last.
	if got := snapshot.Jobs[0].Name; got != "clicks_load" {
		t.Errorf("jobs[0].Name = %q, want clicks_load (paused first)", got)
	}
	if got := snapshot.Jobs[2].State; got != model.RoutineLoadStateStopped {
		t.Errorf("jobs[2].State = %q, want STOPPED last", got)
	}

	running := snapshot.Jobs[1]
	if running.Name != "orders_load" || running.Database != "shop" || running.Table != "orders" {
		t.Errorf("running job identity = %+v", running)
	}
	// Lower-cased state input must normalize to the canonical constant.
	if running.State != model.RoutineLoadStateRunning {
		t.Errorf("running.State = %q, want RUNNING", running.State)
	}
	if running.CurrentTaskNum == nil || *running.CurrentTaskNum != 3 {
		t.Errorf("running.CurrentTaskNum = %v, want 3", running.CurrentTaskNum)
	}
	// "NULL" literals must not leak into the payload.
	if running.PauseTime != "" || running.EndTime != "" {
		t.Errorf("NULL times leaked: pause=%q end=%q", running.PauseTime, running.EndTime)
	}

	stats := running.Statistics
	if stats == nil {
		t.Fatal("running.Statistics = nil, want parsed")
	}
	if stats.TotalRows != 10000 || stats.ErrorRows != 150 || stats.LoadedRows != 9800 {
		t.Errorf("stats = %+v", stats)
	}
	if ratio := stats.ErrorRatio(); ratio != 0.015 {
		t.Errorf("ErrorRatio() = %v, want 0.015", ratio)
	}

	// Lag: (1500-1000) + (2600-2000) = 1100.
	if running.OffsetLag == nil || *running.OffsetLag != 1100 {
		t.Errorf("OffsetLag = %v, want 1100", running.OffsetLag)
	}

	paused := snapshot.Jobs[0]
	if paused.Statistics != nil {
		t.Error("paused job without Statistic column must have nil Statistics")
	}
	if paused.OffsetLag != nil {
		t.Error("paused job without positions must have nil OffsetLag")
	}
}

func TestRoutineLoadSnapshotMapsInfoSchemaRows(t *testing.T) {
	snapshot, err := newTestService(infoSchemaStyleRows()).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if snapshot.Source != repository.RoutineLoadSourceInfoSchema {
		t.Errorf("Source = %q", snapshot.Source)
	}

	job := snapshot.Jobs[0]
	if job.Database != "shop" || job.Table != "orders" {
		t.Errorf("snake_case aliases not applied: %+v", job)
	}
	if job.Statistics == nil || job.Statistics.TotalRows != 10000 {
		t.Errorf("statistics alias not applied: %+v", job.Statistics)
	}
	if job.OffsetLag == nil || *job.OffsetLag != 500 {
		t.Errorf("OffsetLag = %v, want 500", job.OffsetLag)
	}
}

func TestRoutineLoadSummary(t *testing.T) {
	snapshot, err := newTestService(showStyleRows()).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}

	got := snapshot.Summary
	want := model.RoutineLoadSummary{
		Total: 3, Running: 1, Paused: 1, Stopped: 1,
		Unhealthy: 1, TotalErrorRows: 150,
	}
	if got != want {
		t.Errorf("Summary = %+v, want %+v", got, want)
	}
}

func TestRoutineLoadSnapshotWrapsRepositoryError(t *testing.T) {
	svc := NewRoutineLoadService(
		fakeRoutineLoadRepo{err: errors.New("connection refused")},
		RoutineLoadAlertPolicy{},
	)

	_, err := svc.Snapshot(context.Background())
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("error %v does not wrap ErrUnavailable", err)
	}
}

func TestEvaluateAlerts(t *testing.T) {
	svc := newTestService(showStyleRows())
	snapshot, err := svc.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}

	alerts := svc.EvaluateAlerts(snapshot)

	byRule := map[string]alert.Alert{}
	for _, a := range alerts {
		byRule[a.RuleID] = a
	}

	// Paused job -> warning carrying the StarRocks reason.
	paused, ok := byRule[RuleRoutineLoadPaused]
	if !ok {
		t.Fatal("paused rule did not fire")
	}
	if paused.Severity != alert.SeverityWarning {
		t.Errorf("paused severity = %q", paused.Severity)
	}
	if paused.Message == "" || paused.Labels["job"] != "clicks_load" {
		t.Errorf("paused alert = %+v", paused)
	}

	// 1.5% error ratio over the 1% threshold with 10k rows sampled.
	ratio, ok := byRule[RuleRoutineLoadErrorRatio]
	if !ok {
		t.Fatal("error ratio rule did not fire")
	}
	if ratio.Labels["job"] != "orders_load" {
		t.Errorf("ratio alert labels = %v", ratio.Labels)
	}

	// Lag 1100 over the 500 threshold.
	if _, ok := byRule[RuleRoutineLoadOffsetLag]; !ok {
		t.Fatal("offset lag rule did not fire")
	}

	// STOPPED is user intent, never an alert; nothing was cancelled.
	if _, ok := byRule[RuleRoutineLoadCancelled]; ok {
		t.Error("cancelled rule fired without a cancelled job")
	}
	if len(alerts) != 3 {
		t.Errorf("alerts = %d, want 3 (%+v)", len(alerts), alerts)
	}
}

func TestEvaluateAlertsCancelledIsCritical(t *testing.T) {
	rows := repository.RoutineLoadRows{Rows: []repository.Row{{
		"name": "dead_load", "dbname": "shop", "tablename": "orders", "state": "CANCELLED",
		"reasonofstatechanged": "ErrorReason{errCode = 104, msg='be crashed'}",
	}}}
	svc := newTestService(rows)

	snapshot, _ := svc.Snapshot(context.Background())
	alerts := svc.EvaluateAlerts(snapshot)

	if len(alerts) != 1 || alerts[0].RuleID != RuleRoutineLoadCancelled {
		t.Fatalf("alerts = %+v, want single cancelled alert", alerts)
	}
	if alerts[0].Severity != alert.SeverityCritical {
		t.Errorf("severity = %q, want critical", alerts[0].Severity)
	}
}

func TestEvaluateAlertsRespectsDisabledRulesAndMinSample(t *testing.T) {
	rows := repository.RoutineLoadRows{Rows: []repository.Row{{
		"name": "tiny_load", "dbname": "shop", "tablename": "t", "state": "RUNNING",
		// 30% errors but only 10 rows sampled.
		"statistic": `{"totalRows":10,"errorRows":3,"loadedRows":7}`,
		"progress":  `{"0":"0"}`, "latestsourceposition": `{"0":"999999"}`,
	}}}

	svc := NewRoutineLoadService(fakeRoutineLoadRepo{result: rows}, RoutineLoadAlertPolicy{
		ErrorRowsRatio:    0.01,
		ErrorRowsMinTotal: 1_000, // sample too small -> no ratio alert
		MaxOffsetLag:      0,     // disabled -> no lag alert despite huge lag
	})

	snapshot, _ := svc.Snapshot(context.Background())
	if alerts := svc.EvaluateAlerts(snapshot); len(alerts) != 0 {
		t.Errorf("alerts = %+v, want none", alerts)
	}
}
