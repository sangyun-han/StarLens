package repository

import "context"

// Cluster metadata statements. These are FE-only administrative commands; they
// read the in-memory catalog and are cheap enough to poll on an interval.
const (
	showFrontendsStmt    = "SHOW FRONTENDS"
	showBackendsStmt     = "SHOW BACKENDS"
	showComputeNodesStmt = "SHOW COMPUTE NODES"
	// runModeStmt reads the immutable FE config that records how the cluster
	// was deployed: shared_data (storage/compute separation) or shared_nothing.
	runModeStmt = "ADMIN SHOW FRONTEND CONFIG LIKE 'run_mode'"
)

// ClusterRepository reads cluster membership from a StarRocks frontend.
type ClusterRepository struct {
	db *DB
}

// NewClusterRepository wires the repository to a pool.
func NewClusterRepository(db *DB) *ClusterRepository {
	return &ClusterRepository{db: db}
}

// Frontends returns one row per FE node as reported by SHOW FRONTENDS.
func (r *ClusterRepository) Frontends(ctx context.Context) ([]Row, error) {
	return r.db.QueryRows(ctx, showFrontendsStmt)
}

// Backends returns one row per BE node as reported by SHOW BACKENDS.
func (r *ClusterRepository) Backends(ctx context.Context) ([]Row, error) {
	return r.db.QueryRows(ctx, showBackendsStmt)
}

// ComputeNodes returns one row per CN node as reported by SHOW COMPUTE NODES.
// Errors on StarRocks versions predating compute nodes; callers treat that as
// "no CNs" rather than a failure.
func (r *ClusterRepository) ComputeNodes(ctx context.Context) ([]Row, error) {
	return r.db.QueryRows(ctx, showComputeNodesStmt)
}

// RunModeConfig returns the FE config rows matching run_mode. An empty result
// means the config item does not exist (pre-3.0 releases, which are always
// shared-nothing); an error usually means the user lacks ADMIN privilege.
func (r *ClusterRepository) RunModeConfig(ctx context.Context) ([]Row, error) {
	return r.db.QueryRows(ctx, runModeStmt)
}
