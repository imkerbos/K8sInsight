CREATE TABLE IF NOT EXISTS clusters (
    id              UUID PRIMARY KEY,
    name            VARCHAR(255) NOT NULL UNIQUE,
    kubeconfig_data TEXT NOT NULL,
    status          VARCHAR(32) NOT NULL DEFAULT 'inactive',
    status_message  TEXT,
    watch_scope     VARCHAR(32) DEFAULT 'cluster',
    watch_namespaces TEXT,
    label_selector  VARCHAR(512),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- incidents 表增加 cluster_id 列
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS cluster_id VARCHAR(36);
CREATE INDEX IF NOT EXISTS idx_incidents_cluster_id ON incidents(cluster_id);
