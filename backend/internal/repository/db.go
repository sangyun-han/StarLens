// Package repository owns all StarRocks access. StarRocks speaks the MySQL
// wire protocol on the FE query port (9030), so a standard database/sql pool
// backed by go-sql-driver/mysql is all that is needed.
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	// Registers the "mysql" driver used to talk to StarRocks FE nodes.
	_ "github.com/go-sql-driver/mysql"

	"github.com/sangyun-han/StarLens/backend/config"
)

// DB is a StarRocks connection pool plus the query deadline policy applied to
// every statement issued through it.
type DB struct {
	sql *sql.DB

	addr         string
	queryTimeout time.Duration
}

// NewDB builds the pool. It does not connect: database/sql dials lazily, which
// keeps the dashboard bootable while StarRocks is still starting up. Call Ping
// to check reachability.
func NewDB(cfg config.StarRocksConfig) (*DB, error) {
	handle, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("repository: open StarRocks pool: %w", err)
	}

	handle.SetMaxOpenConns(cfg.MaxOpenConns)
	handle.SetMaxIdleConns(cfg.MaxIdleConns)
	handle.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	// FEs drop idle connections on their own; recycling earlier avoids handing
	// a dead connection to a request.
	handle.SetConnMaxIdleTime(cfg.ConnMaxLifetime / 2)

	return &DB{sql: handle, addr: cfg.Addr, queryTimeout: cfg.QueryTimeout}, nil
}

// Addr is the host:port of the frontend this pool dials.
func (db *DB) Addr() string { return db.addr }

// SQL exposes the raw pool for callers that need transactions or driver-level
// features not covered by the helpers here.
func (db *DB) SQL() *sql.DB { return db.sql }

// Ping verifies a live connection can be established.
func (db *DB) Ping(ctx context.Context) error {
	ctx, cancel := db.withTimeout(ctx)
	defer cancel()

	if err := db.sql.PingContext(ctx); err != nil {
		return fmt.Errorf("repository: ping StarRocks at %s: %w", db.addr, err)
	}
	return nil
}

// Close releases pooled connections.
func (db *DB) Close() error { return db.sql.Close() }

func (db *DB) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if db.queryTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, db.queryTimeout)
}

// Row is a single result row keyed by lower-cased column name. StarRocks renames
// and reorders SHOW columns between versions, so reading by name — with
// fallbacks — is considerably more durable than positional scanning.
type Row map[string]string

// Str returns the first non-empty value among the given column names.
func (r Row) Str(names ...string) string {
	for _, name := range names {
		if v, ok := r[strings.ToLower(name)]; ok && v != "" {
			return v
		}
	}
	return ""
}

// Bool parses a StarRocks boolean column ("true", "1", "yes").
func (r Row) Bool(names ...string) bool {
	switch strings.ToLower(r.Str(names...)) {
	case "true", "1", "yes", "y":
		return true
	default:
		return false
	}
}

// Int64 returns a parsed integer column, or nil when absent or unparseable.
func (r Row) Int64(names ...string) *int64 {
	raw := r.Str(names...)
	if raw == "" {
		return nil
	}
	v, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return nil
	}
	return &v
}

// Float64 returns a parsed float column. Percentage columns such as UsedPct
// arrive as "12.34 %", so a trailing unit is tolerated.
func (r Row) Float64(names ...string) *float64 {
	raw := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(r.Str(names...)), "%"))
	if raw == "" {
		return nil
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return nil
	}
	return &v
}

// Bytes parses a human-readable capacity column ("1.500 GB", "0.000 B") into
// bytes. StarRocks formats these for humans, not for machines.
func (r Row) Bytes(names ...string) *int64 {
	raw := strings.TrimSpace(r.Str(names...))
	if raw == "" {
		return nil
	}

	fields := strings.Fields(raw)
	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return nil
	}

	unit := "B"
	if len(fields) > 1 {
		unit = strings.ToUpper(fields[1])
	}
	multiplier, ok := byteUnits[unit]
	if !ok {
		return nil
	}

	out := int64(value * multiplier)
	return &out
}

var byteUnits = map[string]float64{
	"B":  1,
	"KB": 1 << 10,
	"MB": 1 << 20,
	"GB": 1 << 30,
	"TB": 1 << 40,
	"PB": 1 << 50,
}

// QueryRows runs a query and materializes every row into a Row map. It is built
// for the dynamically shaped results of SHOW statements and ad-hoc worksheet
// queries where the column set is unknown until execution.
func (db *DB) QueryRows(ctx context.Context, query string, args ...any) ([]Row, error) {
	ctx, cancel := db.withTimeout(ctx)
	defer cancel()

	rows, err := db.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("repository: query %q: %w", query, err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("repository: read columns for %q: %w", query, err)
	}

	// Every column is scanned as raw text: SHOW statements return VARCHAR and
	// this sidesteps StarRocks' zero-value timestamps ("0000-00-00 00:00:00").
	out := make([]Row, 0, 8)
	for rows.Next() {
		cells := make([]sql.NullString, len(columns))
		targets := make([]any, len(columns))
		for i := range cells {
			targets[i] = &cells[i]
		}

		if err := rows.Scan(targets...); err != nil {
			return nil, fmt.Errorf("repository: scan row for %q: %w", query, err)
		}

		row := make(Row, len(columns))
		for i, column := range columns {
			if cells[i].Valid {
				row[strings.ToLower(column)] = strings.TrimSpace(cells[i].String)
			} else {
				row[strings.ToLower(column)] = ""
			}
		}
		out = append(out, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repository: iterate rows for %q: %w", query, err)
	}
	return out, nil
}
