CREATE TABLE IF NOT EXISTS timeline_entries (
    id          UUID PRIMARY KEY,
    incident_id UUID         NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    timestamp   TIMESTAMPTZ  NOT NULL,
    type        VARCHAR(64)  NOT NULL,
    summary     TEXT         NOT NULL DEFAULT '',
    detail      TEXT         NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- 按事件 ID + 时间排序（时间线展示）
CREATE INDEX idx_timeline_incident_ts ON timeline_entries (incident_id, timestamp ASC);
