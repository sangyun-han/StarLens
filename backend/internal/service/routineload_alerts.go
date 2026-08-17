package service

import (
	"context"
	"fmt"

	"github.com/sangyun-han/StarLens/backend/internal/alert"
	"github.com/sangyun-han/StarLens/backend/internal/model"
)

// Alert rule identifiers. Stable strings: they appear in webhook payloads and
// drive deduplication keys.
const (
	RuleRoutineLoadPaused     = "routine_load_paused"
	RuleRoutineLoadCancelled  = "routine_load_cancelled"
	RuleRoutineLoadErrorRatio = "routine_load_error_ratio"
	RuleRoutineLoadOffsetLag  = "routine_load_offset_lag"
)

// RoutineLoadAlertPolicy holds the thresholds the routine load rules evaluate
// against. Zero-valued thresholds disable their rule.
type RoutineLoadAlertPolicy struct {
	// ErrorRowsRatio fires when errorRows/totalRows exceeds this fraction
	// (0.01 = 1%). <= 0 disables the rule.
	ErrorRowsRatio float64
	// ErrorRowsMinTotal is the minimum totalRows before the ratio rule applies,
	// so a brand-new job with 3 bad rows out of 10 does not page anyone.
	ErrorRowsMinTotal int64
	// MaxOffsetLag fires when a job's summed offset lag exceeds this many
	// messages. <= 0 disables the rule.
	MaxOffsetLag int64
}

// CollectAlerts takes a fresh snapshot and evaluates every rule — the
// alert.CollectFunc used by the background poller.
func (s *RoutineLoadService) CollectAlerts(ctx context.Context) ([]alert.Alert, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	return s.EvaluateAlerts(snapshot), nil
}

// EvaluateAlerts applies the alert rules to a snapshot. Pure function of its
// inputs: repeat suppression is the Manager's job, so an ongoing condition is
// reported on every evaluation.
func (s *RoutineLoadService) EvaluateAlerts(snapshot model.RoutineLoadSnapshot) []alert.Alert {
	var alerts []alert.Alert

	for _, job := range snapshot.Jobs {
		subject := job.Database + "." + job.Name

		switch job.State {
		case model.RoutineLoadStatePaused:
			alerts = append(alerts, alert.Alert{
				Key:      RuleRoutineLoadPaused + "|" + subject,
				RuleID:   RuleRoutineLoadPaused,
				Severity: alert.SeverityWarning,
				Title:    fmt.Sprintf("Routine load job %q is PAUSED", subject),
				Message:  pausedMessage(job),
				Labels:   jobLabels(job),
			})
		case model.RoutineLoadStateCancelled:
			// CANCELLED means StarRocks gave up on the job (fatal error or manual
			// stop of an unrecoverable state); ingestion is not coming back
			// without operator action.
			alerts = append(alerts, alert.Alert{
				Key:      RuleRoutineLoadCancelled + "|" + subject,
				RuleID:   RuleRoutineLoadCancelled,
				Severity: alert.SeverityCritical,
				Title:    fmt.Sprintf("Routine load job %q was CANCELLED", subject),
				Message:  pausedMessage(job),
				Labels:   jobLabels(job),
			})
		}

		if a, ok := s.errorRatioAlert(job, subject); ok {
			alerts = append(alerts, a)
		}
		if a, ok := s.offsetLagAlert(job, subject); ok {
			alerts = append(alerts, a)
		}
	}

	return alerts
}

func (s *RoutineLoadService) errorRatioAlert(job model.RoutineLoadJob, subject string) (alert.Alert, bool) {
	stats := job.Statistics
	if s.policy.ErrorRowsRatio <= 0 || stats == nil || stats.TotalRows < s.policy.ErrorRowsMinTotal {
		return alert.Alert{}, false
	}
	ratio := stats.ErrorRatio()
	if ratio <= s.policy.ErrorRowsRatio {
		return alert.Alert{}, false
	}

	return alert.Alert{
		Key:      RuleRoutineLoadErrorRatio + "|" + subject,
		RuleID:   RuleRoutineLoadErrorRatio,
		Severity: alert.SeverityWarning,
		Title:    fmt.Sprintf("Routine load job %q error rate is %.2f%%", subject, ratio*100),
		Message: fmt.Sprintf(
			"%d of %d consumed rows failed to load (threshold %.2f%%). Check ErrorLogUrls for rejected rows.",
			stats.ErrorRows, stats.TotalRows, s.policy.ErrorRowsRatio*100),
		Labels: jobLabels(job),
	}, true
}

func (s *RoutineLoadService) offsetLagAlert(job model.RoutineLoadJob, subject string) (alert.Alert, bool) {
	if s.policy.MaxOffsetLag <= 0 || job.OffsetLag == nil || *job.OffsetLag <= s.policy.MaxOffsetLag {
		return alert.Alert{}, false
	}

	return alert.Alert{
		Key:      RuleRoutineLoadOffsetLag + "|" + subject,
		RuleID:   RuleRoutineLoadOffsetLag,
		Severity: alert.SeverityWarning,
		Title:    fmt.Sprintf("Routine load job %q is lagging by ~%d messages", subject, *job.OffsetLag),
		Message: fmt.Sprintf(
			"Consumption trails the source log end by ~%d messages (threshold %d). The job may be under-provisioned or the source is bursting.",
			*job.OffsetLag, s.policy.MaxOffsetLag),
		Labels: jobLabels(job),
	}, true
}

func pausedMessage(job model.RoutineLoadJob) string {
	if job.ReasonOfStateChanged != "" {
		return job.ReasonOfStateChanged
	}
	if job.OtherMsg != "" {
		return job.OtherMsg
	}
	return "StarRocks did not report a reason. Inspect the job with SHOW ROUTINE LOAD."
}

func jobLabels(job model.RoutineLoadJob) map[string]string {
	labels := map[string]string{
		"database": job.Database,
		"job":      job.Name,
		"table":    job.Table,
		"state":    job.State,
	}
	if job.DataSourceType != "" {
		labels["source"] = job.DataSourceType
	}
	return labels
}
