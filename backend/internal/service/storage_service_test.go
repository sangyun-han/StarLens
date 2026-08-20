package service

import (
	"context"
	"errors"
	"testing"

	"github.com/sangyun-han/StarLens/backend/internal/repository"
)

type fakeStorageRepo struct {
	statistic  []repository.Row
	tables     []repository.Row
	sizes      []repository.Row
	partitions []repository.Row
	tablets    []repository.Row

	err        error
	sizesErr   error
	tabletsErr error

	gotTableID string
}

func (f *fakeStorageRepo) Statistic(context.Context) ([]repository.Row, error) {
	return f.statistic, f.err
}
func (f *fakeStorageRepo) Tables(context.Context, string) ([]repository.Row, error) {
	return f.tables, f.err
}
func (f *fakeStorageRepo) TableSizes(context.Context, string) ([]repository.Row, error) {
	if f.sizesErr != nil {
		return nil, f.sizesErr
	}
	return f.sizes, nil
}
func (f *fakeStorageRepo) Partitions(context.Context, string, string) ([]repository.Row, error) {
	return f.partitions, nil
}
func (f *fakeStorageRepo) Tablets(_ context.Context, tableID string) ([]repository.Row, error) {
	f.gotTableID = tableID
	if f.tabletsErr != nil {
		return nil, f.tabletsErr
	}
	return f.tablets, nil
}

// Rows mirror SHOW PROC '/statistic', including the trailing summary row whose
// id column reads "Total" and whose name column holds the database count.
func statisticRows() []repository.Row {
	return []repository.Row{
		{"dbid": "10002", "dbname": "_statistics_", "tablenum": "14", "partitionnum": "17",
			"tabletnum": "132", "replicanum": "132", "unhealthytabletnum": "0"},
		{"dbid": "11002", "dbname": "demo", "tablenum": "2", "partitionnum": "2",
			"tabletnum": "3", "replicanum": "3", "unhealthytabletnum": "1", "errorstatetabletnum": "2"},
		{"dbid": "Total", "dbname": "2", "tablenum": "16", "partitionnum": "19",
			"tabletnum": "135", "replicanum": "135", "unhealthytabletnum": "1"},
	}
}

func TestStatisticSkipsSummaryRow(t *testing.T) {
	svc := NewStorageService(&fakeStorageRepo{statistic: statisticRows()})

	got, err := svc.Statistic(context.Background())
	if err != nil {
		t.Fatalf("Statistic() error = %v", err)
	}

	if len(got.Databases) != 2 {
		t.Fatalf("databases = %d, want 2 (the Total row is not a database)", len(got.Databases))
	}
	// Totals are summed from real databases only; counting the summary row
	// would double everything.
	if got.Totals.TableNum != 16 || got.Totals.TabletNum != 135 {
		t.Errorf("totals = %+v, want 16 tables / 135 tablets", got.Totals)
	}
	// The database needing attention sorts first regardless of size.
	if got.Databases[0].Database != "demo" {
		t.Errorf("databases[0] = %q, want demo (unhealthy tablets sort first)", got.Databases[0].Database)
	}
	if !got.Databases[0].NeedsAttention() || got.Databases[1].NeedsAttention() {
		t.Errorf("NeedsAttention flags = %v/%v", got.Databases[0].NeedsAttention(), got.Databases[1].NeedsAttention())
	}
}

func TestStatisticWrapsError(t *testing.T) {
	svc := NewStorageService(&fakeStorageRepo{err: errors.New("connection refused")})
	if _, err := svc.Statistic(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Errorf("error %v does not wrap ErrUnavailable", err)
	}
}

func tableRows() []repository.Row {
	return []repository.Row{
		{"table_schema": "demo", "table_name": "orders", "table_id": "11004",
			"table_model": "DUP_KEYS", "table_engine": "OLAP",
			"distribute_key": "order_id", "distribute_type": "HASH", "distribute_bucket": "2"},
	}
}

func TestTablesMergesSizes(t *testing.T) {
	repo := &fakeStorageRepo{
		tables: tableRows(),
		sizes:  []repository.Row{{"table_name": "orders", "table_rows": "10", "data_length": "3144"}},
	}

	list, err := NewStorageService(repo).Tables(context.Background(), "demo")
	if err != nil {
		t.Fatalf("Tables() error = %v", err)
	}
	if len(list.Tables) != 1 {
		t.Fatalf("tables = %d", len(list.Tables))
	}

	table := list.Tables[0]
	if table.ID != "11004" || table.Model != "DUP_KEYS" {
		t.Errorf("table = %+v", table)
	}
	if table.Rows == nil || *table.Rows != 10 || table.DataBytes == nil || *table.DataBytes != 3144 {
		t.Errorf("sizes not merged: %+v", table)
	}
}

func TestTablesSurviveMissingSizeView(t *testing.T) {
	repo := &fakeStorageRepo{tables: tableRows(), sizesErr: errors.New("unknown table")}

	list, err := NewStorageService(repo).Tables(context.Background(), "demo")
	if err != nil {
		t.Fatalf("a version without the size view must still list tables: %v", err)
	}
	if len(list.Tables) != 1 || list.Tables[0].Rows != nil {
		t.Errorf("tables = %+v", list.Tables)
	}
}

func TestTablesRequiresDatabase(t *testing.T) {
	_, err := NewStorageService(&fakeStorageRepo{}).Tables(context.Background(), "  ")
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("error = %v, want ErrInvalidArgument", err)
	}
}

func TestTableDetailUnknownTableIsNotFound(t *testing.T) {
	repo := &fakeStorageRepo{tables: tableRows()}
	_, err := NewStorageService(repo).TableDetail(context.Background(), "demo", "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

// Four tablets across two backends: one bucket holds 20000 rows while the
// others hold 50, 50 and 0 — the shape a bad distribution key produces.
func skewedTablets() []repository.Row {
	return []repository.Row{
		{"be_id": "10001", "tablet_id": "1", "num_row": "20000", "data_size": "2720", "num_rowset": "4", "num_segment": "1", "state": "RUNNING"},
		{"be_id": "10001", "tablet_id": "2", "num_row": "50", "data_size": "764", "num_rowset": "4", "num_segment": "1", "state": "RUNNING"},
		{"be_id": "10002", "tablet_id": "3", "num_row": "50", "data_size": "764", "num_rowset": "9", "num_segment": "3", "state": "RUNNING"},
		{"be_id": "10002", "tablet_id": "4", "num_row": "0", "data_size": "0", "num_rowset": "4", "num_segment": "0", "state": "CLONE"},
	}
}

func TestTableDetailAggregatesTabletsAndSkew(t *testing.T) {
	repo := &fakeStorageRepo{
		tables:     tableRows(),
		tablets:    skewedTablets(),
		partitions: []repository.Row{{"partition_name": "p1", "row_count": "20100", "data_size": "4.2 KB", "buckets": "4", "max_cs": "12.5", "tablet_balanced": "false"}},
	}

	detail, err := NewStorageService(repo).TableDetail(context.Background(), "demo", "orders")
	if err != nil {
		t.Fatalf("TableDetail() error = %v", err)
	}

	// Tablets are looked up by catalog id, never by name.
	if repo.gotTableID != "11004" {
		t.Errorf("tablet lookup used %q, want the table id", repo.gotTableID)
	}

	if detail.TabletTotal != 4 || detail.RowsetTotal != 21 || detail.SegmentTotal != 5 {
		t.Errorf("totals = %d tablets / %d rowsets / %d segments", detail.TabletTotal, detail.RowsetTotal, detail.SegmentTotal)
	}
	// Compaction pressure is a per-tablet maximum, not an average.
	if detail.MaxRowsetsPerTablet != 9 || detail.MaxSegmentsPerTablet != 3 {
		t.Errorf("max rowsets/segments = %d/%d, want 9/3", detail.MaxRowsetsPerTablet, detail.MaxSegmentsPerTablet)
	}
	// CLONE is not a healthy steady state.
	if detail.AbnormalTablets != 1 {
		t.Errorf("AbnormalTablets = %d, want 1", detail.AbnormalTablets)
	}

	if len(detail.Backends) != 2 {
		t.Fatalf("backends = %d, want 2", len(detail.Backends))
	}
	if detail.Backends[0].Rows != 20050 || detail.Backends[1].Rows != 50 {
		t.Errorf("backend rows = %d/%d", detail.Backends[0].Rows, detail.Backends[1].Rows)
	}

	// Bucket skew: median of [0,50,50,20000] is 50, so 20000/50 = 400.
	tablets := detail.Skew.AcrossTablets
	if tablets.RowsRatio == nil || *tablets.RowsRatio != 400 {
		t.Errorf("AcrossTablets.RowsRatio = %v, want 400", tablets.RowsRatio)
	}
	if !tablets.Skewed || !detail.Skew.Skewed {
		t.Error("a 400x bucket imbalance must be reported as skewed")
	}
	// Backend skew: 20050 vs 50 is also lopsided here.
	if !detail.Skew.AcrossBackends.Skewed {
		t.Error("backend imbalance must be reported too")
	}

	// Partition metadata comes through, including the human-readable size.
	if len(detail.Partitions) != 1 {
		t.Fatalf("partitions = %d", len(detail.Partitions))
	}
	p := detail.Partitions[0]
	if p.DataBytes == nil || *p.DataBytes != 4300 {
		t.Errorf("partition DataBytes = %v, want 4.2 KB parsed", p.DataBytes)
	}
	if p.MaxCompactionScore == nil || *p.MaxCompactionScore != 12.5 {
		t.Errorf("MaxCompactionScore = %v", p.MaxCompactionScore)
	}
	if p.Balanced == nil || *p.Balanced {
		t.Errorf("Balanced = %v, want false", p.Balanced)
	}
}

func TestSkewUndefinedOnSingleBackend(t *testing.T) {
	repo := &fakeStorageRepo{
		tables: tableRows(),
		tablets: []repository.Row{
			{"be_id": "10001", "tablet_id": "1", "num_row": "20000", "data_size": "2720", "state": "RUNNING"},
			{"be_id": "10001", "tablet_id": "2", "num_row": "50", "data_size": "764", "state": "RUNNING"},
		},
	}

	detail, _ := NewStorageService(repo).TableDetail(context.Background(), "demo", "orders")

	// Nothing to compare backends against...
	if detail.Skew.AcrossBackends.RowsRatio != nil || detail.Skew.AcrossBackends.Skewed {
		t.Errorf("single-backend cluster cannot have backend skew: %+v", detail.Skew.AcrossBackends)
	}
	// ...but the bucket imbalance is real and must still surface.
	if !detail.Skew.AcrossTablets.Skewed || !detail.Skew.Skewed {
		t.Error("bucket skew must be reported even on one backend")
	}
}

func TestSkewEmptyTableIsNotSkewed(t *testing.T) {
	repo := &fakeStorageRepo{
		tables: tableRows(),
		tablets: []repository.Row{
			{"be_id": "10001", "tablet_id": "1", "num_row": "0", "data_size": "0", "state": "RUNNING"},
			{"be_id": "10002", "tablet_id": "2", "num_row": "0", "data_size": "0", "state": "RUNNING"},
		},
	}

	detail, _ := NewStorageService(repo).TableDetail(context.Background(), "demo", "orders")
	if detail.Skew.Skewed || detail.Skew.AcrossTablets.RowsRatio != nil {
		t.Errorf("an empty table divides by nothing: %+v", detail.Skew)
	}
}

func TestTableDetailDegradesWhenTabletsUnavailable(t *testing.T) {
	repo := &fakeStorageRepo{tables: tableRows(), tabletsErr: errors.New("unknown table be_tablets")}

	detail, err := NewStorageService(repo).TableDetail(context.Background(), "demo", "orders")
	if err != nil {
		t.Fatalf("missing tablet view must degrade, not fail: %v", err)
	}
	if len(detail.Warnings) == 0 {
		t.Error("the partial read must be reported as a warning")
	}
	if detail.Table.Name != "orders" {
		t.Error("table metadata must still be served")
	}
}
