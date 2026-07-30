-- BDopsFlow 迁移脚本
-- 版本：v8
-- 日期：2026-07-30
-- 描述：新增 Grafana 图表嵌入功能（仪表盘 iframe 嵌入多个 Grafana 图表）

PRAGMA foreign_keys = OFF;

-- ============================================================================
-- 创建 Grafana 图表配置表
-- ============================================================================
CREATE TABLE IF NOT EXISTS bdopsflow_grafana_dashboards (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT    NOT NULL,
    description TEXT,
    embed_url   TEXT    NOT NULL,
    width       TEXT    NOT NULL DEFAULT 'full',
    height_px   INTEGER NOT NULL DEFAULT 300,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_grafana_dashboards_enabled_sort
    ON bdopsflow_grafana_dashboards(enabled, sort_order);

PRAGMA foreign_keys = ON;
