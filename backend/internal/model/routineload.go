package model

import "time"

// Routine load job states as reported by StarRocks. STOPPED and CANCELLED are
// terminal; PAUSED jobs keep their position and can be resumed.
const (
	RoutineLoadStateNeedSchedule = "NEED_SCHEDULE"
	RoutineLoadStateRunning      = "RUNNING"
	RoutineLoadStatePaused       = "PAUSED"
	RoutineLoadStateStopped      = "STOPPED"
	RoutineLoadStateCancelled    = "CANCELLED"
	RoutineLoadStateUnknown      = "UNKNOWN"
)

// RoutineLoadStatistics is the parsed form of the Statistic JSON column that
// StarRocks attaches to every routine load job.
type RoutineLoadStatistics struct {
	TotalRows         int64   `json:"totalRows"`
	LoadedRows        int64   `json:"loadedRows"`
	ErrorRows         int64   `json:"errorRows"`
	UnselectedRows    int64   `json:"unselectedRows"`
	ReceivedBytes     int64   `json:"receivedBytes"`
	TaskExecuteTimeMs int64   `json:"taskExecuteTimeMs"`
	CommittedTaskNum  int64   `json:"committedTaskNum"`
	AbortedTaskNum    int64   `json:"abortedTaskNum"`
	LoadRowsRate      float64 `json:"loadRowsRate,omitempty"`
	ReceivedBytesRate float64 `json:"receivedBytesRate,omitempty"`
}

// ErrorRatio is the fraction of consumed rows that failed to load, in [0, 1].
func (s *RoutineLoadStatistics) ErrorRatio() float64 {
	if s == nil || s.TotalRows <= 0 {
		return 0
	}
	return float64(s.ErrorRows) / float64(s.TotalRows)
}

// RoutineLoadJob is one streaming ingestion job (Kafka/Pulsar -> table).
type RoutineLoadJob struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Database string `json:"database"`
	Table    string `json:"table"`
	// State is one of the RoutineLoadState* constants, upper-cased.
	State          string `json:"state"`
	DataSourceType string `json:"dataSourceType,omitempty"`
	CurrentTaskNum *int64 `json:"currentTaskNum,omitempty"`

	// StarRocks-local wall clock strings, passed through as reported.
	CreateTime string `json:"createTime,omitempty"`
	PauseTime  string `json:"pauseTime,omitempty"`
	EndTime    string `json:"endTime,omitempty"`

	// ReasonOfStateChanged explains why a job left RUNNING — the first thing an
	// operator wants to read on a paused or cancelled job.
	ReasonOfStateChanged string `json:"reasonOfStateChanged,omitempty"`
	ErrorLogURLs         string `json:"errorLogUrls,omitempty"`
	TrackingSQL          string `json:"trackingSql,omitempty"`
	OtherMsg             string `json:"otherMsg,omitempty"`

	// Progress is the raw per-partition offset JSON as StarRocks reports it.
	Progress string `json:"progress,omitempty"`
	// LatestSourcePosition is the raw per-partition log-end-offset JSON, present
	// on newer StarRocks versions only.
	LatestSourcePosition string `json:"latestSourcePosition,omitempty"`
	// OffsetLag is the summed positive delta between LatestSourcePosition and
	// Progress across partitions. Nil when either side is missing or
	// non-numeric (e.g. OFFSET_END markers). It is an approximation.
	OffsetLag *int64 `json:"offsetLag,omitempty"`

	Statistics *RoutineLoadStatistics `json:"statistics,omitempty"`
}

// Unhealthy reports whether the job needs operator attention.
func (j RoutineLoadJob) Unhealthy() bool {
	return j.State == RoutineLoadStatePaused || j.State == RoutineLoadStateCancelled
}

// RoutineLoadSummary is the header strip of the routine load dashboard.
type RoutineLoadSummary struct {
	Total        int `json:"total"`
	Running      int `json:"running"`
	NeedSchedule int `json:"needSchedule"`
	Paused       int `json:"paused"`
	Stopped      int `json:"stopped"`
	Cancelled    int `json:"cancelled"`
	// Unhealthy counts jobs in a state worth alerting on (paused + cancelled).
	Unhealthy int `json:"unhealthy"`
	// TotalErrorRows sums error rows across all jobs.
	TotalErrorRows int64 `json:"totalErrorRows"`
}

// RoutineLoadSnapshot is the payload of GET /api/v1/loads/routine.
type RoutineLoadSnapshot struct {
	CollectedAt time.Time `json:"collectedAt"`
	// Source records which read path produced the data:
	// "information_schema" or "show_routine_load".
	Source string `json:"source"`
	// Warnings lists per-database failures from the SHOW fallback sweep.
	Warnings []string           `json:"warnings,omitempty"`
	Summary  RoutineLoadSummary `json:"summary"`
	Jobs     []RoutineLoadJob   `json:"jobs"`
}
