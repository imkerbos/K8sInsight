CREATE TABLE IF NOT EXISTS incidents (
    id          UUID PRIMARY KEY,
    dedup_key   VARCHAR(512) NOT NULL,
    state       VARCHAR(32)  NOT NULL DEFAULT 'Detecting',
    first_seen  TIMESTAMPTZ  NOT NULL,
    last_seen   TIMESTAMPTZ  NOT NULL,
    count       INTEGER      NOT NULL DEFAULT 1,
    namespace   VARCHAR(253) NOT NULL,
    owner_kind  VARCHAR(64)  NOT NULL DEFAULT '',
    owner_name  VARCHAR(253) NOT NULL DEFAULT '',
    anomaly_type VARCHAR(64) NOT NULL,
    message     TEXT         NOT NULL DEFAULT '',
    pod_names   JSONB        NOT NULL DEFAULT '[]',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- 按 dedup_key 查找活跃事件（聚合时高频使用）
CREATE INDEX idx_incidents_dedup_active ON incidents (dedup_key) WHERE state IN ('Detecting', 'Active');

-- 列表查询常用过滤
CREATE INDEX idx_incidents_namespace ON incidents (namespace);
CREATE INDEX idx_incidents_state ON incidents (state);
CREATE INDEX idx_incidents_anomaly_type ON incidents (anomaly_type);
CREATE INDEX idx_incidents_last_seen ON incidents (last_seen DESC);

-- 按 Owner 聚合查询
CREATE INDEX idx_incidents_owner ON incidents (namespace, owner_kind, owner_name);
