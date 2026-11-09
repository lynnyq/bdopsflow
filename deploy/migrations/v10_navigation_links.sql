-- BDopsFlow 增量迁移脚本 v10：导航快捷链接表
-- 日期：2026-11-09
-- 说明：新增导航快捷链接功能，字段与 Grafana Dashboard 风格对齐

PRAGMA foreign_keys = OFF;

CREATE TABLE IF NOT EXISTS bdopsflow_navigation_links (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT    NOT NULL,
    description     TEXT,
    url             TEXT    NOT NULL,
    width           TEXT    NOT NULL DEFAULT 'third',
    display_mode    TEXT    NOT NULL DEFAULT 'normal',
    height_px       INTEGER,
    icon_data       TEXT,
    icon_fetched_at TEXT,
    sort_order      INTEGER NOT NULL DEFAULT 0,
    enabled         INTEGER NOT NULL DEFAULT 1,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_nav_links_enabled_sort
    ON bdopsflow_navigation_links(enabled, sort_order);

PRAGMA foreign_keys = ON;
