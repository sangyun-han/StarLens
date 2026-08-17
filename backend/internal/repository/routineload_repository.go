package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Read paths for routine load jobs, in preference order.
//
// StarRocks 3.1+ exposes information_schema.routine_load_jobs, which covers
// every database in one query. Older versions only support SHOW ROUTINE LOAD,
// which is scoped to a single database, so the fallback sweeps SHOW DATABASES
// and queries each user database individually.
const (
	infoSchemaRoutineLoadStmt = "SELECT * FROM information_schema.routine_load_jobs"
	showDatabasesStmt         = "SHOW DATABASES"
	showRoutineLoadTmpl       = "SHOW ALL ROUTINE LOAD FROM %s"
)

// Source labels recorded on the snapshot so operators know which path served it.
const (
	RoutineLoadSourceInfoSchema = "information_schema"
	RoutineLoadSourceShow       = "show_routine_load"
)

// systemDatabases are skipped by the fallback sweep: they never own routine
// load jobs.
var systemDatabases = map[string]bool{
	"information_schema": true,
	"sys":                true,
	"_statistics_":       true,
}

// RoutineLoadRows is the raw result of a routine load read.
type RoutineLoadRows struct {
	Rows []Row
	// Source is one of the RoutineLoadSource* constants.
	Source string
	// Warnings lists databases the fallback sweep could not read.
	Warnings []string
}

// RoutineLoadRepository reads streaming ingestion job state from StarRocks.
type RoutineLoadRepository struct {
	db *DB
}

// NewRoutineLoadRepository wires the repository to a pool.
func NewRoutineLoadRepository(db *DB) *RoutineLoadRepository {
	return &RoutineLoadRepository{db: db}
}

// Jobs returns one row per routine load job across all databases.
func (r *RoutineLoadRepository) Jobs(ctx context.Context) (RoutineLoadRows, error) {
	rows, infoSchemaErr := r.db.QueryRows(ctx, infoSchemaRoutineLoadStmt)
	if infoSchemaErr == nil {
		return RoutineLoadRows{Rows: rows, Source: RoutineLoadSourceInfoSchema}, nil
	}
	// Connection-level failures will not get better by switching statements.
	if ctx.Err() != nil {
		return RoutineLoadRows{}, infoSchemaErr
	}

	result, showErr := r.jobsViaShow(ctx)
	if showErr != nil {
		return RoutineLoadRows{}, errors.Join(
			fmt.Errorf("routine_load_jobs query failed: %w", infoSchemaErr),
			fmt.Errorf("SHOW ROUTINE LOAD fallback failed: %w", showErr),
		)
	}
	return result, nil
}

// jobsViaShow sweeps every user database with SHOW ALL ROUTINE LOAD. A database
// that fails (dropped mid-sweep, permission denied) becomes a warning rather
// than failing the whole snapshot; the sweep only errors when nothing succeeds.
func (r *RoutineLoadRepository) jobsViaShow(ctx context.Context) (RoutineLoadRows, error) {
	dbRows, err := r.db.QueryRows(ctx, showDatabasesStmt)
	if err != nil {
		return RoutineLoadRows{}, err
	}

	result := RoutineLoadRows{Source: RoutineLoadSourceShow}
	var lastErr error
	attempted, failed := 0, 0

	for _, dbRow := range dbRows {
		name := dbRow.Str("database")
		if name == "" || systemDatabases[strings.ToLower(name)] {
			continue
		}
		attempted++

		stmt := fmt.Sprintf(showRoutineLoadTmpl, quoteIdentifier(name))
		jobRows, err := r.db.QueryRows(ctx, stmt)
		if err != nil {
			failed++
			lastErr = err
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("database %q could not be read: %v", name, err))
			continue
		}
		result.Rows = append(result.Rows, jobRows...)
	}

	if attempted > 0 && failed == attempted {
		return RoutineLoadRows{}, fmt.Errorf("all %d databases failed, last error: %w", attempted, lastErr)
	}
	return result, nil
}

// quoteIdentifier wraps a StarRocks identifier in backticks, escaping embedded
// backticks. Identifiers cannot be bound as placeholders, so this is the only
// way to interpolate them safely.
func quoteIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}
