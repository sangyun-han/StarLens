package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sangyun-han/StarLens/backend/internal/model"
	"github.com/sangyun-han/StarLens/backend/internal/repository"
)

// routineLoadReader is the repository slice this service depends on.
type routineLoadReader interface {
	Jobs(ctx context.Context) (repository.RoutineLoadRows, error)
}

// RoutineLoadService assembles the routine load monitoring view and evaluates
// its alert rules.
type RoutineLoadService struct {
	repo   routineLoadReader
	policy RoutineLoadAlertPolicy
	now    func() time.Time
}

// NewRoutineLoadService wires the service to a repository and alert thresholds.
func NewRoutineLoadService(repo routineLoadReader, policy RoutineLoadAlertPolicy) *RoutineLoadService {
	return &RoutineLoadService{repo: repo, policy: policy, now: time.Now}
}

// Snapshot reads every routine load job and folds it into a single payload.
func (s *RoutineLoadService) Snapshot(ctx context.Context) (model.RoutineLoadSnapshot, error) {
	result, err := s.repo.Jobs(ctx)
	if err != nil {
		return model.RoutineLoadSnapshot{}, fmt.Errorf("%w: reading routine load jobs failed: %w", ErrUnavailable, err)
	}

	jobs := make([]model.RoutineLoadJob, 0, len(result.Rows))
	for _, row := range result.Rows {
		jobs = append(jobs, mapRoutineLoadJob(row))
	}
	sortRoutineLoadJobs(jobs)

	return model.RoutineLoadSnapshot{
		CollectedAt: s.now().UTC(),
		Source:      result.Source,
		Warnings:    result.Warnings,
		Summary:     summarizeRoutineLoads(jobs),
		Jobs:        jobs,
	}, nil
}

// mapRoutineLoadJob converts one job row. Column aliases cover both read paths:
// SHOW ROUTINE LOAD uses CamelCase names (DbName, TableName) while
// information_schema.routine_load_jobs uses SNAKE_CASE (DB_NAME, TABLE_NAME) —
// Row keys are lower-cased, underscores preserved.
func mapRoutineLoadJob(row repository.Row) model.RoutineLoadJob {
	job := model.RoutineLoadJob{
		ID:             row.Str("id"),
		Name:           row.Str("name"),
		Database:       row.Str("dbname", "db_name"),
		Table:          row.Str("tablename", "table_name"),
		State:          normalizeRoutineLoadState(row.Str("state")),
		DataSourceType: strings.ToUpper(row.Str("datasourcetype", "data_source_type")),
		CurrentTaskNum: row.Int64("currenttasknum", "current_task_num"),

		CreateTime: nonNull(row.Str("createtime", "create_time")),
		PauseTime:  nonNull(row.Str("pausetime", "pause_time")),
		EndTime:    nonNull(row.Str("endtime", "end_time")),

		ReasonOfStateChanged: nonNull(row.Str("reasonofstatechanged", "reasons_of_state_changed", "reason_of_state_changed")),
		ErrorLogURLs:         nonNull(row.Str("errorlogurls", "error_log_urls")),
		TrackingSQL:          nonNull(row.Str("trackingsql", "tracking_sql")),
		OtherMsg:             nonNull(row.Str("othermsg", "other_msg")),

		Progress:             nonNull(row.Str("progress")),
		LatestSourcePosition: nonNull(row.Str("latestsourceposition", "latest_source_position")),
	}

	job.Statistics = parseRoutineLoadStatistics(row.Str("statistic", "statistics"))
	job.OffsetLag = computeOffsetLag(job.Progress, job.LatestSourcePosition)
	return job
}

func normalizeRoutineLoadState(raw string) string {
	state := strings.ToUpper(strings.TrimSpace(raw))
	switch state {
	case model.RoutineLoadStateNeedSchedule, model.RoutineLoadStateRunning,
		model.RoutineLoadStatePaused, model.RoutineLoadStateStopped,
		model.RoutineLoadStateCancelled:
		return state
	case "":
		return model.RoutineLoadStateUnknown
	default:
		// Pass unknown-but-present states through so a new StarRocks state shows
		// up as itself rather than being masked.
		return state
	}
}

// parseRoutineLoadStatistics decodes the Statistic JSON column. A missing or
// malformed column yields nil rather than a misleading all-zero struct.
func parseRoutineLoadStatistics(raw string) *model.RoutineLoadStatistics {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "NULL" {
		return nil
	}

	var stats model.RoutineLoadStatistics
	if err := json.Unmarshal([]byte(raw), &stats); err != nil {
		return nil
	}
	return &stats
}

// computeOffsetLag sums, per partition, how far consumption (Progress) trails
// the source's log end (LatestSourcePosition). Both columns are JSON maps of
// partition -> offset; non-numeric markers such as OFFSET_END are skipped. The
// result is an approximation — good enough to rank jobs and trip a threshold,
// not an exact Kafka lag.
func computeOffsetLag(progressRaw, latestRaw string) *int64 {
	progress := parseOffsetMap(progressRaw)
	latest := parseOffsetMap(latestRaw)
	if len(progress) == 0 || len(latest) == 0 {
		return nil
	}

	var lag int64
	matched := false
	for partition, latestOffset := range latest {
		consumed, ok := progress[partition]
		if !ok {
			continue
		}
		matched = true
		if delta := latestOffset - consumed; delta > 0 {
			lag += delta
		}
	}
	if !matched {
		return nil
	}
	return &lag
}

func parseOffsetMap(raw string) map[string]int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "NULL" || !strings.HasPrefix(raw, "{") {
		return nil
	}

	// Offsets arrive as strings or numbers depending on version; accept both.
	var loose map[string]any
	if err := json.Unmarshal([]byte(raw), &loose); err != nil {
		return nil
	}

	out := make(map[string]int64, len(loose))
	for partition, value := range loose {
		switch v := value.(type) {
		case string:
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				out[partition] = n
			}
		case float64:
			out[partition] = int64(v)
		}
	}
	return out
}

func summarizeRoutineLoads(jobs []model.RoutineLoadJob) model.RoutineLoadSummary {
	summary := model.RoutineLoadSummary{Total: len(jobs)}
	for _, job := range jobs {
		switch job.State {
		case model.RoutineLoadStateRunning:
			summary.Running++
		case model.RoutineLoadStateNeedSchedule:
			summary.NeedSchedule++
		case model.RoutineLoadStatePaused:
			summary.Paused++
		case model.RoutineLoadStateStopped:
			summary.Stopped++
		case model.RoutineLoadStateCancelled:
			summary.Cancelled++
		}
		if job.Unhealthy() {
			summary.Unhealthy++
		}
		if job.Statistics != nil {
			summary.TotalErrorRows += job.Statistics.ErrorRows
		}
	}
	return summary
}

// sortRoutineLoadJobs puts jobs needing attention first: cancelled, then
// paused, then scheduling/running, with terminal stopped jobs last.
func sortRoutineLoadJobs(jobs []model.RoutineLoadJob) {
	sort.SliceStable(jobs, func(i, j int) bool {
		if ri, rj := routineLoadStateRank(jobs[i].State), routineLoadStateRank(jobs[j].State); ri != rj {
			return ri < rj
		}
		if jobs[i].Database != jobs[j].Database {
			return jobs[i].Database < jobs[j].Database
		}
		return jobs[i].Name < jobs[j].Name
	})
}

func routineLoadStateRank(state string) int {
	switch state {
	case model.RoutineLoadStateCancelled:
		return 0
	case model.RoutineLoadStatePaused:
		return 1
	case model.RoutineLoadStateNeedSchedule:
		return 2
	case model.RoutineLoadStateRunning:
		return 3
	case model.RoutineLoadStateStopped:
		return 5
	default:
		return 4
	}
}

// nonNull normalizes the literal "NULL" some SHOW columns print for absent
// values.
func nonNull(s string) string {
	if s == "NULL" {
		return ""
	}
	return s
}
