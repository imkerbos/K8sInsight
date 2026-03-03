DROP INDEX IF EXISTS idx_incidents_cluster_id;
ALTER TABLE incidents DROP COLUMN IF EXISTS cluster_id;
DROP TABLE IF EXISTS clusters;
