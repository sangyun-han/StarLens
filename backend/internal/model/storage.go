package model

import "time"

// DatabaseStatistic is one row of SHOW PROC '/statistic': the catalog counts
// plus the tablet-health counters StarRocks maintains per database.
type DatabaseStatistic struct {
	Database     string `json:"database"`
	TableNum     int64  `json:"tableNum"`
	PartitionNum int64  `json:"partitionNum"`
	TabletNum    int64  `json:"tabletNum"`
	ReplicaNum   int64  `json:"replicaNum"`
	// UnhealthyTabletNum counts tablets missing a healthy replica set. A
	// non-zero value is normal briefly during rebalancing and alarming when it
	// persists.
	UnhealthyTabletNum int64 `json:"unhealthyTabletNum"`
	// InconsistentTabletNum counts replicas that disagree on their contents.
	InconsistentTabletNum int64 `json:"inconsistentTabletNum"`
	CloningTabletNum      int64 `json:"cloningTabletNum"`
	ErrorStateTabletNum   int64 `json:"errorStateTabletNum"`
}

// NeedsAttention reports whether any tablet-health counter is non-zero.
func (s DatabaseStatistic) NeedsAttention() bool {
	return s.UnhealthyTabletNum > 0 || s.InconsistentTabletNum > 0 ||
		s.ErrorStateTabletNum > 0
}

// StorageStatistic is the payload of GET /api/v1/storage/statistic.
type StorageStatistic struct {
	CollectedAt time.Time           `json:"collectedAt"`
	Databases   []DatabaseStatistic `json:"databases"`
	Totals      DatabaseStatistic   `json:"totals"`
}

// TableSummary is one table in the storage browser.
type TableSummary struct {
	Database string `json:"database"`
	Name     string `json:"name"`
	// ID is the numeric catalog id, needed to resolve tablets.
	ID string `json:"id,omitempty"`
	// Model is the StarRocks table model: DUP_KEYS, PRIMARY_KEYS, AGG_KEYS, ...
	Model            string `json:"model,omitempty"`
	Engine           string `json:"engine,omitempty"`
	PartitionKey     string `json:"partitionKey,omitempty"`
	DistributeKey    string `json:"distributeKey,omitempty"`
	DistributeType   string `json:"distributeType,omitempty"`
	DistributeBucket *int64 `json:"distributeBucket,omitempty"`
	Rows             *int64 `json:"rows,omitempty"`
	DataBytes        *int64 `json:"dataBytes,omitempty"`
}

// TableList is the payload of GET /api/v1/storage/tables.
type TableList struct {
	CollectedAt time.Time      `json:"collectedAt"`
	Database    string         `json:"database"`
	Tables      []TableSummary `json:"tables"`
}

// Partition is one partition of a table, from information_schema.partitions_meta.
type Partition struct {
	Name           string `json:"name"`
	Rows           *int64 `json:"rows,omitempty"`
	DataBytes      *int64 `json:"dataBytes,omitempty"`
	Buckets        *int64 `json:"buckets,omitempty"`
	ReplicationNum *int64 `json:"replicationNum,omitempty"`
	VisibleVersion *int64 `json:"visibleVersion,omitempty"`
	StorageMedium  string `json:"storageMedium,omitempty"`
	// MaxCompactionScore is the worst compaction score across the partition's
	// tablets. A high score means reads are paying for un-merged rowsets.
	MaxCompactionScore *float64 `json:"maxCompactionScore,omitempty"`
	AvgCompactionScore *float64 `json:"avgCompactionScore,omitempty"`
	// Balanced mirrors TABLET_BALANCED: false means the partition's tablets
	// are unevenly spread across backends.
	Balanced *bool `json:"balanced,omitempty"`
}

// BackendTabletLoad is one backend's share of a table's tablets — the unit
// data skew is measured in.
type BackendTabletLoad struct {
	BackendID  string `json:"backendId"`
	TabletNum  int64  `json:"tabletNum"`
	Rows       int64  `json:"rows"`
	DataBytes  int64  `json:"dataBytes"`
	RowsetNum  int64  `json:"rowsetNum"`
	SegmentNum int64  `json:"segmentNum"`
}

// SkewMeasure is one max/median comparison. The median is the baseline
// because the mean rises with the outlier it is supposed to expose.
type SkewMeasure struct {
	// RowsRatio is max/median rows. 1.0 is perfectly even; nil when the median
	// is zero (an empty table is not skewed, it is empty).
	RowsRatio *float64 `json:"rowsRatio,omitempty"`
	// BytesRatio is the same measure over on-disk size.
	BytesRatio *float64 `json:"bytesRatio,omitempty"`
	Skewed     bool     `json:"skewed"`
}

// TabletSkew reports data skew at the two levels that fail differently.
type TabletSkew struct {
	// AcrossBackends is load imbalance between backends — the kind rebalancing
	// fixes. Undefined on a single-backend cluster, where nothing can be uneven.
	AcrossBackends SkewMeasure `json:"acrossBackends"`
	// AcrossTablets is how unevenly the distribution key hashes into buckets.
	// This is the root cause and shows up even on one backend: no amount of
	// rebalancing fixes it, only a better distribution key or bucket count.
	AcrossTablets SkewMeasure `json:"acrossTablets"`
	// Skewed is true when either level is over the threshold.
	Skewed bool `json:"skewed"`
}

// TableDetail is the payload of GET /api/v1/storage/tables/:database/:table.
type TableDetail struct {
	CollectedAt time.Time    `json:"collectedAt"`
	Table       TableSummary `json:"table"`
	Partitions  []Partition  `json:"partitions"`

	// TabletTotal counts tablet replicas reported by backends.
	TabletTotal  int64 `json:"tabletTotal"`
	RowsetTotal  int64 `json:"rowsetTotal"`
	SegmentTotal int64 `json:"segmentTotal"`
	// MaxRowsetsPerTablet is the compaction-pressure signal: many rowsets on a
	// single tablet means reads merge more files than they should.
	MaxRowsetsPerTablet  int64 `json:"maxRowsetsPerTablet"`
	MaxSegmentsPerTablet int64 `json:"maxSegmentsPerTablet"`
	// AbnormalTablets counts tablet replicas not in the RUNNING/NORMAL state.
	AbnormalTablets int64 `json:"abnormalTablets"`

	Backends []BackendTabletLoad `json:"backends"`
	Skew     TabletSkew          `json:"skew"`
	// Warnings carries partial-read notes, e.g. tablet detail unavailable
	// because no backend reported it.
	Warnings []string `json:"warnings,omitempty"`
}
