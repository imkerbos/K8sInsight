CREATE TABLE IF NOT EXISTS evidences (
    id           UUID PRIMARY KEY,
    incident_id  UUID         NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    type         VARCHAR(64)  NOT NULL,
    content      TEXT         NOT NULL DEFAULT '',
    error        TEXT         NOT NULL DEFAULT '',
    collected_at TIMESTAMPTZ  NOT NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- 按事件 ID 查询关联证据
CREATE INDEX idx_evidences_incident ON evidences (incident_id);

-- 按采集时间排序
CREATE INDEX idx_evidences_collected_at ON evidences (incident_id, collected_at ASC);
