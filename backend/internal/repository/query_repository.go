package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
)

// QueryResultSet is the raw outcome of an ad-hoc statement.
type QueryResultSet struct {
	Columns []QueryResultColumn
	// Rows hold cells as *string-like values: string for data, nil for NULL.
	Rows [][]any
	// Truncated reports that reading stopped at the row cap with rows left.
	Truncated bool
}

// QueryResultColumn is one column of an ad-hoc result.
type QueryResultColumn struct {
	Name string
	Type string
}

// QueryRepository executes operator-authored SQL from the worksheet.
type QueryRepository struct {
	db *DB
	// defaultDatabase is the DSN's database, restored after a scoped execution
	// so pooled connections never leak a USE from one request into the next.
	defaultDatabase string
}

// NewQueryRepository wires the repository to a pool.
func NewQueryRepository(db *DB, defaultDatabase string) *QueryRepository {
	return &QueryRepository{db: db, defaultDatabase: defaultDatabase}
}

// Databases lists every database visible to the connection.
func (r *QueryRepository) Databases(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryRows(ctx, showDatabasesStmt)
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if name := row.Str("database"); name != "" {
			out = append(out, name)
		}
	}
	return out, nil
}

// Execute runs one statement, optionally scoped to a database, reading at most
// maxRows rows. The caller owns the context deadline.
//
// A dedicated connection is used because USE mutates session state. The
// original database is restored before the connection returns to the pool; if
// that restore fails the connection is discarded rather than returned dirty.
func (r *QueryRepository) Execute(ctx context.Context, statement, database string, maxRows int) (QueryResultSet, error) {
	conn, err := r.db.SQL().Conn(ctx)
	if err != nil {
		return QueryResultSet{}, fmt.Errorf("repository: acquire connection: %w", err)
	}
	defer conn.Close()

	scoped := database != "" && database != r.defaultDatabase
	if scoped {
		if _, err := conn.ExecContext(ctx, "USE "+quoteIdentifier(database)); err != nil {
			return QueryResultSet{}, err
		}
		defer r.restoreDatabase(conn)
	}

	rows, err := conn.QueryContext(ctx, statement)
	if err != nil {
		return QueryResultSet{}, err
	}
	defer rows.Close()

	return collectResultSet(rows, maxRows)
}

// restoreDatabase re-selects the DSN's database. Context.Background keeps the
// cleanup working even when the request context is already cancelled.
func (r *QueryRepository) restoreDatabase(conn *sql.Conn) {
	if r.defaultDatabase == "" {
		poison(conn)
		return
	}
	if _, err := conn.ExecContext(context.Background(), "USE "+quoteIdentifier(r.defaultDatabase)); err != nil {
		poison(conn)
	}
}

// poison marks the connection bad so the pool discards it instead of handing a
// connection with unknown session state to the next request.
func poison(conn *sql.Conn) {
	_ = conn.Raw(func(any) error { return driver.ErrBadConn })
}

func collectResultSet(rows *sql.Rows, maxRows int) (QueryResultSet, error) {
	columnNames, err := rows.Columns()
	if err != nil {
		return QueryResultSet{}, fmt.Errorf("repository: read columns: %w", err)
	}

	result := QueryResultSet{Columns: make([]QueryResultColumn, len(columnNames))}
	for i, name := range columnNames {
		result.Columns[i] = QueryResultColumn{Name: name}
	}
	if types, err := rows.ColumnTypes(); err == nil {
		for i, columnType := range types {
			result.Columns[i].Type = columnType.DatabaseTypeName()
		}
	}

	// Everything scans as text: the worksheet displays values, and strings
	// survive BIGINTs beyond JS number precision and StarRocks' zero-dates.
	result.Rows = make([][]any, 0, min(maxRows, 64))
	for len(result.Rows) < maxRows && rows.Next() {
		cells := make([]sql.NullString, len(columnNames))
		targets := make([]any, len(columnNames))
		for i := range cells {
			targets[i] = &cells[i]
		}
		if err := rows.Scan(targets...); err != nil {
			return QueryResultSet{}, fmt.Errorf("repository: scan row: %w", err)
		}

		row := make([]any, len(columnNames))
		for i, cell := range cells {
			if cell.Valid {
				row[i] = cell.String
			} else {
				row[i] = nil
			}
		}
		result.Rows = append(result.Rows, row)
	}

	// One extra Next distinguishes "exactly maxRows" from "more left".
	if len(result.Rows) == maxRows && rows.Next() {
		result.Truncated = true
	}
	if err := rows.Err(); err != nil {
		return QueryResultSet{}, err
	}
	return result, nil
}
