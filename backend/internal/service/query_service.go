package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/go-sql-driver/mysql"

	"github.com/sangyun-han/StarLens/backend/internal/model"
	"github.com/sangyun-han/StarLens/backend/internal/repository"
)

// Query execution failures the API layer maps to distinct status codes.
var (
	// ErrQueryEmpty: the request carried no executable statement.
	ErrQueryEmpty = errors.New("query is empty")
	// ErrQueryBlocked: the statement is not allowed by the read-only policy.
	ErrQueryBlocked = errors.New("statement blocked by read-only policy")
	// ErrQueryFailed: StarRocks rejected the statement (syntax error, unknown
	// table, ...). The wrapped message is the user's answer.
	ErrQueryFailed = errors.New("query failed")
	// ErrQueryTimeout: execution exceeded the configured time budget.
	ErrQueryTimeout = errors.New("query timed out")
)

// Statement kinds permitted in read-only mode, matched against the first
// keyword after comments are stripped.
var readOnlyKeywords = map[string]bool{
	"select":   true,
	"with":     true,
	"show":     true,
	"desc":     true,
	"describe": true,
	"explain":  true,
}

// QueryPolicy bounds worksheet executions; see config.QueryConfig.
type QueryPolicy struct {
	ReadOnly bool
	MaxRows  int
	Timeout  time.Duration
}

// queryRunner is the repository slice this service depends on.
type queryRunner interface {
	Execute(ctx context.Context, statement, database string, maxRows int) (repository.QueryResultSet, error)
	Databases(ctx context.Context) ([]string, error)
}

// QueryService validates and executes worksheet SQL.
type QueryService struct {
	repo   queryRunner
	policy QueryPolicy
}

// NewQueryService wires the service to a repository and execution policy.
func NewQueryService(repo queryRunner, policy QueryPolicy) *QueryService {
	if policy.MaxRows <= 0 {
		policy.MaxRows = 1000
	}
	return &QueryService{repo: repo, policy: policy}
}

// Databases lists selectable databases for the worksheet's scope picker.
func (s *QueryService) Databases(ctx context.Context) ([]string, error) {
	names, err := s.repo.Databases(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: SHOW DATABASES failed: %w", ErrUnavailable, err)
	}
	return names, nil
}

// Execute runs one validated statement and shapes the result for the UI.
func (s *QueryService) Execute(ctx context.Context, req model.QueryRequest) (model.QueryResult, error) {
	statement := cleanStatement(req.SQL)
	if statement == "" {
		return model.QueryResult{}, ErrQueryEmpty
	}

	if s.policy.ReadOnly {
		if keyword := firstKeyword(statement); !readOnlyKeywords[keyword] {
			return model.QueryResult{}, fmt.Errorf(
				"%w: %q statements are disabled; set QUERY_READ_ONLY=false to allow writes", ErrQueryBlocked, keyword)
		}
	}

	maxRows := req.MaxRows
	if maxRows <= 0 || maxRows > s.policy.MaxRows {
		maxRows = s.policy.MaxRows
	}

	execCtx := ctx
	if s.policy.Timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, s.policy.Timeout)
		defer cancel()
	}

	started := time.Now()
	resultSet, err := s.repo.Execute(execCtx, statement, req.Database, maxRows)
	elapsed := time.Since(started)
	if err != nil {
		return model.QueryResult{}, classifyExecError(err, execCtx)
	}

	columns := make([]model.QueryColumn, len(resultSet.Columns))
	for i, column := range resultSet.Columns {
		columns[i] = model.QueryColumn{Name: column.Name, Type: column.Type}
	}
	rows := resultSet.Rows
	if rows == nil {
		rows = [][]any{}
	}

	return model.QueryResult{
		Columns:   columns,
		Rows:      rows,
		RowCount:  len(rows),
		Truncated: resultSet.Truncated,
		MaxRows:   maxRows,
		ElapsedMs: elapsed.Milliseconds(),
		Statement: statement,
		Database:  req.Database,
	}, nil
}

// classifyExecError separates "your SQL is wrong" from "the cluster is down":
// the former is the operator's answer, the latter is an outage.
func classifyExecError(err error, ctx context.Context) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("%w: execution exceeded the QUERY_TIMEOUT budget", ErrQueryTimeout)
	}

	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return fmt.Errorf("%w: %s", ErrQueryFailed, mysqlErr.Message)
	}

	return fmt.Errorf("%w: executing statement failed: %w", ErrUnavailable, err)
}

// cleanStatement trims whitespace and trailing semicolons. Interior semicolons
// are left alone: multi-statements are rejected by the driver (multiStatements
// stays disabled), and a semicolon may legitimately live inside a literal.
func cleanStatement(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), "; \t\r\n")
}

// firstKeyword returns the first SQL word, lower-cased, skipping line comments
// (-- ...) and block comments (/* ... */).
func firstKeyword(statement string) string {
	rest := statement
	for {
		rest = strings.TrimLeftFunc(rest, unicode.IsSpace)
		switch {
		case strings.HasPrefix(rest, "--"):
			if idx := strings.IndexByte(rest, '\n'); idx >= 0 {
				rest = rest[idx+1:]
				continue
			}
			return ""
		case strings.HasPrefix(rest, "/*"):
			if idx := strings.Index(rest, "*/"); idx >= 0 {
				rest = rest[idx+2:]
				continue
			}
			return ""
		}
		break
	}

	end := strings.IndexFunc(rest, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	})
	if end < 0 {
		end = len(rest)
	}
	return strings.ToLower(rest[:end])
}
