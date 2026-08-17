package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/sangyun-han/StarLens/backend/internal/model"
	"github.com/sangyun-han/StarLens/backend/internal/repository"
)

type fakeQueryRepo struct {
	result repository.QueryResultSet
	err    error

	// captured arguments of the last Execute call.
	gotStatement string
	gotDatabase  string
	gotMaxRows   int
}

func (f *fakeQueryRepo) Execute(_ context.Context, statement, database string, maxRows int) (repository.QueryResultSet, error) {
	f.gotStatement, f.gotDatabase, f.gotMaxRows = statement, database, maxRows
	return f.result, f.err
}

func (f *fakeQueryRepo) Databases(context.Context) ([]string, error) {
	return []string{"shop", "web"}, f.err
}

func defaultPolicy() QueryPolicy {
	return QueryPolicy{ReadOnly: true, MaxRows: 1000, Timeout: time.Minute}
}

func TestExecuteCleansAndRunsStatement(t *testing.T) {
	repo := &fakeQueryRepo{result: repository.QueryResultSet{
		Columns: []repository.QueryResultColumn{{Name: "id", Type: "BIGINT"}, {Name: "name", Type: "VARCHAR"}},
		Rows:    [][]any{{"1", "alpha"}, {"2", nil}},
	}}
	svc := NewQueryService(repo, defaultPolicy())

	result, err := svc.Execute(context.Background(), model.QueryRequest{
		SQL:      "  SELECT id, name FROM orders ;;  \n",
		Database: "shop",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if repo.gotStatement != "SELECT id, name FROM orders" {
		t.Errorf("executed statement = %q, want trailing semicolons stripped", repo.gotStatement)
	}
	if repo.gotDatabase != "shop" || repo.gotMaxRows != 1000 {
		t.Errorf("database/maxRows = %q/%d", repo.gotDatabase, repo.gotMaxRows)
	}
	if result.RowCount != 2 || len(result.Columns) != 2 {
		t.Errorf("result = %+v", result)
	}
	if result.Columns[0].Type != "BIGINT" {
		t.Errorf("column type = %q", result.Columns[0].Type)
	}
	if result.Rows[1][1] != nil {
		t.Errorf("NULL cell must stay nil, got %v", result.Rows[1][1])
	}
	if result.Statement == "" || result.ElapsedMs < 0 {
		t.Errorf("metadata = %+v", result)
	}
}

func TestExecuteReadOnlyGuard(t *testing.T) {
	allowed := []string{
		"SELECT 1",
		"select * from t",
		"WITH x AS (SELECT 1) SELECT * FROM x",
		"SHOW FRONTENDS",
		"DESC orders",
		"DESCRIBE orders",
		"EXPLAIN SELECT 1",
		"-- leading comment\nSELECT 1",
		"/* block */ SELECT 1",
	}
	for _, stmt := range allowed {
		repo := &fakeQueryRepo{}
		if _, err := NewQueryService(repo, defaultPolicy()).Execute(context.Background(), model.QueryRequest{SQL: stmt}); err != nil {
			t.Errorf("Execute(%q) blocked: %v", stmt, err)
		}
	}

	blocked := []string{
		"INSERT INTO t VALUES (1)",
		"DROP TABLE t",
		"UPDATE t SET a = 1",
		"DELETE FROM t",
		"CREATE TABLE t (a int)",
		"TRUNCATE TABLE t",
		"SET GLOBAL x = 1",
		"/* sneaky */ DROP TABLE t",
		"ALTER SYSTEM DROP BACKEND 'x'",
	}
	for _, stmt := range blocked {
		repo := &fakeQueryRepo{}
		_, err := NewQueryService(repo, defaultPolicy()).Execute(context.Background(), model.QueryRequest{SQL: stmt})
		if !errors.Is(err, ErrQueryBlocked) {
			t.Errorf("Execute(%q) = %v, want ErrQueryBlocked", stmt, err)
		}
		if repo.gotStatement != "" {
			t.Errorf("blocked statement %q must never reach the repository", stmt)
		}
	}
}

func TestExecuteWritesAllowedWhenReadOnlyOff(t *testing.T) {
	repo := &fakeQueryRepo{}
	policy := defaultPolicy()
	policy.ReadOnly = false

	if _, err := NewQueryService(repo, policy).Execute(context.Background(), model.QueryRequest{SQL: "INSERT INTO t VALUES (1)"}); err != nil {
		t.Errorf("Execute() with ReadOnly=false blocked a write: %v", err)
	}
}

func TestExecuteEmptyStatement(t *testing.T) {
	for _, stmt := range []string{"", "   ", ";;", "-- only a comment"} {
		_, err := NewQueryService(&fakeQueryRepo{}, defaultPolicy()).Execute(context.Background(), model.QueryRequest{SQL: stmt})
		if !errors.Is(err, ErrQueryEmpty) && !errors.Is(err, ErrQueryBlocked) {
			t.Errorf("Execute(%q) = %v, want empty/blocked", stmt, err)
		}
	}

	_, err := NewQueryService(&fakeQueryRepo{}, defaultPolicy()).Execute(context.Background(), model.QueryRequest{SQL: "  ; "})
	if !errors.Is(err, ErrQueryEmpty) {
		t.Errorf("bare semicolon = %v, want ErrQueryEmpty", err)
	}
}

func TestExecuteClampsMaxRows(t *testing.T) {
	repo := &fakeQueryRepo{}
	svc := NewQueryService(repo, defaultPolicy())

	// A request above the server cap is clamped, not honored.
	if _, err := svc.Execute(context.Background(), model.QueryRequest{SQL: "SELECT 1", MaxRows: 999_999}); err != nil {
		t.Fatal(err)
	}
	if repo.gotMaxRows != 1000 {
		t.Errorf("maxRows = %d, want clamped to 1000", repo.gotMaxRows)
	}

	// A smaller request is honored.
	if _, err := svc.Execute(context.Background(), model.QueryRequest{SQL: "SELECT 1", MaxRows: 50}); err != nil {
		t.Fatal(err)
	}
	if repo.gotMaxRows != 50 {
		t.Errorf("maxRows = %d, want 50", repo.gotMaxRows)
	}
}

func TestExecuteErrorClassification(t *testing.T) {
	t.Run("mysql error is the operator's answer", func(t *testing.T) {
		repo := &fakeQueryRepo{err: &mysql.MySQLError{Number: 1064, Message: "Getting syntax error at line 1"}}
		_, err := NewQueryService(repo, defaultPolicy()).Execute(context.Background(), model.QueryRequest{SQL: "SELECT bogus("})
		if !errors.Is(err, ErrQueryFailed) {
			t.Fatalf("err = %v, want ErrQueryFailed", err)
		}
		if !strings.Contains(err.Error(), "syntax error") {
			t.Errorf("err %q must carry the MySQL message", err)
		}
	})

	t.Run("connection failure is an outage", func(t *testing.T) {
		repo := &fakeQueryRepo{err: errors.New("dial tcp: connection refused")}
		_, err := NewQueryService(repo, defaultPolicy()).Execute(context.Background(), model.QueryRequest{SQL: "SELECT 1"})
		if !errors.Is(err, ErrUnavailable) {
			t.Errorf("err = %v, want ErrUnavailable", err)
		}
	})

	t.Run("deadline is a timeout", func(t *testing.T) {
		repo := &fakeQueryRepo{err: context.DeadlineExceeded}
		_, err := NewQueryService(repo, defaultPolicy()).Execute(context.Background(), model.QueryRequest{SQL: "SELECT 1"})
		if !errors.Is(err, ErrQueryTimeout) {
			t.Errorf("err = %v, want ErrQueryTimeout", err)
		}
	})
}

func TestFirstKeyword(t *testing.T) {
	cases := map[string]string{
		"SELECT 1":                      "select",
		"  \n\tshow backends":           "show",
		"-- hi\n-- again\nSELECT 1":     "select",
		"/* a */ /* b */ WITH x AS ...": "with",
		"/* unterminated":               "",
		"-- only comment":               "",
		"INSERT/*c*/ INTO t":            "insert",
	}
	for stmt, want := range cases {
		if got := firstKeyword(stmt); got != want {
			t.Errorf("firstKeyword(%q) = %q, want %q", stmt, got, want)
		}
	}
}
