package repository

import (
	"context"
	"fmt"
)

// Catalog and tablet metadata statements.
//
// Tablet detail comes from information_schema.be_tablets, which backends
// report: it carries the on-disk truth (rowsets, segments, real sizes) that FE
// metadata alone cannot answer. Every tablet query is scoped by TABLE_ID —
// scanning be_tablets whole is expensive on a large cluster.
const (
	statisticProcStmt = "SHOW PROC '/statistic'"

	tablesConfigStmt = `SELECT TABLE_SCHEMA, TABLE_NAME, TABLE_ID, TABLE_ENGINE, TABLE_MODEL,
		PARTITION_KEY, DISTRIBUTE_KEY, DISTRIBUTE_TYPE, DISTRIBUTE_BUCKET
		FROM information_schema.tables_config WHERE TABLE_SCHEMA = ?`

	tableSizesStmt = `SELECT TABLE_NAME, TABLE_ROWS, DATA_LENGTH
		FROM information_schema.tables WHERE TABLE_SCHEMA = ?`

	partitionsMetaStmt = `SELECT PARTITION_NAME, ROW_COUNT, DATA_SIZE, BUCKETS, REPLICATION_NUM,
		VISIBLE_VERSION, STORAGE_MEDIUM, MAX_CS, AVG_CS, TABLET_BALANCED
		FROM information_schema.partitions_meta WHERE DB_NAME = ? AND TABLE_NAME = ?`

	beTabletsStmt = `SELECT BE_ID, TABLET_ID, PARTITION_ID, NUM_ROW, DATA_SIZE,
		NUM_ROWSET, NUM_SEGMENT, NUM_VERSION, STATE
		FROM information_schema.be_tablets WHERE TABLE_ID = ?`
)

// StorageRepository reads catalog, partition and tablet metadata.
type StorageRepository struct {
	db *DB
}

// NewStorageRepository wires the repository to a pool.
func NewStorageRepository(db *DB) *StorageRepository {
	return &StorageRepository{db: db}
}

// Statistic returns one row per database from SHOW PROC '/statistic', including
// the tablet-health counters.
func (r *StorageRepository) Statistic(ctx context.Context) ([]Row, error) {
	return r.db.QueryRows(ctx, statisticProcStmt)
}

// Tables returns the catalog rows for one database.
func (r *StorageRepository) Tables(ctx context.Context, database string) ([]Row, error) {
	return r.db.QueryRows(ctx, tablesConfigStmt, database)
}

// TableSizes returns row counts and byte sizes for one database. Kept separate
// from Tables so a version that lacks one view still serves the other.
func (r *StorageRepository) TableSizes(ctx context.Context, database string) ([]Row, error) {
	return r.db.QueryRows(ctx, tableSizesStmt, database)
}

// Partitions returns partition metadata, including compaction scores and the
// per-partition balance flag.
func (r *StorageRepository) Partitions(ctx context.Context, database, table string) ([]Row, error) {
	return r.db.QueryRows(ctx, partitionsMetaStmt, database, table)
}

// Tablets returns backend-reported tablet detail for one table id.
func (r *StorageRepository) Tablets(ctx context.Context, tableID string) ([]Row, error) {
	if tableID == "" {
		return nil, fmt.Errorf("repository: tablet lookup needs a table id")
	}
	return r.db.QueryRows(ctx, beTabletsStmt, tableID)
}
