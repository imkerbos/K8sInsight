-- incidents 列表排序与游标分页索引
CREATE INDEX IF NOT EXISTS idx_incidents_last_seen_id ON incidents (last_seen DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_incidents_state_last_seen_id ON incidents (state, last_seen DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_incidents_type_last_seen_id ON incidents (anomaly_type, last_seen DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_incidents_namespace_last_seen_id ON incidents (namespace, last_seen DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_incidents_cluster_last_seen_id ON incidents (cluster_id, last_seen DESC, id DESC);

-- owner_name 模糊检索优化（ILIKE '%keyword%'）
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX IF NOT EXISTS idx_incidents_owner_name_trgm ON incidents USING gin (owner_name gin_trgm_ops);
