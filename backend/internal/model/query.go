package model

// QueryRequest is the payload of POST /api/v1/query.
type QueryRequest struct {
	// SQL is a single statement; a trailing semicolon is tolerated.
	SQL string `json:"sql" binding:"required"`
	// Database scopes the statement (USE <db>) when set.
	Database string `json:"database"`
	// MaxRows caps the result set. Zero means the server default; values above
	// the server maximum are clamped, never honored.
	MaxRows int `json:"maxRows"`
}

// QueryColumn describes one result column.
type QueryColumn struct {
	Name string `json:"name"`
	// Type is the driver-reported database type name, e.g. VARCHAR, BIGINT.
	Type string `json:"type,omitempty"`
}

// QueryResult is the payload of a successful POST /api/v1/query.
type QueryResult struct {
	Columns []QueryColumn `json:"columns"`
	// Rows hold cell values as strings, with nil for SQL NULL. Everything is
	// read as text — the worksheet renders, it does not compute.
	Rows     [][]any `json:"rows"`
	RowCount int     `json:"rowCount"`
	// Truncated is true when the statement produced more rows than MaxRows.
	Truncated bool `json:"truncated"`
	// MaxRows echoes the effective row cap applied to this execution.
	MaxRows int `json:"maxRows"`
	// ElapsedMs is wall-clock execution time measured by the API server,
	// including result streaming — an operator-facing approximation, not the
	// engine's internal profile.
	ElapsedMs int64 `json:"elapsedMs"`
	// Statement is the cleaned SQL that was actually executed.
	Statement string `json:"statement"`
	Database  string `json:"database,omitempty"`
}
