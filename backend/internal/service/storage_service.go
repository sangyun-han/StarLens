package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sangyun-han/StarLens/backend/internal/model"
	"github.com/sangyun-han/StarLens/backend/internal/repository"
)

// skewThreshold is the max/median ratio at which a table is reported as
// skewed. Hash distribution is never perfectly even, so a modest spread is
// normal; 1.5x means one backend carries half again the median's data and the
// queries touching it will show it.
const skewThreshold = 1.5

// tabletStateOK lists the tablet states that need no attention. StarRocks
// spells the healthy state differently across the FE and BE views.
var tabletStateOK = map[string]bool{"RUNNING": true, "NORMAL": true, "": true}

// storageReader is the repository slice this service depends on.
type storageReader interface {
	Statistic(ctx context.Context) ([]repository.Row, error)
	Tables(ctx context.Context, database string) ([]repository.Row, error)
	TableSizes(ctx context.Context, database string) ([]repository.Row, error)
	Partitions(ctx context.Context, database, table string) ([]repository.Row, error)
	Tablets(ctx context.Context, tableID string) ([]repository.Row, error)
}

// StorageService exposes the catalog down to the tablet level: table and
// partition metadata from the frontend, and rowset/segment detail plus data
// skew from what the backends actually hold on disk.
type StorageService struct {
	repo storageReader
	now  func() time.Time
}

// NewStorageService wires the service to a repository.
func NewStorageService(repo storageReader) *StorageService {
	return &StorageService{repo: repo, now: time.Now}
}

// Statistic reports catalog counts and tablet health per database.
func (s *StorageService) Statistic(ctx context.Context) (model.StorageStatistic, error) {
	rows, err := s.repo.Statistic(ctx)
	if err != nil {
		return model.StorageStatistic{}, fmt.Errorf("%w: SHOW PROC '/statistic' failed: %w", ErrUnavailable, err)
	}

	out := model.StorageStatistic{
		CollectedAt: s.now().UTC(),
		Databases:   make([]model.DatabaseStatistic, 0, len(rows)),
	}
	for _, row := range rows {
		// SHOW PROC '/statistic' appends a summary row whose id column reads
		// "Total" and whose name column holds the database count. Counting it as
		// a database would both invent one and double every total.
		if strings.EqualFold(row.Str("dbid", "db_id"), "total") {
			continue
		}
		name := row.Str("dbname", "db_name", "database")
		if name == "" {
			continue
		}

		stat := model.DatabaseStatistic{
			Database:              name,
			TableNum:              intOrZero(row, "tablenum", "table_num"),
			PartitionNum:          intOrZero(row, "partitionnum", "partition_num"),
			TabletNum:             intOrZero(row, "tabletnum", "tablet_num"),
			ReplicaNum:            intOrZero(row, "replicanum", "replica_num"),
			UnhealthyTabletNum:    intOrZero(row, "unhealthytabletnum", "unhealthy_tablet_num"),
			InconsistentTabletNum: intOrZero(row, "inconsistenttabletnum", "inconsistent_tablet_num"),
			CloningTabletNum:      intOrZero(row, "cloningtabletnum", "cloning_tablet_num"),
			ErrorStateTabletNum:   intOrZero(row, "errorstatetabletnum", "error_state_tablet_num"),
		}
		out.Databases = append(out.Databases, stat)

		out.Totals.TableNum += stat.TableNum
		out.Totals.PartitionNum += stat.PartitionNum
		out.Totals.TabletNum += stat.TabletNum
		out.Totals.ReplicaNum += stat.ReplicaNum
		out.Totals.UnhealthyTabletNum += stat.UnhealthyTabletNum
		out.Totals.InconsistentTabletNum += stat.InconsistentTabletNum
		out.Totals.CloningTabletNum += stat.CloningTabletNum
		out.Totals.ErrorStateTabletNum += stat.ErrorStateTabletNum
	}

	// Databases needing attention float to the top, then the largest.
	sort.SliceStable(out.Databases, func(i, j int) bool {
		if a, b := out.Databases[i].NeedsAttention(), out.Databases[j].NeedsAttention(); a != b {
			return a
		}
		if out.Databases[i].TabletNum != out.Databases[j].TabletNum {
			return out.Databases[i].TabletNum > out.Databases[j].TabletNum
		}
		return out.Databases[i].Database < out.Databases[j].Database
	})
	return out, nil
}

// Tables lists one database's tables with size and distribution metadata.
func (s *StorageService) Tables(ctx context.Context, database string) (model.TableList, error) {
	if strings.TrimSpace(database) == "" {
		return model.TableList{}, fmt.Errorf("%w: database is required", ErrInvalidArgument)
	}

	rows, err := s.repo.Tables(ctx, database)
	if err != nil {
		return model.TableList{}, fmt.Errorf("%w: reading tables_config failed: %w", ErrUnavailable, err)
	}

	// Sizes live in a different view; a version that cannot serve them still
	// gets a usable table list.
	sizes := map[string]repository.Row{}
	if sizeRows, err := s.repo.TableSizes(ctx, database); err == nil {
		for _, row := range sizeRows {
			sizes[row.Str("table_name", "tablename")] = row
		}
	}

	list := model.TableList{
		CollectedAt: s.now().UTC(),
		Database:    database,
		Tables:      make([]model.TableSummary, 0, len(rows)),
	}
	for _, row := range rows {
		name := row.Str("table_name", "tablename")
		table := model.TableSummary{
			Database:         database,
			Name:             name,
			ID:               row.Str("table_id", "tableid"),
			Model:            row.Str("table_model", "tablemodel"),
			Engine:           row.Str("table_engine", "tableengine"),
			PartitionKey:     row.Str("partition_key", "partitionkey"),
			DistributeKey:    row.Str("distribute_key", "distributekey"),
			DistributeType:   row.Str("distribute_type", "distributetype"),
			DistributeBucket: row.Int64("distribute_bucket", "distributebucket"),
		}
		if size, ok := sizes[name]; ok {
			table.Rows = size.Int64("table_rows", "tablerows")
			table.DataBytes = size.Int64("data_length", "datalength")
		}
		list.Tables = append(list.Tables, table)
	}

	sort.SliceStable(list.Tables, func(i, j int) bool { return list.Tables[i].Name < list.Tables[j].Name })
	return list, nil
}

// TableDetail returns partitions plus backend-level tablet distribution for
// one table, including the rowset/segment counts that expose compaction
// pressure and the skew ratio that exposes uneven hashing.
func (s *StorageService) TableDetail(ctx context.Context, database, table string) (model.TableDetail, error) {
	if strings.TrimSpace(database) == "" || strings.TrimSpace(table) == "" {
		return model.TableDetail{}, fmt.Errorf("%w: database and table are required", ErrInvalidArgument)
	}

	list, err := s.Tables(ctx, database)
	if err != nil {
		return model.TableDetail{}, err
	}

	var summary *model.TableSummary
	for i := range list.Tables {
		if strings.EqualFold(list.Tables[i].Name, table) {
			summary = &list.Tables[i]
			break
		}
	}
	if summary == nil {
		return model.TableDetail{}, fmt.Errorf("%w: table %s.%s does not exist", ErrNotFound, database, table)
	}

	detail := model.TableDetail{
		CollectedAt: s.now().UTC(),
		Table:       *summary,
		Partitions:  []model.Partition{},
		Backends:    []model.BackendTabletLoad{},
	}

	if partitionRows, err := s.repo.Partitions(ctx, database, summary.Name); err != nil {
		detail.Warnings = append(detail.Warnings, "partition metadata unavailable: "+err.Error())
	} else {
		for _, row := range partitionRows {
			detail.Partitions = append(detail.Partitions, mapPartition(row))
		}
	}

	// Tablet detail is backend-reported: if no backend holds the table (or the
	// version lacks the view) the rest of the payload is still useful.
	tabletRows, err := s.repo.Tablets(ctx, summary.ID)
	if err != nil {
		detail.Warnings = append(detail.Warnings, "tablet detail unavailable: "+err.Error())
		return detail, nil
	}
	applyTabletLoads(&detail, tabletRows)
	return detail, nil
}

func mapPartition(row repository.Row) model.Partition {
	return model.Partition{
		Name:               row.Str("partition_name", "partitionname"),
		Rows:               row.Int64("row_count", "rowcount"),
		DataBytes:          row.Bytes("data_size", "datasize"),
		Buckets:            row.Int64("buckets"),
		ReplicationNum:     row.Int64("replication_num", "replicationnum"),
		VisibleVersion:     row.Int64("visible_version", "visibleversion"),
		StorageMedium:      row.Str("storage_medium", "storagemedium"),
		MaxCompactionScore: row.Float64("max_cs", "maxcs"),
		AvgCompactionScore: row.Float64("avg_cs", "avgcs"),
		Balanced:           boolPtr(row, "tablet_balanced", "tabletbalanced"),
	}
}

// applyTabletLoads folds per-tablet rows into per-backend totals and the
// aggregate compaction/skew signals.
func applyTabletLoads(detail *model.TableDetail, rows []repository.Row) {
	loads := map[string]*model.BackendTabletLoad{}
	// Per-tablet values are kept alongside the per-backend sums: bucket skew is
	// invisible once tablets are folded into backend totals, and it is the
	// level that explains why a backend became hot in the first place.
	tabletRows := make([]int64, 0, len(rows))
	tabletBytes := make([]int64, 0, len(rows))

	for _, row := range rows {
		backendID := row.Str("be_id", "beid")
		load, ok := loads[backendID]
		if !ok {
			load = &model.BackendTabletLoad{BackendID: backendID}
			loads[backendID] = load
		}

		load.TabletNum++
		detail.TabletTotal++

		rowCount := int64(0)
		if v := row.Int64("num_row", "numrow"); v != nil {
			rowCount = *v
			load.Rows += rowCount
		}
		// be_tablets reports DATA_SIZE as a plain byte count.
		byteCount := int64(0)
		if v := row.Int64("data_size", "datasize"); v != nil {
			byteCount = *v
			load.DataBytes += byteCount
		}
		tabletRows = append(tabletRows, rowCount)
		tabletBytes = append(tabletBytes, byteCount)
		if v := row.Int64("num_rowset", "numrowset"); v != nil {
			load.RowsetNum += *v
			detail.RowsetTotal += *v
			detail.MaxRowsetsPerTablet = max(detail.MaxRowsetsPerTablet, *v)
		}
		if v := row.Int64("num_segment", "numsegment"); v != nil {
			load.SegmentNum += *v
			detail.SegmentTotal += *v
			detail.MaxSegmentsPerTablet = max(detail.MaxSegmentsPerTablet, *v)
		}
		if state := strings.ToUpper(row.Str("state")); !tabletStateOK[state] {
			detail.AbnormalTablets++
		}
	}

	detail.Backends = make([]model.BackendTabletLoad, 0, len(loads))
	for _, load := range loads {
		detail.Backends = append(detail.Backends, *load)
	}
	sort.SliceStable(detail.Backends, func(i, j int) bool {
		return detail.Backends[i].BackendID < detail.Backends[j].BackendID
	})

	detail.Skew = computeSkew(detail.Backends, tabletRows, tabletBytes)
}

// computeSkew measures imbalance at both levels that matter: between backends
// (fixable by rebalancing) and between tablets (fixable only by changing the
// distribution key or bucket count).
func computeSkew(loads []model.BackendTabletLoad, tabletRows, tabletBytes []int64) model.TabletSkew {
	skew := model.TabletSkew{
		AcrossTablets: measureSkew(tabletRows, tabletBytes),
	}

	// Skew between backends is undefined with a single backend — there is
	// nothing for it to be uneven with.
	if len(loads) >= 2 {
		rows := make([]int64, 0, len(loads))
		bytes := make([]int64, 0, len(loads))
		for _, load := range loads {
			rows = append(rows, load.Rows)
			bytes = append(bytes, load.DataBytes)
		}
		skew.AcrossBackends = measureSkew(rows, bytes)
	}

	skew.Skewed = skew.AcrossBackends.Skewed || skew.AcrossTablets.Skewed
	return skew
}

// measureSkew compares the worst member against the median one.
func measureSkew(rows, bytes []int64) model.SkewMeasure {
	if len(rows) < 2 {
		return model.SkewMeasure{}
	}

	measure := model.SkewMeasure{
		RowsRatio:  ratioToMedian(rows),
		BytesRatio: ratioToMedian(bytes),
	}
	measure.Skewed = (measure.RowsRatio != nil && *measure.RowsRatio > skewThreshold) ||
		(measure.BytesRatio != nil && *measure.BytesRatio > skewThreshold)
	return measure
}

// ratioToMedian returns max/median, or nil when the median is zero (an empty
// table divides by nothing and is not skewed, it is just empty).
func ratioToMedian(values []int64) *float64 {
	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	median := sorted[len(sorted)/2]
	if len(sorted)%2 == 0 {
		median = (sorted[len(sorted)/2-1] + sorted[len(sorted)/2]) / 2
	}
	if median <= 0 {
		return nil
	}

	ratio := float64(sorted[len(sorted)-1]) / float64(median)
	return &ratio
}

func intOrZero(row repository.Row, names ...string) int64 {
	if v := row.Int64(names...); v != nil {
		return *v
	}
	return 0
}
